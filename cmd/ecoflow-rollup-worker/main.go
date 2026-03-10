package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("rollup-worker")
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

	dbDSN := strings.TrimSpace(os.Getenv("ROLLUP_DB_DSN"))
	if dbDSN == "" {
		dbDSN = strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	}
	store, err := rollupworker.NewPostgresStore(dbDSN)
	if err != nil {
		log.Error("init rollup store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Warn("close rollup store failed", slog.String("error", err.Error()))
		}
	}()

	natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-rollup-worker")
	natsCfg.ConnectTimeout = runtimecfg.DurationPositive("NATS_CONNECT_TIMEOUT", natsCfg.ConnectTimeout)
	natsCfg.ReconnectWait = runtimecfg.DurationPositive("NATS_RECONNECT_WAIT", natsCfg.ReconnectWait)
	natsCfg.ReconnectJitter = runtimecfg.DurationPositive("NATS_RECONNECT_JITTER", natsCfg.ReconnectJitter)
	natsCfg.PingInterval = runtimecfg.DurationPositive("NATS_PING_INTERVAL", natsCfg.PingInterval)
	natsCfg.MaxPingsOut = runtimecfg.IntMin("NATS_MAX_PINGS_OUT", natsCfg.MaxPingsOut, 1)
	natsCfg.MaxReconnects = runtimecfg.IntMin("NATS_MAX_RECONNECTS", natsCfg.MaxReconnects, -1)

	natsConn, err := telemetrybus.DialNATS(log, natsCfg)
	if err != nil {
		log.Error("init nats connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer natsConn.Close()

	cfg := loadRollupWorkerConfigFromEnv()

	worker, err := rollupworker.New(log, natsConn, store, cfg)
	if err != nil {
		log.Error("init rollup worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "rollup-worker", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	log.Info("rollup worker starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
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
		log.Error("rollup worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("rollup worker stopped")
}

func loadRollupWorkerConfigFromEnv() rollupworker.Config {
	cfg := rollupworker.DefaultConfig()
	cfg.SubjectConfig = rollupworker.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	cfg.StreamName = runtimecfg.EnvOrDefault("ROLLUP_INGEST_STREAM_NAME", cfg.StreamName)
	cfg.Durable = runtimecfg.EnvOrDefault("ROLLUP_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = runtimecfg.EnvOrDefault("ROLLUP_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = runtimecfg.DurationPositive("ROLLUP_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = runtimecfg.IntMin("ROLLUP_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = runtimecfg.DurationPositive("ROLLUP_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = runtimecfg.DurationPositive("ROLLUP_DRAIN_TIMEOUT", cfg.DrainTimeout)
	return cfg
}
