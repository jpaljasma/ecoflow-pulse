package weatherd_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/store"
	valkey "github.com/valkey-io/valkey-go"
)

type fakeUpstream struct {
	forecast      *weatherd.Bundle
	forecastBatch []weatherd.Bundle
	forecastCalls int
	batchCalls    int
}

func (f *fakeUpstream) FetchForecast(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	f.forecastCalls++
	return cloneBundleForTest(f.forecast), nil
}

func (f *fakeUpstream) FetchForecastBatch(_ context.Context, _ []weatherd.Request) ([]weatherd.Bundle, error) {
	f.batchCalls++
	out := make([]weatherd.Bundle, len(f.forecastBatch))
	copy(out, f.forecastBatch)
	return out, nil
}

func (f *fakeUpstream) FetchHistoricalForecast(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	return nil, nil
}

func TestForecastValuesConvertsImperialLocally(t *testing.T) {
	temp := 10.0
	wind := 16.09344
	precip := 25.4
	visibility := 1609.344
	got := weatherd.ForecastValues(weatherd.ForecastValueSet{
		Temperature:   &temp,
		WindSpeed:     &wind,
		Precipitation: &precip,
		Visibility:    &visibility,
	}, weatherd.UnitSystemImperial)

	assertClose(t, value(got.Temperature), 50.0)
	assertClose(t, value(got.WindSpeed), 10.0)
	assertClose(t, value(got.Precipitation), 1.0)
	assertClose(t, value(got.Visibility), 1.0)
}

func TestGet7DayForecastServesStaleCacheWhenBudgetIsExhausted(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	nowRef := now
	cache := store.NewMemoryHotCache(func() time.Time { return nowRef })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return nowRef })
	upstream := &fakeUpstream{}
	svc, err := weatherd.NewService(
		upstream,
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 1, PerMinuteLimit: 1, NowFn: func() time.Time { return nowRef }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return nowRef }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	req := weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemMetric,
		Timezone:   "America/New_York",
	}
	bundle := sampleBundle(now.Add(-2*time.Hour), "grid-key", []time.Time{now.Add(-24 * time.Hour)}, []float64{12})
	if err := snapshots.TouchRefreshCandidate(context.Background(), "grid-key", req, now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("TouchRefreshCandidate() error = %v", err)
	}
	if err := cache.PutForecast(context.Background(), "grid-key", *bundle, 10*time.Minute); err != nil {
		t.Fatalf("PutForecast() error = %v", err)
	}
	nowRef = now.Add(20 * time.Minute)

	got, err := svc.Get7DayForecast(context.Background(), req)
	if err != nil {
		t.Fatalf("Get7DayForecast() error = %v", err)
	}
	if upstream.forecastCalls != 0 {
		t.Fatalf("expected no upstream forecast call, got %d", upstream.forecastCalls)
	}
	if got.Provenance.CanonicalLocationKey != "grid-key" {
		t.Fatalf("canonical key = %q, want grid-key", got.Provenance.CanonicalLocationKey)
	}
}

func TestGet7DayForecastFetchesUpstreamAndConvertsImperialResponse(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	upstream := &fakeUpstream{
		forecast: sampleBundle(now, "", []time.Time{now}, []float64{10}),
	}
	svc, err := weatherd.NewService(
		upstream,
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 10, PerMinuteLimit: 10, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Get7DayForecast(context.Background(), weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemImperial,
		Timezone:   "UTC",
	})
	if err != nil {
		t.Fatalf("Get7DayForecast() error = %v", err)
	}
	if upstream.forecastCalls != 1 {
		t.Fatalf("forecast calls = %d, want 1", upstream.forecastCalls)
	}
	if got.Provenance.CanonicalLocationKey == "" {
		t.Fatal("expected canonical key to be set")
	}
	assertClose(t, value(got.Hourly[0].Raw.Temperature), 50)
}

