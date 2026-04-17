DROP INDEX IF EXISTS idx_solar_forecast_run_daily_rollup_site_date;
ALTER TABLE IF EXISTS solar_forecast_verification_daily_run_rollup
    DROP CONSTRAINT IF EXISTS solar_forecast_verification_daily_run_rollup_pkey;

DROP INDEX IF EXISTS idx_solar_forecast_serving_state_updated_at;
ALTER TABLE IF EXISTS solar_forecast_site_serving_state
    DROP CONSTRAINT IF EXISTS solar_forecast_site_serving_state_pkey;

DROP INDEX IF EXISTS idx_solar_forecast_calibration_updated_at;
ALTER TABLE IF EXISTS solar_forecast_calibration_state
    DROP CONSTRAINT IF EXISTS solar_forecast_calibration_state_pkey;

DROP INDEX IF EXISTS idx_solar_forecast_verification_daily_date;
ALTER TABLE IF EXISTS solar_forecast_verification_daily
    DROP CONSTRAINT IF EXISTS solar_forecast_verification_daily_pkey;

DROP INDEX IF EXISTS idx_solar_forecast_hourly_rollup_cover;
DROP INDEX IF EXISTS idx_solar_forecast_hourly_pending_claim_lookup;
DROP INDEX IF EXISTS idx_solar_forecast_hourly_verified_site_local_date;
DROP INDEX IF EXISTS idx_solar_forecast_hourly_pending_lookup;
DROP INDEX IF EXISTS idx_solar_forecast_hourly_verified_at;
DROP INDEX IF EXISTS idx_solar_forecast_hourly_site_target_time;
ALTER TABLE IF EXISTS solar_forecast_hourly_training_records
    DROP CONSTRAINT IF EXISTS solar_forecast_hourly_training_records_pkey;

DROP INDEX IF EXISTS idx_solar_forecast_runs_site_local_date_issued;
DROP INDEX IF EXISTS idx_solar_forecast_runs_scope_issued_at;
DROP INDEX IF EXISTS idx_solar_forecast_runs_location_issued_at;
ALTER TABLE IF EXISTS solar_forecast_runs
    DROP CONSTRAINT IF EXISTS uq_solar_forecast_runs_site_issued_version;
ALTER TABLE IF EXISTS solar_forecast_runs
    DROP CONSTRAINT IF EXISTS solar_forecast_runs_pkey;
