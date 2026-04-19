package inference

import (
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

func TestBuildEnergyComparisonInsightSolarFreedomUp(t *testing.T) {
	now := time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC)
	record := BuildEnergyComparisonInsight(EnergyComparisonInput{
		Now: now,
		Scope: EnergyComparisonScope{
			Mode:              "all",
			ResolvedDeviceIDs: []string{"dev-a", "dev-b"},
		},
		Preset:          "last7d",
		Timezone:        "America/New_York",
		GridPricePerKwh: 0.30,
		Currency:        "USD",
		CurrentEnergy:   testEnergySeries(8.0, 2.0, 95, 0.36, 1.10),
		PreviousEnergy:  testEnergySeries(0.5, 4.0, 10, 0.52, 0.74),
		CurrentPower:    telemetryquery.Series{Points: make([]telemetryquery.Point, 12)},
		PreviousPower:   telemetryquery.Series{Points: make([]telemetryquery.Point, 12)},
	})

	if record.Status != StatusReady {
		t.Fatalf("status mismatch: got=%q want=%q", record.Status, StatusReady)
	}
	if record.Insight == nil {
		t.Fatal("expected insight")
	}
	if got, want := record.Insight.VerdictClass, EnergyComparisonVerdictSolarFreedomUp; got != want {
		t.Fatalf("verdict mismatch: got=%q want=%q", got, want)
	}
	if record.Insight.Headline == "" {
		t.Fatal("expected headline")
	}
	if len(record.Insight.Cards) == 0 {
		t.Fatal("expected cards")
	}
	if record.Insight.ExpiresAt.Before(now.Add(59 * time.Minute)) {
		t.Fatalf("expected hourly expiry, got=%v", record.Insight.ExpiresAt)
	}
}

func testEnergySeries(solarKwh, loadKwh, selfSufficiencyPct, _ float64, _ float64) telemetryquery.Series {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	return telemetryquery.Series{
		Points: []telemetryquery.Point{
			{
				BucketStart: start,
				BucketEnd:   end,
				Metrics: telemetryquery.Metrics{
					SolarGeneratedWh:         floatPtr(solarKwh * 1000),
					LoadEnergyWh:             floatPtr(loadKwh * 1000),
					ACInputEnergyWh:          floatPtr((1 - selfSufficiencyPct/100) * loadKwh * 1000),
					BatteryChargeEnergyWh:    floatPtr(220),
					BatteryDischargeEnergyWh: floatPtr(90),
				},
			},
		},
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func TestEnergyComparisonKeyDeterministicAcrossDeviceOrder(t *testing.T) {
	t.Parallel()

	store := &ValkeyStore{keyPrefix: defaultKeyPrefix}
	a := store.energyComparisonKey(EnergyComparisonCacheKey{
		ScopeMode:         "all",
		ResolvedDeviceIDs: []string{"dev-b", "dev-a"},
		Preset:            "last7d",
		Timezone:          "America/New_York",
		GridPricePerKwh:   0.30,
		Currency:          "USD",
		RefreshSlotUnixMs: 123456789,
	})
	b := store.energyComparisonKey(EnergyComparisonCacheKey{
		ScopeMode:         "all",
		ResolvedDeviceIDs: []string{"dev-a", "dev-b"},
		Preset:            "last7d",
		Timezone:          "America/New_York",
		GridPricePerKwh:   0.30,
		Currency:          "USD",
		RefreshSlotUnixMs: 123456789,
	})
	if a != b {
		t.Fatalf("expected stable key across resolved-device ordering:\n%s\n%s", a, b)
	}
	if !strings.HasPrefix(a, defaultKeyPrefix+":{energy-comparison:") {
		t.Fatalf("unexpected cache key format: %q", a)
	}
}
