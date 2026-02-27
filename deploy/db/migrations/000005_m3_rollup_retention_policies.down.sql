SELECT remove_retention_policy('public.telemetry_rollup_day', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_hour', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_minute', if_exists => TRUE);
