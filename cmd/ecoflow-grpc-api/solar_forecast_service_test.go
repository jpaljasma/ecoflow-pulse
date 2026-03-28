package main

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	solarforecastv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/solarforecast/v1"
	weatherv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/weather/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakeSolarForecastDomain struct {
	response *solarforecastd.Outlook
	err      error
	calls    atomic.Int32

	blockFirstCall chan struct{}
	releaseFirst   chan struct{}
	once           sync.Once
}

func (f *fakeSolarForecastDomain) GetSolarOutlook(_ context.Context, _ solarforecastd.Input) (*solarforecastd.Outlook, error) {
	call := f.calls.Add(1)
	if call == 1 {
		if f.blockFirstCall != nil {
			f.once.Do(func() { close(f.blockFirstCall) })
		}
		if f.releaseFirst != nil {
			<-f.releaseFirst
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func newTestSolarForecastService(store controlplane.Store, domain solarForecastDomain) *SolarForecastService {
	return NewSolarForecastServiceWithDeps(SolarForecastServiceDeps{
		Log:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		Service:           domain,
		ControlPlaneStore: store,
	})
}

func TestSolarForecastServiceCachesEquivalentRequests(t *testing.T) {
	t.Parallel()

	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {
			{DeviceID: "device-a"},
			{DeviceID: "device-b"},
		},
	})
	domain := &fakeSolarForecastDomain{response: sampleSolarForecastOutlook(t)}
	svc := newTestSolarForecastService(store, domain)
	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"})
	req := &solarforecastv1.GetSolarOutlookRequest{
		Location:      testSolarForecastLocation(),
		UseAllDevices: true,
	}

	resp1, err := svc.GetSolarOutlook(ctx, req)
	if err != nil {
		t.Fatalf("first GetSolarOutlook() error = %v", err)
	}
	if got := domain.calls.Load(); got != 1 {
		t.Fatalf("domain calls after first request = %d, want 1", got)
	}

	resp1.GetScope().ResolvedDeviceIds[0] = "mutated-device"

	resp2, err := svc.GetSolarOutlook(ctx, req)
	if err != nil {
		t.Fatalf("second GetSolarOutlook() error = %v", err)
	}
	if got := domain.calls.Load(); got != 1 {
		t.Fatalf("domain calls after cache hit = %d, want 1", got)
	}
	if !proto.Equal(resp2, solarOutlookToProto(sampleSolarForecastOutlook(t))) {
		t.Fatalf("cached response changed after caller mutation")
	}
}

func TestSolarForecastServiceCoalescesConcurrentEquivalentRequests(t *testing.T) {
	store := newFakeControlPlaneStore(map[string][]controlplane.UserDevice{
		"dev-user": {
			{DeviceID: "device-a"},
		},
	})
	started := make(chan struct{})
	release := make(chan struct{})
	domain := &fakeSolarForecastDomain{
		response:       sampleSolarForecastOutlook(t),
		blockFirstCall: started,
		releaseFirst:   release,
	}
	svc := newTestSolarForecastService(store, domain)
	ctx := grpcmw.ContextWithClaims(context.Background(), grpcmw.Claims{Subject: "dev-user"})
	req := &solarforecastv1.GetSolarOutlookRequest{
		Location:      testSolarForecastLocation(),
		DeviceId:      "device-a",
		UseAllDevices: false,
	}

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := svc.GetSolarOutlook(ctx, req)
		errs <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first loader call to start")
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := svc.GetSolarOutlook(ctx, req)
		errs <- err
	}()

	time.Sleep(100 * time.Millisecond)
	if got := domain.calls.Load(); got != 1 {
		t.Fatalf("domain calls while first request is in flight = %d, want 1", got)
	}

	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("GetSolarOutlook() error = %v", err)
		}
	}
	if got := domain.calls.Load(); got != 1 {
		t.Fatalf("domain calls after coalesced requests = %d, want 1", got)
	}
}

func TestSolarForecastOutlookCacheKeyNormalizesVisibleDevices(t *testing.T) {
	t.Parallel()

	req := weatherd.Request{
		Latitude:            42.6,
		Longitude:           -77.4,
		UnitSystem:          weatherd.UnitSystemMetric,
		PanelTiltDegrees:    solarFloatPtr(45),
		PanelAzimuthDegrees: solarFloatPtr(180),
		Timezone:            "UTC",
	}

	normalized := solarForecastOutlookCacheKey(req, "all", "", []string{" dev-b ", "dev-a", "dev-b"})
	if got := solarForecastOutlookCacheKey(req, "all", "", []string{"dev-a", "dev-b"}); got != normalized {
		t.Fatalf("normalized key mismatch: got %q want %q", got, normalized)
	}
	if got := solarForecastOutlookCacheKey(req, "device", "dev-a", []string{"dev-a"}); got == normalized {
		t.Fatalf("expected scope-specific cache key to differ, but both were %q", got)
	}
}

