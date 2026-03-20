ALTER TABLE solar_forecast_hourly_training_records
    DROP COLUMN IF EXISTS baseline_squared_error_wh2,
    DROP COLUMN IF EXISTS baseline_absolute_error_wh,
    DROP COLUMN IF EXISTS baseline_forecast_generation_wh;
