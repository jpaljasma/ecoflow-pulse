package main

import (
	"testing"
	"time"
)

func TestLoadInferenceWorkerConfigFromEnv(t *testing.T) {
	t.Setenv("TELEMETRY_SUBJECT_PREFIX", "pulse.inference")
	t.Setenv("TELEMETRY_SHARD_COUNT", "64")
	t.Setenv("INFERENCE_INGEST_STREAM_NAME", "INF_STREAM")
	t.Setenv("INFERENCE_CONSUMER_DURABLE", "inf-durable")
	t.Setenv("INFERENCE_QUEUE_GROUP", "inf-group")
	t.Setenv("INFERENCE_ACK_WAIT", "33s")
	t.Setenv("INFERENCE_MAX_ACK_PENDING", "88")
	t.Setenv("INFERENCE_PROCESS_TIMEOUT", "4s")
	t.Setenv("INFERENCE_DRAIN_TIMEOUT", "7s")

	cfg := loadInferenceWorkerConfigFromEnv()
	if cfg.SubjectConfig.Prefix != "pulse.inference" || cfg.SubjectConfig.ShardCount != 64 {
		t.Fatalf("subject config mismatch: %+v", cfg.SubjectConfig)
	}
	if cfg.StreamName != "INF_STREAM" || cfg.Durable != "inf-durable" || cfg.QueueGroup != "inf-group" {
		t.Fatalf("stream/durable/group mismatch: %+v", cfg)
	}
	if cfg.AckWait != 33*time.Second || cfg.MaxAckPending != 88 || cfg.ProcessTimeout != 4*time.Second || cfg.DrainTimeout != 7*time.Second {
		t.Fatalf("timing config mismatch: %+v", cfg)
	}
}
