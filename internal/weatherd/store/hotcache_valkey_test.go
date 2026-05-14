package store

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	valkey "github.com/valkey-io/valkey-go"
)

func TestValkeyHotCacheSharesForecastsAcrossInstances(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	writerClient := newWeatherValkeyClient(t, server)
	readerClient := newWeatherValkeyClient(t, server)
	now := time.Date(2026, time.May, 14, 9, 30, 0, 0, time.UTC)
	ttl := 45 * time.Minute

	writer, err := NewValkeyHotCache(writerClient, "pulse:weather", func() time.Time { return now })
	if err != nil {
		t.Fatalf("new writer hot cache: %v", err)
	}
	reader, err := NewValkeyHotCache(readerClient, "pulse:weather", func() time.Time { return now.Add(time.Minute) })
	if err != nil {
		t.Fatalf("new reader hot cache: %v", err)
	}

	if err := writer.PutForecast(context.Background(), "home weather {north}", weatherCacheBundle(now), ttl); err != nil {
		t.Fatalf("PutForecast() error = %v", err)
	}
	got, err := reader.GetForecast(context.Background(), "home weather {north}")
	if err != nil {
		t.Fatalf("GetForecast() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected forecast cache hit")
	}
	if got.Bundle.Provenance.CanonicalLocationKey != "home-weather-north" {
		t.Fatalf("canonical location key = %q", got.Bundle.Provenance.CanonicalLocationKey)
	}
	if got.CachedAt != now {
		t.Fatalf("cached at = %s, want %s", got.CachedAt, now)
	}
	if got.StaleAfter != now.Add(ttl) {
		t.Fatalf("stale after = %s, want %s", got.StaleAfter, now.Add(ttl))
	}
	if got, want := *got.Bundle.Hourly[0].Raw.Temperature, 18.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("temperature = %v, want %v", got, want)
	}
	for _, key := range server.Keys() {
		if strings.HasPrefix(key, "pulse:weather:{home_weather__north_}:xxh3-128:") {
			return
		}
	}
	t.Fatalf("expected cluster-ready weather cache key, got %#v", server.Keys())
}

func newWeatherValkeyClient(t *testing.T, server *miniredis.Miniredis) valkey.Client {
	t.Helper()

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{server.Addr()},
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("new valkey client: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func weatherCacheBundle(now time.Time) weatherd.Bundle {
	temperature := 18.5
	shortwave := 612.0
	sunshine := 3600.0
	return weatherd.Bundle{
		Provenance: weatherd.Provenance{
			Source:               "open-meteo",
			Timezone:             "America/New_York",
			CanonicalLocationKey: "home-weather-north",
			IssuedAt:             now,
			Latitude:             42.6,
			Longitude:            -77.4,
		},
		Hourly: []weatherd.HourlyForecastPoint{{
			Time:      now.Add(time.Hour),
			Condition: weatherd.WeatherCondition{WeatherCode: 1, WeatherText: "mainly clear"},
			Raw: weatherd.ForecastValueSet{
				Temperature:        &temperature,
				ShortwaveRadiation: &shortwave,
			},
			Corrected: weatherd.ForecastValueSet{
				Temperature:        &temperature,
				ShortwaveRadiation: &shortwave,
			},
		}},
		Daily: []weatherd.DailyForecastPoint{{
			Date: now,
			Raw: weatherd.DailyValueSet{
				SunshineDurationSeconds: &sunshine,
			},
			Corrected: weatherd.DailyValueSet{
				SunshineDurationSeconds: &sunshine,
			},
		}},
	}
}
