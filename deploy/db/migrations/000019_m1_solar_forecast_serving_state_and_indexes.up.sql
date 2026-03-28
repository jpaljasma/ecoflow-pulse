CREATE TABLE IF NOT EXISTS solar_forecast_site_serving_state (
    site_key TEXT NOT NULL,
    forecast_version TEXT NOT NULL,
    timezone TEXT NOT NULL,
    recent_site_ratio DOUBLE PRECISION,
    recent_site_sample_count INTEGER NOT NULL DEFAULT 0,
    recent_site_updated_at TIMESTAMPTZ,
    potential_base_envelope_w DOUBLE PRECISION,
    potential_saturated_envelope_w DOUBLE PRECISION,
    potential_final_envelope_w DOUBLE PRECISION,
    qualified_saturated_days INTEGER NOT NULL DEFAULT 0,
    qualified_saturated_hours INTEGER NOT NULL DEFAULT 0,
    history_from TIMESTAMPTZ,
    history_to TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (site_key, forecast_version),
    CONSTRAINT chk_solar_forecast_serving_state_site_key_nonempty CHECK (btrim(site_key) <> ''),
    CONSTRAINT chk_solar_forecast_serving_state_version_nonempty CHECK (btrim(forecast_version) <> ''),
    CONSTRAINT chk_solar_forecast_serving_state_timezone_nonempty CHECK (btrim(timezone) <> ''),
    CONSTRAINT chk_solar_forecast_serving_state_recent_sample_count_nonnegative CHECK (recent_site_sample_count >= 0),
    CONSTRAINT chk_solar_forecast_serving_state_saturated_days_nonnegative CHECK (qualified_saturated_days >= 0),
    CONSTRAINT chk_solar_forecast_serving_state_saturated_hours_nonnegative CHECK (qualified_saturated_hours >= 0),
    CONSTRAINT chk_solar_forecast_serving_state_recent_ratio_positive CHECK (recent_site_ratio IS NULL OR recent_site_ratio > 0),
    CONSTRAINT chk_solar_forecast_serving_state_base_envelope_nonnegative CHECK (potential_base_envelope_w IS NULL OR potential_base_envelope_w >= 0),
    CONSTRAINT chk_solar_forecast_serving_state_saturated_envelope_nonnegative CHECK (potential_saturated_envelope_w IS NULL OR potential_saturated_envelope_w >= 0),
    CONSTRAINT chk_solar_forecast_serving_state_final_envelope_nonnegative CHECK (potential_final_envelope_w IS NULL OR potential_final_envelope_w >= 0)
);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_serving_state_updated_at
    ON solar_forecast_site_serving_state (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_pending_lookup
    ON solar_forecast_hourly_training_records (verification_status, target_time ASC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_verified_site_local_date
    ON solar_forecast_hourly_training_records (site_key, verification_status, target_local_date DESC, target_time DESC, issued_at DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_runs_site_local_date_issued
    ON solar_forecast_runs (site_key, issue_local_date DESC, issued_at DESC);
