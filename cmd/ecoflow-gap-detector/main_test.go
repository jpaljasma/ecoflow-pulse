package main

import (
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

func TestLoadGapDetectorConfigFromEnv(t *testing.T) {
	t.Setenv("GAP_REPAIR_PROVIDER", " EcoFlow ")
	t.Setenv("GAP_REPAIR_POLL_INTERVAL", "15s")
	t.Setenv("GAP_REPAIR_POLL_JITTER", "0.5")
	t.Setenv("GAP_REPAIR_LOOKBACK_WINDOW", "2h")
	t.Setenv("GAP_REPAIR_LAG_THRESHOLD", "3m")
	t.Setenv("GAP_REPAIR_LAG_ALERT_WINDOW", "20m")
	t.Setenv("GAP_REPAIR_LAG_ALERT_THRESHOLD", "7")
	t.Setenv("GAP_REPAIR_LAG_ALERT_COOLDOWN", "4m")
	t.Setenv("GAP_REPAIR_WINDOW_PADDING", "30s")
	t.Setenv("GAP_REPAIR_MAX_REPLAY_WINDOW", "1h")
	t.Setenv("GAP_REPAIR_SAFE_DELAY", "10s")
	t.Setenv("GAP_REPAIR_MAX_OBJECTS_PER_JOB", "9")
	t.Setenv("GAP_REPAIR_MAX_JOBS_PER_CYCLE", "5")
	t.Setenv("GAP_REPAIR_EVAL_WORKERS", "12")
	t.Setenv("GAP_REPAIR_DRY_RUN", "true")

	cfg := loadGapDetectorConfigFromEnv(telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 32})
	if cfg.ProviderFilter != "ecoflow" || cfg.PollInterval != 15*time.Second || cfg.LookbackWindow != 2*time.Hour {
		t.Fatalf("basic config mismatch: %+v", cfg)
	}
	if cfg.LagAlertThreshold != 7 || cfg.MaxObjectsPerJob != 9 || cfg.MaxJobsPerCycle != 5 || cfg.EvaluationWorkers != 12 || !cfg.DryRun {
		t.Fatalf("limit config mismatch: %+v", cfg)
	}
	if cfg.SubjectShardCount != 32 {
		t.Fatalf("subject shard count=%d want=32", cfg.SubjectShardCount)
	}
}
