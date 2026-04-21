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

type MetricError struct {
	MeanAbsoluteError *float64 `json:"mean_absolute_error,omitempty"`
	Bias              *float64 `json:"bias,omitempty"`
}

type VerificationHour struct {
	Time              time.Time        `json:"time"`
	ForecastCondition WeatherCondition `json:"forecast_condition"`
	ActualCondition   WeatherCondition `json:"actual_condition"`
	ForecastRaw       ForecastValueSet `json:"forecast_raw"`
	ForecastCorrected ForecastValueSet `json:"forecast_corrected"`
	Actual            ForecastValueSet `json:"actual"`
}

type VerificationSummary struct {
	Temperature                            MetricError `json:"temperature"`
	WindSpeed                              MetricError `json:"wind_speed"`
	CloudCover                             MetricError `json:"cloud_cover"`
	Visibility                             MetricError `json:"visibility"`
	UVIndex                                MetricError `json:"uv_index"`
	ShortwaveRadiation                     MetricError `json:"shortwave_radiation"`
	GlobalTiltedIrradiance                 MetricError `json:"global_tilted_irradiance"`
	Precipitation                          MetricError `json:"precipitation"`
	CircularWindDirectionMeanAbsoluteError *float64    `json:"circular_wind_direction_mean_absolute_error,omitempty"`
}

type VerificationResult struct {
	Provenance       Provenance          `json:"provenance"`
	UnitSystem       UnitSystem          `json:"unit_system"`
	VerificationDate time.Time           `json:"verification_date"`
	Hourly           []VerificationHour  `json:"hourly"`
	Summary          VerificationSummary `json:"summary"`
}

type BiasMetric string

const (
	BiasMetricTemperature            BiasMetric = "temperature"
	BiasMetricWindSpeed              BiasMetric = "wind_speed"
	BiasMetricCloudCover             BiasMetric = "cloud_cover"
	BiasMetricVisibility             BiasMetric = "visibility"
	BiasMetricUVIndex                BiasMetric = "uv_index"
	BiasMetricShortwaveRadiation     BiasMetric = "shortwave_radiation"
	BiasMetricGlobalTiltedIrradiance BiasMetric = "global_tilted_irradiance"
)

type BiasState struct {
	CanonicalLocationKey string     `json:"canonical_location_key"`
	Metric               BiasMetric `json:"metric"`
	HourOfDay            int        `json:"hour_of_day"`
	SampleCount          int        `json:"sample_count"`
	AdditiveBias         *float64   `json:"additive_bias,omitempty"`
	MultiplicativeRatio  *float64   `json:"multiplicative_ratio,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
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
	LoadVerification(ctx context.Context, canonicalLocationKey string, verificationDate time.Time) (*VerificationResult, error)
	SaveVerification(ctx context.Context, result VerificationResult) error
	LoadBiasStates(ctx context.Context, canonicalLocationKey string) ([]BiasState, error)
	UpsertBiasStates(ctx context.Context, states []BiasState) error
	TouchRefreshCandidate(ctx context.Context, canonicalLocationKey string, req Request, requestedAt time.Time) error
	ListRecentRefreshCandidates(ctx context.Context, since time.Time) ([]RefreshCandidate, error)
	ListDueRefreshCandidates(ctx context.Context, since, dueBefore time.Time) ([]RefreshCandidate, error)
	MarkRefreshCandidateRefreshed(ctx context.Context, canonicalLocationKey string, refreshedAt, nextRefreshAt time.Time) error
	Close() error
}
