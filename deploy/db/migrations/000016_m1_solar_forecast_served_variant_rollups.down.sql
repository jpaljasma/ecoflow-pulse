ALTER TABLE solar_forecast_verification_daily
    DROP CONSTRAINT IF EXISTS solar_forecast_verification_daily_pkey;

ALTER TABLE solar_forecast_verification_daily
    ADD PRIMARY KEY (site_key, verification_local_date, forecast_version, horizon_bucket);

ALTER TABLE solar_forecast_verification_daily
    DROP CONSTRAINT IF EXISTS chk_solar_forecast_verification_served_variant;

ALTER TABLE solar_forecast_verification_daily
    DROP COLUMN IF EXISTS served_variant;

ALTER TABLE solar_forecast_runs
    DROP CONSTRAINT IF EXISTS chk_solar_forecast_runs_served_variant;

ALTER TABLE solar_forecast_runs
    DROP COLUMN IF EXISTS served_variant;
