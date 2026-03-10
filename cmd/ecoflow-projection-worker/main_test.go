package main

import (
	"testing"
	"time"
)

func TestLoadProjectionWorkerConfigFromEnv(t *testing.T) {
	t.Setenv("TELEMETRY_SUBJECT_PREFIX", "pulse.test")
	t.Setenv("TELEMETRY_SHARD_COUNT", "32")
	t.Setenv("PROJECTION_INGEST_STREAM_NAME", "TEST_STREAM")
	t.Setenv("PROJECTION_CONSUMER_DURABLE", "proj-durable")
	t.Setenv("PROJECTION_QUEUE_GROUP", "proj-group")
	t.Setenv("PROJECTION_ACK_WAIT", "45s")
	t.Setenv("PROJECTION_MAX_ACK_PENDING", "77")
	t.Setenv("PROJECTION_PROCESS_TIMEOUT", "5s")
	t.Setenv("PROJECTION_DRAIN_TIMEOUT", "9s")

	cfg := loadProjectionWorkerConfigFromEnv()
	if cfg.SubjectConfig.Prefix != "pulse.test" || cfg.SubjectConfig.ShardCount != 32 {
		t.Fatalf("subject config mismatch: %+v", cfg.SubjectConfig)
	}
	if cfg.StreamName != "TEST_STREAM" || cfg.Durable != "proj-durable" || cfg.QueueGroup != "proj-group" {
		t.Fatalf("stream/durable/group mismatch: %+v", cfg)
	}
	if cfg.AckWait != 45*time.Second || cfg.MaxAckPending != 77 || cfg.ProcessTimeout != 5*time.Second || cfg.DrainTimeout != 9*time.Second {
		t.Fatalf("timing config mismatch: %+v", cfg)
	}
}
