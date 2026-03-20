ALTER TABLE solar_forecast_hourly_training_records
    ADD COLUMN IF NOT EXISTS baseline_forecast_generation_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS baseline_absolute_error_wh DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS baseline_squared_error_wh2 DOUBLE PRECISION;
