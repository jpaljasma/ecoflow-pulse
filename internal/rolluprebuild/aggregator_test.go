package rolluprebuild

import (
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
)

func TestAggregatorIntegratesSolarAcrossMinuteBoundary(t *testing.T) {
	t.Parallel()

	agg := NewAggregator()
	base := time.Date(2026, time.March, 6, 12, 0, 30, 0, time.UTC)

	first, err := rollupworker.SampleFromEnvelope(testEnvelope(base, `{"params":{"inLvMpptPwr":120}}`))
	if err != nil {
		t.Fatalf("first sample failed: %v", err)
	}
	secondAt := base.Add(45 * time.Second)
	second, err := rollupworker.SampleFromEnvelope(testEnvelope(secondAt, `{"params":{"inLvMpptPwr":60}}`))
	if err != nil {
		t.Fatalf("second sample failed: %v", err)
	}

	agg.ApplySample(first)
	agg.ApplySample(second)

	rows := agg.Rows(ResolutionMinute)
	if len(rows) != 2 {
		t.Fatalf("minute row count mismatch: got=%d want=2", len(rows))
	}
	if rows[0].SolarGeneratedWh != 1 || !rows[0].HasSolarGeneratedWh {
		t.Fatalf("first minute solar mismatch: got=%f has=%v want=1", rows[0].SolarGeneratedWh, rows[0].HasSolarGeneratedWh)
	}
	if rows[1].SolarGeneratedWh != 0.5 || !rows[1].HasSolarGeneratedWh {
		t.Fatalf("second minute solar mismatch: got=%f has=%v want=0.5", rows[1].SolarGeneratedWh, rows[1].HasSolarGeneratedWh)
	}
}

func TestAggregatorFinalizeCarriesSolarToWindowEnd(t *testing.T) {
	t.Parallel()

	agg := NewAggregator()
	base := time.Date(2026, time.March, 6, 12, 0, 30, 0, time.UTC)

	first, err := rollupworker.SampleFromEnvelope(testEnvelope(base, `{"params":{"inLvMpptPwr":120}}`))
	if err != nil {
		t.Fatalf("first sample failed: %v", err)
	}

	agg.ApplySample(first)
	agg.Finalize(base.Add(30 * time.Second))

	rows := agg.Rows(ResolutionMinute)
	if len(rows) != 1 {
		t.Fatalf("minute row count mismatch: got=%d want=1", len(rows))
	}
	if rows[0].SolarGeneratedWh != 1 || !rows[0].HasSolarGeneratedWh {
		t.Fatalf("finalized minute solar mismatch: got=%f has=%v want=1", rows[0].SolarGeneratedWh, rows[0].HasSolarGeneratedWh)
	}
}

func testEnvelope(at time.Time, payload string) *envelopev1.TelemetryEnvelope {
	return &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "Y711ZABA9H2P0294",
		ObservedTimeUnixMs: at.UnixMilli(),
		Payload:            []byte(payload),
		Labels:             map[string]string{"provider": "ecoflow"},
	}
}
