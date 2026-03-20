package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	weatherv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/weather/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakeUpstream struct {
	forecast *weatherd.Bundle
}

func (f *fakeUpstream) FetchForecast(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	return cloneBundle(f.forecast), nil
}

func (f *fakeUpstream) FetchForecastBatch(_ context.Context, _ []weatherd.Request) ([]weatherd.Bundle, error) {
	return nil, nil
}

func (f *fakeUpstream) FetchPreviousRuns(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	return nil, nil
}

func (f *fakeUpstream) FetchHistoricalForecast(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	return nil, nil
}

func TestWeatherServiceRejectsInvalidLatitude(t *testing.T) {
	svc := NewWeatherServiceWithDeps(WeatherServiceDeps{})

	_, err := svc.Get7DayForecast(context.Background(), &weatherv1.Get7DayForecastRequest{
		Location: &weatherv1.WeatherLocationRequest{
			Latitude:  200,
			Longitude: -77,
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestWeatherServiceMapsDomainBundleToProto(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	upstream := &fakeUpstream{
		forecast: sampleBundle(now),
	}
	domain, err := weatherd.NewService(
		upstream,
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 10, PerMinuteLimit: 10, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc := NewWeatherServiceWithDeps(WeatherServiceDeps{Service: domain})

	resp, err := svc.Get7DayForecast(context.Background(), &weatherv1.Get7DayForecastRequest{
		Location: &weatherv1.WeatherLocationRequest{
			Latitude:         42.6,
			Longitude:        -77.4,
			UnitSystem:       weatherv1.UnitSystem_UNIT_SYSTEM_METRIC,
			PanelTiltDegrees: wrapperspb.Double(45),
			Timezone:         "UTC",
		},
	})
	if err != nil {
		t.Fatalf("Get7DayForecast() error = %v", err)
	}
	if resp.GetProvenance().GetCanonicalLocationKey() == "" {
		t.Fatal("expected canonical location key in proto response")
	}
	if len(resp.GetHourly()) != 1 {
		t.Fatalf("hourly len = %d, want 1", len(resp.GetHourly()))
	}
	if got := resp.GetHourly()[0].GetCondition().GetWeatherText(); got != "Partly cloudy" {
		t.Fatalf("weather text = %q, want Partly cloudy", got)
	}
}

func TestWeatherServiceMapsYesterdayVerificationToProto(t *testing.T) {
	now := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	cache := store.NewMemoryHotCache(func() time.Time { return now })
	snapshots := store.NewMemorySnapshotStore(func() time.Time { return now })
	req := weatherd.Request{
		Latitude:   42.6,
		Longitude:  -77.4,
		UnitSystem: weatherd.UnitSystemMetric,
		Timezone:   "UTC",
	}
	actual := sampleBundle(now)
	actual.Hourly[0].Time = time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	prior := sampleBundle(time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC))
	prior.Hourly[0].Time = actual.Hourly[0].Time
	if err := snapshots.SaveForecastBundle(context.Background(), req, *actual); err != nil {
		t.Fatalf("SaveForecastBundle(actual) error = %v", err)
	}
	if err := snapshots.SaveForecastBundle(context.Background(), req, *prior); err != nil {
		t.Fatalf("SaveForecastBundle(prior) error = %v", err)
	}
	domain, err := weatherd.NewService(
		&fakeUpstream{},
		cache,
		snapshots,
		budget.New(budget.Config{DailyLimit: 10, PerMinuteLimit: 10, NowFn: func() time.Time { return now }}),
		weatherd.Config{HotTTL: 50 * time.Minute, NowFn: func() time.Time { return now }},
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	svc := NewWeatherServiceWithDeps(WeatherServiceDeps{Service: domain})

	resp, err := svc.GetYesterdayVerification(context.Background(), &weatherv1.GetYesterdayVerificationRequest{
		Location: &weatherv1.WeatherLocationRequest{
			Latitude:   42.6,
			Longitude:  -77.4,
			UnitSystem: weatherv1.UnitSystem_UNIT_SYSTEM_METRIC,
			Timezone:   "UTC",
		},
	})
	if err != nil {
		t.Fatalf("GetYesterdayVerification() error = %v", err)
	}
	if got := resp.GetProvenance().GetVerificationSource(); got != "snapshot" {
		t.Fatalf("verification source = %q, want snapshot", got)
	}
	if len(resp.GetHourly()) != 1 {
		t.Fatalf("hourly len = %d, want 1", len(resp.GetHourly()))
	}
}

func TestNewWeatherDomainFromEnvUsesFallbackStores(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "")
	t.Setenv("VALKEY_ADDRS", "")

	svc, cleanup, err := newWeatherDomainFromEnv(slog.Default(), nil)
	if err != nil {
		t.Fatalf("newWeatherDomainFromEnv() error = %v", err)
	}
	defer cleanup()
	if svc == nil {
		t.Fatal("expected weather domain service")
	}
}

func TestStartWeatherRefreshLoopCanBeDisabled(t *testing.T) {
	t.Setenv("WEATHER_REFRESH_INTERVAL", "0s")
	stop := startWeatherRefreshLoop(context.Background(), slog.Default(), nil)
	stop()
}

func sampleBundle(now time.Time) *weatherd.Bundle {
	temp := 10.0
	return &weatherd.Bundle{
		Provenance: weatherd.Provenance{
			Source:               "open_meteo",
			ModelSelection:       "best_match",
			ActualSource:         "past_days",
			Timezone:             "UTC",
			CanonicalLocationKey: "grid-key",
			IssuedAt:             now,
			Latitude:             42.6,
			Longitude:            -77.4,
			Elevation:            290,
		},
		Hourly: []weatherd.HourlyForecastPoint{
			{
				Time: now,
				Condition: weatherd.WeatherCondition{
					WeatherCode: 2,
					WeatherText: "Partly cloudy",
				},
				Raw: weatherd.ForecastValueSet{
					Temperature: &temp,
				},
				Corrected: weatherd.ForecastValueSet{
					Temperature: &temp,
				},
			},
		},
	}
}

func cloneBundle(in *weatherd.Bundle) *weatherd.Bundle {
	if in == nil {
		return nil
	}
	out := *in
	out.Hourly = append([]weatherd.HourlyForecastPoint(nil), in.Hourly...)
	out.Daily = append([]weatherd.DailyForecastPoint(nil), in.Daily...)
	return &out
}