func TestSolarForecastOutlookCacheExpiresAndReloads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	current := now
	cache := newSolarForecastOutlookCache(10*time.Second, func() time.Time { return current })
	var calls atomic.Int32
	key := "solar:cache:test"
	base := testSolarForecastResponse(now)

	resp1, err := cache.Get(context.Background(), key, func(context.Context) (*solarforecastv1.GetSolarOutlookResponse, error) {
		calls.Add(1)
		return base, nil
	})
	if err != nil {
		t.Fatalf("first cache get error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader calls after first get = %d, want 1", got)
	}
	resp1.GetScope().ResolvedDeviceIds[0] = "mutated"

	current = current.Add(5 * time.Second)
	resp2, err := cache.Get(context.Background(), key, func(context.Context) (*solarforecastv1.GetSolarOutlookResponse, error) {
		calls.Add(1)
		return base, nil
	})
	if err != nil {
		t.Fatalf("second cache get error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loader calls before expiry = %d, want 1", got)
	}
	if resp2.GetScope().GetResolvedDeviceIds()[0] != "device-a" {
		t.Fatalf("cached response was mutated, got %q want %q", resp2.GetScope().GetResolvedDeviceIds()[0], "device-a")
	}

	current = current.Add(6 * time.Second)
	resp3, err := cache.Get(context.Background(), key, func(context.Context) (*solarforecastv1.GetSolarOutlookResponse, error) {
		calls.Add(1)
		return base, nil
	})
	if err != nil {
		t.Fatalf("third cache get error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("loader calls after expiry = %d, want 2", got)
	}
	if resp3.GetScope().GetResolvedDeviceIds()[0] != "device-a" {
		t.Fatalf("refreshed cache response mismatch, got %q want %q", resp3.GetScope().GetResolvedDeviceIds()[0], "device-a")
	}
}

func sampleSolarForecastOutlook(t *testing.T) *solarforecastd.Outlook {
	t.Helper()

	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	peak := 4120.5
	observed := 3900.25
	actualKWh := 12.4
	remainingKWh := 7.8
	totalKWh := 20.2
	peakTime := now.Add(2 * time.Hour)
	calibrationUpdatedAt := now.Add(-5 * time.Minute)
	return &solarforecastd.Outlook{
		Scope: solarforecastd.Scope{
			Mode:              "all",
			DeviceID:          "",
			ResolvedDeviceIDs: []string{"device-a", "device-b"},
		},
		Provenance: solarforecastd.Provenance{
			ForecastSource:            "solarforecastd",
			ForecastModel:             "deterministic_baseline_v1",
			ServedVariant:             "baseline",
			BaselineModel:             "deterministic_baseline_v1",
			CalibrationApplied:        true,
			CalibrationSampleCount:    3,
			CalibrationUpdatedAt:      &calibrationUpdatedAt,
			SameDayCurtailmentApplied: false,
			ActualsSource:             "telemetry_rollups",
			WeatherSource:             "open_meteo",
			WeatherModelSelection:     "best_match",
			Timezone:                  "UTC",
			CanonicalLocationKey:      "42.600000|-77.400000",
			IssuedAt:                  now,
			RefreshedAt:               now,
		},
		Capacity: solarforecastd.CapacityEstimate{
			EstimatedPeakWatts: &peak,
			ObservedPvWatts:    &observed,
			Method:             "calibrated",
		},
		Today: solarforecastd.GenerationDay{
			Date:                 now,
			ActualGeneratedKWh:   &actualKWh,
			ForecastRemainingKWh: &remainingKWh,
			ForecastTotalKWh:     &totalKWh,
			EstimatedPeakWatts:   &peak,
			PeakTime:             &peakTime,
			Confidence:           solarforecastd.ConfidenceHigh,
		},
		Next7Days: []solarforecastd.GenerationDay{
			{
				Date:                 now.Add(24 * time.Hour),
				ActualGeneratedKWh:   &actualKWh,
				ForecastRemainingKWh: &remainingKWh,
				ForecastTotalKWh:     &totalKWh,
				EstimatedPeakWatts:   &peak,
				PeakTime:             &peakTime,
				Confidence:           solarforecastd.ConfidenceMedium,
			},
		},
		Next24Hours: []solarforecastd.GenerationPoint{
			{
				Time:                   now,
				ActualGeneratedWh:      &observed,
				ForecastGeneratedWh:    &remainingKWh,
				EstimatedPeakWatts:     &peak,
				ShortwaveRadiation:     &observed,
				GlobalTiltedIrradiance: &observed,
				CloudCover:             &observed,
				Confidence:             solarforecastd.ConfidenceLow,
			},
		},
	}
}

