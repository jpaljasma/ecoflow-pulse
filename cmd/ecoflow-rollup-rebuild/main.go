package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/archiveworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/logredact"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/rolluprebuild"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	var (
		fromRaw            string
		toRaw              string
		provider           string
		deviceIDsRaw       string
		providerIDsRaw     string
		rawLogsRaw         string
		archiveBucket      string
		archivePrefix      string
		directArchive      bool
		maxObjects         int
		chunkSize          int
		parallelism        int
		dbDSN              string
		objectProvider     string
		objectEndpoint     string
		objectAccessKey    string
		objectSecretKey    string
		objectRegion       string
		objectSecure       bool
		objectGCSProjectID string
		archiveOutboxDir   string
		allowPendingOutbox bool
	)

	flag.StringVar(&fromRaw, "from", "", "Range start (RFC3339 or unix-ms). Default: now-24h")
	flag.StringVar(&toRaw, "to", "", "Range end (RFC3339 or unix-ms). Default: now")
	flag.StringVar(&provider, "provider", "", "Optional provider filter (for example ecoflow)")
	flag.StringVar(&deviceIDsRaw, "device-ids", "", "Comma-delimited internal device ids to rebuild")
	flag.StringVar(&providerIDsRaw, "provider-device-ids", "", "Comma-delimited provider device ids to rebuild")
	flag.StringVar(&rawLogsRaw, "raw-logs", "", "Comma-delimited raw MQTT log paths or globs to rebuild from instead of archive objects")
	flag.BoolVar(&directArchive, "direct-archive", false, "List raw archive objects directly from object storage instead of trusting manifest rows")
	flag.StringVar(&archiveBucket, "archive-bucket", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_BUCKET", "pulse-telemetry-raw"), "Archive bucket for direct object listing")
	flag.StringVar(&archivePrefix, "archive-prefix", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PREFIX", "raw"), "Archive prefix for direct object listing")
	flag.IntVar(&maxObjects, "max-objects", 0, "Optional object scan cap (0 means no limit)")
	flag.IntVar(&chunkSize, "chunk-size", 500, "Bucket replacement chunk size per transaction")
	flag.IntVar(&parallelism, "parallelism", 4, "Shard worker parallelism for archive object processing")
	flag.StringVar(&dbDSN, "db-dsn", strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN")), "Postgres DSN for manifest + rollup tables")
	objectDefaults := replaycli.DefaultObjectReaderConfig()
	flag.StringVar(&objectProvider, "object-provider", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PROVIDER", string(objectDefaults.Provider)), "Object store provider: minio|gcs")
	flag.StringVar(&objectEndpoint, "object-endpoint", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectDefaults.Endpoint), "Object store endpoint")
	flag.StringVar(&objectAccessKey, "object-access-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectDefaults.AccessKeyID), "Object store access key")
	flag.StringVar(&objectSecretKey, "object-secret-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectDefaults.SecretAccessKey), "Object store secret key")
	flag.StringVar(&objectRegion, "object-region", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", objectDefaults.Region), "Object store region")
	flag.BoolVar(&objectSecure, "object-secure", runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", objectDefaults.Secure), "Object store tls")
	flag.StringVar(&objectGCSProjectID, "object-gcs-project-id", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_GCS_PROJECT_ID", objectDefaults.GCSProjectID), "Optional GCS project id for logging or bucket auto-create")
	flag.StringVar(&archiveOutboxDir, "archive-upload-outbox-dir", strings.TrimSpace(os.Getenv("ARCHIVE_UPLOAD_OUTBOX_DIR")), "Archive upload outbox directory; non-empty pending entries block archive-backed rebuilds")
	flag.BoolVar(&allowPendingOutbox, "allow-pending-archive-outbox", runtimecfg.Bool("ROLLUP_REBUILD_ALLOW_PENDING_ARCHIVE_OUTBOX", false), "Allow archive-backed rebuild while local archive upload outbox entries are pending")
	flag.Parse()

	logCfg := pulselog.DefaultServiceConfig("rollup-rebuild")
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

	fromUnixMS, toUnixMS, err := resolveWindow(fromRaw, toRaw, time.Now().UTC())
	if err != nil {
		log.Error("invalid rebuild window", slog.String("error", err.Error()))
		os.Exit(1)
	}
	contextFromUnixMS, contextToUnixMS := expandRebuildContextWindow(fromUnixMS, toUnixMS)

	writer, err := rolluprebuild.NewPostgresWriter(dbDSN)
	if err != nil {
		log.Error("init postgres writer failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	rawLogInputs := runtimecfg.SplitNonEmpty(rawLogsRaw)
	if err := checkArchiveUploadOutboxGuard(archiveOutboxDir, allowPendingOutbox, len(rawLogInputs) == 0); err != nil {
		log.Error("archive upload outbox guard failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	deviceIDs := runtimecfg.SplitNonEmpty(deviceIDsRaw)
	providerDeviceIDs := runtimecfg.SplitNonEmpty(providerIDsRaw)
	reportFilter := rolluprebuild.ReportFilter{
		Provider:          strings.TrimSpace(provider),
		DeviceIDs:         deviceIDs,
		ProviderDeviceIDs: providerDeviceIDs,
		From:              time.UnixMilli(fromUnixMS).UTC(),
		To:                time.UnixMilli(toUnixMS).UTC(),
	}
	preArchiveFootprint, err := writer.ArchiveFootprint(context.Background(), reportFilter)
	if err != nil {
		log.Warn("archive footprint precheck failed", slog.String("error", err.Error()))
	}
	preMinuteSummary, err := writer.MinuteWindowSummary(context.Background(), reportFilter)
	if err != nil {
		log.Warn("pre-rebuild minute summary failed", slog.String("error", err.Error()))
	}

	var runner *rolluprebuild.Runner
	if len(rawLogInputs) == 0 {
		manifest, err := replaycli.NewPostgresManifestStore(dbDSN)
		if err != nil {
			log.Error("init manifest store failed", slog.String("error", err.Error()))
			os.Exit(1)
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
		runner, err = rolluprebuild.NewRunner(log, manifest, objectReader, writer, chunkSize, parallelism)
		if err != nil {
			log.Error("init rollup rebuild runner failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer func() { _ = runner.Close() }()
	} else {
		defer func() { _ = writer.Close() }()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("rollup rebuild starting",
		slog.Time("from", time.UnixMilli(fromUnixMS).UTC()),
		slog.Time("to", time.UnixMilli(toUnixMS).UTC()),
		slog.String("provider", strings.TrimSpace(provider)),
		slog.Int("device_id_count", len(runtimecfg.SplitNonEmpty(deviceIDsRaw))),
		slog.Int("provider_device_id_count", len(runtimecfg.SplitNonEmpty(providerIDsRaw))),
		slog.Any("raw_logs", rawLogInputs),
		slog.Bool("direct_archive", directArchive),
		slog.String("archive_bucket", strings.TrimSpace(archiveBucket)),
		slog.String("archive_prefix", strings.TrimSpace(archivePrefix)),
		slog.Int("max_objects", maxObjects),
		slog.Int("chunk_size", chunkSize),
		slog.Int("parallelism", parallelism),
	)
	var report rolluprebuild.Report
	if len(rawLogInputs) > 0 {
		report, err = rolluprebuild.RebuildFromRawLogs(
			ctx,
			writer,
			strings.TrimSpace(provider),
			time.UnixMilli(fromUnixMS).UTC(),
			time.UnixMilli(toUnixMS).UTC(),
			rawLogInputs,
			deviceIDs,
			providerDeviceIDs,
			chunkSize,
		)
	} else if directArchive {
		objects, listErr := replaycli.ListObjectsDirect(
			ctx,
			replaycli.ObjectReaderConfig{
				Provider:        replaycli.ObjectProvider(objectProvider),
				Endpoint:        objectEndpoint,
				AccessKeyID:     objectAccessKey,
				SecretAccessKey: objectSecretKey,
				Region:          objectRegion,
				Secure:          objectSecure,
				GCSProjectID:    objectGCSProjectID,
			},
			strings.TrimSpace(archiveBucket),
			strings.TrimSpace(archivePrefix),
			time.UnixMilli(contextFromUnixMS).UTC(),
			time.UnixMilli(contextToUnixMS).UTC(),
			maxObjects,
		)
		if listErr != nil {
			log.Error("list direct archive objects failed", slog.String("error", listErr.Error()))
			os.Exit(1)
		}
		report, err = runner.RebuildObjects(ctx, objects, fromUnixMS, toUnixMS)
	} else if len(deviceIDs) > 0 || len(providerDeviceIDs) > 0 {
		report, err = runner.RebuildDevices(ctx, replaycli.DeviceQuery{
			Provider:           strings.TrimSpace(provider),
			FromUnixMS:         contextFromUnixMS,
			ToUnixMS:           contextToUnixMS,
			DeviceIDs:          deviceIDs,
			ProviderDeviceIDs:  providerDeviceIDs,
			MaxObjectsReturned: maxObjects,
		})
	} else {
		report, err = runner.RebuildFleet(ctx, replaycli.FleetQuery{
			FromUnixMS:         contextFromUnixMS,
			ToUnixMS:           contextToUnixMS,
			MaxObjectsReturned: maxObjects,
		})
	}
	if err != nil {
		log.Error("rollup rebuild failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if len(rawLogInputs) == 0 && report.ObjectsMatched == 0 {
		log.Error("authoritative archive has no objects for requested rebuild window",
			slog.Time("from", time.UnixMilli(fromUnixMS).UTC()),
			slog.Time("to", time.UnixMilli(toUnixMS).UTC()),
			slog.String("provider", strings.TrimSpace(provider)),
			slog.Int("device_id_count", len(deviceIDs)),
			slog.Int("provider_device_id_count", len(providerDeviceIDs)),
			slog.Bool("direct_archive", directArchive),
			slog.String("archive_bucket", strings.TrimSpace(archiveBucket)),
			slog.String("archive_prefix", strings.TrimSpace(archivePrefix)),
		)
		os.Exit(1)
	}
	log.Info("rollup rebuild completed",
		slog.Int("objects_matched", report.ObjectsMatched),
		slog.Int("objects_processed", report.ObjectsProcessed),
		slog.Int("missing_objects", report.MissingObjects),
		slog.Int64("object_bytes", report.ObjectBytes),
		slog.Int("object_records", report.ObjectRecords),
		slog.Int("messages_decoded", report.MessagesDecoded),
		slog.Int("messages_applied", report.MessagesApplied),
		slog.Int("quota_messages", report.QuotaMessages),
		slog.Int("minute_rows", report.MinuteRows),
		slog.Int("hour_rows", report.HourRows),
		slog.Int("day_rows", report.DayRows),
		slog.Duration("duration", report.FinishedAt.Sub(report.StartedAt)),
	)
	log.Info("archive footprint before rebuild",
		slog.Int("objects", preArchiveFootprint.Objects),
		slog.Int64("total_bytes", preArchiveFootprint.TotalBytes),
		slog.Int("total_records", preArchiveFootprint.TotalRecords),
		slog.Int("provider_devices", preArchiveFootprint.ProviderDevices),
		slog.Int64("window_ts_min_unix_ms", preArchiveFootprint.WindowTSMinMS),
		slog.Int64("window_ts_max_unix_ms", preArchiveFootprint.WindowTSMaxMS),
	)
	postArchiveFootprint, err := writer.ArchiveFootprint(context.Background(), reportFilter)
	if err != nil {
		log.Warn("post-rebuild archive footprint failed", slog.String("error", err.Error()))
	} else {
		log.Info("archive footprint after rebuild",
			slog.Int("objects", postArchiveFootprint.Objects),
			slog.Int64("total_bytes", postArchiveFootprint.TotalBytes),
			slog.Int("total_records", postArchiveFootprint.TotalRecords),
			slog.Int("provider_devices", postArchiveFootprint.ProviderDevices),
			slog.Int64("window_ts_min_unix_ms", postArchiveFootprint.WindowTSMinMS),
			slog.Int64("window_ts_max_unix_ms", postArchiveFootprint.WindowTSMaxMS),
		)
	}
	postMinuteSummary, err := writer.MinuteWindowSummary(context.Background(), reportFilter)
	if err != nil {
		log.Warn("post-rebuild minute summary failed", slog.String("error", err.Error()))
	} else {
		for _, line := range diffMinuteSummaries(preMinuteSummary, postMinuteSummary) {
			log.Info("rollup bucket diff",
				slog.String("provider_device_ref", logredact.Identifier(line.ProviderDeviceID)),
				slog.Int("pre_total_buckets", line.PreTotalBuckets),
				slog.Int("post_total_buckets", line.PostTotalBuckets),
				slog.Int("bucket_delta", line.PostTotalBuckets-line.PreTotalBuckets),
				slog.Float64("pre_current_wh", line.PreCurrentWh),
				slog.Float64("post_current_wh", line.PostCurrentWh),
				slog.Float64("current_wh_delta", line.PostCurrentWh-line.PreCurrentWh),
				slog.String("pre_latest_bucket_utc", line.PreLatestBucketUTC),
				slog.String("post_latest_bucket_utc", line.PostLatestBucketUTC),
			)
		}
	}
}

func resolveWindow(fromRaw string, toRaw string, now time.Time) (int64, int64, error) {
	var (
		from time.Time
		to   time.Time
		err  error
	)
	if strings.TrimSpace(fromRaw) == "" {
		from = now.Add(-48 * time.Hour)
	} else {
		from, err = parseTimeInput(fromRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("parse from: %w", err)
		}
	}
	if strings.TrimSpace(toRaw) == "" {
		to = now
	} else {
		to, err = parseTimeInput(toRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("parse to: %w", err)
		}
	}
	if !from.Before(to) {
		return 0, 0, fmt.Errorf("from must be before to")
	}
	return from.UnixMilli(), to.UnixMilli(), nil
}

func parseTimeInput(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("time value is required")
	}
	if isDigits(raw) {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return time.UnixMilli(value).UTC(), nil
		}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", raw)
}

func expandRebuildContextWindow(fromUnixMS, toUnixMS int64) (int64, int64) {
	gapMillis := rollupworker.DefaultSolarCarryForwardMaxGap.Milliseconds()
	if gapMillis <= 0 {
		return fromUnixMS, toUnixMS
	}
	return fromUnixMS - gapMillis, toUnixMS + gapMillis
}

func checkArchiveUploadOutboxGuard(dir string, allowPending bool, archiveBacked bool) error {
	if !archiveBacked || allowPending || strings.TrimSpace(dir) == "" {
		return nil
	}
	pending, err := archiveworker.CountFileArchiveUploadOutboxPending(dir)
	if err != nil {
		return err
	}
	if pending > 0 {
		return fmt.Errorf("pending archive upload outbox entries=%d in %s; wait for GCS upload or set -allow-pending-archive-outbox for a manual recovery override", pending, dir)
	}
	return nil
}

func isDigits(raw string) bool {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return raw != ""
}

type bucketDiffLine struct {
	ProviderDeviceID    string
	PreTotalBuckets     int
	PostTotalBuckets    int
	PreCurrentWh        float64
	PostCurrentWh       float64
	PreLatestBucketUTC  string
	PostLatestBucketUTC string
}

func diffMinuteSummaries(
	pre map[string]rolluprebuild.BucketWindowSummary,
	post map[string]rolluprebuild.BucketWindowSummary,
) []bucketDiffLine {
	keys := map[string]struct{}{}
	for key := range pre {
		keys[key] = struct{}{}
	}
	for key := range post {
		keys[key] = struct{}{}
	}
	out := make([]bucketDiffLine, 0, len(keys))
	for key := range keys {
		preRow := pre[key]
		postRow := post[key]
		out = append(out, bucketDiffLine{
			ProviderDeviceID:    key,
			PreTotalBuckets:     preRow.TotalBuckets,
			PostTotalBuckets:    postRow.TotalBuckets,
			PreCurrentWh:        preRow.CurrentWh,
			PostCurrentWh:       postRow.CurrentWh,
			PreLatestBucketUTC:  preRow.LatestBucketUTC,
			PostLatestBucketUTC: postRow.LatestBucketUTC,
		})
	}
	slices.SortFunc(out, func(a, b bucketDiffLine) int {
		return strings.Compare(a.ProviderDeviceID, b.ProviderDeviceID)
	})
	return out
}
