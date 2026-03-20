CREATE TABLE IF NOT EXISTS solar_forecast_calibration_state (
    site_key TEXT NOT NULL,
    forecast_version TEXT NOT NULL,
    horizon_bucket TEXT NOT NULL,
    hour_of_day SMALLINT NOT NULL,
    sample_count INTEGER NOT NULL,
    multiplicative_ratio DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_key, forecast_version, horizon_bucket, hour_of_day),
    CONSTRAINT chk_solar_forecast_calibration_site_key_nonempty CHECK (btrim(site_key) <> ''),
    CONSTRAINT chk_solar_forecast_calibration_version_nonempty CHECK (btrim(forecast_version) <> ''),
    CONSTRAINT chk_solar_forecast_calibration_horizon_bucket CHECK (horizon_bucket IN ('same_day', 'day_1', 'day_3', 'day_7')),
    CONSTRAINT chk_solar_forecast_calibration_hour CHECK (hour_of_day BETWEEN 0 AND 23),
    CONSTRAINT chk_solar_forecast_calibration_sample_count_nonnegative CHECK (sample_count >= 0),
    CONSTRAINT chk_solar_forecast_calibration_ratio_positive CHECK (multiplicative_ratio > 0)
);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_calibration_updated_at
    ON solar_forecast_calibration_state (updated_at DESC);
