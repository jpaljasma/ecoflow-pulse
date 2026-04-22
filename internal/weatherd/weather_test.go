package weatherd_test

import (
	"context"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/store"
)

type fakeUpstream struct {
	forecast         *weatherd.Bundle
	forecastBatch    []weatherd.Bundle
	previousRuns     *weatherd.Bundle
	forecastCalls    int
	batchCalls       int
	previousRunCalls int
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

func (f *fakeUpstream) FetchPreviousRuns(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	f.previousRunCalls++
	return cloneBundleForTest(f.previousRuns), nil
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

func TestCircularWindDirectionHelpers(t *testing.T) {
	diff := weatherd.CircularErrorDegrees(350, 10)
	assertClose(t, diff, 20)

	mae := weatherd.CircularWindDirectionMAE([]float64{20, -30})
	if mae == nil {
		t.Fatal("CircularWindDirectionMAE() = nil")
		return
	}
	assertClose(t, *mae, 25)
}

func TestUpdateBiasStatesUsesAdditiveAndRatioEWMA(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	forecastTemp := 10.0
	actualTemp := 13.0
	forecastRadiation := 100.0
	actualRadiation := 150.0

	states := weatherd.UpdateBiasStates(
		now,
		"grid-key",
		weatherd.ForecastValueSet{
			Temperature:        &forecastTemp,
			ShortwaveRadiation: &forecastRadiation,
		},
		weatherd.ForecastValueSet{
			Temperature:        &actualTemp,
			ShortwaveRadiation: &actualRadiation,
		},
		14,
		nil,
	)

	index := weatherd.BuildBiasIndex(states)
	tempState := index[weatherd.BiasMetricTemperature][14]
	radiationState := index[weatherd.BiasMetricShortwaveRadiation][14]

	assertClose(t, value(tempState.AdditiveBias), 3)
	assertClose(t, value(radiationState.MultiplicativeRatio), 1.5)
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

func TestGetYesterdayVerificationFallsBackToPreviousRuns(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	req := weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemMetric,
		Timezone:   "UTC",
	}
	actualBundle := sampleBundle(now, "grid-key", []time.Time{
		time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 17, 1, 0, 0, 0, time.UTC),
	}, []float64{10, 12})
	forecastBundle := sampleBundle(now, "grid-key", []time.Time{
		time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 17, 1, 0, 0, 0, time.UTC),
	}, []float64{8, 11})
	upstream := &fakeUpstream{previousRuns: forecastBundle}
	if err := snapshots.SaveForecastBundle(context.Background(), req, *actualBundle); err != nil {
		t.Fatalf("SaveForecastBundle() error = %v", err)
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

	result, err := svc.GetYesterdayVerification(context.Background(), req)
	if err != nil {
		t.Fatalf("GetYesterdayVerification() error = %v", err)
	}
	if got := result.Provenance.VerificationSource; got != "previous_runs" {
		t.Fatalf("verification source = %q, want previous_runs", got)
	}
	if upstream.previousRunCalls != 1 {
		t.Fatalf("previous run calls = %d, want 1", upstream.previousRunCalls)
	}
	if len(result.Hourly) != 2 {
		t.Fatalf("hourly rows = %d, want 2", len(result.Hourly))
	}

	resultAgain, err := svc.GetYesterdayVerification(context.Background(), req)
	if err != nil {
		t.Fatalf("second GetYesterdayVerification() error = %v", err)
	}
	if got := resultAgain.Provenance.VerificationSource; got != "snapshot" {
		t.Fatalf("second verification source = %q, want snapshot", got)
	}
	if upstream.previousRunCalls != 1 {
		t.Fatalf("previous run calls after cache reuse = %d, want 1", upstream.previousRunCalls)
	}
}

func TestGetYesterdayVerificationUsesSnapshotAndImperialConversion(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	req := weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemMetric,
		Timezone:   "UTC",
	}
	actualBundle := sampleBundle(now, "grid-key", []time.Time{
		time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
	}, []float64{10})
	priorForecast := sampleBundle(time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC), "grid-key", []time.Time{
		time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
	}, []float64{8})
	if err := snapshots.SaveForecastBundle(context.Background(), req, *actualBundle); err != nil {
		t.Fatalf("SaveForecastBundle(actual) error = %v", err)
	}
	if err := snapshots.SaveForecastBundle(context.Background(), req, *priorForecast); err != nil {
		t.Fatalf("SaveForecastBundle(prior) error = %v", err)
	}
	svc, err := weatherd.NewService(
		&fakeUpstream{previousRuns: priorForecast},
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 10, PerMinuteLimit: 10, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := svc.GetYesterdayVerification(context.Background(), weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemImperial,
		Timezone:   "UTC",
	})
	if err != nil {
		t.Fatalf("GetYesterdayVerification() error = %v", err)
	}
	if got := result.Provenance.VerificationSource; got != "snapshot" {
		t.Fatalf("verification source = %q, want snapshot", got)
	}
	assertClose(t, value(result.Hourly[0].ForecastRaw.Temperature), 46.4)
	assertClose(t, value(result.Hourly[0].Actual.Temperature), 50)
}

func TestGetYesterdayVerificationReturnsBudgetErrorWithoutStoredBundle(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	svc, err := weatherd.NewService(
		&fakeUpstream{},
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 1, PerMinuteLimit: 1, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.GetYesterdayVerification(context.Background(), weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemMetric,
		Timezone:   "UTC",
	})
	if err == nil {
		t.Fatal("expected budget error")
	}
	if err != weatherd.ErrUpstreamBudgetExceeded {
		t.Fatalf("err = %v, want %v", err, weatherd.ErrUpstreamBudgetExceeded)
	}
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
