package inference

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
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
	if !strings.HasPrefix(a, "pulse:inference-energy-comparison:{energy-comparison-all}:xxh3-128:") {
		t.Fatalf("unexpected cache key format: %q", a)
	}
}

func TestEnergyComparisonCacheSharesInsightsAcrossStoreInstances(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	writer := setupInferenceStoreOnServer(t, server)
	reader := setupInferenceStoreOnServer(t, server)
	now := time.Date(2026, time.May, 14, 10, 0, 0, 0, time.UTC)
	key := EnergyComparisonCacheKey{
		ScopeMode:         "all",
		ResolvedDeviceIDs: []string{"dev-b", "dev-a"},
		Preset:            "last7d",
		Timezone:          "America/New_York",
		Date:              "2026-05-14",
		GridPricePerKwh:   0.30,
		Currency:          "USD",
		RefreshSlotUnixMs: now.Truncate(time.Hour).UnixMilli(),
	}
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
	if err := writer.PutEnergyComparison(context.Background(), key, record); err != nil {
		t.Fatalf("PutEnergyComparison() error = %v", err)
	}

	cached, err := reader.GetEnergyComparison(context.Background(), key)
	if err != nil {
		t.Fatalf("GetEnergyComparison() error = %v", err)
	}
	if cached == nil || cached.Insight == nil {
		t.Fatal("expected shared energy comparison cache hit")
	}
	if got, want := cached.Insight.VerdictClass, EnergyComparisonVerdictSolarFreedomUp; got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
	cached.Insight.Headline = "mutated response"

	cachedAgain, err := reader.GetEnergyComparison(context.Background(), key)
	if err != nil {
		t.Fatalf("second GetEnergyComparison() error = %v", err)
	}
	if cachedAgain == nil || cachedAgain.Insight == nil {
		t.Fatal("expected second shared energy comparison cache hit")
	}
	if cachedAgain.Insight.Headline == "mutated response" {
		t.Fatal("cached energy comparison insight was mutated by caller")
	}
	for _, cacheKey := range server.Keys() {
		if strings.HasPrefix(cacheKey, "pulse:inference-energy-comparison:{energy-comparison-all}:xxh3-128:") {
			return
		}
	}
	t.Fatalf("expected energy comparison valkey key, got %#v", server.Keys())
}

func setupInferenceStoreOnServer(tb testing.TB, server *miniredis.Miniredis) *ValkeyStore {
	tb.Helper()

	client, err := ingestlease.NewValkeyClient(ingestlease.DefaultValkeyClientConfig([]string{server.Addr()}))
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
