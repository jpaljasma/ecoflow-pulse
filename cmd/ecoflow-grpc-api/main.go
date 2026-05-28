package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	edgev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/edge/v1"
	inferencev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/inference/v1"
	solarforecastv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/solarforecast/v1"
	weatherv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/weather/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/inference"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcserver"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

type grpcServiceMode string

const (
	grpcServiceModeTelemetry grpcServiceMode = "telemetry"
	grpcServiceModeEnergy    grpcServiceMode = "energy"
)

func main() {
	env := os.Getenv("PULSE_ENV")
	if env == "" {
		env = "local"
	}

	serviceMode, err := grpcServiceModeFromEnv()
	if err != nil {
		_, _ = os.Stderr.WriteString("invalid grpc service mode: " + err.Error() + "\n")
		os.Exit(1)
	}

	serviceName := "grpc-api"
	if serviceMode == grpcServiceModeEnergy {
		serviceName = "energy-api"
	}
	logCfg := pulselog.DefaultServiceConfig(serviceName)
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

	cfg := grpcserver.DefaultConfig(env)
	if addr := os.Getenv("GRPC_LISTEN_ADDR"); addr != "" {
		cfg.ListenAddr = addr
	}

	authorizer, err := newAuthorizerFromEnv(context.Background(), log)
	if err != nil {
		log.Error("grpc auth init failed", "error", err.Error())
		os.Exit(1)
	}
	grpcMetrics := grpcmw.NewMetrics()

	// Middleware chain (order matters):
	// request-id -> recovery -> auth -> metrics -> logging
	unary := []grpc.UnaryServerInterceptor{
		grpcmw.RequestIDUnary(),
		grpcmw.RecoveryUnary(),
		grpcmw.AuthUnary(authorizer),
		grpcMetrics.UnaryServerInterceptor(),
		grpcmw.LoggingUnary(log),
	}
	stream := []grpc.StreamServerInterceptor{
		grpcmw.RequestIDStream(),
		grpcmw.RecoveryStream(),
		grpcmw.AuthStream(authorizer),
		grpcMetrics.StreamServerInterceptor(),
		grpcmw.LoggingStream(log),
	}

	s, lis, err := grpcserver.New(cfg, unary, stream)
	if err != nil {
		log.Error("grpc listen failed", "error", err.Error())
		os.Exit(1)
	}

	// Health
	hs := health.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	controlPlaneStore, cleanupStore, err := newControlPlaneStoreFromEnv(log)
	if err != nil {
		log.Error("control-plane store init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupStore()
	weatherDomain, cleanupWeatherDomain, err := newWeatherDomainFromEnv(log, grpcMetrics.Registry())
	if err != nil {
		log.Error("weather domain init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupWeatherDomain()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	switch serviceMode {
	case grpcServiceModeTelemetry:
		snapshotReader, cleanupSnapshotReader, snapshotErr := newTelemetrySnapshotReaderFromEnv(log)
		if snapshotErr != nil {
			log.Error("telemetry snapshot reader init failed", "error", snapshotErr.Error())
			os.Exit(1)
		}
		defer cleanupSnapshotReader()

		inferenceReader, cleanupInferenceReader, inferenceErr := newInferenceReaderFromEnv(log)
		if inferenceErr != nil {
			log.Error("inference reader init failed", "error", inferenceErr.Error())
			os.Exit(1)
		}
		defer cleanupInferenceReader()

		inferenceQueryReader, cleanupInferenceQueryReader, inferenceQueryErr := newTelemetryQueryReaderFromEnv(log)
		if inferenceQueryErr != nil {
			log.Error("inference telemetry query reader init failed", "error", inferenceQueryErr.Error())
			os.Exit(1)
		}
		defer cleanupInferenceQueryReader()
		solarQueryReader, cleanupSolarQueryReader, solarQueryErr := newTelemetryQueryReaderFromEnv(log)
		if solarQueryErr != nil {
			log.Error("solar telemetry query reader init failed", "error", solarQueryErr.Error())
			os.Exit(1)
		}
		defer cleanupSolarQueryReader()
		solarDomain, cleanupSolarDomain, solarDomainErr := newSolarForecastDomainFromEnv(log, grpcMetrics.Registry(), weatherDomain, solarQueryReader)
		if solarDomainErr != nil {
			log.Error("solar forecast domain init failed", "error", solarDomainErr.Error())
			os.Exit(1)
		}
		defer cleanupSolarDomain()

		telemetryv1.RegisterTelemetryServiceServer(s, NewTelemetryServiceWithDeps(TelemetryServiceDeps{
			Log:               log,
			SnapshotReader:    snapshotReader,
			ControlPlaneStore: controlPlaneStore,
		}))
		adapterRegistry := provideradapter.NewRegistry()
		ecoFlowAdapter, adapterErr := provideradapter.NewRuntimeEcoFlowAdapter(log)
		if adapterErr != nil {
			log.Error("init ecoflow adapter failed", "error", adapterErr.Error())
			os.Exit(1)
		}
		pulseMQTTAdapter, pulseMQTTErr := provideradapter.NewRuntimePulseMQTTAdapter(log)
		if pulseMQTTErr != nil && !errors.Is(pulseMQTTErr, provideradapter.ErrPulseMQTTDisabled) {
			log.Error("init pulse mqtt adapter failed", "error", pulseMQTTErr.Error())
			os.Exit(1)
		}
		if pulseMQTTErr != nil {
			log.Info("pulse mqtt adapter disabled", "reason", pulseMQTTErr.Error())
		}
		pecronAdapter, pecronErr := provideradapter.NewRuntimePecronAdapter()
		if pecronErr != nil {
			log.Error("init pecron adapter failed", "error", pecronErr.Error())
			os.Exit(1)
		}
		ankerSolixAdapter, ankerSolixErr := provideradapter.NewRuntimeAnkerSolixAdapter()
		if ankerSolixErr != nil {
			log.Error("init anker solix adapter failed", "error", ankerSolixErr.Error())
			os.Exit(1)
		}
		registerControlPlaneDiscoverers(adapterRegistry, ecoFlowAdapter, pecronAdapter, ankerSolixAdapter, pulseMQTTAdapter)
		controlPlaneService := NewControlPlaneService(log, controlPlaneStore, adapterRegistry)
		controlplanev1.RegisterControlPlaneServiceServer(s, controlPlaneService)
		edgePublisher, cleanupEdgePublisher, edgeSubjectCfg, edgePublisherErr := newEdgeIngestPublisherFromEnv(log)
		if edgePublisherErr != nil {
			log.Error("edge ingest publisher init failed", "error", edgePublisherErr.Error())
			os.Exit(1)
		}
		defer cleanupEdgePublisher()
		edgeStore, ok := controlPlaneStore.(edgeControlStore)
		if !ok {
			log.Error("edge ingest store init failed", "error", "control-plane store does not implement edge collector state")
			os.Exit(1)
		}
		edgev1.RegisterEdgeIngestServiceServer(s, NewEdgeIngestService(EdgeIngestServiceDeps{
			Log:        log,
			Store:      edgeStore,
			Publisher:  edgePublisher,
			SubjectCfg: edgeSubjectCfg,
		}))
		weatherv1.RegisterWeatherServiceServer(s, NewWeatherServiceWithDeps(WeatherServiceDeps{
			Log:     log,
			Service: weatherDomain,
		}))
		solarforecastv1.RegisterSolarForecastServiceServer(s, NewSolarForecastServiceWithDeps(SolarForecastServiceDeps{
			Log:               log,
			Service:           solarDomain,
			ControlPlaneStore: controlPlaneStore,
		}))
		var comparisonCache inference.EnergyComparisonCache
		if cache, ok := inferenceReader.(inference.EnergyComparisonCache); ok {
			comparisonCache = cache
		}
		inferencev1.RegisterInferenceServiceServer(s, NewInferenceServiceWithDeps(InferenceServiceDeps{
			Log:               log,
			Reader:            inferenceReader,
			ComparisonCache:   comparisonCache,
			QueryReader:       inferenceQueryReader,
			ControlPlaneStore: controlPlaneStore,
		}))
	case grpcServiceModeEnergy:
		queryReader, cleanupQueryReader, queryErr := newTelemetryQueryReaderFromEnv(log)
		if queryErr != nil {
			log.Error("telemetry query reader init failed", "error", queryErr.Error())
			os.Exit(1)
		}
		defer cleanupQueryReader()

		archiveManifestStore, archiveObjectReader, cleanupArchiveReaders, archiveErr := newArchiveReadersFromEnv(log)
		if archiveErr != nil {
			log.Error("archive history reader init failed", "error", archiveErr.Error())
			os.Exit(1)
		}
		defer cleanupArchiveReaders()
		energyCalendarCache, pvPortHistoryCache, cleanupEnergyCaches, energyCacheErr := newEnergyValkeyCachesFromEnv(log)
		if energyCacheErr != nil {
			log.Error("energy valkey cache init failed", "error", energyCacheErr.Error())
			os.Exit(1)
		}
		defer cleanupEnergyCaches()

		telemetryv1.RegisterEnergyServiceServer(s, NewEnergyServiceWithDeps(EnergyServiceDeps{
			Log:                  log,
			QueryReader:          queryReader,
			ControlPlaneStore:    controlPlaneStore,
			ArchiveManifestStore: archiveManifestStore,
			ArchiveObjectReader:  archiveObjectReader,
			HistoryGzipMinBytes:  runtimecfg.IntMin("GRPC_HISTORY_GZIP_MIN_BYTES", defaultHistoryGzipMinBytes, 0),
			EnergyCalendarCache:  energyCalendarCache,
			PVPortHistoryCache:   pvPortHistoryCache,
		}))
	}

	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	metricsListenAddr := strings.TrimSpace(os.Getenv("GRPC_METRICS_LISTEN_ADDR"))
	pprofListenAddr := strings.TrimSpace(os.Getenv("GRPC_PPROF_LISTEN_ADDR"))
	stopPprofServer, pprofListenSource, err := newPprofServerFromEnv(log)
	if err != nil {
		log.Error("grpc pprof init failed", "error", err.Error())
		os.Exit(1)
	}
	defer stopPprofServer()
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, serviceName, asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()
	stopGRPCMetrics := workermetrics.StartServer(ctx, log, grpcMetrics.Registry(), metricsListenAddr)
	defer stopGRPCMetrics()
	stopWeatherRefresh := startWeatherRefreshLoop(ctx, log, weatherDomain)
	defer stopWeatherRefresh()
	stopWeatherMetrics := startWeatherMetricsLoop(ctx, log, weatherDomain)
	defer stopWeatherMetrics()

	log.Info("grpc server starting",
		"addr", cfg.ListenAddr,
		"env", env,
		"log_level", logCfg.Level.String(),
		"log_async_enabled", logCfg.AsyncEnabled,
		"log_async_queue_size", logCfg.AsyncQueueSize,
		"log_async_bypass_level", logCfg.AsyncBypassLevel.String(),
		"log_metrics_interval", logMetricsInterval,
		"metrics_listen_addr", metricsListenAddr,
		"pprof_listen_addr", pprofListenAddr,
		"pprof_source", pprofListenSource,
		"service_mode", serviceMode,
	)

	if err := grpcserver.ServeWithSignal(ctx, s, lis, 15*time.Second); err != nil {
		log.Error("grpc server stopped", "error", err.Error())
		os.Exit(1)
	}
}

func grpcServiceModeFromEnv() (grpcServiceMode, error) {
	switch mode := grpcServiceMode(strings.ToLower(strings.TrimSpace(runtimecfg.EnvOrDefault("GRPC_SERVICE_MODE", string(grpcServiceModeTelemetry))))); mode {
	case grpcServiceModeTelemetry, grpcServiceModeEnergy:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported GRPC_SERVICE_MODE %q", mode)
	}
}

func registerControlPlaneDiscoverers(
	registry *provideradapter.Registry,
	ecoFlow provideradapter.Discoverer,
	pecron provideradapter.Discoverer,
	ankerSolix provideradapter.Discoverer,
	pulseMQTT provideradapter.Discoverer,
) {
	if registry == nil {
		return
	}
	registry.RegisterDiscoverer(controlplane.ProviderEcoFlow, ecoFlow)
	registry.RegisterDiscoverer(controlplane.ProviderPecron, pecron)
	registry.RegisterDiscoverer(controlplane.ProviderAnkerSolix, ankerSolix)
	if pulseMQTT != nil {
		registry.RegisterDiscoverer(controlplane.ProviderPulseMQTT, pulseMQTT)
	}
}

func newControlPlaneStoreFromEnv(log *slog.Logger) (controlplane.Store, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Info("using in-memory control-plane store", "source", "fallback")
		store := controlplane.NewMemoryStore()
		store.EnsureUser("dev-user")
		return store, func() {}, nil
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, nil, err
	}
	store, err := controlplane.NewPostgresStore(dsn)
	if err != nil {
		return nil, nil, err
	}
	log.Info("using postgres control-plane store", "source", "CONTROL_PLANE_DB_DSN")
	return store, func() { _ = store.Close() }, nil
}

func newAuthorizerFromEnv(ctx context.Context, log *slog.Logger) (grpcmw.Authorizer, error) {
	mode := strings.ToLower(strings.TrimSpace(runtimecfg.EnvOrDefault("GRPC_AUTH_MODE", "noop")))
	switch mode {
	case "noop":
		log.Warn("grpc auth mode: noop (development only)")
		return grpcmw.NoopAuthorizer{}, nil
	case "keycloak":
		issuer := strings.TrimSpace(os.Getenv("KEYCLOAK_ISSUER_URL"))
		if issuer == "" {
			return nil, fmt.Errorf("KEYCLOAK_ISSUER_URL is required when GRPC_AUTH_MODE=keycloak")
		}
		audience := strings.TrimSpace(os.Getenv("KEYCLOAK_AUDIENCE"))
		jwksURL := strings.TrimSpace(os.Getenv("KEYCLOAK_JWKS_URL"))
		allowMissingJWT := runtimecfg.Bool("GRPC_AUTH_ALLOW_MISSING_JWT", false)
		authorizer, err := grpcmw.NewKeycloakJWKSAuthorizer(ctx, grpcmw.KeycloakJWKSAuthorizerConfig{
			IssuerURL:       issuer,
			Audience:        audience,
			JWKSURL:         jwksURL,
			AllowMissingJWT: allowMissingJWT,
		})
		if err != nil {
			return nil, err
		}
		log.Info("grpc auth mode: keycloak", "issuer", issuer, "audience", audience, "allow_missing_jwt", allowMissingJWT)
		return authorizer, nil
	default:
		return nil, fmt.Errorf("unsupported GRPC_AUTH_MODE %q", mode)
	}
}

func newEdgeIngestPublisherFromEnv(log *slog.Logger) (telemetrybus.EnvelopePublisher, func(), telemetrybus.SubjectConfig, error) {
	subjectCfg := telemetrybus.SubjectConfig{
		Prefix:     runtimecfg.EnvOrDefault("TELEMETRY_SUBJECT_PREFIX", telemetrybus.DefaultSubjectPrefix),
		ShardCount: runtimecfg.Uint32("TELEMETRY_SHARD_COUNT", telemetrybus.DefaultShardCount),
	}.Normalized()
	natsURLs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("NATS_URLS")))
	if len(natsURLs) == 0 {
		log.Info("edge ingest publisher disabled", "reason", "NATS_URLS not set")
		return nil, func() {}, subjectCfg, nil
	}
	natsCfg := telemetrybus.DefaultNATSConnConfig(natsURLs)
	natsCfg.Name = runtimecfg.EnvOrDefault("NATS_NAME", "ecoflow-grpc-api-edge-ingest")
	natsCfg.ConnectTimeout = runtimecfg.DurationPositive("NATS_CONNECT_TIMEOUT", natsCfg.ConnectTimeout)
	natsCfg.ReconnectWait = runtimecfg.DurationPositive("NATS_RECONNECT_WAIT", natsCfg.ReconnectWait)
	natsCfg.ReconnectJitter = runtimecfg.DurationPositive("NATS_RECONNECT_JITTER", natsCfg.ReconnectJitter)
	natsCfg.PingInterval = runtimecfg.DurationPositive("NATS_PING_INTERVAL", natsCfg.PingInterval)
	natsCfg.MaxPingsOut = runtimecfg.IntMin("NATS_MAX_PINGS_OUT", natsCfg.MaxPingsOut, 1)
	natsCfg.MaxReconnects = runtimecfg.IntMin("NATS_MAX_RECONNECTS", natsCfg.MaxReconnects, -1)

	natsConn, err := telemetrybus.DialNATS(log, natsCfg)
	if err != nil {
		return nil, nil, subjectCfg, err
	}
	publishOpts := telemetrybus.NATSEnvelopePublisherOptions{
		StripLabels:                runtimecfg.Bool("INGEST_DISABLE_ENVELOPE_LABELS", false),
		UseJetStream:               runtimecfg.Bool("EDGE_INGEST_NATS_USE_JETSTREAM", true),
		PublishTimeout:             runtimecfg.DurationPositive("EDGE_INGEST_NATS_PUBLISH_TIMEOUT", 3*time.Second),
		PublishMaxRetries:          runtimecfg.IntMin("EDGE_INGEST_NATS_PUBLISH_MAX_RETRIES", 3, 0),
		PublishRetryInitialBackoff: runtimecfg.DurationPositive("EDGE_INGEST_NATS_PUBLISH_RETRY_INITIAL_BACKOFF", 50*time.Millisecond),
		PublishRetryMaxBackoff:     runtimecfg.DurationPositive("EDGE_INGEST_NATS_PUBLISH_RETRY_MAX_BACKOFF", 500*time.Millisecond),
		PublishRetryJitter:         runtimecfg.Float64NonNegative("EDGE_INGEST_NATS_PUBLISH_RETRY_JITTER", 0.20),
	}
	jetstreamCfg := telemetrybus.DefaultJetStreamIngestBootstrapConfig()
	jetstreamCfg.Enabled = runtimecfg.Bool("EDGE_INGEST_NATS_JS_BOOTSTRAP_ENABLED", publishOpts.UseJetStream)
	jetstreamCfg.StreamName = runtimecfg.EnvOrDefault("INGEST_NATS_JS_STREAM_NAME", jetstreamCfg.StreamName)
	jetstreamCfg.Replicas = runtimecfg.IntMin("INGEST_NATS_JS_REPLICAS", jetstreamCfg.Replicas, 1)
	jetstreamCfg.MaxAge = runtimecfg.DurationPositive("INGEST_NATS_JS_MAX_AGE", jetstreamCfg.MaxAge)
	jetstreamCfg.MaxBytes = runtimecfg.Int64Min("INGEST_NATS_JS_MAX_BYTES", jetstreamCfg.MaxBytes, 0)
	if jetstreamCfg.Enabled {
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := telemetrybus.EnsureJetStreamIngestStream(bootstrapCtx, natsConn, subjectCfg, jetstreamCfg); err != nil {
			cancel()
			natsConn.Close()
			return nil, nil, subjectCfg, err
		}
		cancel()
	}
	publisher, err := telemetrybus.NewNATSEnvelopePublisherWithOptions(natsConn, subjectCfg, publishOpts)
	if err != nil {
		natsConn.Close()
		return nil, nil, subjectCfg, err
	}
	cleanup := func() {
		if closeErr := publisher.Close(); closeErr != nil {
			log.Warn("close edge ingest publisher failed", slog.String("error", closeErr.Error()))
		}
	}
	log.Info("edge ingest publisher enabled", "shards", subjectCfg.ShardCount)
	return publisher, cleanup, subjectCfg, nil
}

