package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("ingest-worker")
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
		log.Error("CONTROL_PLANE_DB_DSN is required for ingest worker")
		os.Exit(1)
	}
	store, err := controlplane.NewPostgresStore(dbDSN)
	if err != nil {
		log.Error("init postgres store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	valkeyAddrs := runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("VALKEY_ADDRS", "127.0.0.1:6379"))
	valkeyCfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	valkeyCfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	valkeyCfg.Password = os.Getenv("VALKEY_PASSWORD")
	client, err := ingestlease.NewValkeyClient(valkeyCfg)
	if err != nil {
		log.Error("init valkey client failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer client.Close()

	leaseMgr, err := ingestlease.NewManager(client, ingestlease.DefaultConfig())
	if err != nil {
		log.Error("init lease manager failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	natsCfg := telemetrybus.DefaultNATSConnConfig(runtimecfg.SplitNonEmpty(runtimecfg.EnvOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-ingest-worker")
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

	ecoCfg := ecoflow.DefaultConfig()
	ecoCfg.Logging.Debug = false
	ecoCfg.Logging.AdvancedDebugTelemetry = false
	ecoCfg.Logging.DebugLogHeaders = false
	ecoCfg.Logging.Logger = log
	adapter := provideradapter.NewEcoFlowAdapter(provideradapter.NewDefaultEcoFlowClientFactory(ecoCfg))
	subjectCfg := telemetrybus.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	disableEnvelopeLabels := runtimecfg.Bool("INGEST_DISABLE_ENVELOPE_LABELS", false)

	publishOpts := telemetrybus.NATSEnvelopePublisherOptions{
		StripLabels:                disableEnvelopeLabels,
		UseJetStream:               runtimecfg.Bool("INGEST_NATS_USE_JETSTREAM", true),
		PublishTimeout:             runtimecfg.DurationPositive("INGEST_NATS_PUBLISH_TIMEOUT", 3*time.Second),
		PublishMaxRetries:          runtimecfg.IntMin("INGEST_NATS_PUBLISH_MAX_RETRIES", 3, 0),
		PublishRetryInitialBackoff: runtimecfg.DurationPositive("INGEST_NATS_PUBLISH_RETRY_INITIAL_BACKOFF", 50*time.Millisecond),
		PublishRetryMaxBackoff:     runtimecfg.DurationPositive("INGEST_NATS_PUBLISH_RETRY_MAX_BACKOFF", 500*time.Millisecond),
		PublishRetryJitter:         runtimecfg.Float64NonNegative("INGEST_NATS_PUBLISH_RETRY_JITTER", 0.20),
	}
	jetstreamCfg := telemetrybus.DefaultJetStreamIngestBootstrapConfig()
	jetstreamCfg.Enabled = runtimecfg.Bool("INGEST_NATS_JS_BOOTSTRAP_ENABLED", publishOpts.UseJetStream)
	jetstreamCfg.StreamName = runtimecfg.EnvOrDefault("INGEST_NATS_JS_STREAM_NAME", jetstreamCfg.StreamName)
	jetstreamCfg.Replicas = runtimecfg.IntMin("INGEST_NATS_JS_REPLICAS", jetstreamCfg.Replicas, 1)
	jetstreamCfg.MaxAge = runtimecfg.DurationPositive("INGEST_NATS_JS_MAX_AGE", jetstreamCfg.MaxAge)
	jetstreamCfg.MaxBytes = runtimecfg.Int64Min("INGEST_NATS_JS_MAX_BYTES", jetstreamCfg.MaxBytes, 0)

	if jetstreamCfg.Enabled {
		bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := telemetrybus.EnsureJetStreamIngestStream(bootstrapCtx, natsConn, subjectCfg, jetstreamCfg); err != nil {
			bootstrapCancel()
			log.Error("bootstrap ingest jetstream stream failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		bootstrapCancel()
	}

	publisher, err := telemetrybus.NewNATSEnvelopePublisherWithOptions(
		natsConn,
		subjectCfg,
		publishOpts,
	)
	if err != nil {
		log.Error("init telemetry envelope publisher failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			log.Warn("close telemetry envelope publisher failed", slog.String("error", closeErr.Error()))
		}
	}()

	sessionCfg := ingestworker.DefaultEcoFlowSessionConfig()
	sessionCfg.ShardCount = subjectCfg.ShardCount
	sessionCfg.KeepAlive = runtimecfg.DurationPositive("INGEST_MQTT_KEEPALIVE", sessionCfg.KeepAlive)
	sessionCfg.ConnectTimeout = runtimecfg.DurationPositive("INGEST_MQTT_CONNECT_TIMEOUT", sessionCfg.ConnectTimeout)
	sessionCfg.ReadTimeout = runtimecfg.DurationPositive("INGEST_MQTT_READ_TIMEOUT", sessionCfg.ReadTimeout)
	sessionCfg.WriteTimeout = runtimecfg.DurationPositive("INGEST_MQTT_WRITE_TIMEOUT", sessionCfg.WriteTimeout)
	sessionCfg.ReconnectInitialBackoff = runtimecfg.DurationPositive("INGEST_MQTT_RECONNECT_INITIAL_BACKOFF", sessionCfg.ReconnectInitialBackoff)
	sessionCfg.ReconnectMaxBackoff = runtimecfg.DurationPositive("INGEST_MQTT_RECONNECT_MAX_BACKOFF", sessionCfg.ReconnectMaxBackoff)
	sessionCfg.ReconnectJitter = runtimecfg.Float64NonNegative("INGEST_MQTT_RECONNECT_JITTER", sessionCfg.ReconnectJitter)
	sessionCfg.ReconnectAlertWindow = runtimecfg.DurationPositive("INGEST_MQTT_RECONNECT_ALERT_WINDOW", sessionCfg.ReconnectAlertWindow)
	sessionCfg.ReconnectAlertThreshold = runtimecfg.IntPositive("INGEST_MQTT_RECONNECT_ALERT_THRESHOLD", sessionCfg.ReconnectAlertThreshold)
	sessionCfg.ReconnectAlertCooldown = runtimecfg.DurationPositive("INGEST_MQTT_RECONNECT_ALERT_COOLDOWN", sessionCfg.ReconnectAlertCooldown)
	sessionCfg.AuthAlertWindow = runtimecfg.DurationPositive("INGEST_MQTT_AUTH_ALERT_WINDOW", sessionCfg.AuthAlertWindow)
	sessionCfg.AuthAlertThreshold = runtimecfg.IntPositive("INGEST_MQTT_AUTH_ALERT_THRESHOLD", sessionCfg.AuthAlertThreshold)
	sessionCfg.AuthAlertCooldown = runtimecfg.DurationPositive("INGEST_MQTT_AUTH_ALERT_COOLDOWN", sessionCfg.AuthAlertCooldown)
	sessionCfg.QuotaFetchTimeout = runtimecfg.DurationPositive("INGEST_QUOTA_FETCH_TIMEOUT", sessionCfg.QuotaFetchTimeout)
	sessionCfg.QuotaRefreshInterval = runtimecfg.DurationPositive("INGEST_QUOTA_REFRESH_INTERVAL", sessionCfg.QuotaRefreshInterval)
	sessionCfg.QuotaRefreshJitter = runtimecfg.Float64NonNegative("INGEST_QUOTA_REFRESH_JITTER", sessionCfg.QuotaRefreshJitter)
	sessionCfg.PublishQueueSize = runtimecfg.IntPositive("INGEST_PUBLISH_QUEUE_SIZE", sessionCfg.PublishQueueSize)
	sessionCfg.PublishWorkers = runtimecfg.IntPositive("INGEST_PUBLISH_WORKERS", sessionCfg.PublishWorkers)
	sessionCfg.PublishEnqueueTimeout = runtimecfg.DurationPositive("INGEST_PUBLISH_ENQUEUE_TIMEOUT", sessionCfg.PublishEnqueueTimeout)
	sessionCfg.AllowUnorderedPublish = runtimecfg.Bool("INGEST_ALLOW_UNORDERED_PUBLISH", sessionCfg.AllowUnorderedPublish)
	sessionCfg.DisableEnvelopeLabels = disableEnvelopeLabels
	sessionCfg.LogMQTTPayloadDebug = runtimecfg.Bool("INGEST_MQTT_LOG_PAYLOAD_DEBUG", sessionCfg.LogMQTTPayloadDebug)
	sessionCfg.LogMQTTPayloadSampleEvery = runtimecfg.IntMin("INGEST_MQTT_LOG_PAYLOAD_SAMPLE_EVERY", sessionCfg.LogMQTTPayloadSampleEvery, 1)
	ecoFlowRunner, err := ingestworker.NewEcoFlowSessionRunner(log, adapter, publisher, store, sessionCfg)
	if err != nil {
		log.Error("init session runner failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	runner := ingestworker.NewProviderSessionRunner()
	runner.Register(controlplane.ProviderEcoFlow, ecoFlowRunner)

	loopCfg := loadIngestLoopConfigFromEnv()
	autoscaleMetrics := ingestworker.NewAutoscaleMetrics()

	loop, err := ingestworker.NewLoop(log, store, leaseMgr, runner, ingestworker.Config{
		WorkerID:                   loopCfg.WorkerID,
		ProviderFilter:             loopCfg.ProviderFilter,
		PollInterval:               loopCfg.PollInterval,
		PollJitter:                 loopCfg.PollJitter,
		StopTimeout:                loopCfg.StopTimeout,
		StartWorkers:               loopCfg.StartWorkers,
		StartQueueSize:             loopCfg.StartQueueSize,
		LeaseMissingAlertWindow:    loopCfg.LeaseMissingAlertWindow,
		LeaseMissingAlertThreshold: loopCfg.LeaseMissingAlertThreshold,
		LeaseMissingAlertCooldown:  loopCfg.LeaseMissingAlertCooldown,
		AutoscaleMetrics:           autoscaleMetrics,
	})
	if err != nil {
		log.Error("init ingest worker loop failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	quotaMetricsInterval := runtimecfg.DurationNonNegative("INGEST_QUOTA_METRICS_INTERVAL", ingestworker.DefaultQuotaMetricsInterval())
	metricsListenAddr := strings.TrimSpace(os.Getenv("INGEST_METRICS_LISTEN_ADDR"))
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "ingest-worker", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()
	stopQuotaMetrics := ingestworker.StartQuotaMetricsReporter(ctx, log, "ingest-worker", ecoFlowRunner.QuotaMetrics(), quotaMetricsInterval)
	defer stopQuotaMetrics()
	stopAutoscaleMetrics := startAutoscaleMetricsServer(ctx, log, autoscaleMetrics.Registry(), metricsListenAddr)
	defer stopAutoscaleMetrics()

	log.Info("ingest worker starting",
		slog.String("log_level", logCfg.Level.String()),
		slog.Bool("log_async_enabled", logCfg.AsyncEnabled),
		slog.Int("log_async_queue_size", logCfg.AsyncQueueSize),
		slog.String("log_async_bypass_level", logCfg.AsyncBypassLevel.String()),
		slog.Duration("log_metrics_interval", logMetricsInterval),
		slog.Duration("quota_metrics_interval", quotaMetricsInterval),
		slog.String("metrics_listen_addr", metricsListenAddr),
		slog.String("worker_id", loopCfg.WorkerID),
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", subjectCfg.Prefix),
		slog.Uint64("shards", uint64(subjectCfg.ShardCount)),
		slog.Duration("poll_interval", loopCfg.PollInterval),
		slog.Float64("poll_jitter", loopCfg.PollJitter),
		slog.Int("start_workers", loopCfg.StartWorkers),
		slog.Int("start_queue_size", loopCfg.StartQueueSize),
		slog.Duration("lease_missing_alert_window", loopCfg.LeaseMissingAlertWindow),
		slog.Int("lease_missing_alert_threshold", loopCfg.LeaseMissingAlertThreshold),
		slog.Duration("lease_missing_alert_cooldown", loopCfg.LeaseMissingAlertCooldown),
		slog.Int("publish_queue_size", sessionCfg.PublishQueueSize),
		slog.Int("publish_workers", sessionCfg.PublishWorkers),
		slog.Duration("publish_enqueue_timeout", sessionCfg.PublishEnqueueTimeout),
		slog.Duration("mqtt_keepalive", sessionCfg.KeepAlive),
		slog.Duration("mqtt_read_timeout", sessionCfg.ReadTimeout),
		slog.Duration("mqtt_reconnect_alert_window", sessionCfg.ReconnectAlertWindow),
		slog.Int("mqtt_reconnect_alert_threshold", sessionCfg.ReconnectAlertThreshold),
		slog.Duration("mqtt_reconnect_alert_cooldown", sessionCfg.ReconnectAlertCooldown),
		slog.Duration("mqtt_auth_alert_window", sessionCfg.AuthAlertWindow),
		slog.Int("mqtt_auth_alert_threshold", sessionCfg.AuthAlertThreshold),
		slog.Duration("mqtt_auth_alert_cooldown", sessionCfg.AuthAlertCooldown),
		slog.Duration("quota_fetch_timeout", sessionCfg.QuotaFetchTimeout),
		slog.Duration("quota_refresh_interval", sessionCfg.QuotaRefreshInterval),
		slog.Float64("quota_refresh_jitter", sessionCfg.QuotaRefreshJitter),
		slog.Bool("mqtt_payload_debug", sessionCfg.LogMQTTPayloadDebug),
		slog.Int("mqtt_payload_sample_every", sessionCfg.LogMQTTPayloadSampleEvery),
		slog.Bool("allow_unordered_publish", sessionCfg.AllowUnorderedPublish),
		slog.Bool("disable_envelope_labels", sessionCfg.DisableEnvelopeLabels),
		slog.Bool("nats_use_jetstream", publishOpts.UseJetStream),
		slog.Duration("nats_publish_timeout", publishOpts.PublishTimeout),
		slog.Int("nats_publish_max_retries", publishOpts.PublishMaxRetries),
		slog.Duration("nats_publish_retry_initial_backoff", publishOpts.PublishRetryInitialBackoff),
		slog.Duration("nats_publish_retry_max_backoff", publishOpts.PublishRetryMaxBackoff),
		slog.Float64("nats_publish_retry_jitter", publishOpts.PublishRetryJitter),
		slog.Bool("nats_js_bootstrap_enabled", jetstreamCfg.Enabled),
		slog.String("nats_js_stream_name", jetstreamCfg.StreamName),
		slog.Int("nats_js_replicas", jetstreamCfg.Replicas),
		slog.Duration("nats_js_max_age", jetstreamCfg.MaxAge),
		slog.Int64("nats_js_max_bytes", jetstreamCfg.MaxBytes),
	)
	if err := loop.Run(ctx); err != nil {
		log.Error("ingest worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("ingest worker stopped")
}

func loadIngestLoopConfigFromEnv() ingestworker.Config {
	workerID := strings.TrimSpace(os.Getenv("INGEST_WORKER_ID"))
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}
	pollInterval := runtimecfg.DurationPositive("INGEST_POLL_INTERVAL", 4*time.Second)
	pollJitter := runtimecfg.Float64NonNegative("INGEST_POLL_JITTER", 0.20)
	stopTimeout := runtimecfg.DurationPositive("INGEST_STOP_TIMEOUT", 8*time.Second)
	startWorkersDefault := ingestworker.RecommendedStartWorkers(runtime.GOMAXPROCS(0))
	startWorkers := runtimecfg.IntPositive("INGEST_START_WORKERS", startWorkersDefault)
	startQueueDefault := ingestworker.RecommendedStartQueueSize(startWorkers)
	startQueueSize := runtimecfg.IntPositive("INGEST_START_QUEUE_SIZE", startQueueDefault)
	leaseMissingAlertWindow := runtimecfg.DurationPositive("INGEST_LEASE_MISSING_ALERT_WINDOW", 5*time.Minute)
	leaseMissingAlertThreshold := runtimecfg.IntPositive("INGEST_LEASE_MISSING_ALERT_THRESHOLD", 4)
	leaseMissingAlertCooldown := runtimecfg.DurationPositive("INGEST_LEASE_MISSING_ALERT_COOLDOWN", 2*time.Minute)
	return ingestworker.Config{
		WorkerID:                   workerID,
		ProviderFilter:             controlplane.NormalizeProvider(strings.TrimSpace(os.Getenv("INGEST_PROVIDER"))),
		PollInterval:               pollInterval,
		PollJitter:                 pollJitter,
		StopTimeout:                stopTimeout,
		StartWorkers:               startWorkers,
		StartQueueSize:             startQueueSize,
		LeaseMissingAlertWindow:    leaseMissingAlertWindow,
		LeaseMissingAlertThreshold: leaseMissingAlertThreshold,
		LeaseMissingAlertCooldown:  leaseMissingAlertCooldown,
	}
}

func startAutoscaleMetricsServer(ctx context.Context, log *slog.Logger, registry *prometheus.Registry, listenAddr string) func() {
	if ctx == nil || log == nil || registry == nil || listenAddr == "" {
		return func() {}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn("ingest autoscale metrics server stopped", slog.String("error", err.Error()))
		}
	}()

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			log.Warn("shutdown ingest autoscale metrics server failed", slog.String("error", err.Error()))
		}
		<-done
	}
}
