package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/gaprepair"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("gap-detector")
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

	dbDSN := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dbDSN == "" {
		log.Error("CONTROL_PLANE_DB_DSN is required for gap detector")
		os.Exit(1)
	}
	assignmentStore, err := controlplane.NewPostgresStore(dbDSN)
	if err != nil {
		log.Error("init control-plane store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = assignmentStore.Close() }()

	coverageStore, err := gaprepair.NewPostgresCoverageStore(dbDSN)
	if err != nil {
		log.Error("init coverage store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if closeErr := coverageStore.Close(); closeErr != nil {
			log.Warn("close coverage store failed", slog.String("error", closeErr.Error()))
		}
	}()

	valkeyAddrs := runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("VALKEY_ADDRS", "127.0.0.1:6379"))
	valkeyCfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	valkeyCfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	valkeyCfg.Password = os.Getenv("VALKEY_PASSWORD")
	valkeyClient, err := ingestlease.NewValkeyClient(valkeyCfg)
	if err != nil {
		log.Error("init valkey client failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer valkeyClient.Close()

	snapshotStore, err := projectionworker.NewValkeySnapshotStore(valkeyClient, projectionworker.ValkeySnapshotStoreConfig{
		KeyPrefix: runtimecfg.EnvOrDefault("PROJECTION_KEY_PREFIX", projectionworker.DefaultValkeySnapshotStoreConfig().KeyPrefix),
	})
	if err != nil {
		log.Error("init projection snapshot store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-gap-detector")
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

	subjectCfg := telemetrybus.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}.Normalized()

	streamCfg := telemetrybus.DefaultJetStreamGapRepairBootstrapConfig()
	streamCfg.Enabled = runtimecfg.Bool("GAP_REPAIR_NATS_JS_BOOTSTRAP_ENABLED", true)
	streamCfg.StreamName = runtimecfg.EnvOrDefault("GAP_REPAIR_NATS_JS_STREAM_NAME", streamCfg.StreamName)
	streamCfg.Replicas = runtimecfg.IntMin("GAP_REPAIR_NATS_JS_REPLICAS", streamCfg.Replicas, 1)
	streamCfg.MaxAge = runtimecfg.DurationPositive("GAP_REPAIR_NATS_JS_MAX_AGE", streamCfg.MaxAge)
	streamCfg.MaxBytes = runtimecfg.Int64Min("GAP_REPAIR_NATS_JS_MAX_BYTES", streamCfg.MaxBytes, 0)
	if streamCfg.Enabled {
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := telemetrybus.EnsureJetStreamGapRepairStream(bootstrapCtx, natsConn, subjectCfg, streamCfg); err != nil {
			cancel()
			log.Error("bootstrap gap-repair stream failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		cancel()
	}

	publisherCfg := gaprepair.DefaultNATSPublisherConfig(subjectCfg)
	publisherCfg.UseJetStream = runtimecfg.Bool("GAP_REPAIR_NATS_USE_JETSTREAM", publisherCfg.UseJetStream)
	publisherCfg.MsgIDBucket = runtimecfg.DurationPositive("GAP_REPAIR_MSG_ID_BUCKET", publisherCfg.MsgIDBucket)

	publisher, err := gaprepair.NewNATSPublisher(natsConn, publisherCfg)
	if err != nil {
		log.Error("init gap-repair publisher failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			log.Warn("close gap-repair publisher failed", slog.String("error", closeErr.Error()))
		}
	}()

	cfg := gaprepair.DefaultDetectorConfig()
	cfg.ProviderFilter = controlplane.NormalizeProvider(strings.TrimSpace(os.Getenv("GAP_REPAIR_PROVIDER")))
	cfg.PollInterval = runtimecfg.DurationPositive("GAP_REPAIR_POLL_INTERVAL", cfg.PollInterval)
	cfg.PollJitter = runtimecfg.Float64NonNegative("GAP_REPAIR_POLL_JITTER", cfg.PollJitter)
	cfg.LookbackWindow = runtimecfg.DurationPositive("GAP_REPAIR_LOOKBACK_WINDOW", cfg.LookbackWindow)
	cfg.LagThreshold = runtimecfg.DurationPositive("GAP_REPAIR_LAG_THRESHOLD", cfg.LagThreshold)
	cfg.WindowPadding = runtimecfg.DurationNonNegative("GAP_REPAIR_WINDOW_PADDING", cfg.WindowPadding)
	cfg.MaxReplayWindow = runtimecfg.DurationPositive("GAP_REPAIR_MAX_REPLAY_WINDOW", cfg.MaxReplayWindow)
	cfg.SafeDelay = runtimecfg.DurationNonNegative("GAP_REPAIR_SAFE_DELAY", cfg.SafeDelay)
	cfg.MaxObjectsPerJob = runtimecfg.IntMin("GAP_REPAIR_MAX_OBJECTS_PER_JOB", cfg.MaxObjectsPerJob, 0)
	cfg.MaxJobsPerCycle = runtimecfg.IntMin("GAP_REPAIR_MAX_JOBS_PER_CYCLE", cfg.MaxJobsPerCycle, 1)
	cfg.EvaluationWorkers = runtimecfg.IntMin("GAP_REPAIR_EVAL_WORKERS", cfg.EvaluationWorkers, 1)
	cfg.SubjectShardCount = subjectCfg.ShardCount
	cfg.DryRun = runtimecfg.Bool("GAP_REPAIR_DRY_RUN", false)

	if cfg.EvaluationWorkers <= 0 {
		cfg.EvaluationWorkers = minInt(maxInt(runtime.GOMAXPROCS(0)*2, 16), 64)
	}

	detector, err := gaprepair.NewDetector(log, assignmentStore, coverageStore, snapshotStore, publisher, cfg)
	if err != nil {
		log.Error("init gap detector failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "gap-detector", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	log.Info("gap detector starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
		slog.String("provider_filter", cfg.ProviderFilter),
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", subjectCfg.Prefix),
		slog.Uint64("shards", uint64(subjectCfg.ShardCount)),
		slog.Bool("dry_run", cfg.DryRun),
		slog.Duration("poll_interval", cfg.PollInterval),
		slog.Float64("poll_jitter", cfg.PollJitter),
		slog.Duration("lookback_window", cfg.LookbackWindow),
		slog.Duration("lag_threshold", cfg.LagThreshold),
		slog.Duration("max_replay_window", cfg.MaxReplayWindow),
		slog.Int("max_jobs_per_cycle", cfg.MaxJobsPerCycle),
		slog.Int("evaluation_workers", cfg.EvaluationWorkers),
		slog.Bool("nats_use_jetstream", publisherCfg.UseJetStream),
		slog.Bool("nats_js_bootstrap_enabled", streamCfg.Enabled),
		slog.String("nats_js_stream_name", streamCfg.StreamName),
	)
	if err := detector.Run(ctx); err != nil {
		log.Error("gap detector stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("gap detector stopped")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
