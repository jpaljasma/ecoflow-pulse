ALTER TABLE telemetry_rollup_minute
    ADD COLUMN IF NOT EXISTS ac_output_avg_w DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS ac_output_max_w DOUBLE PRECISION;

ALTER TABLE telemetry_rollup_hour
    ADD COLUMN IF NOT EXISTS ac_output_avg_w DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS ac_output_max_w DOUBLE PRECISION;

ALTER TABLE telemetry_rollup_day
    ADD COLUMN IF NOT EXISTS ac_output_avg_w DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS ac_output_max_w DOUBLE PRECISION;
