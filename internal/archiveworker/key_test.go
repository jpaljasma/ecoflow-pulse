package archiveworker

import (
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestEnvelopePartitionTimePrefersIngestedTime(t *testing.T) {
	t.Parallel()

	env := &envelopev1.TelemetryEnvelope{
		IngestedTimeUnixMs: 1700000000123,
		ObservedTimeUnixMs: 1700009999123,
		DeviceTimeUnixMs:   1700001111123,
	}
	got := envelopePartitionTime(env, time.Unix(10, 0))
	if got.UnixMilli() != env.GetIngestedTimeUnixMs() {
		t.Fatalf("partition time mismatch: got=%d want=%d", got.UnixMilli(), env.GetIngestedTimeUnixMs())
	}
}

func TestEnvelopePartitionTimeFallsBack(t *testing.T) {
	t.Parallel()

	fallback := time.Unix(999, 0)
	got := envelopePartitionTime(&envelopev1.TelemetryEnvelope{}, fallback)
	if got.Unix() != fallback.Unix() {
		t.Fatalf("fallback partition time mismatch: got=%d want=%d", got.Unix(), fallback.Unix())
	}
}

func TestBuildArchiveObjectKey(t *testing.T) {
	t.Parallel()

	partition := time.Date(2026, time.February, 24, 18, 12, 3, 0, time.UTC)
	got := buildArchiveObjectKey("raw", partition, 7, 12, "node#1")
	want := "raw/yyyy=2026/mm=02/dd=24/hh=18/shard=007/part-00012-node-1.pb.zst"
	if got != want {
		t.Fatalf("object key mismatch:\n got=%s\nwant=%s", got, want)
	}
}
