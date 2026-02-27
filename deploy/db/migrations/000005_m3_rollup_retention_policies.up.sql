-- M3 rollup retention policies (v1)
-- Scope:
--   telemetry_rollup_minute  -> 90 days
--   telemetry_rollup_hour    -> 3 years
--   telemetry_rollup_day     -> 3 years

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        RAISE EXCEPTION 'timescaledb extension is required for M3 rollup retention policies';
    END IF;
END
$$;

SELECT remove_retention_policy('public.telemetry_rollup_minute', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_hour', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_day', if_exists => TRUE);

SELECT add_retention_policy(
    'public.telemetry_rollup_minute',
    drop_after => INTERVAL '90 days',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'public.telemetry_rollup_hour',
    drop_after => INTERVAL '3 years',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'public.telemetry_rollup_day',
    drop_after => INTERVAL '3 years',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);
