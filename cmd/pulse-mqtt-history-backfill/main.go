package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/logredact"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/rolluprebuild"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	defaultDBDSN              = "postgres://pulse:pulse-local-dev-password@127.0.0.1:15432/pulse?sslmode=disable"
	defaultEmulatorURL        = "http://127.0.0.1:18080"
	defaultProviderDeviceID   = "PULSEDPUX24K001"
	defaultSampleInterval     = time.Minute
	defaultReplaceChunkSize   = 500
	defaultArchiveWaitTimeout = 45 * time.Second
	defaultArchiveWaitPoll    = 2 * time.Second
)

type config struct {
	dbDSN              string
	provider           string
	providerDeviceID   string
	emulatorURL        string
	sampleInterval     time.Duration
	chunkSize          int
	maxObjects         int
	parallelism        int
	archiveWaitTime    time.Duration
	archivePollTime    time.Duration
	objectEndpoint     string
	objectAccessKey    string
	objectSecretKey    string
	objectRegion       string
	objectSecure       bool
	objectProvider     string
	objectGCSProjectID string
	from               time.Time
	to                 time.Time
}

type replayResponse struct {
	From             string `json:"from"`
	To               string `json:"to"`
	Step             string `json:"step"`
	SamplesPublished int    `json:"samplesPublished"`
	FramesPublished  int    `json:"framesPublished"`
	Clients          int    `json:"clients"`
}

