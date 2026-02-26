package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/gaprepair"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("gap-repair-worker")
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

	natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-gap-repair-worker")
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
	defer func() { _ = manifestStore.Close() }()

	objectCfg := replaycli.DefaultMinIOObjectReaderConfig()
	objectCfg.Endpoint = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectCfg.Endpoint)
	objectCfg.AccessKeyID = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectCfg.AccessKeyID)
	objectCfg.SecretAccessKey = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectCfg.SecretAccessKey)
	objectCfg.Region = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", objectCfg.Region)
	objectCfg.Secure = runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", objectCfg.Secure)
	objectReader, err := replaycli.NewMinIOObjectReader(objectCfg)
	if err != nil {
		log.Error("init object reader failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = objectReader.Close() }()

	replayPublisher, err := replaycli.NewNATSPublisherWithConfig(natsConn, replaycli.NATSPublisherConfig{
		SubjectConfig: subjectCfg,
		Target:        replaycli.NATSPublishTargetIngest,
	})
	if err != nil {
		log.Error("init ingest replay publisher failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = replayPublisher.Close() }()

	runner, err := replaycli.NewRunner(log, manifestStore, objectReader, replayPublisher)
	if err != nil {
		log.Error("init replay runner failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = runner.Close() }()

	cfg := gaprepair.DefaultWorkerConfig()
	cfg.StreamName = runtimecfg.EnvOrDefault("GAP_REPAIR_STREAM_NAME", cfg.StreamName)
	cfg.Durable = runtimecfg.EnvOrDefault("GAP_REPAIR_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = runtimecfg.EnvOrDefault("GAP_REPAIR_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = runtimecfg.DurationPositive("GAP_REPAIR_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = runtimecfg.IntMin("GAP_REPAIR_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = runtimecfg.DurationPositive("GAP_REPAIR_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = runtimecfg.DurationPositive("GAP_REPAIR_DRAIN_TIMEOUT", cfg.DrainTimeout)
	cfg.DefaultMaxObjects = runtimecfg.IntMin("GAP_REPAIR_DEFAULT_MAX_OBJECTS", cfg.DefaultMaxObjects, 0)

	worker, err := gaprepair.NewWorker(log, natsConn, runner, cfg)
	if err != nil {
		log.Error("init gap-repair worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "gap-repair-worker", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	log.Info("gap-repair worker starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
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