func TestGet7DayForecastServesFreshHotCache(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	upstream := &fakeUpstream{}
	svc, err := weatherd.NewService(
		upstream,
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 1, PerMinuteLimit: 1, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	req := weatherd.Request{Latitude: 42.6, Longitude: -77.4, UnitSystem: weatherd.UnitSystemMetric, Timezone: "UTC"}
	if err := snapshots.TouchRefreshCandidate(context.Background(), "grid-key", req, now); err != nil {
		t.Fatalf("TouchRefreshCandidate() error = %v", err)
	}
	if err := cache.PutForecast(context.Background(), "grid-key", *sampleBundle(now, "grid-key", []time.Time{now}, []float64{9}), time.Hour); err != nil {
		t.Fatalf("PutForecast() error = %v", err)
	}

	got, err := svc.Get7DayForecast(context.Background(), req)
	if err != nil {
		t.Fatalf("Get7DayForecast() error = %v", err)
	}
	if upstream.forecastCalls != 0 {
		t.Fatalf("forecast calls = %d, want 0", upstream.forecastCalls)
	}
	assertClose(t, value(got.Hourly[0].Raw.Temperature), 9)
}

func TestGet7DayForecastUsesSharedValkeyHotCacheAcrossServices(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	req := weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemMetric,
		Timezone:   "UTC",
	}
	firstUpstream := &fakeUpstream{
		forecast: sampleBundle(now, "", []time.Time{now}, []float64{21}),
	}
	firstSvc, err := weatherd.NewService(
		firstUpstream,
		newValkeyHotCacheForWeatherTest(t, server, func() time.Time { return now }),
		snapshots,
		budget.New(budget.Config{DailyLimit: 10, PerMinuteLimit: 10, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: time.Hour, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("first NewService() error = %v", err)
	}
	first, err := firstSvc.Get7DayForecast(context.Background(), req)
	if err != nil {
		t.Fatalf("first Get7DayForecast() error = %v", err)
	}
	if firstUpstream.forecastCalls != 1 {
		t.Fatalf("first upstream forecast calls = %d, want 1", firstUpstream.forecastCalls)
	}
	if first.Provenance.CanonicalLocationKey == "" {
		t.Fatal("expected first forecast to establish a canonical cache key")
	}

	secondUpstream := &fakeUpstream{}
	secondSvc, err := weatherd.NewService(
		secondUpstream,
		newValkeyHotCacheForWeatherTest(t, server, func() time.Time { return now.Add(time.Minute) }),
		snapshots,
		budget.New(budget.Config{DailyLimit: 0, PerMinuteLimit: 0, NowFn: func() time.Time { return now.Add(time.Minute) }}),
		weatherd.Config{HotTTL: time.Hour, NowFn: func() time.Time { return now.Add(time.Minute) }},
	)
	if err != nil {
		t.Fatalf("second NewService() error = %v", err)
	}
	second, err := secondSvc.Get7DayForecast(context.Background(), req)
	if err != nil {
		t.Fatalf("second Get7DayForecast() error = %v", err)
	}
	if secondUpstream.forecastCalls != 0 {
		t.Fatalf("second upstream forecast calls = %d, want 0", secondUpstream.forecastCalls)
	}
	if second.Provenance.CanonicalLocationKey != first.Provenance.CanonicalLocationKey {
		t.Fatalf("cached canonical key = %q, want %q", second.Provenance.CanonicalLocationKey, first.Provenance.CanonicalLocationKey)
	}
	assertClose(t, value(second.Hourly[0].Raw.Temperature), 21)
}

func TestRefreshRecentLocationsBatchesMatchingRequests(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	upstream := &fakeUpstream{
		forecastBatch: []weatherd.Bundle{
			*sampleBundle(now, "grid-a", []time.Time{now}, []float64{10}),
			*sampleBundle(now, "grid-b", []time.Time{now}, []float64{11}),
		},
	}
	svc, err := weatherd.NewService(
		upstream,
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 20, PerMinuteLimit: 20, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, RecentActiveWindow: 7 * 24 * time.Hour, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	tilt := 45.0
	azimuth := 0.0
	reqA := weatherd.Request{Latitude: 42.6, Longitude: -77.4, UnitSystem: weatherd.UnitSystemMetric, PanelTiltDegrees: &tilt, PanelAzimuthDegrees: &azimuth, Timezone: "UTC"}
	reqB := weatherd.Request{Latitude: 42.7, Longitude: -77.5, UnitSystem: weatherd.UnitSystemMetric, PanelTiltDegrees: &tilt, PanelAzimuthDegrees: &azimuth, Timezone: "UTC"}
	if err := snapshots.TouchRefreshCandidate(context.Background(), "grid-a", reqA, now); err != nil {
		t.Fatalf("TouchRefreshCandidate(A) error = %v", err)
	}
	if err := snapshots.TouchRefreshCandidate(context.Background(), "grid-b", reqB, now); err != nil {
		t.Fatalf("TouchRefreshCandidate(B) error = %v", err)
	}

	if err := svc.RefreshRecentLocations(context.Background()); err != nil {
		t.Fatalf("RefreshRecentLocations() error = %v", err)
	}
	if upstream.batchCalls != 1 {
		t.Fatalf("batch calls = %d, want 1", upstream.batchCalls)
	}
}

func sampleBundle(issuedAt time.Time, key string, timestamps []time.Time, temperatures []float64) *weatherd.Bundle {
	hourly := make([]weatherd.HourlyForecastPoint, 0, len(timestamps))
	for idx, ts := range timestamps {
		temp := temperatures[idx]
		dir := 180.0
		hourly = append(hourly, weatherd.HourlyForecastPoint{
			Time: ts.UTC(),
			Condition: weatherd.WeatherCondition{
				WeatherCode: 2,
				WeatherText: "Partly cloudy",
			},
			Raw: weatherd.ForecastValueSet{
				Temperature:          &temp,
				WindDirectionDegrees: &dir,
			},
			Corrected: weatherd.ForecastValueSet{
				Temperature:          &temp,
				WindDirectionDegrees: &dir,
			},
		})
	}
	return &weatherd.Bundle{
		Provenance: weatherd.Provenance{
			Source:               "open_meteo",
			ModelSelection:       "best_match",
			ActualSource:         "past_days",
			Timezone:             "UTC",
			CanonicalLocationKey: key,
			IssuedAt:             issuedAt.UTC(),
			Latitude:             42.6,
			Longitude:            -77.4,
			Elevation:            290,
		},
		Hourly: hourly,
	}
}

func cloneBundleForTest(in *weatherd.Bundle) *weatherd.Bundle {
	if in == nil {
		return nil
	}
	out := *in
	out.Hourly = append([]weatherd.HourlyForecastPoint(nil), in.Hourly...)
	out.Daily = append([]weatherd.DailyForecastPoint(nil), in.Daily...)
	return &out
}

func newValkeyHotCacheForWeatherTest(t *testing.T, server *miniredis.Miniredis, nowFn func() time.Time) *store.ValkeyHotCache {
	t.Helper()

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{server.Addr()},
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("new valkey client: %v", err)
	}
	t.Cleanup(client.Close)
	cache, err := store.NewValkeyHotCache(client, "pulse:weather", nowFn)
	if err != nil {
		t.Fatalf("new valkey hot cache: %v", err)
	}
	return cache
}

func value(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if got < want-0.0001 || got > want+0.0001 {
		t.Fatalf("got %.6f, want %.6f", got, want)
	}
}
