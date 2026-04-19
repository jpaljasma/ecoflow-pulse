package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

const (
	modeListDevices = "list-devices"
	modeDevice      = "device"
	modeFleet       = "fleet"
)

func main() {
	var (
		mode               string
		provider           string
		fromRaw            string
		toRaw              string
		deviceIDsRaw       string
		providerIDsRaw     string
		shardsRaw          string
		dryRun             bool
		maxObjects         int
		manifestDSN        string
		objectProvider     string
		objectEndpoint     string
		objectAccessKey    string
		objectSecretKey    string
		objectRegion       string
		objectSecure       bool
		objectGCSProjectID string
		natsURLsRaw        string
		natsName           string
		natsTarget         string
		subjectPrefix      string
		subjectShardCount  uint
	)

	flag.StringVar(&mode, "mode", modeListDevices, "Replay mode: list-devices|device|fleet")
	flag.StringVar(&provider, "provider", "", "Optional provider filter (for example ecoflow)")
	flag.StringVar(&fromRaw, "from", "", "Range start (RFC3339 or unix-ms). Default: now-1h")
	flag.StringVar(&toRaw, "to", "", "Range end (RFC3339 or unix-ms). Default: now")
	flag.StringVar(&deviceIDsRaw, "device-ids", "", "Comma-delimited internal device ids for device mode")
	flag.StringVar(&providerIDsRaw, "provider-device-ids", "", "Comma-delimited provider device ids (for example SN) for device mode")
	flag.StringVar(&shardsRaw, "shards", "", "Comma-delimited shard ids for fleet mode")
	flag.BoolVar(&dryRun, "dry-run", false, "Decode/filter without publishing replay messages to NATS")
	flag.IntVar(&maxObjects, "max-objects", 0, "Optional object scan cap (0 means no limit)")

	objectDefaults := replaycli.DefaultObjectReaderConfig()
	flag.StringVar(&manifestDSN, "manifest-dsn", strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN")), "Postgres DSN for archive manifest index")
	flag.StringVar(&objectProvider, "object-provider", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PROVIDER", string(objectDefaults.Provider)), "Object store provider: minio|gcs")
	flag.StringVar(&objectEndpoint, "object-endpoint", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectDefaults.Endpoint), "Object store endpoint")
	flag.StringVar(&objectAccessKey, "object-access-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectDefaults.AccessKeyID), "Object store access key")
	flag.StringVar(&objectSecretKey, "object-secret-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectDefaults.SecretAccessKey), "Object store secret key")
	flag.StringVar(&objectRegion, "object-region", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", objectDefaults.Region), "Object store region")
	flag.BoolVar(&objectSecure, "object-secure", runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", objectDefaults.Secure), "Object store tls")
	flag.StringVar(&objectGCSProjectID, "object-gcs-project-id", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_GCS_PROJECT_ID", objectDefaults.GCSProjectID), "Optional GCS project id for logging or bucket auto-create")

	flag.StringVar(&natsURLsRaw, "nats-urls", runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222"), "Comma-delimited NATS URLs")
	flag.StringVar(&natsName, "nats-name", runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-replay-cli"), "NATS client name")
	flag.StringVar(&natsTarget, "nats-target", runtimecfg.EnvOrDefault("REPLAY_NATS_TARGET", string(replaycli.NATSPublishTargetReplay)), "NATS publish target: replay|ingest")
	flag.StringVar(&subjectPrefix, "subject-prefix", runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix), "Telemetry subject prefix")
	flag.UintVar(&subjectShardCount, "subject-shards", uint(runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount)), "Telemetry subject shard count")
	flag.Parse()

	logCfg := pulselog.DefaultServiceConfig("replay-cli")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	logCfg.AsyncEnabled = !runtimecfg.Bool("LOG_ASYNC_DISABLED", false)
	logCfg.AsyncQueueSize = runtimecfg.IntMin("LOG_ASYNC_QUEUE_SIZE", logCfg.AsyncQueueSize, 128)
	logCfg.AsyncBypassLevel = pulselog.ParseLevel(runtimecfg.EnvOrDefault("LOG_ASYNC_BYPASS_LEVEL", "warn"), slog.LevelWarn)

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

	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	metricsCtx, cancelMetrics := context.WithCancel(context.Background())
	defer cancelMetrics()
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(metricsCtx, log, "replay-cli", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	log.Info("replay cli starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
	)

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != modeListDevices && mode != modeDevice && mode != modeFleet {
		log.Error("invalid mode", slog.String("mode", mode))
		os.Exit(1)
	}
	natsTarget = strings.ToLower(strings.TrimSpace(natsTarget))

	fromUnixMS, toUnixMS, err := resolveWindow(fromRaw, toRaw, time.Now().UTC())
	if err != nil {
		log.Error("invalid replay window", slog.String("error", err.Error()))
		os.Exit(1)
	}

	manifest, err := replaycli.NewPostgresManifestStore(manifestDSN)
	if err != nil {
		log.Error("init manifest store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = manifest.Close() }()

	if mode == modeListDevices {
		if err := runListDevices(context.Background(), log, manifest, fromUnixMS, toUnixMS, maxObjects); err != nil {
			log.Error("list devices failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}

	objectReader, err := replaycli.NewObjectReader(replaycli.ObjectReaderConfig{
		Provider:        replaycli.ObjectProvider(objectProvider),
		Endpoint:        objectEndpoint,
		AccessKeyID:     objectAccessKey,
		SecretAccessKey: objectSecretKey,
		Region:          objectRegion,
		Secure:          objectSecure,
		GCSProjectID:    objectGCSProjectID,
	})
	if err != nil {
		log.Error("init object reader failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	subjectCfg := telemetrybus.SubjectConfig{
		Prefix:     strings.TrimSpace(subjectPrefix),
		ShardCount: uint32(subjectShardCount),
	}.Normalized()

	var publisher replaycli.ReplayPublisher = replaycli.NoopPublisher{}
	if !dryRun {
		natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(natsURLsRaw))
		natsCfg.Name = strings.TrimSpace(natsName)
		natsConn, err := telemetrybus.DialNATS(log, natsCfg)
		if err != nil {
			log.Error("init nats connection failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer natsConn.Close()
		publisher, err = replaycli.NewNATSPublisherWithConfig(natsConn, replaycli.NATSPublisherConfig{
			SubjectConfig: subjectCfg,
			Target:        replaycli.NATSPublishTarget(natsTarget),
		})
		if err != nil {
			log.Error("init replay publisher failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	runner, err := replaycli.NewRunner(log, manifest, objectReader, publisher)
	if err != nil {
		log.Error("init replay runner failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = runner.Close() }()

	request := replaycli.ReplayRequest{
		Provider:          strings.TrimSpace(provider),
		FromUnixMS:        fromUnixMS,
		ToUnixMS:          toUnixMS,
		DeviceIDs:         runtimecfg.SplitNonEmpty(deviceIDsRaw),
		ProviderDeviceIDs: runtimecfg.SplitNonEmpty(providerIDsRaw),
		Shards:            parseShards(shardsRaw),
		MaxObjects:        maxObjects,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Info("replay starting",
		slog.String("mode", mode),
		slog.Bool("dry_run", dryRun),
		slog.String("nats_target", natsTarget),
		slog.Time("from", time.UnixMilli(fromUnixMS).UTC()),
		slog.Time("to", time.UnixMilli(toUnixMS).UTC()),
		slog.Int("max_objects", maxObjects),
	)

	var report replaycli.ReplayReport
	switch mode {
	case modeDevice:
		report, err = runner.ReplayDevices(ctx, request)
	case modeFleet:
		report, err = runner.ReplayFleet(ctx, request)
	default:
		err = errors.New("unsupported mode")
	}
	if err != nil {
		log.Error("replay failed", slog.String("mode", mode), slog.String("error", err.Error()))
		os.Exit(1)
	}
	logReplayReport(log, report)
}

func runListDevices(
	ctx context.Context,
	log *slog.Logger,
	manifest *replaycli.PostgresManifestStore,
	fromUnixMS int64,
	toUnixMS int64,
	maxObjects int,
) error {
	objects, err := manifest.ListByFleetRange(ctx, replaycli.FleetQuery{
		FromUnixMS:         fromUnixMS,
		ToUnixMS:           toUnixMS,
		MaxObjectsReturned: maxObjects,
	})
	if err != nil {
		return err
	}
	deviceSet := make(map[string]struct{})
	providerSet := make(map[string]struct{})
	for _, object := range objects {
		for _, deviceID := range object.DeviceIDs {
			deviceSet[deviceID] = struct{}{}
		}
		for _, providerID := range object.ProviderDeviceIDs {
			providerSet[strings.ToUpper(providerID)] = struct{}{}
		}
	}
	devices := mapKeys(deviceSet)
	providers := mapKeys(providerSet)
	log.Info("manifest device listing",
		slog.Int("objects_matched", len(objects)),
		slog.Int("device_ids", len(devices)),
		slog.Int("provider_device_ids", len(providers)),
		slog.Time("from", time.UnixMilli(fromUnixMS).UTC()),
		slog.Time("to", time.UnixMilli(toUnixMS).UTC()),
	)
	fmt.Println("device_ids:", strings.Join(devices, ","))
	fmt.Println("provider_device_ids:", strings.Join(providers, ","))
	return nil
}

func logReplayReport(log *slog.Logger, report replaycli.ReplayReport) {
	log.Info("replay completed",
		slog.String("mode", report.Mode),
		slog.Int("objects_matched", report.ObjectsMatched),
		slog.Int("objects_processed", report.ObjectsProcessed),
		slog.Int("messages_decoded", report.MessagesDecoded),
		slog.Int("messages_published", report.MessagesPublished),
		slog.Int("messages_filtered", report.MessagesFiltered),
		slog.Int("messages_failed", report.MessagesFailed),
		slog.Int64("first_message_unix_ms", report.FirstMessageUnixMS),
		slog.Int64("last_message_unix_ms", report.LastMessageUnixMS),
		slog.Duration("duration", report.FinishedAt.Sub(report.StartedAt)),
	)
	if len(report.PublishedByShard) == 0 {
		return
	}
	shards := make([]int, 0, len(report.PublishedByShard))
	for shard := range report.PublishedByShard {
		shards = append(shards, int(shard))
	}
	sort.Ints(shards)
	for _, shard := range shards {
		log.Info("replay shard summary", slog.Int("shard", shard), slog.Int("messages", report.PublishedByShard[uint32(shard)]))
	}
}

func resolveWindow(fromRaw string, toRaw string, now time.Time) (int64, int64, error) {
	now = now.UTC()
	var (
		from time.Time
		to   time.Time
		err  error
	)
	if strings.TrimSpace(fromRaw) == "" {
		from = now.Add(-1 * time.Hour)
	} else {
		from, err = parseTimeArg(fromRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("parse from: %w", err)
		}
	}
	if strings.TrimSpace(toRaw) == "" {
		to = now
	} else {
		to, err = parseTimeArg(toRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("parse to: %w", err)
		}
	}
	if from.After(to) {
		return 0, 0, errors.New("from must be <= to")
	}
	return from.UnixMilli(), to.UnixMilli(), nil
}

func parseTimeArg(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("time arg is empty")
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil && ms > 0 {
		return time.UnixMilli(ms).UTC(), nil
	}
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected unix-ms or RFC3339: %w", err)
	}
	return v.UTC(), nil
}

func parseShards(raw string) []uint32 {
	values := runtimecfg.SplitNonEmpty(raw)
	if len(values) == 0 {
		return nil
	}
	shards := make([]uint32, 0, len(values))
	seen := make(map[uint32]struct{}, len(values))
	for _, value := range values {
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			continue
		}
		shard := uint32(n)
		if _, ok := seen[shard]; ok {
			continue
		}
		seen[shard] = struct{}{}
		shards = append(shards, shard)
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i] < shards[j] })
	return shards
}

func mapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
