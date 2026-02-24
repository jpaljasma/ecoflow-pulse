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

	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	valkeyAddrs := splitNonEmpty(envOrDefault("VALKEY_ADDRS", "127.0.0.1:6379"))
	valkeyCfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	valkeyCfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	valkeyCfg.Password = os.Getenv("VALKEY_PASSWORD")
	client, err := ingestlease.NewValkeyClient(valkeyCfg)
	if err != nil {
		log.Error("init valkey client failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer client.Close()

	snapshotStore, err := projectionworker.NewValkeySnapshotStore(client, projectionworker.ValkeySnapshotStoreConfig{
		KeyPrefix: envOrDefault("PROJECTION_KEY_PREFIX", "pulse:projection"),
	})
	if err != nil {
		log.Error("init projection snapshot store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	natsCfg := telemetrybus.DefaultNATSConnConfig(splitNonEmpty(envOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = envOrDefault("NATS_NAME", "ecoflow-projection-worker")
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

	cfg := projectionworker.DefaultConfig()
	cfg.SubjectConfig = projectionworker.SubjectConfig{
		Prefix:     envOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: mustUint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	cfg.StreamName = envOrDefault("PROJECTION_INGEST_STREAM_NAME", cfg.StreamName)
	cfg.Durable = envOrDefault("PROJECTION_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = envOrDefault("PROJECTION_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = mustDuration("PROJECTION_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = mustIntMin("PROJECTION_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = mustDuration("PROJECTION_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = mustDuration("PROJECTION_DRAIN_TIMEOUT", cfg.DrainTimeout)

	worker, err := projectionworker.New(log, natsConn, snapshotStore, cfg)
	if err != nil {
		log.Error("init projection worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Info("projection worker starting",
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", cfg.SubjectConfig.Prefix),
		slog.Uint64("shards", uint64(cfg.SubjectConfig.ShardCount)),
		slog.String("ingest_stream", cfg.StreamName),
		slog.String("durable", cfg.Durable),
		slog.String("queue_group", cfg.QueueGroup),
		slog.Duration("ack_wait", cfg.AckWait),
		slog.Int("max_ack_pending", cfg.MaxAckPending),
		slog.Duration("process_timeout", cfg.ProcessTimeout),
		slog.Duration("drain_timeout", cfg.DrainTimeout),
	)
	if err := worker.Run(ctx); err != nil {
		log.Error("projection worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("projection worker stopped")
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
