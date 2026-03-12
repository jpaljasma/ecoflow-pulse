ALTER TABLE telemetry_rollup_day
    DROP COLUMN IF EXISTS ac_output_max_w,
    DROP COLUMN IF EXISTS ac_output_avg_w;

ALTER TABLE telemetry_rollup_hour
    DROP COLUMN IF EXISTS ac_output_max_w,
    DROP COLUMN IF EXISTS ac_output_avg_w;

ALTER TABLE telemetry_rollup_minute
    DROP COLUMN IF EXISTS ac_output_max_w,
    DROP COLUMN IF EXISTS ac_output_avg_w;
