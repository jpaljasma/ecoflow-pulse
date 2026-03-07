DELETE FROM telemetry_rollup_minute WHERE sample_count = 0;
DELETE FROM telemetry_rollup_hour WHERE sample_count = 0;
DELETE FROM telemetry_rollup_day WHERE sample_count = 0;

ALTER TABLE telemetry_rollup_minute
    DROP CONSTRAINT IF EXISTS chk_rollup_minute_sample_count_nonnegative;
ALTER TABLE telemetry_rollup_minute
    ADD CONSTRAINT chk_rollup_minute_sample_count_positive CHECK (sample_count > 0);

ALTER TABLE telemetry_rollup_hour
    DROP CONSTRAINT IF EXISTS chk_rollup_hour_sample_count_nonnegative;
ALTER TABLE telemetry_rollup_hour
    ADD CONSTRAINT chk_rollup_hour_sample_count_positive CHECK (sample_count > 0);

ALTER TABLE telemetry_rollup_day
    DROP CONSTRAINT IF EXISTS chk_rollup_day_sample_count_nonnegative;
ALTER TABLE telemetry_rollup_day
    ADD CONSTRAINT chk_rollup_day_sample_count_positive CHECK (sample_count > 0);
