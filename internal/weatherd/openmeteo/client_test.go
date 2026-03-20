package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

func TestFetchForecastRequestsConfiguredFieldsAndParsesNullableValues(t *testing.T) {
	var captured url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.Query()
		_, _ = w.Write([]byte(`{
  "latitude": 42.5,
  "longitude": -77.5,
  "elevation": 300,
  "timezone": "America/New_York",
  "timezone_abbreviation": "EDT",
  "hourly": {
    "time": ["2026-03-17T00:00"],
    "weather_code": [2],
    "temperature_2m": [10.5],
    "wind_speed_10m": [12.2],
    "wind_direction_10m": [185],
    "precipitation": [0.3],
    "cloud_cover": [45],
    "visibility": [14000],
    "sunshine_duration": [1200],
    "shortwave_radiation": [250],
    "uv_index": [null],
    "global_tilted_irradiance": [410]
  },
  "daily": {
    "time": ["2026-03-17"],
    "weather_code": [2],
    "sunrise": ["2026-03-17T07:11"],
    "sunset": ["2026-03-17T19:15"],
    "daylight_duration": [43560],
    "sunshine_duration": [25000],
    "shortwave_radiation_sum": [12.5],
    "uv_index_max": [5.3]
  }
}`))
	}))
	defer server.Close()

	tilt := 45.0
	azimuth := 182.0
	client := NewClient(Config{
		ForecastBaseURL: server.URL,
		HTTPClient:      server.Client(),
	})
	bundle, err := client.FetchForecast(context.Background(), weatherd.Request{
		Latitude:            42.6,
		Longitude:           -77.4,
		PanelTiltDegrees:    &tilt,
		PanelAzimuthDegrees: &azimuth,
		Timezone:            "America/New_York",
	})
	if err != nil {
		t.Fatalf("FetchForecast() error = %v", err)
	}

	if got := captured.Get("forecast_days"); got != "7" {
		t.Fatalf("forecast_days = %q, want 7", got)
	}
	if got := captured.Get("past_days"); got != "1" {
		t.Fatalf("past_days = %q, want 1", got)
	}
	if got := captured.Get("timezone"); got != "America/New_York" {
		t.Fatalf("timezone = %q, want America/New_York", got)
	}
	if got := captured.Get("tilt"); got != "45" {
		t.Fatalf("tilt = %q, want 45", got)
	}
	if got := captured.Get("azimuth"); got != "180" {
		t.Fatalf("azimuth = %q, want 180", got)
	}
	hourly := captured.Get("hourly")
	for _, field := range []string{
		"weather_code",
		"temperature_2m",
		"wind_speed_10m",
		"wind_direction_10m",
		"precipitation",
		"cloud_cover",
		"visibility",
		"sunshine_duration",
		"shortwave_radiation",
		"uv_index",
		"global_tilted_irradiance",
	} {
		if !strings.Contains(hourly, field) {
			t.Fatalf("hourly fields missing %q in %q", field, hourly)
		}
	}

	if bundle.Provenance.TimezoneAbbreviation != "EDT" {
		t.Fatalf("timezone abbreviation = %q, want EDT", bundle.Provenance.TimezoneAbbreviation)
	}
	if len(bundle.Hourly) != 1 || bundle.Hourly[0].Raw.UVIndex != nil {
		t.Fatalf("expected nullable uv_index to stay nil, got %+v", bundle.Hourly)
	}
	if got := bundle.Hourly[0].Condition.WeatherText; got != "Partly cloudy" {
		t.Fatalf("weather text = %q, want Partly cloudy", got)
	}
}
