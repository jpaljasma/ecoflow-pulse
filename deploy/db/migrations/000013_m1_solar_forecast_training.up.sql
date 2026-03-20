-- M1 solar forecast deterministic baseline runway
-- Scope: structured forecast-vs-actual training records and verification rollups

CREATE TABLE IF NOT EXISTS solar_forecast_runs (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    site_key TEXT NOT NULL,
    scope_kind TEXT NOT NULL,
    device_id UUID,
    canonical_location_key TEXT NOT NULL,
    timezone TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    issue_local_date DATE NOT NULL,
    issue_local_hour SMALLINT NOT NULL,
    issue_utc_offset_minutes SMALLINT NOT NULL,
    forecast_version TEXT NOT NULL,
    feature_version TEXT NOT NULL,
    weather_snapshot_id UUID NOT NULL REFERENCES weather_forecast_snapshots(id) ON DELETE RESTRICT,
    capacity_estimate_w DOUBLE PRECISION,
    actual_so_far_wh DOUBLE PRECISION NOT NULL,
    forecast_remaining_today_wh DOUBLE PRECISION NOT NULL,
    forecast_total_today_wh DOUBLE PRECISION NOT NULL,
    site_metadata_json JSONB NOT NULL,
    provenance_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_solar_forecast_runs_scope_kind CHECK (scope_kind IN ('device')),
    CONSTRAINT chk_solar_forecast_runs_timezone_nonempty CHECK (btrim(timezone) <> ''),
    CONSTRAINT chk_solar_forecast_runs_site_key_nonempty CHECK (btrim(site_key) <> ''),
    CONSTRAINT chk_solar_forecast_runs_forecast_version_nonempty CHECK (btrim(forecast_version) <> ''),
    CONSTRAINT chk_solar_forecast_runs_feature_version_nonempty CHECK (btrim(feature_version) <> ''),
    CONSTRAINT chk_solar_forecast_runs_issue_local_hour CHECK (issue_local_hour BETWEEN 0 AND 23)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_solar_forecast_runs_site_issued_version
    ON solar_forecast_runs (site_key, issued_at, forecast_version);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_runs_location_issued_at
    ON solar_forecast_runs (canonical_location_key, issued_at DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_runs_scope_issued_at
    ON solar_forecast_runs (scope_kind, issued_at DESC);

CREATE TABLE IF NOT EXISTS solar_forecast_hourly_training_records (
    run_id UUID NOT NULL REFERENCES solar_forecast_runs(id) ON DELETE CASCADE,
    site_key TEXT NOT NULL,
    device_id UUID,
    issued_at TIMESTAMPTZ NOT NULL,
    target_time TIMESTAMPTZ NOT NULL,
    target_local_date DATE NOT NULL,
    target_local_hour SMALLINT NOT NULL,
    target_utc_offset_minutes SMALLINT NOT NULL,
    horizon_hours INTEGER NOT NULL,
    horizon_bucket TEXT NOT NULL,
    forecast_generation_wh DOUBLE PRECISION NOT NULL,
    forecast_gti_wm2 DOUBLE PRECISION,
    forecast_shortwave_wm2 DOUBLE PRECISION,
    forecast_temperature_c DOUBLE PRECISION,
    forecast_cloud_cover_pct DOUBLE PRECISION,
    forecast_irradiance_source TEXT NOT NULL,
    actual_generation_wh DOUBLE PRECISION,
    actual_gti_wm2 DOUBLE PRECISION,
    actual_shortwave_wm2 DOUBLE PRECISION,
    actual_temperature_c DOUBLE PRECISION,
    actual_cloud_cover_pct DOUBLE PRECISION,
    verification_status TEXT NOT NULL,
    signed_error_wh DOUBLE PRECISION,
    absolute_error_wh DOUBLE PRECISION,
    squared_error_wh2 DOUBLE PRECISION,
    verified_at TIMESTAMPTZ,
    feature_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    weather_raw_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    weather_corrected_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, target_time),
    CONSTRAINT chk_solar_forecast_hourly_site_key_nonempty CHECK (btrim(site_key) <> ''),
    CONSTRAINT chk_solar_forecast_hourly_target_hour CHECK (target_local_hour BETWEEN 0 AND 23),
    CONSTRAINT chk_solar_forecast_hourly_horizon_nonnegative CHECK (horizon_hours >= 0),
    CONSTRAINT chk_solar_forecast_hourly_horizon_bucket CHECK (horizon_bucket IN ('same_day', 'day_1', 'day_3', 'day_7')),
    CONSTRAINT chk_solar_forecast_hourly_irradiance_source CHECK (forecast_irradiance_source IN ('gti', 'shortwave_radiation', 'unavailable')),
    CONSTRAINT chk_solar_forecast_hourly_verification_status CHECK (verification_status IN ('pending', 'verified', 'missing_truth', 'missing_weather'))
);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_site_target_time
    ON solar_forecast_hourly_training_records (site_key, target_time DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_verified_at
    ON solar_forecast_hourly_training_records (verified_at DESC NULLS LAST);

CREATE TABLE IF NOT EXISTS solar_forecast_verification_daily (
    site_key TEXT NOT NULL,
    device_id UUID,
    verification_local_date DATE NOT NULL,
    timezone TEXT NOT NULL,
    forecast_version TEXT NOT NULL,
    horizon_bucket TEXT NOT NULL,
    forecast_hours INTEGER NOT NULL,
    verified_hours INTEGER NOT NULL,
    missing_truth_hours INTEGER NOT NULL,
    missing_weather_hours INTEGER NOT NULL,
    hourly_abs_error_wh_sum DOUBLE PRECISION NOT NULL,
    hourly_sq_error_wh2_sum DOUBLE PRECISION NOT NULL,
    daily_abs_error_wh_sum DOUBLE PRECISION NOT NULL,
    peak_power_abs_error_w_sum DOUBLE PRECISION NOT NULL,
    peak_time_abs_error_minutes_sum DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_key, verification_local_date, forecast_version, horizon_bucket),
    CONSTRAINT chk_solar_forecast_verification_site_key_nonempty CHECK (btrim(site_key) <> ''),
    CONSTRAINT chk_solar_forecast_verification_timezone_nonempty CHECK (btrim(timezone) <> ''),
    CONSTRAINT chk_solar_forecast_verification_version_nonempty CHECK (btrim(forecast_version) <> ''),
    CONSTRAINT chk_solar_forecast_verification_horizon_bucket CHECK (horizon_bucket IN ('same_day', 'day_1', 'day_3', 'day_7')),
    CONSTRAINT chk_solar_forecast_verification_counts_nonnegative CHECK (
        forecast_hours >= 0 AND
        verified_hours >= 0 AND
        missing_truth_hours >= 0 AND
        missing_weather_hours >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_verification_daily_date
    ON solar_forecast_verification_daily (verification_local_date DESC);
