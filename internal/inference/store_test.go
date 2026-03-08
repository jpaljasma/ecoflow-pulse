package inference

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
)

func TestValkeyStoreApplyEnvelopeBuildsBatteryExpansionInsight(t *testing.T) {
	t.Parallel()

	store := setupInferenceStore(t)
	ctx := context.Background()

	readModel, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		DeviceId:           "dev-1",
		EcoflowSn:          "SN-1",
		IngestedTimeUnixMs: 1_000,
		Payload:            []byte(`{"params":{"soc":54,"wattsInSum":120}}`),
	}, DeviceContext{
		DeviceID:     "dev-1",
		EcoflowSN:    "SN-1",
		ProductName:  "Kitchen Delta 2 Max",
		Model:        "DELTA 2 Max",
		Capabilities: map[string]any{"battery_pack_count": float64(1), "supports_extra_battery": true},
	})
	if err != nil {
		t.Fatalf("apply envelope: %v", err)
	}
	if got := readModel.Cursor.Seq; got != 1 {
		t.Fatalf("cursor seq mismatch: got=%d want=1", got)
	}
	if got := readModel.RawMetrics["params.soc"]; got != 54 {
		t.Fatalf("raw metric mismatch: got=%v want=54", got)
	}
	if got := readModel.DeviceInsights.Status; got != StatusReady {
		t.Fatalf("status mismatch: got=%q want=%q", got, StatusReady)
	}
	if got := len(readModel.DeviceInsights.Insights); got != 1 {
		t.Fatalf("insight count mismatch: got=%d want=1", got)
	}
	insight := readModel.DeviceInsights.Insights[0]
	if insight.Kind != KindBatteryExpansion {
		t.Fatalf("kind mismatch: got=%q want=%q", insight.Kind, KindBatteryExpansion)
	}
	if got := insight.Attributes["current_battery_packs"]; got != 1 {
		t.Fatalf("current_battery_packs mismatch: got=%v want=1", got)
	}
	if got := insight.Attributes["max_battery_packs"]; got != 3 {
		t.Fatalf("max_battery_packs mismatch: got=%v want=3", got)
	}

	filtered, err := store.GetDeviceInsights(ctx, "dev-1", Filter{Kinds: []Kind{KindBatteryExpansion}, MaxItems: 1})
	if err != nil {
		t.Fatalf("get device insights: %v", err)
	}
	if got := len(filtered.Insights); got != 1 {
		t.Fatalf("filtered insight count mismatch: got=%d want=1", got)
	}
}

func TestValkeyStoreApplyEnvelopeIgnoresDuplicateAndStale(t *testing.T) {
	t.Parallel()

	store := setupInferenceStore(t)
	ctx := context.Background()
	deviceCtx := DeviceContext{
		DeviceID:     "dev-dup",
		EcoflowSN:    "SN-DUP",
		ProductName:  "DELTA Pro Ultra",
		Model:        "DELTA Pro Ultra",
		Capabilities: map[string]any{"battery_pack_count": float64(2), "supports_extra_battery": true},
	}

	first, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		DeviceId:           "dev-dup",
		EcoflowSn:          "SN-DUP",
		IngestedTimeUnixMs: 5_000,
		Payload:            []byte(`{"params":{"soc":60}}`),
	}, deviceCtx)
	if err != nil {
		t.Fatalf("apply first envelope: %v", err)
	}
	if got := first.Cursor.Seq; got != 1 {
		t.Fatalf("cursor seq mismatch: got=%d want=1", got)
	}

	dup, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-1",
		DeviceId:           "dev-dup",
		EcoflowSn:          "SN-DUP",
		IngestedTimeUnixMs: 6_000,
		Payload:            []byte(`{"params":{"soc":80}}`),
	}, deviceCtx)
	if err != nil {
		t.Fatalf("apply duplicate envelope: %v", err)
	}
	if got := dup.Cursor.Seq; got != 1 {
		t.Fatalf("duplicate should not advance seq: got=%d want=1", got)
	}
	if got := dup.RawMetrics["params.soc"]; got != 60 {
		t.Fatalf("duplicate should not mutate metrics: got=%v want=60", got)
	}

	stale, err := store.ApplyEnvelope(ctx, &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "env-2",
		DeviceId:           "dev-dup",
		EcoflowSn:          "SN-DUP",
		IngestedTimeUnixMs: 4_000,
		Payload:            []byte(`{"params":{"soc":10}}`),
	}, deviceCtx)
	if err != nil {
		t.Fatalf("apply stale envelope: %v", err)
	}
	if got := stale.Cursor.Seq; got != 1 {
		t.Fatalf("stale should not advance seq: got=%d want=1", got)
	}
	if got := stale.RawMetrics["params.soc"]; got != 60 {
		t.Fatalf("stale should not mutate metrics: got=%v want=60", got)
	}
}

func setupInferenceStore(tb testing.TB) *ValkeyStore {
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

	store, err := NewValkeyStore(client, DefaultValkeyStoreConfig())
	if err != nil {
		tb.Fatalf("new valkey store: %v", err)
	}
	return store
}
