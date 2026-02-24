package projectionworker

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
)

func TestValkeySnapshotStoreApplyAndMerge(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	ctx := context.Background()

	first, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		DeviceId:           "dev-1",
		EcoflowSn:          "R351ZABAPH331057",
		Shard:              7,
		ShardCount:         128,
		IngestedTimeUnixMs: 1000,
		TypeCode:           "pdStatus",
		Payload:            []byte(`{"params":{"wattsInSum":100}}`),
		SourceKind:         envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
	})
	if err != nil {
		t.Fatalf("apply first envelope: %v", err)
	}
	if first.CursorSeq != 1 {
		t.Fatalf("cursor seq mismatch: got=%d want=1", first.CursorSeq)
	}
	if got := first.Metrics["params.wattsInSum"]; got != 100 {
		t.Fatalf("metric mismatch: got=%v want=100", got)
	}

	second, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-2",
		DeviceId:           "dev-1",
		EcoflowSn:          "R351ZABAPH331057",
		Shard:              7,
		ShardCount:         128,
		IngestedTimeUnixMs: 2000,
		TypeCode:           "bmsStatus",
		Payload:            []byte(`{"params":{"outputWatts":35}}`),
		SourceKind:         envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
	})
	if err != nil {
		t.Fatalf("apply second envelope: %v", err)
	}
	if second.CursorSeq != 2 {
		t.Fatalf("cursor seq mismatch: got=%d want=2", second.CursorSeq)
	}
	if got := second.Metrics["params.wattsInSum"]; got != 100 {
		t.Fatalf("merged metric missing from previous envelope: got=%v", got)
	}
	if got := second.Metrics["params.outputWatts"]; got != 35 {
		t.Fatalf("new metric mismatch: got=%v want=35", got)
	}

	readBack, err := store.GetSnapshot(ctx, "dev-1", "")
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if readBack == nil {
		t.Fatalf("expected snapshot to exist")
	}
	if readBack.CursorSeq != second.CursorSeq {
		t.Fatalf("snapshot cursor mismatch: got=%d want=%d", readBack.CursorSeq, second.CursorSeq)
	}
}

func TestValkeySnapshotStoreIgnoresStaleAndDuplicate(t *testing.T) {
	t.Parallel()

	store := setupSnapshotStore(t)
	ctx := context.Background()

	initial, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-10",
		DeviceId:           "dev-stale",
		EcoflowSn:          "SN-STALE",
		IngestedTimeUnixMs: 5000,
		Payload:            []byte(`{"params":{"load":42}}`),
	})
	if err != nil {
		t.Fatalf("apply initial envelope: %v", err)
	}
	if initial.CursorSeq != 1 {
		t.Fatalf("cursor seq mismatch: got=%d want=1", initial.CursorSeq)
	}

	dup, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-10",
		DeviceId:           "dev-stale",
		EcoflowSn:          "SN-STALE",
		IngestedTimeUnixMs: 6000,
		Payload:            []byte(`{"params":{"load":99}}`),
	})
	if err != nil {
		t.Fatalf("apply duplicate envelope: %v", err)
	}
	if dup.CursorSeq != 1 {
		t.Fatalf("duplicate envelope should not advance seq: got=%d want=1", dup.CursorSeq)
	}
	if got := dup.Metrics["params.load"]; got != 42 {
		t.Fatalf("duplicate envelope should not mutate metrics: got=%v want=42", got)
	}

	stale, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-11",
		DeviceId:           "dev-stale",
		EcoflowSn:          "SN-STALE",
		IngestedTimeUnixMs: 4000,
		Payload:            []byte(`{"params":{"load":13}}`),
	})
	if err != nil {
		t.Fatalf("apply stale envelope: %v", err)
	}
	if stale.CursorSeq != 1 {
		t.Fatalf("stale envelope should not advance seq: got=%d want=1", stale.CursorSeq)
	}
	if got := stale.Metrics["params.load"]; got != 42 {
		t.Fatalf("stale envelope should not mutate metrics: got=%v want=42", got)
	}
}

func TestSnapshotTagRequiresIdentity(t *testing.T) {
	t.Parallel()
	if _, err := snapshotTag("", ""); err == nil {
		t.Fatalf("expected error for missing snapshot identity")
	}
}

func setupSnapshotStore(tb testing.TB) *ValkeySnapshotStore {
	tb.Helper()
	mini, err := miniredis.Run()
	if err != nil {
		tb.Fatalf("start miniredis: %v", err)
	}
	tb.Cleanup(mini.Close)

	client, err := ingestlease.NewValkeyClient(ingestlease.DefaultValkeyClientConfig([]string{mini.Addr()}))
	if err != nil {
		tb.Fatalf("new valkey client: %v", err)
	}
	tb.Cleanup(client.Close)

	store, err := NewValkeySnapshotStore(client, DefaultValkeySnapshotStoreConfig())
	if err != nil {
		tb.Fatalf("new snapshot store: %v", err)
	}
	return store
}
