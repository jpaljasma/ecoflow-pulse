package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
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

	ecoCfg := ecoflow.DefaultConfig()
	ecoCfg.Logging.Debug = false
	ecoCfg.Logging.AdvancedDebugTelemetry = false
	ecoCfg.Logging.DebugLogHeaders = false
	ecoCfg.Logging.Logger = log
	adapter := provideradapter.NewEcoFlowAdapter(provideradapter.NewDefaultEcoFlowClientFactory(ecoCfg))
	runner := ingestworker.NewEcoFlowSessionRunner(log, adapter)

	workerID := strings.TrimSpace(os.Getenv("INGEST_WORKER_ID"))
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}
	pollInterval := mustDuration("INGEST_POLL_INTERVAL", 4*time.Second)
	pollJitter := mustFloat64("INGEST_POLL_JITTER", 0.20)
	stopTimeout := mustDuration("INGEST_STOP_TIMEOUT", 8*time.Second)

	loop, err := ingestworker.NewLoop(log, store, leaseMgr, runner, ingestworker.Config{
		WorkerID:       workerID,
		ProviderFilter: controlplane.NormalizeProvider(strings.TrimSpace(os.Getenv("INGEST_PROVIDER"))),
		PollInterval:   pollInterval,
		PollJitter:     pollJitter,
		StopTimeout:    stopTimeout,
	})
	if err != nil {
		log.Error("init ingest worker loop failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("ingest worker starting",
		slog.String("worker_id", workerID),
		slog.Duration("poll_interval", pollInterval),
		slog.Float64("poll_jitter", pollJitter),
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
