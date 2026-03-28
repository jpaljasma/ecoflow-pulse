CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_pending_claim_lookup
    ON solar_forecast_hourly_training_records (target_time, run_id)
    WHERE verification_status = 'pending';

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_rollup_cover
    ON solar_forecast_hourly_training_records (site_key, target_local_date, target_time, run_id)
    INCLUDE (
        device_id,
        horizon_bucket,
        forecast_generation_wh,
        baseline_forecast_generation_wh,
        actual_generation_wh,
        verification_status,
        absolute_error_wh,
        squared_error_wh2
    );
