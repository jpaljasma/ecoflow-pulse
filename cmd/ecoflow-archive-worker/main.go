package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jpaljasma/ecoflow-pulse/internal/archiveworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/startupretry"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/nats-io/nats.go"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("archive-worker")
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
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-archive-worker")
	natsCfg.ConnectTimeout = runtimecfg.DurationPositive("NATS_CONNECT_TIMEOUT", natsCfg.ConnectTimeout)
	natsCfg.ReconnectWait = runtimecfg.DurationPositive("NATS_RECONNECT_WAIT", natsCfg.ReconnectWait)
	natsCfg.ReconnectJitter = runtimecfg.DurationPositive("NATS_RECONNECT_JITTER", natsCfg.ReconnectJitter)
	natsCfg.PingInterval = runtimecfg.DurationPositive("NATS_PING_INTERVAL", natsCfg.PingInterval)
	natsCfg.MaxPingsOut = runtimecfg.IntMin("NATS_MAX_PINGS_OUT", natsCfg.MaxPingsOut, 1)
	natsCfg.MaxReconnects = runtimecfg.IntMin("NATS_MAX_RECONNECTS", natsCfg.MaxReconnects, -1)

	natsConn, err := startupretry.Retry(context.Background(), log, "archive nats connection", startupretry.DefaultOptions(), func(_ context.Context) (*nats.Conn, error) {
		return telemetrybus.DialNATS(log, natsCfg)
	})
	if err != nil {
		log.Error("init nats connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer natsConn.Close()

	storeCfg := archiveworker.DefaultObjectStoreConfig()
	storeCfg.Provider = archiveworker.ObjectStoreProvider(runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PROVIDER", string(storeCfg.Provider)))
	storeCfg.Endpoint = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", storeCfg.Endpoint)
	storeCfg.AccessKeyID = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", storeCfg.AccessKeyID)
	storeCfg.SecretAccessKey = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", storeCfg.SecretAccessKey)
	storeCfg.Region = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", storeCfg.Region)
	storeCfg.Secure = runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", storeCfg.Secure)
	storeCfg.AutoCreateBucket = runtimecfg.Bool("ARCHIVE_OBJECT_AUTO_CREATE_BUCKET", storeCfg.AutoCreateBucket)
	storeCfg.GCSProjectID = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_GCS_PROJECT_ID", storeCfg.GCSProjectID)
	objectStore, err := startupretry.Retry(context.Background(), log, "archive object store", startupretry.DefaultOptions(), func(ctx context.Context) (archiveworker.ObjectStore, error) {
		return archiveworker.NewObjectStore(ctx, storeCfg)
	})
	if err != nil {
		log.Error("init archive object store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg := loadArchiveWorkerConfigFromEnv()
	outboxCfg, outboxEnabled := loadArchiveUploadOutboxConfigFromEnv()
	var archiveOutbox archiveworker.ArchiveUploadOutbox
	if outboxEnabled {
		archiveOutbox, err = archiveworker.NewFileArchiveUploadOutbox(outboxCfg)
		if err != nil {
			log.Error("init archive upload outbox failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	manifestDSN := resolveArchiveManifestDSN()
	var manifestStore archiveworker.ManifestStore
	if manifestDSN != "" {
		manifestStore, err = startupretry.Retry(context.Background(), log, "archive manifest store", startupretry.DefaultOptions(), func(_ context.Context) (*archiveworker.PostgresManifestStore, error) {
			return archiveworker.NewPostgresManifestStore(manifestDSN)
		})
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

	metrics := workermetrics.New("archive")
	metrics.RequireConsumerSubscription()
	workerOptions := []archiveworker.WorkerOption{
		archiveworker.WithManifestStore(manifestStore),
		archiveworker.WithMetrics(metrics),
	}
	if archiveOutbox != nil {
		workerOptions = append(workerOptions, archiveworker.WithArchiveUploadOutbox(archiveOutbox))
	}
	worker, err := archiveworker.New(log, natsConn, objectStore, cfg, workerOptions...)
	if err != nil {
		log.Error("init archive worker failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	metricsListenAddr := strings.TrimSpace(os.Getenv("ARCHIVE_METRICS_LISTEN_ADDR"))
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "archive-worker", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()
	stopMetricsServer := workermetrics.StartServerWithReadiness(ctx, log, metrics.Registry(), metricsListenAddr, metrics.ReadyStatus)
	defer stopMetricsServer()

	log.Info("archive worker starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
		slog.String("metrics_listen_addr", metricsListenAddr),
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", cfg.SubjectConfig.Prefix),
		slog.Uint64("shards", uint64(cfg.SubjectConfig.ShardCount)),
		slog.String("stream", cfg.StreamName),
		slog.String("durable", cfg.Durable),
		slog.String("queue_group", cfg.QueueGroup),
		slog.Duration("ack_wait", cfg.AckWait),
		slog.Int("max_ack_pending", cfg.MaxAckPending),
		slog.Duration("failure_alert_window", cfg.FailureAlertWindow),
		slog.Int("failure_alert_threshold", cfg.FailureAlertThreshold),
		slog.Duration("failure_alert_cooldown", cfg.FailureAlertCooldown),
		slog.Duration("flush_interval", cfg.FlushInterval),
		slog.Int("max_records_per_part", cfg.MaxRecordsPerPart),
		slog.Int("max_bytes_per_part", cfg.MaxBytesPerPart),
		slog.String("object_bucket", cfg.ObjectBucket),
		slog.String("object_prefix", cfg.ObjectPrefix),
		slog.String("writer_id", cfg.WriterID),
		slog.String("object_provider", string(storeCfg.Provider)),
		slog.String("object_endpoint", storeCfg.Endpoint),
		slog.Bool("object_secure", storeCfg.Secure),
		slog.String("object_gcs_project_id", storeCfg.GCSProjectID),
		slog.Bool("manifest_enabled", manifestStore != nil),
		slog.Bool("archive_upload_outbox_enabled", archiveOutbox != nil),
		slog.String("archive_upload_outbox_dir", outboxCfg.Dir),
		slog.Int64("archive_upload_outbox_max_bytes", outboxCfg.MaxBytes),
	)
	if err := worker.Run(ctx); err != nil {
		log.Error("archive worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("archive worker stopped")
}

func loadArchiveWorkerConfigFromEnv() archiveworker.Config {
	cfg := archiveworker.DefaultConfig()
	cfg.SubjectConfig = archiveworker.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	cfg.StreamName = runtimecfg.EnvOrDefault("ARCHIVE_INGEST_STREAM_NAME", cfg.StreamName)
	cfg.Durable = runtimecfg.EnvOrDefault("ARCHIVE_CONSUMER_DURABLE", cfg.Durable)
	cfg.QueueGroup = runtimecfg.EnvOrDefault("ARCHIVE_QUEUE_GROUP", cfg.QueueGroup)
	cfg.AckWait = runtimecfg.DurationPositive("ARCHIVE_ACK_WAIT", cfg.AckWait)
	cfg.MaxAckPending = runtimecfg.IntMin("ARCHIVE_MAX_ACK_PENDING", cfg.MaxAckPending, 1)
	cfg.ProcessTimeout = runtimecfg.DurationPositive("ARCHIVE_PROCESS_TIMEOUT", cfg.ProcessTimeout)
	cfg.DrainTimeout = runtimecfg.DurationPositive("ARCHIVE_DRAIN_TIMEOUT", cfg.DrainTimeout)
	cfg.FailureAlertWindow = runtimecfg.DurationPositive("ARCHIVE_FAILURE_ALERT_WINDOW", cfg.FailureAlertWindow)
	cfg.FailureAlertThreshold = runtimecfg.IntPositive("ARCHIVE_FAILURE_ALERT_THRESHOLD", cfg.FailureAlertThreshold)
	cfg.FailureAlertCooldown = runtimecfg.DurationPositive("ARCHIVE_FAILURE_ALERT_COOLDOWN", cfg.FailureAlertCooldown)
	cfg.FlushInterval = runtimecfg.DurationPositive("ARCHIVE_FLUSH_INTERVAL", cfg.FlushInterval)
	cfg.FlushTimeout = runtimecfg.DurationPositive("ARCHIVE_FLUSH_TIMEOUT", cfg.FlushTimeout)
	cfg.MaxRecordsPerPart = runtimecfg.IntMin("ARCHIVE_MAX_RECORDS_PER_PART", cfg.MaxRecordsPerPart, 1)
	cfg.MaxBytesPerPart = runtimecfg.IntMin("ARCHIVE_MAX_BYTES_PER_PART", cfg.MaxBytesPerPart, 1024)
	cfg.ObjectBucket = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_BUCKET", cfg.ObjectBucket)
	cfg.ObjectPrefix = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PREFIX", cfg.ObjectPrefix)
	cfg.WriterID = runtimecfg.EnvOrDefault("ARCHIVE_WRITER_ID", cfg.WriterID)
	cfg.ZstdEncoderLevel = runtimecfg.IntAny("ARCHIVE_ZSTD_LEVEL", cfg.ZstdEncoderLevel)
	return cfg
}

func loadArchiveUploadOutboxConfigFromEnv() (archiveworker.FileArchiveUploadOutboxConfig, bool) {
	cfg := archiveworker.FileArchiveUploadOutboxConfig{
		Dir:      strings.TrimSpace(os.Getenv("ARCHIVE_UPLOAD_OUTBOX_DIR")),
		MaxBytes: runtimecfg.Int64Min("ARCHIVE_UPLOAD_OUTBOX_MAX_BYTES", 0, 0),
	}
	return cfg, cfg.Dir != ""
}

func resolveArchiveManifestDSN() string {
	manifestDSN := strings.TrimSpace(os.Getenv("ARCHIVE_MANIFEST_DB_DSN"))
	if manifestDSN == "" {
		manifestDSN = strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	}
	return manifestDSN
}