func main() {
	logCfg := pulselog.DefaultServiceConfig("pulse-mqtt-history-backfill")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	cfg, err := parseConfig(os.Args[1:], time.Now())
	if err != nil {
		log.Error("parse pulse mqtt history backfill config failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := run(context.Background(), log, cfg); err != nil {
		log.Error("pulse mqtt history backfill failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func parseConfig(args []string, now time.Time) (config, error) {
	fs := flag.NewFlagSet("pulse-mqtt-history-backfill", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	objectDefaults := replaycli.DefaultObjectReaderConfig()

	var (
		fromRaw string
		toRaw   string
		cfg     config
	)
	fs.StringVar(&cfg.dbDSN, "db-dsn", envOrDefault("CONTROL_PLANE_DB_DSN", defaultDBDSN), "Postgres DSN for the local/dev control-plane + rollup database")
	fs.StringVar(&cfg.provider, "provider", controlplane.ProviderPulseMQTT, "Provider id to rebuild")
	fs.StringVar(&cfg.providerDeviceID, "provider-device-id", defaultProviderDeviceID, "Provider device id / SN to rebuild")
	fs.StringVar(&cfg.emulatorURL, "emulator-url", envOrDefault("PULSE_MQTT_EMULATOR_URL", defaultEmulatorURL), "HTTP base URL for the running pulse-mqtt emulator replay endpoint")
	fs.DurationVar(&cfg.sampleInterval, "sample-interval", defaultSampleInterval, "Historical sample interval to replay through the emulator MQTT broker")
	fs.IntVar(&cfg.chunkSize, "chunk-size", defaultReplaceChunkSize, "Rollup replace chunk size")
	fs.IntVar(&cfg.maxObjects, "max-objects", 0, "Optional archive object scan cap passed to rollup rebuild")
	fs.IntVar(&cfg.parallelism, "parallelism", 4, "Archive rebuild worker parallelism")
	fs.DurationVar(&cfg.archiveWaitTime, "archive-wait-timeout", defaultArchiveWaitTimeout, "How long to wait for replayed MQTT envelopes to land in the raw archive")
	fs.DurationVar(&cfg.archivePollTime, "archive-wait-interval", defaultArchiveWaitPoll, "Archive coverage poll cadence after replay")
	fs.StringVar(&cfg.objectProvider, "object-provider", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PROVIDER", string(objectDefaults.Provider)), "Object store provider: minio|gcs")
	fs.StringVar(&cfg.objectEndpoint, "object-endpoint", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectDefaults.Endpoint), "Object store endpoint for archive-backed rollup rebuild")
	fs.StringVar(&cfg.objectAccessKey, "object-access-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectDefaults.AccessKeyID), "Object store access key for archive-backed rollup rebuild")
	fs.StringVar(&cfg.objectSecretKey, "object-secret-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectDefaults.SecretAccessKey), "Object store secret key for archive-backed rollup rebuild")
	fs.StringVar(&cfg.objectRegion, "object-region", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", objectDefaults.Region), "Object store region for archive-backed rollup rebuild")
	fs.BoolVar(&cfg.objectSecure, "object-secure", runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", objectDefaults.Secure), "Object store tls")
	fs.StringVar(&cfg.objectGCSProjectID, "object-gcs-project-id", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_GCS_PROJECT_ID", objectDefaults.GCSProjectID), "Optional GCS project id for logging or bucket auto-create")
	fs.StringVar(&fromRaw, "from", "", "Window start (RFC3339 or unix-ms). Defaults to local midnight.")
	fs.StringVar(&toRaw, "to", "", "Window end (RFC3339 or unix-ms). Defaults to next local minute boundary.")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	from, to, err := resolveBackfillWindow(fromRaw, toRaw, now)
	if err != nil {
		return config{}, err
	}
	cfg.provider = controlplane.NormalizeProvider(cfg.provider)
	cfg.providerDeviceID = strings.ToUpper(strings.TrimSpace(cfg.providerDeviceID))
	cfg.emulatorURL = strings.TrimRight(strings.TrimSpace(cfg.emulatorURL), "/")
	cfg.from = from
	cfg.to = to
	if strings.TrimSpace(cfg.dbDSN) == "" {
		return config{}, fmt.Errorf("db-dsn is required")
	}
	if cfg.provider == "" {
		return config{}, fmt.Errorf("provider is required")
	}
	if cfg.providerDeviceID == "" {
		return config{}, fmt.Errorf("provider-device-id is required")
	}
	if cfg.emulatorURL == "" {
		return config{}, fmt.Errorf("emulator-url is required")
	}
	if cfg.sampleInterval <= 0 {
		return config{}, fmt.Errorf("sample-interval must be positive")
	}
	if cfg.chunkSize <= 0 {
		return config{}, fmt.Errorf("chunk-size must be positive")
	}
	if cfg.parallelism <= 0 {
		return config{}, fmt.Errorf("parallelism must be positive")
	}
	if cfg.archiveWaitTime <= 0 {
		return config{}, fmt.Errorf("archive-wait-timeout must be positive")
	}
	if cfg.archivePollTime <= 0 {
		return config{}, fmt.Errorf("archive-wait-interval must be positive")
	}
	return cfg, nil
}

func resolveBackfillWindow(fromRaw string, toRaw string, now time.Time) (time.Time, time.Time, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(now.Location())

	var (
		from time.Time
		to   time.Time
		err  error
	)
	if strings.TrimSpace(fromRaw) == "" {
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		from, err = parseTimeInput(fromRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse from: %w", err)
		}
	}
	if strings.TrimSpace(toRaw) == "" {
		to = now.Truncate(time.Minute).Add(time.Minute)
	} else {
		to, err = parseTimeInput(toRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse to: %w", err)
		}
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("from must be before to")
	}
	return from, to, nil
}

func parseTimeInput(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("time value is required")
	}
	if millis, err := timeFromUnixMillis(raw); err == nil {
		return millis, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed, nil
	}
	parsed, err = time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", raw)
}

func timeFromUnixMillis(raw string) (time.Time, error) {
	if !isDigits(raw) {
		return time.Time{}, fmt.Errorf("not unix millis")
	}
	value, err := parseInt64(raw)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(value).UTC(), nil
}

func run(ctx context.Context, log *slog.Logger, cfg config) error {
	writer, err := rolluprebuild.NewPostgresWriter(cfg.dbDSN)
	if err != nil {
		return fmt.Errorf("init rollup rebuild writer: %w", err)
	}
	defer func() { _ = writer.Close() }()

	manifest, err := replaycli.NewPostgresManifestStore(cfg.dbDSN)
	if err != nil {
		return fmt.Errorf("init manifest store: %w", err)
	}
	defer func() { _ = manifest.Close() }()

	contextFromUnixMS, contextToUnixMS := expandRebuildContextWindow(cfg.from.UTC().UnixMilli(), cfg.to.UTC().UnixMilli())
	contextQuery := replaycli.DeviceQuery{
		Provider:           cfg.provider,
		FromUnixMS:         contextFromUnixMS,
		ToUnixMS:           contextToUnixMS,
		ProviderDeviceIDs:  []string{cfg.providerDeviceID},
		MaxObjectsReturned: cfg.maxObjects,
	}
	preObjects, err := manifest.ListByDevices(ctx, contextQuery)
	if err != nil {
		return fmt.Errorf("list archive manifests before replay: %w", err)
	}

	filter := rolluprebuild.ReportFilter{
		Provider:          cfg.provider,
		ProviderDeviceIDs: []string{cfg.providerDeviceID},
		From:              cfg.from.UTC(),
		To:                cfg.to.UTC(),
	}
	preArchive, err := writer.ArchiveFootprint(ctx, filter)
	if err != nil {
		return fmt.Errorf("query archive footprint before replay: %w", err)
	}
	replayResult, err := triggerReplay(ctx, cfg, time.UnixMilli(contextFromUnixMS).UTC(), time.UnixMilli(contextToUnixMS).UTC())
	if err != nil {
		return err
	}
	postObjects, replayObjects, err := waitForReplayObjects(ctx, manifest, contextQuery, preObjects, cfg.archiveWaitTime, cfg.archivePollTime)
	if err != nil {
		return err
	}
	postArchive, err := writer.ArchiveFootprint(ctx, filter)
	if err != nil {
		return fmt.Errorf("query archive footprint after replay: %w", err)
	}

	objectReader, err := replaycli.NewObjectReader(replaycli.ObjectReaderConfig{
		Provider:        replaycli.ObjectProvider(cfg.objectProvider),
		Endpoint:        cfg.objectEndpoint,
		AccessKeyID:     cfg.objectAccessKey,
		SecretAccessKey: cfg.objectSecretKey,
		Region:          cfg.objectRegion,
		Secure:          cfg.objectSecure,
		GCSProjectID:    cfg.objectGCSProjectID,
	})
	if err != nil {
		return fmt.Errorf("init object reader: %w", err)
	}
	runner, err := rolluprebuild.NewRunner(log, manifest, objectReader, writer, cfg.chunkSize, cfg.parallelism)
	if err != nil {
		return fmt.Errorf("init rollup rebuild runner: %w", err)
	}
	defer func() { _ = runner.Close() }()

	report, err := runner.RebuildObjects(ctx, replayObjects, cfg.from.UTC().UnixMilli(), cfg.to.UTC().UnixMilli())
	if err != nil {
		return fmt.Errorf("rebuild archive window after replay: %w", err)
	}
	if report.ObjectsMatched == 0 {
		return fmt.Errorf("authoritative archive has no objects for replayed window")
	}
	log.Info("pulse mqtt history backfill completed",
		slog.String("provider", cfg.provider),
		slog.String("provider_device_ref", logredact.Identifier(cfg.providerDeviceID)),
		slog.Time("from", cfg.from),
		slog.Time("to", cfg.to),
		slog.Int("replay_samples", replayResult.SamplesPublished),
		slog.Int("replay_frames", replayResult.FramesPublished),
		slog.Int("replay_clients", replayResult.Clients),
		slog.Int("manifest_objects_before", len(preObjects)),
		slog.Int("manifest_objects_after", len(postObjects)),
		slog.Int("replay_objects", len(replayObjects)),
		slog.Int("archive_objects_before", preArchive.Objects),
		slog.Int("archive_objects_after", postArchive.Objects),
		slog.Int("archive_records_before", preArchive.TotalRecords),
		slog.Int("archive_records_after", postArchive.TotalRecords),
		slog.Int("objects_matched", report.ObjectsMatched),
		slog.Int("objects_processed", report.ObjectsProcessed),
		slog.Int("messages_decoded", report.MessagesDecoded),
		slog.Int("messages_applied", report.MessagesApplied),
		slog.Int("minute_rows", report.MinuteRows),
		slog.Int("hour_rows", report.HourRows),
		slog.Int("day_rows", report.DayRows),
		slog.Int("pv_port_minute_rows", report.PVPortMinuteRows),
	)
	return nil
}

func triggerReplay(ctx context.Context, cfg config, replayFrom time.Time, replayTo time.Time) (replayResponse, error) {
	replayURL, err := buildReplayURL(cfg, replayFrom, replayTo)
	if err != nil {
		return replayResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, replayURL, nil)
	if err != nil {
		return replayResponse{}, fmt.Errorf("build replay request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return replayResponse{}, fmt.Errorf("post replay request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return replayResponse{}, fmt.Errorf("replay request failed with status %d", resp.StatusCode)
	}
	var out replayResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return replayResponse{}, fmt.Errorf("decode replay response: %w", err)
	}
	if out.Clients <= 0 || out.FramesPublished <= 0 {
		return replayResponse{}, fmt.Errorf("replay produced no mqtt frames; verify the pulsemqtt ingest session is connected")
	}
	return out, nil
}

func buildReplayURL(cfg config, replayFrom time.Time, replayTo time.Time) (string, error) {
	base, err := url.Parse(cfg.emulatorURL)
	if err != nil {
		return "", fmt.Errorf("parse emulator-url: %w", err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/replay"
	query := base.Query()
	query.Set("from", replayFrom.Format(time.RFC3339Nano))
	query.Set("to", replayTo.Format(time.RFC3339Nano))
	query.Set("step", cfg.sampleInterval.String())
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func waitForReplayObjects(
	ctx context.Context,
	manifest replaycli.ManifestStore,
	query replaycli.DeviceQuery,
	pre []replaycli.ManifestObject,
	timeout time.Duration,
	pollInterval time.Duration,
) ([]replaycli.ManifestObject, []replaycli.ManifestObject, error) {
	if timeout <= 0 {
		timeout = defaultArchiveWaitTimeout
	}
	if pollInterval <= 0 {
		pollInterval = defaultArchiveWaitPoll
	}
	deadline := time.Now().Add(timeout)
	for {
		post, err := manifest.ListByDevices(ctx, query)
		if err != nil {
			return nil, nil, fmt.Errorf("list archive manifests after replay: %w", err)
		}
		replayObjects := diffManifestObjects(pre, post)
		if len(replayObjects) > 0 {
			return post, replayObjects, nil
		}
		if time.Now().After(deadline) {
			return post, nil, fmt.Errorf("timed out waiting for replayed mqtt archive objects")
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func diffManifestObjects(pre []replaycli.ManifestObject, post []replaycli.ManifestObject) []replaycli.ManifestObject {
	seen := make(map[string]struct{}, len(pre))
	for _, object := range pre {
		seen[manifestObjectKey(object)] = struct{}{}
	}
	out := make([]replaycli.ManifestObject, 0, len(post))
	for _, object := range post {
		key := manifestObjectKey(object)
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, object)
	}
	return out
}

func manifestObjectKey(object replaycli.ManifestObject) string {
	return strings.TrimSpace(object.ObjectBucket) + "|" + strings.TrimSpace(object.ObjectKey)
}

func expandRebuildContextWindow(fromUnixMS, toUnixMS int64) (int64, int64) {
	gapMillis := rollupworker.DefaultSolarCarryForwardMaxGap.Milliseconds()
	if gapMillis <= 0 {
		return fromUnixMS, toUnixMS
	}
	return fromUnixMS - gapMillis, toUnixMS + gapMillis
}

func isDigits(raw string) bool {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return raw != ""
}

func parseInt64(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
