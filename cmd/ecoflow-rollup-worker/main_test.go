package main

import (
	"testing"
	"time"
)

func TestLoadRollupWorkerConfigFromEnv(t *testing.T) {
	t.Setenv("TELEMETRY_SUBJECT_PREFIX", "pulse.rollup")
	t.Setenv("TELEMETRY_SHARD_COUNT", "16")
	t.Setenv("ROLLUP_INGEST_STREAM_NAME", "ROLLUP_STREAM")
	t.Setenv("ROLLUP_CONSUMER_DURABLE", "rollup-durable")
	t.Setenv("ROLLUP_QUEUE_GROUP", "rollup-group")
	t.Setenv("ROLLUP_ACK_WAIT", "31s")
	t.Setenv("ROLLUP_MAX_ACK_PENDING", "99")
	t.Setenv("ROLLUP_PROCESS_TIMEOUT", "6s")
	t.Setenv("ROLLUP_DRAIN_TIMEOUT", "11s")

	cfg := loadRollupWorkerConfigFromEnv()
	if cfg.SubjectConfig.Prefix != "pulse.rollup" || cfg.SubjectConfig.ShardCount != 16 {
		t.Fatalf("subject config mismatch: %+v", cfg.SubjectConfig)
	}
	if cfg.StreamName != "ROLLUP_STREAM" || cfg.Durable != "rollup-durable" || cfg.QueueGroup != "rollup-group" {
		t.Fatalf("stream/durable/group mismatch: %+v", cfg)
	}
	if cfg.AckWait != 31*time.Second || cfg.MaxAckPending != 99 || cfg.ProcessTimeout != 6*time.Second || cfg.DrainTimeout != 11*time.Second {
		t.Fatalf("timing config mismatch: %+v", cfg)
	}
}
