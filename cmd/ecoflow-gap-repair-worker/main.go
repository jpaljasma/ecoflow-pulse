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

	"github.com/jpaljasma/ecoflow-pulse/internal/gaprepair"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	natsCfg := telemetrybus.DefaultNATSConnConfig(splitNonEmpty(envOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = envOrDefault("NATS_NAME", "ecoflow-gap-repair-worker")
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

	subjectCfg := telemetrybus.SubjectConfig{
		Prefix:     envOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: mustUint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}.Normalized()

	streamCfg := telemetrybus.DefaultJetStreamGapRepairBootstrapConfig()
	streamCfg.Enabled = mustBool("GAP_REPAIR_NATS_JS_BOOTSTRAP_ENABLED", true)
	streamCfg.StreamName = envOrDefault("GAP_REPAIR_NATS_JS_STREAM_NAME", streamCfg.StreamName)
	streamCfg.Replicas = mustIntMin("GAP_REPAIR_NATS_JS_REPLICAS", streamCfg.Replicas, 1)
	streamCfg.MaxAge = mustDuration("GAP_REPAIR_NATS_JS_MAX_AGE", streamCfg.MaxAge)
	streamCfg.MaxBytes = mustInt64Min("GAP_REPAIR_NATS_JS_MAX_BYTES", streamCfg.MaxBytes, 0)
	if streamCfg.Enabled {
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := telemetrybus.EnsureJetStreamGapRepairStream(bootstrapCtx, natsConn, subjectCfg, streamCfg); err != nil {
			cancel()
			log.Error("bootstrap gap-repair stream failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		cancel()
	}

	manifestDSN := strings.TrimSpace(os.Getenv("ARCHIVE_MANIFEST_DB_DSN"))
	if manifestDSN == "" {
		manifestDSN = strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	}
	if manifestDSN == "" {
		log.Error("ARCHIVE_MANIFEST_DB_DSN or CONTROL_PLANE_DB_DSN is required")
		os.Exit(1)
	}
	manifestStore, err := replaycli.NewPostgresManifestStore(manifestDSN)
	if err != nil {
		log.Error("init manifest store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer manifestStore.Close()

	objectCfg := replaycli.DefaultMinIOObjectReaderConfig()
	objectCfg.Endpoint = envOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectCfg.Endpoint)
	objectCfg.AccessKeyID = envOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectCfg.AccessKeyID)
	objectCfg.SecretAccessKey = envOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectCfg.SecretAccessKey)
	objectCfg.Region = envOrDefault("ARCHIVE_OBJECT_REGION", objectCfg.Region)
	objectCfg.Secure = mustBool("ARCHIVE_OBJECT_SECURE", objectCfg.Secure)
	objectReader, err := replaycli.NewMinIOObjectReader(objectCfg)
	if err != nil {
		log.Error("init object reader failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer objectReader.Close()

	replayPublisher, err := replaycli.NewNATSPublisherWithConfig(natsConn, replaycli.NATSPublisherConfig{
		SubjectConfig: subjectCfg,
		Target:        replaycli.NATSPublishTargetIngest,
	})
	if err != nil {
		log.Error("init ingest replay publisher failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer replayPublisher.Close()

	runner, err := replaycli.NewRunner(log, manifestStore, objectReader, replayPublisher)
	if err != nil {
		log.Error("init replay runner failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer runner.Close()

	cfg := gaprepair.DefaultWorkerConfig()
	cfg.StreamName = envOrDefault("GAP_REPAIR_STREAM_NAME", cfg.StreamName)
	cfg.Durable = envOrDefault("GAP_REPAIR_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = envOrDefault("GAP_REPAIR_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = mustDuration("GAP_REPAIR_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = mustIntMin("GAP_REPAIR_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = mustDuration("GAP_REPAIR_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = mustDuration("GAP_REPAIR_DRAIN_TIMEOUT", cfg.DrainTimeout)
	cfg.DefaultMaxObjects = mustIntMin("GAP_REPAIR_DEFAULT_MAX_OBJECTS", cfg.DefaultMaxObjects, 0)

	worker, err := gaprepair.NewWorker(log, natsConn, runner, cfg)
	if err != nil {
		log.Error("init gap-repair worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("gap-repair worker starting",
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", subjectCfg.Prefix),
		slog.Uint64("shards", uint64(subjectCfg.ShardCount)),
		slog.String("stream", cfg.StreamName),
		slog.String("durable", cfg.Durable),
		slog.String("queue_group", cfg.QueueGroup),
		slog.Duration("ack_wait", cfg.AckWait),
		slog.Int("max_ack_pending", cfg.MaxAckPending),
		slog.Duration("process_timeout", cfg.ProcessTimeout),
		slog.Int("default_max_objects", cfg.DefaultMaxObjects),
		slog.Bool("nats_js_bootstrap_enabled", streamCfg.Enabled),
	)
	if err := worker.Run(ctx, subjectCfg); err != nil {
		log.Error("gap-repair worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("gap-repair worker stopped")
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

func mustInt64Min(key string, fallback int64, min int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
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
