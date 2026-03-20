ALTER TABLE solar_forecast_verification_daily
    DROP COLUMN IF EXISTS baseline_peak_time_abs_error_minutes_sum,
    DROP COLUMN IF EXISTS baseline_peak_power_abs_error_w_sum,
    DROP COLUMN IF EXISTS baseline_daily_abs_error_wh_sum;
