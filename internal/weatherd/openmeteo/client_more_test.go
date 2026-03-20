package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

func TestFetchForecastBatchSupportsMultipleCoordinatesWithSharedShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("latitude"); got != "42.6,42.7" {
			t.Fatalf("latitude query = %q, want batch coordinates", got)
		}
		if got := r.URL.Query().Get("timezone"); got != "auto" {
			t.Fatalf("timezone query = %q, want auto", got)
		}
		if strings.Contains(r.URL.Query().Get("hourly"), "global_tilted_irradiance") {
			t.Fatal("global_tilted_irradiance should be omitted when tilt is absent")
		}
		_, _ = w.Write([]byte(`[
  {
    "latitude": 42.6,
    "longitude": -77.4,
    "elevation": 290,
    "timezone": "UTC",
    "timezone_abbreviation": "UTC",
    "hourly": {"time": [], "weather_code": []},
    "daily": {"time": [], "weather_code": []}
  },
  {
    "latitude": 42.7,
    "longitude": -77.5,
    "elevation": 291,
    "timezone": "UTC",
    "timezone_abbreviation": "UTC",
    "hourly": {"time": [], "weather_code": []},
    "daily": {"time": [], "weather_code": []}
  }
]`))
	}))
	defer server.Close()

	client := NewClient(Config{
		ForecastBaseURL: server.URL,
		HTTPClient:      server.Client(),
	})
	out, err := client.FetchForecastBatch(context.Background(), []weatherd.Request{
		{Latitude: 42.6, Longitude: -77.4, Timezone: "auto"},
		{Latitude: 42.7, Longitude: -77.5, Timezone: "auto"},
	})
	if err != nil {
		t.Fatalf("FetchForecastBatch() error = %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("bundle len = %d, want 2", len(out))
	}
}
