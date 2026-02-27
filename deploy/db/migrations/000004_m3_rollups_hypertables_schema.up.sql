-- M3 rollups schema (v1)
-- Scope:
--   telemetry_rollup_minute
--   telemetry_rollup_hour
--   telemetry_rollup_day
--
-- Conventions:
--   - UTC timestamps are app-managed (`created_at`, `updated_at`)
--   - rollups are keyed by provider + provider_device_id + bucket_start
--   - TimescaleDB hypertables partitioned on `bucket_start`

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        RAISE EXCEPTION 'timescaledb extension is required for M3 rollup hypertables';
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS telemetry_rollup_minute (
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    device_id UUID NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL,
    first_ts_unix_ms BIGINT NOT NULL,
    last_ts_unix_ms BIGINT NOT NULL,
    soc_avg_pct DOUBLE PRECISION,
    soc_min_pct DOUBLE PRECISION,
    soc_max_pct DOUBLE PRECISION,
    ac_in_avg_w DOUBLE PRECISION,
    ac_in_max_w DOUBLE PRECISION,
    pv_avg_w DOUBLE PRECISION,
    pv_max_w DOUBLE PRECISION,
    dc_avg_w DOUBLE PRECISION,
    dc_max_w DOUBLE PRECISION,
    load_avg_w DOUBLE PRECISION,
    load_max_w DOUBLE PRECISION,
    net_avg_w DOUBLE PRECISION,
    net_min_w DOUBLE PRECISION,
    net_max_w DOUBLE PRECISION,
    battery_avg_w DOUBLE PRECISION,
    battery_min_w DOUBLE PRECISION,
    battery_max_w DOUBLE PRECISION,
    temp_avg_c DOUBLE PRECISION,
    temp_min_c DOUBLE PRECISION,
    temp_max_c DOUBLE PRECISION,
    solar_generated_wh DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_rollup_minute PRIMARY KEY (provider, provider_device_id, bucket_start),
    CONSTRAINT chk_rollup_minute_provider_nonempty CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_rollup_minute_provider_device_id_nonempty CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_rollup_minute_sample_count_positive CHECK (sample_count > 0),
    CONSTRAINT chk_rollup_minute_ts_order CHECK (first_ts_unix_ms <= last_ts_unix_ms)
);

SELECT create_hypertable(
    'telemetry_rollup_minute',
    'bucket_start',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day'
);

CREATE INDEX IF NOT EXISTS idx_rollup_minute_device_bucket
    ON telemetry_rollup_minute (device_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_minute_provider_bucket
    ON telemetry_rollup_minute (provider, provider_device_id, bucket_start DESC);

CREATE TABLE IF NOT EXISTS telemetry_rollup_hour (
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    device_id UUID NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL,
    first_ts_unix_ms BIGINT NOT NULL,
    last_ts_unix_ms BIGINT NOT NULL,
    soc_avg_pct DOUBLE PRECISION,
    soc_min_pct DOUBLE PRECISION,
    soc_max_pct DOUBLE PRECISION,
    ac_in_avg_w DOUBLE PRECISION,
    ac_in_max_w DOUBLE PRECISION,
    pv_avg_w DOUBLE PRECISION,
    pv_max_w DOUBLE PRECISION,
    dc_avg_w DOUBLE PRECISION,
    dc_max_w DOUBLE PRECISION,
    load_avg_w DOUBLE PRECISION,
    load_max_w DOUBLE PRECISION,
    net_avg_w DOUBLE PRECISION,
    net_min_w DOUBLE PRECISION,
    net_max_w DOUBLE PRECISION,
    battery_avg_w DOUBLE PRECISION,
    battery_min_w DOUBLE PRECISION,
    battery_max_w DOUBLE PRECISION,
    temp_avg_c DOUBLE PRECISION,
    temp_min_c DOUBLE PRECISION,
    temp_max_c DOUBLE PRECISION,
    solar_generated_wh DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_rollup_hour PRIMARY KEY (provider, provider_device_id, bucket_start),
    CONSTRAINT chk_rollup_hour_provider_nonempty CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_rollup_hour_provider_device_id_nonempty CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_rollup_hour_sample_count_positive CHECK (sample_count > 0),
    CONSTRAINT chk_rollup_hour_ts_order CHECK (first_ts_unix_ms <= last_ts_unix_ms)
);

SELECT create_hypertable(
    'telemetry_rollup_hour',
    'bucket_start',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '7 days'
);

CREATE INDEX IF NOT EXISTS idx_rollup_hour_device_bucket
    ON telemetry_rollup_hour (device_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_hour_provider_bucket
    ON telemetry_rollup_hour (provider, provider_device_id, bucket_start DESC);

CREATE TABLE IF NOT EXISTS telemetry_rollup_day (
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    device_id UUID NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL,
    first_ts_unix_ms BIGINT NOT NULL,
    last_ts_unix_ms BIGINT NOT NULL,
    soc_avg_pct DOUBLE PRECISION,
    soc_min_pct DOUBLE PRECISION,
    soc_max_pct DOUBLE PRECISION,
    ac_in_avg_w DOUBLE PRECISION,
    ac_in_max_w DOUBLE PRECISION,
    pv_avg_w DOUBLE PRECISION,
    pv_max_w DOUBLE PRECISION,
    dc_avg_w DOUBLE PRECISION,
    dc_max_w DOUBLE PRECISION,
    load_avg_w DOUBLE PRECISION,
    load_max_w DOUBLE PRECISION,
    net_avg_w DOUBLE PRECISION,
    net_min_w DOUBLE PRECISION,
    net_max_w DOUBLE PRECISION,
    battery_avg_w DOUBLE PRECISION,
    battery_min_w DOUBLE PRECISION,
    battery_max_w DOUBLE PRECISION,
    temp_avg_c DOUBLE PRECISION,
    temp_min_c DOUBLE PRECISION,
    temp_max_c DOUBLE PRECISION,
    solar_generated_wh DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_rollup_day PRIMARY KEY (provider, provider_device_id, bucket_start),
    CONSTRAINT chk_rollup_day_provider_nonempty CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_rollup_day_provider_device_id_nonempty CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_rollup_day_sample_count_positive CHECK (sample_count > 0),
    CONSTRAINT chk_rollup_day_ts_order CHECK (first_ts_unix_ms <= last_ts_unix_ms)
);

SELECT create_hypertable(
    'telemetry_rollup_day',
    'bucket_start',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '30 days'
);

CREATE INDEX IF NOT EXISTS idx_rollup_day_device_bucket
    ON telemetry_rollup_day (device_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_day_provider_bucket
    ON telemetry_rollup_day (provider, provider_device_id, bucket_start DESC);