func testSolarForecastResponse(now time.Time) *solarforecastv1.GetSolarOutlookResponse {
	peak := 4120.5
	observed := 3900.25
	actualKWh := 12.4
	remainingKWh := 7.8
	totalKWh := 20.2
	peakTime := now.Add(2 * time.Hour)
	return &solarforecastv1.GetSolarOutlookResponse{
		Scope: &solarforecastv1.SolarForecastScope{
			Mode:              "all",
			DeviceId:          "",
			ResolvedDeviceIds: []string{"device-a", "device-b"},
		},
		Provenance: &solarforecastv1.SolarForecastProvenance{
			ForecastSource:             "solarforecastd",
			ForecastModel:              "deterministic_baseline_v1",
			ServedVariant:              "baseline",
			BaselineModel:              "deterministic_baseline_v1",
			CalibrationApplied:         true,
			CalibrationSampleCount:     3,
			CalibrationUpdatedAtUnixMs: now.Add(-5 * time.Minute).UTC().UnixMilli(),
			SameDayCurtailmentApplied:  false,
			ActualsSource:              "telemetry_rollups",
			WeatherSource:              "open_meteo",
			WeatherModelSelection:      "best_match",
			Timezone:                   "UTC",
			CanonicalLocationKey:       "42.600000|-77.400000",
			IssuedAtUnixMs:             now.UTC().UnixMilli(),
			RefreshedAtUnixMs:          now.UTC().UnixMilli(),
		},
		Capacity: &solarforecastv1.SolarCapacityEstimate{
			EstimatedPeakWatts: wrapperspb.Double(peak),
			ObservedPvWatts:    wrapperspb.Double(observed),
			Method:             "calibrated",
		},
		Today: &solarforecastv1.SolarGenerationDay{
			DateUnixMs:           now.UTC().UnixMilli(),
			ActualGeneratedKwh:   wrapperspb.Double(actualKWh),
			ForecastRemainingKwh: wrapperspb.Double(remainingKWh),
			ForecastTotalKwh:     wrapperspb.Double(totalKWh),
			EstimatedPeakWatts:   wrapperspb.Double(peak),
			PeakTimeUnixMs:       peakTime.UTC().UnixMilli(),
			Confidence:           solarforecastv1.SolarForecastConfidence_SOLAR_FORECAST_CONFIDENCE_HIGH,
		},
		Next_7Days: []*solarforecastv1.SolarGenerationDay{
			{
				DateUnixMs:           now.Add(24 * time.Hour).UTC().UnixMilli(),
				ActualGeneratedKwh:   wrapperspb.Double(actualKWh),
				ForecastRemainingKwh: wrapperspb.Double(remainingKWh),
				ForecastTotalKwh:     wrapperspb.Double(totalKWh),
				EstimatedPeakWatts:   wrapperspb.Double(peak),
				PeakTimeUnixMs:       peakTime.UTC().UnixMilli(),
				Confidence:           solarforecastv1.SolarForecastConfidence_SOLAR_FORECAST_CONFIDENCE_MEDIUM,
			},
		},
		Next_24Hours: []*solarforecastv1.SolarGenerationPoint{
			{
				TimeUnixMs:             now.UTC().UnixMilli(),
				ActualGeneratedWh:      wrapperspb.Double(observed),
				ForecastGeneratedWh:    wrapperspb.Double(remainingKWh),
				EstimatedPeakWatts:     wrapperspb.Double(peak),
				ShortwaveRadiation:     wrapperspb.Double(observed),
				GlobalTiltedIrradiance: wrapperspb.Double(observed),
				CloudCover:             wrapperspb.Double(observed),
				Confidence:             solarforecastv1.SolarForecastConfidence_SOLAR_FORECAST_CONFIDENCE_LOW,
			},
		},
	}
}

func testSolarForecastLocation() *weatherv1.WeatherLocationRequest {
	return &weatherv1.WeatherLocationRequest{
		Latitude:            42.6,
		Longitude:           -77.4,
		UnitSystem:          weatherv1.UnitSystem_UNIT_SYSTEM_METRIC,
		PanelTiltDegrees:    wrapperspb.Double(45),
		PanelAzimuthDegrees: wrapperspb.Double(180),
		Timezone:            "UTC",
	}
}

func solarFloatPtr(value float64) *float64 {
	return &value
}
