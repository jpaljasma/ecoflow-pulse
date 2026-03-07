package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/rolluprebuild"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	var (
		fromRaw         string
		toRaw           string
		provider        string
		deviceIDsRaw    string
		providerIDsRaw  string
		rawLogsRaw      string
		maxObjects      int
		chunkSize       int
		parallelism     int
		dbDSN           string
		objectEndpoint  string
		objectAccessKey string
		objectSecretKey string
		objectRegion    string
		objectSecure    bool
	)

	flag.StringVar(&fromRaw, "from", "", "Range start (RFC3339 or unix-ms). Default: now-24h")
	flag.StringVar(&toRaw, "to", "", "Range end (RFC3339 or unix-ms). Default: now")
	flag.StringVar(&provider, "provider", "", "Optional provider filter (for example ecoflow)")
	flag.StringVar(&deviceIDsRaw, "device-ids", "", "Comma-delimited internal device ids to rebuild")
	flag.StringVar(&providerIDsRaw, "provider-device-ids", "", "Comma-delimited provider device ids to rebuild")
	flag.StringVar(&rawLogsRaw, "raw-logs", "", "Comma-delimited raw MQTT log paths or globs to rebuild from instead of archive objects")
	flag.IntVar(&maxObjects, "max-objects", 0, "Optional object scan cap (0 means no limit)")
	flag.IntVar(&chunkSize, "chunk-size", 500, "Bucket replacement chunk size per transaction")
	flag.IntVar(&parallelism, "parallelism", 4, "Shard worker parallelism for archive object processing")
	flag.StringVar(&dbDSN, "db-dsn", strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN")), "Postgres DSN for manifest + rollup tables")
	flag.StringVar(&objectEndpoint, "object-endpoint", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", replaycli.DefaultMinIOObjectReaderConfig().Endpoint), "Object store endpoint")
	flag.StringVar(&objectAccessKey, "object-access-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", replaycli.DefaultMinIOObjectReaderConfig().AccessKeyID), "Object store access key")
	flag.StringVar(&objectSecretKey, "object-secret-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", replaycli.DefaultMinIOObjectReaderConfig().SecretAccessKey), "Object store secret key")
	flag.StringVar(&objectRegion, "object-region", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", replaycli.DefaultMinIOObjectReaderConfig().Region), "Object store region")
	flag.BoolVar(&objectSecure, "object-secure", runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", replaycli.DefaultMinIOObjectReaderConfig().Secure), "Object store tls")
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

	writer, err := rolluprebuild.NewPostgresWriter(dbDSN)
	if err != nil {
		log.Error("init postgres writer failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	rawLogInputs := runtimecfg.SplitNonEmpty(rawLogsRaw)

	var runner *rolluprebuild.Runner
	if len(rawLogInputs) == 0 {
		manifest, err := replaycli.NewPostgresManifestStore(dbDSN)
		if err != nil {
			log.Error("init manifest store failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		objectReader, err := replaycli.NewMinIOObjectReader(replaycli.MinIOObjectReaderConfig{
			Endpoint:        objectEndpoint,
			AccessKeyID:     objectAccessKey,
			SecretAccessKey: objectSecretKey,
			Region:          objectRegion,
			Secure:          objectSecure,
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
		slog.Any("device_ids", runtimecfg.SplitNonEmpty(deviceIDsRaw)),
		slog.Any("provider_device_ids", runtimecfg.SplitNonEmpty(providerIDsRaw)),
		slog.Any("raw_logs", rawLogInputs),
		slog.Int("max_objects", maxObjects),
		slog.Int("chunk_size", chunkSize),
		slog.Int("parallelism", parallelism),
	)
	deviceIDs := runtimecfg.SplitNonEmpty(deviceIDsRaw)
	providerDeviceIDs := runtimecfg.SplitNonEmpty(providerIDsRaw)
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
	} else if len(deviceIDs) > 0 || len(providerDeviceIDs) > 0 {
		report, err = runner.RebuildDevices(ctx, replaycli.DeviceQuery{
			Provider:           strings.TrimSpace(provider),
			FromUnixMS:         fromUnixMS,
			ToUnixMS:           toUnixMS,
			DeviceIDs:          deviceIDs,
			ProviderDeviceIDs:  providerDeviceIDs,
			MaxObjectsReturned: maxObjects,
		})
	} else {
		report, err = runner.RebuildFleet(ctx, replaycli.FleetQuery{
			FromUnixMS:         fromUnixMS,
			ToUnixMS:           toUnixMS,
			MaxObjectsReturned: maxObjects,
		})
	}
	if err != nil {
		log.Error("rollup rebuild failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("rollup rebuild completed",
		slog.Int("objects_matched", report.ObjectsMatched),
		slog.Int("objects_processed", report.ObjectsProcessed),
		slog.Int("messages_decoded", report.MessagesDecoded),
		slog.Int("messages_applied", report.MessagesApplied),
		slog.Int("minute_rows", report.MinuteRows),
		slog.Int("hour_rows", report.HourRows),
		slog.Int("day_rows", report.DayRows),
		slog.Duration("duration", report.FinishedAt.Sub(report.StartedAt)),
	)
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

func isDigits(raw string) bool {
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return raw != ""
}
