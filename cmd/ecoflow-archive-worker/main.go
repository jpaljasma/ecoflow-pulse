package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/archiveworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	natsCfg := telemetrybus.DefaultNATSConnConfig(splitNonEmpty(envOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = envOrDefault("NATS_NAME", "ecoflow-archive-worker")
	natsCfg.ConnectTimeout = mustDuration("NATS_CONNECT_TIMEOUT", natsCfg.ConnectTimeout)
	natsCfg.ReconnectWait = mustDuration("NATS_RECONNECT_WAIT", natsCfg.ReconnectWait)
	natsCfg.ReconnectJitter = mustDuration("NATS_RECONNECT_JITTER", natsCfg.ReconnectJitter)
	natsCfg.PingInterval = mustDuration("NATS_PING_INTERVAL", natsCfg.PingInterval)
	natsCfg.MaxPingsOut = mustIntMin("NATS_MAX_PINGS_OUT", natsCfg.MaxPingsOut, 1)
	natsCfg.MaxReconnects = mustIntMin("NATS_MAX_RECONNECTS", natsCfg.MaxReconnects, -1)

	natsConn, err := telemetrybus.DialNATS(log, natsCfg)
	if err != nil {
		log.Error("init nats connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer natsConn.Close()

	storeCfg := archiveworker.DefaultMinIOObjectStoreConfig()
	storeCfg.Endpoint = envOrDefault("ARCHIVE_OBJECT_ENDPOINT", storeCfg.Endpoint)
	storeCfg.AccessKeyID = envOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", storeCfg.AccessKeyID)
	storeCfg.SecretAccessKey = envOrDefault("ARCHIVE_OBJECT_SECRET_KEY", storeCfg.SecretAccessKey)
	storeCfg.Region = envOrDefault("ARCHIVE_OBJECT_REGION", storeCfg.Region)
	storeCfg.Secure = mustBool("ARCHIVE_OBJECT_SECURE", storeCfg.Secure)
	storeCfg.AutoCreateBucket = mustBool("ARCHIVE_OBJECT_AUTO_CREATE_BUCKET", storeCfg.AutoCreateBucket)
	objectStore, err := archiveworker.NewMinIOObjectStore(storeCfg)
	if err != nil {
		log.Error("init archive object store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := archiveworker.DefaultConfig()
	cfg.SubjectConfig = archiveworker.SubjectConfig{
		Prefix:     envOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: mustUint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	cfg.StreamName = envOrDefault("ARCHIVE_INGEST_STREAM_NAME", cfg.StreamName)
	cfg.Durable = envOrDefault("ARCHIVE_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = envOrDefault("ARCHIVE_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = mustDuration("ARCHIVE_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = mustIntMin("ARCHIVE_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = mustDuration("ARCHIVE_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = mustDuration("ARCHIVE_DRAIN_TIMEOUT", cfg.DrainTimeout)
	cfg.FlushInterval = mustDuration("ARCHIVE_FLUSH_INTERVAL", cfg.FlushInterval)
	cfg.FlushTimeout = mustDuration("ARCHIVE_FLUSH_TIMEOUT", cfg.FlushTimeout)
	cfg.MaxRecordsPerPart = mustIntMin("ARCHIVE_MAX_RECORDS_PER_PART", cfg.MaxRecordsPerPart, 1)
	cfg.MaxBytesPerPart = mustIntMin("ARCHIVE_MAX_BYTES_PER_PART", cfg.MaxBytesPerPart, 1024)
	cfg.ObjectBucket = envOrDefault("ARCHIVE_OBJECT_BUCKET", cfg.ObjectBucket)
	cfg.ObjectPrefix = envOrDefault("ARCHIVE_OBJECT_PREFIX", cfg.ObjectPrefix)
	cfg.WriterID = envOrDefault("ARCHIVE_WRITER_ID", cfg.WriterID)
	cfg.ZstdEncoderLevel = mustInt("ARCHIVE_ZSTD_LEVEL", cfg.ZstdEncoderLevel)

	manifestDSN := strings.TrimSpace(os.Getenv("ARCHIVE_MANIFEST_DB_DSN"))
	if manifestDSN == "" {
		manifestDSN = strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	}
	var manifestStore archiveworker.ManifestStore
	if manifestDSN != "" {
		manifestStore, err = archiveworker.NewPostgresManifestStore(manifestDSN)
		if err != nil {
			log.Error("init archive manifest store failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer func() {
			if closeErr := manifestStore.Close(); closeErr != nil {
				log.Warn("close archive manifest store failed", slog.String("error", closeErr.Error()))
			}
		}()
	}

	worker, err := archiveworker.New(log, natsConn, objectStore, cfg, archiveworker.WithManifestStore(manifestStore))
	if err != nil {
		log.Error("init archive worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("archive worker starting",
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", cfg.SubjectConfig.Prefix),
		slog.Uint64("shards", uint64(cfg.SubjectConfig.ShardCount)),
		slog.String("stream", cfg.StreamName),
		slog.String("durable", cfg.Durable),
		slog.String("queue_group", cfg.QueueGroup),
		slog.Duration("ack_wait", cfg.AckWait),
		slog.Int("max_ack_pending", cfg.MaxAckPending),
		slog.Duration("flush_interval", cfg.FlushInterval),
		slog.Int("max_records_per_part", cfg.MaxRecordsPerPart),
		slog.Int("max_bytes_per_part", cfg.MaxBytesPerPart),
		slog.String("object_bucket", cfg.ObjectBucket),
		slog.String("object_prefix", cfg.ObjectPrefix),
		slog.String("writer_id", cfg.WriterID),
		slog.String("object_endpoint", storeCfg.Endpoint),
		slog.Bool("object_secure", storeCfg.Secure),
		slog.Bool("manifest_enabled", manifestStore != nil),
	)
	if err := worker.Run(ctx); err != nil {
		log.Error("archive worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("archive worker stopped")
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitNonEmpty(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func mustDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func mustInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func mustIntMin(key string, fallback int, min int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min {
		return fallback
	}
	return v
}

func mustUint32(key string, fallback uint32) uint32 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return fallback
	}
	return uint32(v)
}

func mustBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}
