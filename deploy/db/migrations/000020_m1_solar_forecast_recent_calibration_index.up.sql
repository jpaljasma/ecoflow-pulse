CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_recent_calibration
    ON solar_forecast_hourly_training_records (
        site_key,
        target_local_date DESC,
        target_time ASC,
        issued_at DESC
    )
    INCLUDE (run_id)
    WHERE verification_status = 'verified'
      AND actual_generation_wh IS NOT NULL;
