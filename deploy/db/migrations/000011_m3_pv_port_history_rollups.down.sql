SELECT remove_retention_policy('public.telemetry_rollup_pv_port_minute', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_pv_port_hour', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_pv_port_day', if_exists => TRUE);

DROP TABLE IF EXISTS telemetry_rollup_pv_port_day;
DROP TABLE IF EXISTS telemetry_rollup_pv_port_hour;
DROP TABLE IF EXISTS telemetry_rollup_pv_port_minute;
