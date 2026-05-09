package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/inference"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/startupretry"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/nats-io/nats.go"
	valkey "github.com/valkey-io/valkey-go"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("inference-worker")
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

	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Error("CONTROL_PLANE_DB_DSN is required")
		os.Exit(1)
	}
	controlPlaneStore, err := startupretry.Retry(context.Background(), log, "inference postgres store", startupretry.DefaultOptions(), func(_ context.Context) (*controlplane.PostgresStore, error) {
		return controlplane.NewPostgresStore(dsn)
	})
	if err != nil {
		log.Error("init control-plane store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = controlPlaneStore.Close() }()

	valkeyAddrs := runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("VALKEY_ADDRS", "127.0.0.1:6379"))
	valkeyCfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	valkeyCfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	valkeyCfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureSentinelFromEnv(&valkeyCfg)
	client, err := startupretry.Retry(context.Background(), log, "inference valkey client", startupretry.DefaultOptions(), func(_ context.Context) (valkey.Client, error) {
		return ingestlease.NewValkeyClient(valkeyCfg)
	})
	if err != nil {
		log.Error("init valkey client failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer client.Close()

	store, err := inference.NewValkeyStore(client, inference.ValkeyStoreConfig{
		KeyPrefix: runtimecfg.EnvOrDefault("INFERENCE_KEY_PREFIX", "pulse:inference"),
	})
	if err != nil {
		log.Error("init inference store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	resolver, err := inference.NewControlPlaneResolver(controlPlaneStore, inference.ControlPlaneResolverConfig{
		CacheTTL: runtimecfg.DurationPositive("INFERENCE_CONTEXT_CACHE_TTL", inference.DefaultControlPlaneResolverConfig().CacheTTL),
	})
	if err != nil {
		log.Error("init inference resolver failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-inference-worker")
	natsCfg.ConnectTimeout = runtimecfg.DurationPositive("NATS_CONNECT_TIMEOUT", natsCfg.ConnectTimeout)
	natsCfg.ReconnectWait = runtimecfg.DurationPositive("NATS_RECONNECT_WAIT", natsCfg.ReconnectWait)
	natsCfg.ReconnectJitter = runtimecfg.DurationPositive("NATS_RECONNECT_JITTER", natsCfg.ReconnectJitter)
	natsCfg.PingInterval = runtimecfg.DurationPositive("NATS_PING_INTERVAL", natsCfg.PingInterval)
	natsCfg.MaxPingsOut = runtimecfg.IntMin("NATS_MAX_PINGS_OUT", natsCfg.MaxPingsOut, 1)
	natsCfg.MaxReconnects = runtimecfg.IntMin("NATS_MAX_RECONNECTS", natsCfg.MaxReconnects, -1)

	natsConn, err := startupretry.Retry(context.Background(), log, "inference nats connection", startupretry.DefaultOptions(), func(_ context.Context) (*nats.Conn, error) {
		return telemetrybus.DialNATS(log, natsCfg)
	})
	if err != nil {
		log.Error("init nats connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer natsConn.Close()

	cfg := loadInferenceWorkerConfigFromEnv()

	worker, err := inference.NewWorker(log, natsConn, store, resolver, cfg)
	if err != nil {
		log.Error("init inference worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	metrics := workermetrics.New("inference")
	metrics.RequireConsumerSubscription()
	worker.SetMetrics(metrics)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	metricsListenAddr := strings.TrimSpace(os.Getenv("INFERENCE_METRICS_LISTEN_ADDR"))
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "inference-worker", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()
	stopMetricsServer := workermetrics.StartServerWithReadiness(ctx, log, metrics.Registry(), metricsListenAddr, metrics.ReadyStatus)
	defer stopMetricsServer()

	log.Info("inference worker starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
		slog.String("metrics_listen_addr", metricsListenAddr),
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
		slog.String("inference_key_prefix", runtimecfg.EnvOrDefault("INFERENCE_KEY_PREFIX", "pulse:inference")),
	)
	if err := worker.Run(ctx); err != nil {
		log.Error("inference worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("inference worker stopped")
}

func loadInferenceWorkerConfigFromEnv() inference.Config {
	cfg := inference.DefaultWorkerConfig()
	cfg.SubjectConfig = inference.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	cfg.StreamName = runtimecfg.EnvOrDefault("INFERENCE_INGEST_STREAM_NAME", cfg.StreamName)
	cfg.Durable = runtimecfg.EnvOrDefault("INFERENCE_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = runtimecfg.EnvOrDefault("INFERENCE_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = runtimecfg.DurationPositive("INFERENCE_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = runtimecfg.IntMin("INFERENCE_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = runtimecfg.DurationPositive("INFERENCE_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = runtimecfg.DurationPositive("INFERENCE_DRAIN_TIMEOUT", cfg.DrainTimeout)
	return cfg
}
