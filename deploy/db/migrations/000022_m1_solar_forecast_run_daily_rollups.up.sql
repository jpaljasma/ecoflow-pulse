CREATE TABLE IF NOT EXISTS solar_forecast_verification_daily_run_rollup (
    run_id UUID NOT NULL REFERENCES solar_forecast_runs(id) ON DELETE CASCADE,
    site_key TEXT NOT NULL,
    device_id UUID,
    verification_local_date DATE NOT NULL,
    timezone TEXT NOT NULL,
    forecast_version TEXT NOT NULL,
    served_variant TEXT NOT NULL,
    horizon_bucket TEXT NOT NULL,
    forecast_hours INTEGER NOT NULL,
    verified_hours INTEGER NOT NULL,
    missing_truth_hours INTEGER NOT NULL,
    missing_weather_hours INTEGER NOT NULL,
    hourly_abs_error_wh_sum DOUBLE PRECISION NOT NULL,
    hourly_sq_error_wh2_sum DOUBLE PRECISION NOT NULL,
    forecast_total_wh DOUBLE PRECISION NOT NULL,
    baseline_forecast_total_wh DOUBLE PRECISION NOT NULL,
    actual_total_wh DOUBLE PRECISION NOT NULL,
    forecast_peak_wh DOUBLE PRECISION NOT NULL,
    forecast_peak_time TIMESTAMPTZ,
    baseline_forecast_peak_wh DOUBLE PRECISION NOT NULL,
    baseline_forecast_peak_time TIMESTAMPTZ,
    actual_peak_wh DOUBLE PRECISION NOT NULL,
    actual_peak_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, verification_local_date, horizon_bucket),
    CONSTRAINT chk_solar_forecast_run_daily_rollup_site_key_nonempty CHECK (btrim(site_key) <> ''),
    CONSTRAINT chk_solar_forecast_run_daily_rollup_timezone_nonempty CHECK (btrim(timezone) <> ''),
    CONSTRAINT chk_solar_forecast_run_daily_rollup_forecast_version_nonempty CHECK (btrim(forecast_version) <> ''),
    CONSTRAINT chk_solar_forecast_run_daily_rollup_served_variant CHECK (served_variant IN ('baseline', 'site_calibrated')),
    CONSTRAINT chk_solar_forecast_run_daily_rollup_horizon_bucket CHECK (horizon_bucket IN ('same_day', 'day_1', 'day_3', 'day_7')),
    CONSTRAINT chk_solar_forecast_run_daily_rollup_counts_nonnegative CHECK (
        forecast_hours >= 0 AND
        verified_hours >= 0 AND
        missing_truth_hours >= 0 AND
        missing_weather_hours >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_run_daily_rollup_site_date
    ON solar_forecast_verification_daily_run_rollup (site_key, verification_local_date, forecast_version, served_variant, horizon_bucket);
