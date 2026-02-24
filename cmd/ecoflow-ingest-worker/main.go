package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

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

	leaseMgr, err := ingestlease.NewManager(client, ingestlease.DefaultConfig())
	if err != nil {
		log.Error("init lease manager failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	natsCfg := telemetrybus.DefaultNATSConnConfig(splitNonEmpty(envOrDefault("NATS_URLS", "nats://127.0.0.1:4222")))
	natsCfg.Name = envOrDefault("NATS_NAME", "ecoflow-ingest-worker")
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

	ecoCfg := ecoflow.DefaultConfig()
	ecoCfg.Logging.Debug = false
	ecoCfg.Logging.AdvancedDebugTelemetry = false
	ecoCfg.Logging.DebugLogHeaders = false
	ecoCfg.Logging.Logger = log
	adapter := provideradapter.NewEcoFlowAdapter(provideradapter.NewDefaultEcoFlowClientFactory(ecoCfg))
	subjectCfg := telemetrybus.SubjectConfig{
		Prefix:     envOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: mustUint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}
	disableEnvelopeLabels := mustBool("INGEST_DISABLE_ENVELOPE_LABELS", false)

	publishOpts := telemetrybus.NATSEnvelopePublisherOptions{
		StripLabels:                disableEnvelopeLabels,
		UseJetStream:               mustBool("INGEST_NATS_USE_JETSTREAM", true),
		PublishTimeout:             mustDuration("INGEST_NATS_PUBLISH_TIMEOUT", 3*time.Second),
		PublishMaxRetries:          mustIntMin("INGEST_NATS_PUBLISH_MAX_RETRIES", 3, 0),
		PublishRetryInitialBackoff: mustDuration("INGEST_NATS_PUBLISH_RETRY_INITIAL_BACKOFF", 50*time.Millisecond),
		PublishRetryMaxBackoff:     mustDuration("INGEST_NATS_PUBLISH_RETRY_MAX_BACKOFF", 500*time.Millisecond),
		PublishRetryJitter:         mustFloat64("INGEST_NATS_PUBLISH_RETRY_JITTER", 0.20),
	}
	jetstreamCfg := telemetrybus.DefaultJetStreamIngestBootstrapConfig()
	jetstreamCfg.Enabled = mustBool("INGEST_NATS_JS_BOOTSTRAP_ENABLED", publishOpts.UseJetStream)
	jetstreamCfg.StreamName = envOrDefault("INGEST_NATS_JS_STREAM_NAME", jetstreamCfg.StreamName)
	jetstreamCfg.Replicas = mustIntMin("INGEST_NATS_JS_REPLICAS", jetstreamCfg.Replicas, 1)
	jetstreamCfg.MaxAge = mustDuration("INGEST_NATS_JS_MAX_AGE", jetstreamCfg.MaxAge)
	jetstreamCfg.MaxBytes = mustInt64Min("INGEST_NATS_JS_MAX_BYTES", jetstreamCfg.MaxBytes, 0)

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
	sessionCfg.KeepAlive = mustDuration("INGEST_MQTT_KEEPALIVE", sessionCfg.KeepAlive)
	sessionCfg.ConnectTimeout = mustDuration("INGEST_MQTT_CONNECT_TIMEOUT", sessionCfg.ConnectTimeout)
	sessionCfg.ReadTimeout = mustDuration("INGEST_MQTT_READ_TIMEOUT", sessionCfg.ReadTimeout)
	sessionCfg.WriteTimeout = mustDuration("INGEST_MQTT_WRITE_TIMEOUT", sessionCfg.WriteTimeout)
	sessionCfg.ReconnectInitialBackoff = mustDuration("INGEST_MQTT_RECONNECT_INITIAL_BACKOFF", sessionCfg.ReconnectInitialBackoff)
	sessionCfg.ReconnectMaxBackoff = mustDuration("INGEST_MQTT_RECONNECT_MAX_BACKOFF", sessionCfg.ReconnectMaxBackoff)
	sessionCfg.ReconnectJitter = mustFloat64("INGEST_MQTT_RECONNECT_JITTER", sessionCfg.ReconnectJitter)
	sessionCfg.PublishQueueSize = mustInt("INGEST_PUBLISH_QUEUE_SIZE", sessionCfg.PublishQueueSize)
	sessionCfg.PublishWorkers = mustInt("INGEST_PUBLISH_WORKERS", sessionCfg.PublishWorkers)
	sessionCfg.PublishEnqueueTimeout = mustDuration("INGEST_PUBLISH_ENQUEUE_TIMEOUT", sessionCfg.PublishEnqueueTimeout)
	sessionCfg.AllowUnorderedPublish = mustBool("INGEST_ALLOW_UNORDERED_PUBLISH", sessionCfg.AllowUnorderedPublish)
	sessionCfg.DisableEnvelopeLabels = disableEnvelopeLabels
	runner, err := ingestworker.NewEcoFlowSessionRunner(log, adapter, publisher, sessionCfg)
	if err != nil {
		log.Error("init session runner failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	workerID := strings.TrimSpace(os.Getenv("INGEST_WORKER_ID"))
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}
	pollInterval := mustDuration("INGEST_POLL_INTERVAL", 4*time.Second)
	pollJitter := mustFloat64("INGEST_POLL_JITTER", 0.20)
	stopTimeout := mustDuration("INGEST_STOP_TIMEOUT", 8*time.Second)
	startWorkersDefault := ingestworker.RecommendedStartWorkers(runtime.GOMAXPROCS(0))
	startWorkers := mustInt("INGEST_START_WORKERS", startWorkersDefault)
	startQueueDefault := ingestworker.RecommendedStartQueueSize(startWorkers)
	startQueueSize := mustInt("INGEST_START_QUEUE_SIZE", startQueueDefault)

	loop, err := ingestworker.NewLoop(log, store, leaseMgr, runner, ingestworker.Config{
		WorkerID:       workerID,
		ProviderFilter: controlplane.NormalizeProvider(strings.TrimSpace(os.Getenv("INGEST_PROVIDER"))),
		PollInterval:   pollInterval,
		PollJitter:     pollJitter,
		StopTimeout:    stopTimeout,
		StartWorkers:   startWorkers,
		StartQueueSize: startQueueSize,
	})
	if err != nil {
		log.Error("init ingest worker loop failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("ingest worker starting",
		slog.String("worker_id", workerID),
		slog.String("nats_urls", strings.Join(natsCfg.URLs, ",")),
		slog.String("subject_prefix", subjectCfg.Prefix),
		slog.Uint64("shards", uint64(subjectCfg.ShardCount)),
		slog.Duration("poll_interval", pollInterval),
		slog.Float64("poll_jitter", pollJitter),
		slog.Int("start_workers", startWorkers),
		slog.Int("start_queue_size", startQueueSize),
		slog.Int("publish_queue_size", sessionCfg.PublishQueueSize),
		slog.Int("publish_workers", sessionCfg.PublishWorkers),
		slog.Duration("publish_enqueue_timeout", sessionCfg.PublishEnqueueTimeout),
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

func mustInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
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
