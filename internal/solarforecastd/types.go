package solarforecastd

import (
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type Scope struct {
	Mode              string   `json:"mode"`
	DeviceID          string   `json:"device_id,omitempty"`
	ResolvedDeviceIDs []string `json:"resolved_device_ids,omitempty"`
}

type Input struct {
	WeatherRequest        weatherd.Request `json:"weather_request"`
	Scope                 Scope            `json:"scope"`
	ResolvedDeviceIDs     []string         `json:"resolved_device_ids"`
	SiteResolvedDeviceIDs []string         `json:"site_resolved_device_ids,omitempty"`
}

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type GenerationPoint struct {
	Time                   time.Time  `json:"time"`
	ActualGeneratedWh      *float64   `json:"actual_generated_wh,omitempty"`
	ForecastGeneratedWh    *float64   `json:"forecast_generated_wh,omitempty"`
	EstimatedPeakWatts     *float64   `json:"estimated_peak_watts,omitempty"`
	ShortwaveRadiation     *float64   `json:"shortwave_radiation,omitempty"`
	GlobalTiltedIrradiance *float64   `json:"global_tilted_irradiance,omitempty"`
	CloudCover             *float64   `json:"cloud_cover,omitempty"`
	Confidence             Confidence `json:"confidence"`
}

type GenerationDay struct {
	Date                 time.Time  `json:"date"`
	ActualGeneratedKWh   *float64   `json:"actual_generated_kwh,omitempty"`
	ForecastRemainingKWh *float64   `json:"forecast_remaining_kwh,omitempty"`
	ForecastTotalKWh     *float64   `json:"forecast_total_kwh,omitempty"`
	EstimatedPeakWatts   *float64   `json:"estimated_peak_watts,omitempty"`
	PeakTime             *time.Time `json:"peak_time,omitempty"`
	Confidence           Confidence `json:"confidence"`
}

type CapacityEstimate struct {
	EstimatedPeakWatts *float64 `json:"estimated_peak_watts,omitempty"`
	ObservedPvWatts    *float64 `json:"observed_pv_watts,omitempty"`
	Method             string   `json:"method"`
}

type Provenance struct {
	ForecastSource            string     `json:"forecast_source"`
	ForecastModel             string     `json:"forecast_model"`
	ServedVariant             string     `json:"served_variant"`
	BaselineModel             string     `json:"baseline_model"`
	CalibrationApplied        bool       `json:"calibration_applied"`
	CalibrationSampleCount    int        `json:"calibration_sample_count"`
	CalibrationUpdatedAt      *time.Time `json:"calibration_updated_at,omitempty"`
	SameDayCurtailmentApplied bool       `json:"same_day_curtailment_applied"`
	SameDayCurtailmentReason  string     `json:"same_day_curtailment_reason,omitempty"`
	ActualsSource             string     `json:"actuals_source"`
	WeatherSource             string     `json:"weather_source"`
	WeatherModelSelection     string     `json:"weather_model_selection"`
	Timezone                  string     `json:"timezone"`
	CanonicalLocationKey      string     `json:"canonical_location_key"`
	IssuedAt                  time.Time  `json:"issued_at"`
	RefreshedAt               time.Time  `json:"refreshed_at"`
}

type Outlook struct {
	Scope       Scope             `json:"scope"`
	Provenance  Provenance        `json:"provenance"`
	Capacity    CapacityEstimate  `json:"capacity"`
	Today       GenerationDay     `json:"today"`
	Next7Days   []GenerationDay   `json:"next_7_days"`
	Next24Hours []GenerationPoint `json:"next_24_hours"`
}

type HorizonBucket string

const (
	HorizonBucketSameDay HorizonBucket = "same_day"
	HorizonBucketDay1    HorizonBucket = "day_1"
	HorizonBucketDay3    HorizonBucket = "day_3"
	HorizonBucketDay7    HorizonBucket = "day_7"
)

type IrradianceSource string

const (
	IrradianceSourceGTI       IrradianceSource = "gti"
	IrradianceSourceShortwave IrradianceSource = "shortwave_radiation"
	IrradianceSourceUnknown   IrradianceSource = "unavailable"
)

type VerificationStatus string

const (
	VerificationStatusPending        VerificationStatus = "pending"
	VerificationStatusVerified       VerificationStatus = "verified"
	VerificationStatusMissingTruth   VerificationStatus = "missing_truth"
	VerificationStatusMissingWeather VerificationStatus = "missing_weather"
)

type Run struct {
	ID                       string    `json:"id"`
	SiteKey                  string    `json:"site_key"`
	ScopeKind                string    `json:"scope_kind"`
	DeviceID                 *string   `json:"device_id,omitempty"`
	ServedVariant            string    `json:"served_variant"`
	CanonicalLocationKey     string    `json:"canonical_location_key"`
	Timezone                 string    `json:"timezone"`
	IssuedAt                 time.Time `json:"issued_at"`
	IssueLocalDate           time.Time `json:"issue_local_date"`
	IssueLocalHour           int       `json:"issue_local_hour"`
	IssueUTCOffsetMinutes    int       `json:"issue_utc_offset_minutes"`
	ForecastVersion          string    `json:"forecast_version"`
	FeatureVersion           string    `json:"feature_version"`
	WeatherSnapshotID        string    `json:"weather_snapshot_id"`
	CapacityEstimateW        *float64  `json:"capacity_estimate_w,omitempty"`
	ActualSoFarWh            float64   `json:"actual_so_far_wh"`
	ForecastRemainingTodayWh float64   `json:"forecast_remaining_today_wh"`
	ForecastTotalTodayWh     float64   `json:"forecast_total_today_wh"`
	SiteMetadataJSON         []byte    `json:"site_metadata_json,omitempty"`
	ProvenanceJSON           []byte    `json:"provenance_json,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type HourlyTrainingRecord struct {
	RunID                        string                    `json:"run_id"`
	SiteKey                      string                    `json:"site_key"`
	DeviceID                     *string                   `json:"device_id,omitempty"`
	IssuedAt                     time.Time                 `json:"issued_at"`
	TargetTime                   time.Time                 `json:"target_time"`
	TargetLocalDate              time.Time                 `json:"target_local_date"`
	TargetLocalHour              int                       `json:"target_local_hour"`
	TargetUTCOffsetMinutes       int                       `json:"target_utc_offset_minutes"`
	HorizonHours                 int                       `json:"horizon_hours"`
	HorizonBucket                HorizonBucket             `json:"horizon_bucket"`
	ForecastGenerationWh         float64                   `json:"forecast_generation_wh"`
	BaselineForecastGenerationWh *float64                  `json:"baseline_forecast_generation_wh,omitempty"`
	ForecastGTIWm2               *float64                  `json:"forecast_gti_wm2,omitempty"`
	ForecastShortwaveWm2         *float64                  `json:"forecast_shortwave_wm2,omitempty"`
	ForecastTemperatureC         *float64                  `json:"forecast_temperature_c,omitempty"`
	ForecastCloudCoverPct        *float64                  `json:"forecast_cloud_cover_pct,omitempty"`
	ForecastIrradianceSource     IrradianceSource          `json:"forecast_irradiance_source"`
	ActualGenerationWh           *float64                  `json:"actual_generation_wh,omitempty"`
	ActualGTIWm2                 *float64                  `json:"actual_gti_wm2,omitempty"`
	ActualShortwaveWm2           *float64                  `json:"actual_shortwave_wm2,omitempty"`
	ActualTemperatureC           *float64                  `json:"actual_temperature_c,omitempty"`
	ActualCloudCoverPct          *float64                  `json:"actual_cloud_cover_pct,omitempty"`
	VerificationStatus           VerificationStatus        `json:"verification_status"`
	SignedErrorWh                *float64                  `json:"signed_error_wh,omitempty"`
	AbsoluteErrorWh              *float64                  `json:"absolute_error_wh,omitempty"`
	SquaredErrorWh2              *float64                  `json:"squared_error_wh2,omitempty"`
	BaselineAbsoluteErrorWh      *float64                  `json:"baseline_absolute_error_wh,omitempty"`
	BaselineSquaredErrorWh2      *float64                  `json:"baseline_squared_error_wh2,omitempty"`
	VerifiedAt                   *time.Time                `json:"verified_at,omitempty"`
	FeatureSnapshotJSON          []byte                    `json:"feature_snapshot_json,omitempty"`
	WeatherRaw                   weatherd.ForecastValueSet `json:"weather_raw"`
	WeatherCorrected             weatherd.ForecastValueSet `json:"weather_corrected"`
	CreatedAt                    time.Time                 `json:"created_at"`
	UpdatedAt                    time.Time                 `json:"updated_at"`
}

type DailyVerificationRollup struct {
	SiteKey                            string        `json:"site_key"`
	DeviceID                           *string       `json:"device_id,omitempty"`
	ServedVariant                      string        `json:"served_variant"`
	VerificationLocalDate              time.Time     `json:"verification_local_date"`
	Timezone                           string        `json:"timezone"`
	ForecastVersion                    string        `json:"forecast_version"`
	HorizonBucket                      HorizonBucket `json:"horizon_bucket"`
	ForecastHours                      int           `json:"forecast_hours"`
	VerifiedHours                      int           `json:"verified_hours"`
	MissingTruthHours                  int           `json:"missing_truth_hours"`
	MissingWeatherHours                int           `json:"missing_weather_hours"`
	HourlyAbsErrorWhSum                float64       `json:"hourly_abs_error_wh_sum"`
	HourlySqErrorWh2Sum                float64       `json:"hourly_sq_error_wh2_sum"`
	DailyAbsErrorWhSum                 float64       `json:"daily_abs_error_wh_sum"`
	BaselineDailyAbsErrorWhSum         float64       `json:"baseline_daily_abs_error_wh_sum"`
	PeakPowerAbsErrorWSum              float64       `json:"peak_power_abs_error_w_sum"`
	BaselinePeakPowerAbsErrorWSum      float64       `json:"baseline_peak_power_abs_error_w_sum"`
	PeakTimeAbsErrorMinutesSum         float64       `json:"peak_time_abs_error_minutes_sum"`
	BaselinePeakTimeAbsErrorMinutesSum float64       `json:"baseline_peak_time_abs_error_minutes_sum"`
	CreatedAt                          time.Time     `json:"created_at"`
	UpdatedAt                          time.Time     `json:"updated_at"`
}

type VerificationRecord struct {
	HourlyTrainingRecord
	ForecastVersion string `json:"forecast_version"`
	ServedVariant   string `json:"served_variant"`
	Timezone        string `json:"timezone"`
}

type CalibrationState struct {
	SiteKey             string        `json:"site_key"`
	ForecastVersion     string        `json:"forecast_version"`
	HorizonBucket       HorizonBucket `json:"horizon_bucket"`
	HourOfDay           int           `json:"hour_of_day"`
	SampleCount         int           `json:"sample_count"`
	MultiplicativeRatio *float64      `json:"multiplicative_ratio,omitempty"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type ServingState struct {
	SiteKey                 string     `json:"site_key"`
	ForecastVersion         string     `json:"forecast_version"`
	Timezone                string     `json:"timezone"`
	RecentSiteRatio         *float64   `json:"recent_site_ratio,omitempty"`
	RecentSiteSampleCount   int        `json:"recent_site_sample_count"`
	RecentSiteUpdatedAt     *time.Time `json:"recent_site_updated_at,omitempty"`
	PotentialBaseEnvelopeW  *float64   `json:"potential_base_envelope_w,omitempty"`
	PotentialSaturatedW     *float64   `json:"potential_saturated_envelope_w,omitempty"`
	PotentialFinalEnvelopeW *float64   `json:"potential_final_envelope_w,omitempty"`
	QualifiedSaturatedDays  int        `json:"qualified_saturated_days"`
	QualifiedSaturatedHours int        `json:"qualified_saturated_hours"`
	HistoryFrom             *time.Time `json:"history_from,omitempty"`
	HistoryTo               *time.Time `json:"history_to,omitempty"`
	UpdatedAt               time.Time  `json:"updated_at"`
}