func newTelemetrySnapshotReaderFromEnv(log *slog.Logger) (projectionworker.SnapshotReader, func(), error) {
	valkeyAddrs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("VALKEY_ADDRS")))
	if len(valkeyAddrs) == 0 {
		log.Info("telemetry snapshot reader disabled", "reason", "VALKEY_ADDRS not set")
		return nil, func() {}, nil
	}
	cfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	cfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	cfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureSentinelFromEnv(&cfg)

	client, err := ingestlease.NewValkeyClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	store, err := projectionworker.NewValkeySnapshotStore(client, projectionworker.ValkeySnapshotStoreConfig{
		KeyPrefix: strings.TrimSpace(runtimecfg.EnvOrDefault("PROJECTION_KEY_PREFIX", "pulse:projection")),
	})
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	log.Info("telemetry snapshot reader enabled",
		"source", "valkey",
		"valkey_addrs", strings.Join(valkeyAddrs, ","),
		"key_prefix", strings.TrimSpace(runtimecfg.EnvOrDefault("PROJECTION_KEY_PREFIX", "pulse:projection")),
	)
	return store, func() { client.Close() }, nil
}

func newEnergyValkeyCachesFromEnv(log *slog.Logger) (*valkeycache.Client, *valkeycache.Client, func(), error) {
	valkeyAddrs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("VALKEY_ADDRS")))
	if len(valkeyAddrs) == 0 {
		log.Info("energy valkey caches disabled", "reason", "VALKEY_ADDRS not set")
		return nil, nil, func() {}, nil
	}
	cfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	cfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	cfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureClientSideCacheFromEnv(&cfg)
	ingestlease.ConfigureSentinelFromEnv(&cfg)

	client, err := ingestlease.NewValkeyClient(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	prefix, namespace := valkeycache.SplitKeyPrefix(runtimecfg.EnvOrDefault("ENERGY_CACHE_KEY_PREFIX", "pulse:energy"), "pulse", "energy")
	calendar, err := valkeycache.New(client, valkeycache.Options{
		Prefix:          prefix,
		Namespace:       namespace + "-calendar",
		ContentType:     "application/protobuf",
		DefaultLocalTTL: runtimecfg.DurationNonNegative("ENERGY_CALENDAR_CACHE_LOCAL_TTL", 5*time.Second),
		Now:             time.Now,
	})
	if err != nil {
		client.Close()
		return nil, nil, nil, err
	}
	pvPortHistory, err := valkeycache.New(client, valkeycache.Options{
		Prefix:          prefix,
		Namespace:       namespace + "-pv-port-history",
		ContentType:     "application/json",
		DefaultLocalTTL: runtimecfg.DurationNonNegative("ENERGY_PV_PORT_HISTORY_CACHE_LOCAL_TTL", 5*time.Second),
		Now:             time.Now,
	})
	if err != nil {
		client.Close()
		return nil, nil, nil, err
	}
	log.Info("energy valkey caches enabled",
		"valkey_addrs", strings.Join(valkeyAddrs, ","),
		"key_prefix", runtimecfg.EnvOrDefault("ENERGY_CACHE_KEY_PREFIX", "pulse:energy"),
		"client_side_cache_enabled", cfg.ClientSideCacheEnabled,
	)
	return calendar, pvPortHistory, func() { client.Close() }, nil
}

