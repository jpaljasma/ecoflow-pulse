package main

import (
	"strings"
	"testing"
	"time"
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
