package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/ingestworker"
	"github.com/jpaljasma/ecoflow-pulse/pkg/pecron"
	"github.com/prometheus/client_golang/prometheus"
)

func TestLoadIngestLoopConfigFromEnv(t *testing.T) {
	t.Setenv("INGEST_WORKER_ID", "worker-test")
	t.Setenv("INGEST_PROVIDER", " EcoFlow ")
	t.Setenv("INGEST_POLL_INTERVAL", "9s")
	t.Setenv("INGEST_POLL_JITTER", "0.4")
	t.Setenv("INGEST_STOP_TIMEOUT", "12s")
	t.Setenv("INGEST_START_WORKERS", "17")
	t.Setenv("INGEST_START_QUEUE_SIZE", "99")
	t.Setenv("INGEST_LEASE_MISSING_ALERT_WINDOW", "6m")
	t.Setenv("INGEST_LEASE_MISSING_ALERT_THRESHOLD", "7")
	t.Setenv("INGEST_LEASE_MISSING_ALERT_COOLDOWN", "3m")

	cfg := loadIngestLoopConfigFromEnv()
	if cfg.WorkerID != "worker-test" || cfg.ProviderFilter != "ecoflow" {
		t.Fatalf("worker/provider mismatch: %+v", cfg)
	}
	if cfg.PollInterval != 9*time.Second || cfg.PollJitter != 0.4 || cfg.StopTimeout != 12*time.Second {
		t.Fatalf("timing config mismatch: %+v", cfg)
	}
	if cfg.StartWorkers != 17 || cfg.StartQueueSize != 99 || cfg.LeaseMissingAlertThreshold != 7 || cfg.LeaseMissingAlertWindow != 6*time.Minute || cfg.LeaseMissingAlertCooldown != 3*time.Minute {
		t.Fatalf("worker sizing mismatch: %+v", cfg)
	}
}

func TestLoadIngestLoopConfigFromEnvGeneratesWorkerID(t *testing.T) {
	cfg := loadIngestLoopConfigFromEnv()
	if strings.TrimSpace(cfg.WorkerID) == "" {
		t.Fatal("expected generated worker id")
	}
}

func TestLoadPecronSessionConfigUsesProviderSpecificRESTCadence(t *testing.T) {
	base := ingestworker.DefaultEcoFlowSessionConfig()
	base.QuotaRefreshInterval = 30 * time.Second
	base.QuotaRefreshJitter = 0.4

	cfg := loadPecronSessionConfigFromEnv(base)
	if cfg.SnapshotRefreshInterval != pecron.RecommendedCloudRESTPollInterval {
		t.Fatalf("snapshot refresh interval = %s, want %s", cfg.SnapshotRefreshInterval, pecron.RecommendedCloudRESTPollInterval)
	}
	if cfg.SnapshotRefreshJitter != 0.20 {
		t.Fatalf("snapshot refresh jitter = %v, want provider default 0.20", cfg.SnapshotRefreshJitter)
	}
}

func TestLoadPecronSessionConfigAllowsExplicitSafeRESTCadence(t *testing.T) {
	t.Setenv("INGEST_PECRON_SNAPSHOT_REFRESH_INTERVAL", "90s")
	t.Setenv("INGEST_PECRON_SNAPSHOT_REFRESH_JITTER", "0.1")

	cfg := loadPecronSessionConfigFromEnv(ingestworker.DefaultEcoFlowSessionConfig())
	if cfg.SnapshotRefreshInterval != 90*time.Second {
		t.Fatalf("snapshot refresh interval = %s, want 90s", cfg.SnapshotRefreshInterval)
	}
	if cfg.SnapshotRefreshJitter != 0.1 {
		t.Fatalf("snapshot refresh jitter = %v, want 0.1", cfg.SnapshotRefreshJitter)
	}
}

func TestStartAutoscaleMetricsServerDrainEndpointMarksReadyFalse(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := startAutoscaleMetricsServer(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), prometheus.NewRegistry(), "127.0.0.1:19114")
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:19114/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:19114/drainz", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /drainz error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /drainz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Get("http://127.0.0.1:19114/readyz")
	if err != nil {
		t.Fatalf("GET /readyz after drain error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz after drain status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}
