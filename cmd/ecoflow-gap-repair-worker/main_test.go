package main

import (
	"testing"
	"time"
)

func TestLoadGapRepairWorkerConfigFromEnv(t *testing.T) {
	t.Setenv("GAP_REPAIR_STREAM_NAME", "GAP_STREAM")
	t.Setenv("GAP_REPAIR_CONSUMER_DURABLE", "gap-durable")
	t.Setenv("GAP_REPAIR_QUEUE_GROUP", "gap-group")
	t.Setenv("GAP_REPAIR_ACK_WAIT", "90s")
	t.Setenv("GAP_REPAIR_MAX_ACK_PENDING", "55")
	t.Setenv("GAP_REPAIR_PROCESS_TIMEOUT", "44s")
	t.Setenv("GAP_REPAIR_DRAIN_TIMEOUT", "12s")
	t.Setenv("GAP_REPAIR_DEFAULT_MAX_OBJECTS", "13")
	t.Setenv("GAP_REPAIR_REPLAY_FAILURE_ALERT_WINDOW", "12m")
	t.Setenv("GAP_REPAIR_REPLAY_FAILURE_ALERT_THRESHOLD", "9")
	t.Setenv("GAP_REPAIR_REPLAY_FAILURE_ALERT_COOLDOWN", "6m")

	cfg := loadGapRepairWorkerConfigFromEnv()
	if cfg.StreamName != "GAP_STREAM" || cfg.Durable != "gap-durable" || cfg.QueueGroup != "gap-group" {
		t.Fatalf("stream config mismatch: %+v", cfg)
	}
	if cfg.AckWait != 90*time.Second || cfg.MaxAckPending != 55 || cfg.ProcessTimeout != 44*time.Second || cfg.DrainTimeout != 12*time.Second {
		t.Fatalf("timing config mismatch: %+v", cfg)
	}
	if cfg.DefaultMaxObjects != 13 || cfg.ReplayFailureAlertThreshold != 9 || cfg.ReplayFailureAlertWindow != 12*time.Minute || cfg.ReplayFailureAlertCooldown != 6*time.Minute {
		t.Fatalf("alert/max objects mismatch: %+v", cfg)
	}
}
