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

func TestAggregatorIntegratesExplicitEnergyAcrossSparseMinuteBoundary(t *testing.T) {
	t.Parallel()

	agg := NewAggregator()
	base := time.Date(2026, time.March, 6, 12, 0, 30, 0, time.UTC)

	first, err := rollupworker.SampleFromEnvelope(testEnvelope(base, `{"params":{"wattsInSum":259,"pv1ChargeWatts":52,"wattsOutSum":217,"f32ShowSoc":25.5}}`))
	if err != nil {
		t.Fatalf("first sample failed: %v", err)
	}
	secondAt := base.Add(45 * time.Second)
	second, err := rollupworker.SampleFromEnvelope(testEnvelope(secondAt, `{"params":{"wattsInSum":240,"pv1ChargeWatts":40,"wattsOutSum":210,"f32ShowSoc":25.0}}`))
	if err != nil {
		t.Fatalf("second sample failed: %v", err)
	}

	agg.ApplySample(first)
	agg.ApplySample(second)

	rows := agg.Rows(ResolutionMinute)
	if len(rows) != 2 {
		t.Fatalf("minute row count mismatch: got=%d want=2", len(rows))
	}
	if !rows[0].HasACInputEnergyWh || rows[0].ACInputEnergyWh <= 0 {
		t.Fatalf("expected first minute ac input energy, got=%f has=%v", rows[0].ACInputEnergyWh, rows[0].HasACInputEnergyWh)
	}
	if !rows[0].HasLoadEnergyWh || rows[0].LoadEnergyWh <= 0 {
		t.Fatalf("expected first minute load energy, got=%f has=%v", rows[0].LoadEnergyWh, rows[0].HasLoadEnergyWh)
	}
	if !rows[0].HasACOutputEnergyWh || rows[0].ACOutputEnergyWh <= 0 {
		t.Fatalf("expected first minute ac output energy, got=%f has=%v", rows[0].ACOutputEnergyWh, rows[0].HasACOutputEnergyWh)
	}
	if !rows[1].HasACInputEnergyWh || rows[1].ACInputEnergyWh <= 0 {
		t.Fatalf("expected second minute ac input energy, got=%f has=%v", rows[1].ACInputEnergyWh, rows[1].HasACInputEnergyWh)
	}
}

func TestAggregatorAccumulatesPVPortRows(t *testing.T) {
	t.Parallel()

	agg := NewAggregator()
	base := time.Date(2026, time.March, 6, 12, 0, 30, 0, time.UTC)

	first, err := rollupworker.SampleFromEnvelope(testEnvelope(base, `{"params":{"inLvMpptVol":48.2,"inLvMpptAmp":4.1,"pv1ChargeWatts":190,"cmsBattSoc":25.5}}`))
	if err != nil {
		t.Fatalf("first sample failed: %v", err)
	}
	secondAt := base.Add(15 * time.Second)
	second, err := rollupworker.SampleFromEnvelope(testEnvelope(secondAt, `{"params":{"inLvMpptVol":49.1,"inLvMpptAmp":4.4,"pv1ChargeWatts":215,"cmsBattSoc":25.0}}`))
	if err != nil {
		t.Fatalf("second sample failed: %v", err)
	}

	agg.ApplySample(first)
	agg.ApplySample(second)

	rows := agg.PVPortRows(ResolutionMinute)
	if len(rows) != 1 {
		t.Fatalf("pv port minute row count mismatch: got=%d want=1", len(rows))
	}
	row := rows[0]
	if row.PortID != "pv-low" || row.PortLabel != "PV Low" {
		t.Fatalf("port mismatch: %+v", row)
	}
	if row.SampleCount != 2 {
		t.Fatalf("sample count mismatch: got=%d want=2", row.SampleCount)
	}
	if row.MaxObservedVolts != 49.1 || row.MaxObservedAmps != 4.4 || row.MaxObservedWatts != 215 {
		t.Fatalf("max observation mismatch: %+v", row)
	}
	if row.LastObservedVolts != 49.1 || row.LastObservedAmps != 4.4 || row.LastObservedWatts != 215 {
		t.Fatalf("last observation mismatch: %+v", row)
	}
	if row.LastObservedAtUnixMS != secondAt.UnixMilli() {
		t.Fatalf("last observed at mismatch: got=%d want=%d", row.LastObservedAtUnixMS, secondAt.UnixMilli())
	}
}

func testEnvelope(at time.Time, payload string) *envelopev1.TelemetryEnvelope {
	return &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "DEMODPU0000294",
		ObservedTimeUnixMs: at.UnixMilli(),
		Payload:            []byte(payload),
		Labels:             map[string]string{"provider": "ecoflow"},
	}
}
