package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	controlplanev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/controlplane/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcserver"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	env := os.Getenv("PULSE_ENV")
	if env == "" {
		env = "local"
	}

	logCfg := pulselog.DefaultServiceConfig("grpc-api")
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

	// Middleware chain (order matters):
	// request-id -> recovery -> auth -> logging
	unary := []grpc.UnaryServerInterceptor{
		grpcmw.RequestIDUnary(),
		grpcmw.RecoveryUnary(),
		grpcmw.AuthUnary(grpcmw.NoopAuthorizer{}), // TODO: replace with Keycloak/JWKS authorizer in M1
		grpcmw.LoggingUnary(log),
	}
	stream := []grpc.StreamServerInterceptor{
		grpcmw.RequestIDStream(),
		grpcmw.RecoveryStream(),
		grpcmw.AuthStream(grpcmw.NoopAuthorizer{}),
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

	snapshotReader, cleanupSnapshotReader, err := newTelemetrySnapshotReaderFromEnv(log)
	if err != nil {
		log.Error("telemetry snapshot reader init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupSnapshotReader()

	// Register services
	telemetryv1.RegisterTelemetryServiceServer(s, NewTelemetryServiceWithSnapshotReader(log, snapshotReader))
	controlPlaneStore, cleanupStore, err := newControlPlaneStoreFromEnv(log)
	if err != nil {
		log.Error("control-plane store init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupStore()
	ecoflowClientConfig := ecoflow.DefaultConfig()
	ecoflowClientConfig.Logging.Debug = false
	ecoflowClientConfig.Logging.AdvancedDebugTelemetry = false
	ecoflowClientConfig.Logging.DebugLogHeaders = false
	ecoflowClientConfig.Logging.Logger = log

	controlPlaneService := NewControlPlaneService(log, controlPlaneStore)
	controlPlaneService.RegisterDiscoverer(
		controlplane.ProviderEcoFlow,
		provideradapter.NewEcoFlowAdapter(
			provideradapter.NewDefaultEcoFlowClientFactory(ecoflowClientConfig),
		),
	)
	controlplanev1.RegisterControlPlaneServiceServer(s, controlPlaneService)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "grpc-api", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	log.Info("grpc server starting",
		"addr", cfg.ListenAddr,
		"env", env,
		"log_level", logCfg.Level.String(),
		"log_async_enabled", logCfg.AsyncEnabled,
		"log_async_queue_size", logCfg.AsyncQueueSize,
		"log_async_bypass_level", logCfg.AsyncBypassLevel.String(),
		"log_metrics_interval", logMetricsInterval,
	)

	if err := grpcserver.ServeWithSignal(ctx, s, lis, 15*time.Second); err != nil {
		log.Error("grpc server stopped", "error", err.Error())
		os.Exit(1)
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
	store, err := controlplane.NewPostgresStore(dsn)
	if err != nil {
		return nil, nil, err
	}
	log.Info("using postgres control-plane store", "source", "CONTROL_PLANE_DB_DSN")
	return store, func() { _ = store.Close() }, nil
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
