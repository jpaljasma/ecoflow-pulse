package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/gaprepair"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("gap-detector")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	logCfg.AsyncEnabled = !mustBool("LOG_ASYNC_DISABLED", false)
	logCfg.AsyncQueueSize = mustIntMin("LOG_ASYNC_QUEUE_SIZE", logCfg.AsyncQueueSize, 128)
	logCfg.AsyncBypassLevel = pulselog.ParseLevel(envOrDefault("LOG_ASYNC_BYPASS_LEVEL", "warn"), slog.LevelWarn)

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

	valkeyAddrs := splitNonEmpty(envOrDefault("VALKEY_ADDRS", "127.0.0.1:6379"))
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
		KeyPrefix: envOrDefault("PROJECTION_KEY_PREFIX", projectionworker.DefaultValkeySnapshotStoreConfig().KeyPrefix),
	})
	if err != nil {
		log.Error("init projection snapshot store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	natsCfg := telemetrybus.DefaultNATSConnConfig(splitNonEmpty(envOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = envOrDefault("NATS_NAME", "ecoflow-gap-detector")
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

	publisherCfg := gaprepair.DefaultNATSPublisherConfig(subjectCfg)
	publisherCfg.UseJetStream = mustBool("GAP_REPAIR_NATS_USE_JETSTREAM", publisherCfg.UseJetStream)
	publisherCfg.MsgIDBucket = mustDuration("GAP_REPAIR_MSG_ID_BUCKET", publisherCfg.MsgIDBucket)

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
	cfg.PollInterval = mustDuration("GAP_REPAIR_POLL_INTERVAL", cfg.PollInterval)
	cfg.PollJitter = mustFloat64("GAP_REPAIR_POLL_JITTER", cfg.PollJitter)
	cfg.LookbackWindow = mustDuration("GAP_REPAIR_LOOKBACK_WINDOW", cfg.LookbackWindow)
	cfg.LagThreshold = mustDuration("GAP_REPAIR_LAG_THRESHOLD", cfg.LagThreshold)
	cfg.WindowPadding = mustDurationAllowZero("GAP_REPAIR_WINDOW_PADDING", cfg.WindowPadding)
	cfg.MaxReplayWindow = mustDuration("GAP_REPAIR_MAX_REPLAY_WINDOW", cfg.MaxReplayWindow)
	cfg.SafeDelay = mustDurationAllowZero("GAP_REPAIR_SAFE_DELAY", cfg.SafeDelay)
	cfg.MaxObjectsPerJob = mustIntMin("GAP_REPAIR_MAX_OBJECTS_PER_JOB", cfg.MaxObjectsPerJob, 0)
	cfg.MaxJobsPerCycle = mustIntMin("GAP_REPAIR_MAX_JOBS_PER_CYCLE", cfg.MaxJobsPerCycle, 1)
	cfg.EvaluationWorkers = mustIntMin("GAP_REPAIR_EVAL_WORKERS", cfg.EvaluationWorkers, 1)
	cfg.SubjectShardCount = subjectCfg.ShardCount
	cfg.DryRun = mustBool("GAP_REPAIR_DRY_RUN", false)

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
	logMetricsInterval := mustDurationAllowZero("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
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

func mustDurationAllowZero(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func mustFloat64(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
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