func newTelemetryQueryReaderFromEnv(log *slog.Logger) (telemetryquery.Reader, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Info("telemetry query reader disabled", "reason", "CONTROL_PLANE_DB_DSN not set")
		return nil, func() {}, nil
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, nil, err
	}

	reader, err := telemetryquery.NewPostgresReader(dsn)
	if err != nil {
		return nil, nil, err
	}

	log.Info("telemetry query reader enabled", "source", "postgres")
	return reader, func() { _ = reader.Close() }, nil
}

func newArchiveReadersFromEnv(log *slog.Logger) (replaycli.ManifestStore, replaycli.ObjectReader, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	objectCfg := replaycli.DefaultObjectReaderConfig()
	objectCfg.Provider = replaycli.ObjectProvider(runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PROVIDER", string(objectCfg.Provider)))
	objectCfg.Endpoint = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectCfg.Endpoint)
	objectCfg.AccessKeyID = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectCfg.AccessKeyID)
	objectCfg.SecretAccessKey = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectCfg.SecretAccessKey)
	objectCfg.Region = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", objectCfg.Region)
	objectCfg.Secure = runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", objectCfg.Secure)
	objectCfg.GCSProjectID = runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_GCS_PROJECT_ID", objectCfg.GCSProjectID)
	if dsn == "" || (objectCfg.Provider == replaycli.ObjectProviderMinIO &&
		(strings.TrimSpace(objectCfg.Endpoint) == "" || strings.TrimSpace(objectCfg.AccessKeyID) == "" || strings.TrimSpace(objectCfg.SecretAccessKey) == "")) {
		log.Info("archive history readers disabled", "reason", "archive or postgres env not fully configured")
		return nil, nil, func() {}, nil
	}

	manifestStore, err := replaycli.NewPostgresManifestStore(dsn)
	if err != nil {
		return nil, nil, nil, err
	}
	objectReader, err := replaycli.NewObjectReader(objectCfg)
	if err != nil {
		_ = manifestStore.Close()
		return nil, nil, nil, err
	}
	log.Info("archive history readers enabled",
		"source", "manifest+object-store",
		"provider", string(objectCfg.Provider),
		"endpoint", objectCfg.Endpoint,
	)
	return manifestStore, objectReader, func() {
		_ = objectReader.Close()
		_ = manifestStore.Close()
	}, nil
}

func newInferenceReaderFromEnv(log *slog.Logger) (inference.Reader, func(), error) {
	valkeyAddrs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("VALKEY_ADDRS")))
	if len(valkeyAddrs) == 0 {
		log.Info("inference reader disabled", "reason", "VALKEY_ADDRS not set")
		return nil, func() {}, nil
	}
	cfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	cfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	cfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureClientSideCacheFromEnv(&cfg)
	ingestlease.ConfigureSentinelFromEnv(&cfg)

	client, err := ingestlease.NewValkeyClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	store, err := inference.NewValkeyStore(client, inference.ValkeyStoreConfig{
		KeyPrefix: strings.TrimSpace(runtimecfg.EnvOrDefault("INFERENCE_KEY_PREFIX", "pulse:inference")),
	})
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	log.Info("inference reader enabled",
		"source", "valkey",
		"valkey_addrs", strings.Join(valkeyAddrs, ","),
		"key_prefix", strings.TrimSpace(runtimecfg.EnvOrDefault("INFERENCE_KEY_PREFIX", "pulse:inference")),
	)
	return store, func() { client.Close() }, nil
}
