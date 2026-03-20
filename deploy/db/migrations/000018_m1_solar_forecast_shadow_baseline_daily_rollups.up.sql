ALTER TABLE solar_forecast_verification_daily
    ADD COLUMN IF NOT EXISTS baseline_daily_abs_error_wh_sum DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS baseline_peak_power_abs_error_w_sum DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS baseline_peak_time_abs_error_minutes_sum DOUBLE PRECISION NOT NULL DEFAULT 0;
