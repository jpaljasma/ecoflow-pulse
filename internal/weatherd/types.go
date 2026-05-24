package weatherd

import (
	"context"
	"time"
)

type UnitSystem string

const (
	UnitSystemMetric   UnitSystem = "metric"
	UnitSystemImperial UnitSystem = "imperial"
)

type Request struct {
	Latitude            float64
	Longitude           float64
	UnitSystem          UnitSystem
	PanelTiltDegrees    *float64
	PanelAzimuthDegrees *float64
	Timezone            string
}

func (r Request) Normalized() Request {
	out := r
	if out.UnitSystem == "" {
		out.UnitSystem = UnitSystemMetric
	}
	if out.Timezone == "" {
		out.Timezone = "auto"
	}
	return out
}

type ForecastValueSet struct {
	Temperature             *float64 `json:"temperature,omitempty"`
	WindSpeed               *float64 `json:"wind_speed,omitempty"`
	WindDirectionDegrees    *float64 `json:"wind_direction_degrees,omitempty"`
	Precipitation           *float64 `json:"precipitation,omitempty"`
	CloudCover              *float64 `json:"cloud_cover,omitempty"`
	Visibility              *float64 `json:"visibility,omitempty"`
	SunshineDurationSeconds *float64 `json:"sunshine_duration_seconds,omitempty"`
	ShortwaveRadiation      *float64 `json:"shortwave_radiation,omitempty"`
	UVIndex                 *float64 `json:"uv_index,omitempty"`
	GlobalTiltedIrradiance  *float64 `json:"global_tilted_irradiance,omitempty"`
}

type DailyValueSet struct {
	SunshineDurationSeconds *float64 `json:"sunshine_duration_seconds,omitempty"`
	ShortwaveRadiationSum   *float64 `json:"shortwave_radiation_sum,omitempty"`
	UVIndexMax              *float64 `json:"uv_index_max,omitempty"`
}

type WeatherCondition struct {
	WeatherCode int32  `json:"weather_code"`
	WeatherText string `json:"weather_text"`
}

type HourlyForecastPoint struct {
	Time      time.Time        `json:"time"`
	Condition WeatherCondition `json:"condition"`
	Raw       ForecastValueSet `json:"raw"`
	Corrected ForecastValueSet `json:"corrected"`
}

type DailyForecastPoint struct {
	Date                    time.Time        `json:"date"`
	Condition               WeatherCondition `json:"condition"`
	Sunrise                 time.Time        `json:"sunrise"`
	Sunset                  time.Time        `json:"sunset"`
	DaylightDurationSeconds *float64         `json:"daylight_duration_seconds,omitempty"`
	Raw                     DailyValueSet    `json:"raw"`
	Corrected               DailyValueSet    `json:"corrected"`
}

type Provenance struct {
	Source               string    `json:"source"`
	ModelSelection       string    `json:"model_selection"`
	ActualSource         string    `json:"actual_source"`
	VerificationSource   string    `json:"verification_source,omitempty"`
	Timezone             string    `json:"timezone"`
	TimezoneAbbreviation string    `json:"timezone_abbreviation,omitempty"`
	CanonicalLocationKey string    `json:"canonical_location_key"`
	IssuedAt             time.Time `json:"issued_at"`
	Latitude             float64   `json:"latitude"`
	Longitude            float64   `json:"longitude"`
	Elevation            float64   `json:"elevation"`
}

type Bundle struct {
	Provenance Provenance            `json:"provenance"`
	Hourly     []HourlyForecastPoint `json:"hourly"`
	Daily      []DailyForecastPoint  `json:"daily"`
}

type RefreshCandidate struct {
	CanonicalLocationKey string     `json:"canonical_location_key"`
	Request              Request    `json:"request"`
	LastRequestedAt      time.Time  `json:"last_requested_at"`
	LastRefreshedAt      *time.Time `json:"last_refreshed_at,omitempty"`
	NextRefreshAt        *time.Time `json:"next_refresh_at,omitempty"`
}

type CachedBundle struct {
	Bundle     Bundle    `json:"bundle"`
	CachedAt   time.Time `json:"cached_at"`
	StaleAfter time.Time `json:"stale_after"`
}

type HotCache interface {
	GetForecast(ctx context.Context, key string) (*CachedBundle, error)
	PutForecast(ctx context.Context, key string, bundle Bundle, ttl time.Duration) error
}

type SnapshotStore interface {
	SaveForecastBundle(ctx context.Context, req Request, bundle Bundle) error
	LatestBundle(ctx context.Context, canonicalLocationKey string) (*Bundle, error)
	LatestBundleBefore(ctx context.Context, canonicalLocationKey string, before time.Time) (*Bundle, error)
	FindCanonicalLocationKeyByRequest(ctx context.Context, req Request) (string, error)
	TouchRefreshCandidate(ctx context.Context, canonicalLocationKey string, req Request, requestedAt time.Time) error
	ListRecentRefreshCandidates(ctx context.Context, since time.Time) ([]RefreshCandidate, error)
	ListDueRefreshCandidates(ctx context.Context, since, dueBefore time.Time) ([]RefreshCandidate, error)
	MarkRefreshCandidateRefreshed(ctx context.Context, canonicalLocationKey string, refreshedAt, nextRefreshAt time.Time) error
	Close() error
}
