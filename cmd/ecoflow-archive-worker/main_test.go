package main

import (
	"testing"
	"time"
)

func TestLoadArchiveWorkerConfigFromEnvAndManifestFallback(t *testing.T) {
	t.Setenv("TELEMETRY_SUBJECT_PREFIX", "pulse.archive")
	t.Setenv("TELEMETRY_SHARD_COUNT", "8")
	t.Setenv("ARCHIVE_INGEST_STREAM_NAME", "ARCHIVE_STREAM")
	t.Setenv("ARCHIVE_CONSUMER_DURABLE", "archive-durable")
	t.Setenv("ARCHIVE_QUEUE_GROUP", "archive-group")
	t.Setenv("ARCHIVE_ACK_WAIT", "75s")
	t.Setenv("ARCHIVE_MAX_ACK_PENDING", "123")
	t.Setenv("ARCHIVE_PROCESS_TIMEOUT", "9s")
	t.Setenv("ARCHIVE_DRAIN_TIMEOUT", "15s")
	t.Setenv("ARCHIVE_FAILURE_ALERT_WINDOW", "11m")
	t.Setenv("ARCHIVE_FAILURE_ALERT_THRESHOLD", "8")
	t.Setenv("ARCHIVE_FAILURE_ALERT_COOLDOWN", "6m")
	t.Setenv("ARCHIVE_FLUSH_INTERVAL", "45s")
	t.Setenv("ARCHIVE_FLUSH_TIMEOUT", "14s")
	t.Setenv("ARCHIVE_MAX_RECORDS_PER_PART", "77")
	t.Setenv("ARCHIVE_MAX_BYTES_PER_PART", "8192")
	t.Setenv("ARCHIVE_OBJECT_BUCKET", "bucket-a")
	t.Setenv("ARCHIVE_OBJECT_PREFIX", "raw-a")
	t.Setenv("ARCHIVE_WRITER_ID", "writer-a")
	t.Setenv("ARCHIVE_ZSTD_LEVEL", "5")
	t.Setenv("CONTROL_PLANE_DB_DSN", "postgres://control")

	cfg := loadArchiveWorkerConfigFromEnv()
	if cfg.SubjectConfig.Prefix != "pulse.archive" || cfg.SubjectConfig.ShardCount != 8 {
		t.Fatalf("subject config mismatch: %+v", cfg.SubjectConfig)
	}
	if cfg.StreamName != "ARCHIVE_STREAM" || cfg.Durable != "archive-durable" || cfg.QueueGroup != "archive-group" {
		t.Fatalf("stream config mismatch: %+v", cfg)
	}
	if cfg.AckWait != 75*time.Second || cfg.MaxAckPending != 123 || cfg.ProcessTimeout != 9*time.Second || cfg.DrainTimeout != 15*time.Second {
		t.Fatalf("timing config mismatch: %+v", cfg)
	}
	if cfg.ObjectBucket != "bucket-a" || cfg.ObjectPrefix != "raw-a" || cfg.WriterID != "writer-a" || cfg.ZstdEncoderLevel != 5 {
		t.Fatalf("object config mismatch: %+v", cfg)
	}
	if got := resolveArchiveManifestDSN(); got != "postgres://control" {
		t.Fatalf("manifest DSN fallback mismatch: %q", got)
	}
}
