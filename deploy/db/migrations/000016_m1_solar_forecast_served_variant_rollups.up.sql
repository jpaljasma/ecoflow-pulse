ALTER TABLE solar_forecast_runs
    ADD COLUMN IF NOT EXISTS served_variant TEXT;

UPDATE solar_forecast_runs
SET served_variant = COALESCE(NULLIF(btrim(provenance_json ->> 'served_variant'), ''), 'baseline')
WHERE served_variant IS NULL OR btrim(served_variant) = '';

ALTER TABLE solar_forecast_runs
    ALTER COLUMN served_variant SET NOT NULL;

ALTER TABLE solar_forecast_runs
    ADD CONSTRAINT chk_solar_forecast_runs_served_variant
        CHECK (served_variant IN ('baseline', 'site_calibrated'));

ALTER TABLE solar_forecast_verification_daily
    ADD COLUMN IF NOT EXISTS served_variant TEXT;

UPDATE solar_forecast_verification_daily
SET served_variant = 'baseline'
WHERE served_variant IS NULL OR btrim(served_variant) = '';

ALTER TABLE solar_forecast_verification_daily
    ALTER COLUMN served_variant SET NOT NULL;

ALTER TABLE solar_forecast_verification_daily
    ADD CONSTRAINT chk_solar_forecast_verification_served_variant
        CHECK (served_variant IN ('baseline', 'site_calibrated'));

ALTER TABLE solar_forecast_verification_daily
    DROP CONSTRAINT IF EXISTS solar_forecast_verification_daily_pkey;

ALTER TABLE solar_forecast_verification_daily
    ADD PRIMARY KEY (site_key, verification_local_date, forecast_version, served_variant, horizon_bucket);
