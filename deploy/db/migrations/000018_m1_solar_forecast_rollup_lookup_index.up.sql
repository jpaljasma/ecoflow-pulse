CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_rollup_lookup
    ON solar_forecast_hourly_training_records (site_key, target_local_date, target_time)
    INCLUDE (run_id);
