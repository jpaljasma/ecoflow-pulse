ALTER TABLE solar_forecast_runs
    ALTER COLUMN weather_snapshot_id SET NOT NULL;

ALTER TABLE solar_forecast_runs
    DROP CONSTRAINT IF EXISTS chk_solar_forecast_runs_scope_kind;

ALTER TABLE solar_forecast_runs
    ADD CONSTRAINT chk_solar_forecast_runs_scope_kind
        CHECK (scope_kind IN ('device'));
