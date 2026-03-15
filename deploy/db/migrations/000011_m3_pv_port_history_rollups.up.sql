CREATE TABLE IF NOT EXISTS telemetry_rollup_pv_port_minute (
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    device_id UUID NOT NULL,
    port_id TEXT NOT NULL,
    port_label TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL,
    first_ts_unix_ms BIGINT NOT NULL,
    last_ts_unix_ms BIGINT NOT NULL,
    max_observed_volts DOUBLE PRECISION,
    max_observed_amps DOUBLE PRECISION,
    max_observed_watts DOUBLE PRECISION,
    last_observed_volts DOUBLE PRECISION,
    last_observed_amps DOUBLE PRECISION,
    last_observed_watts DOUBLE PRECISION,
    last_observed_at_unix_ms BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_rollup_pv_port_minute PRIMARY KEY (provider, provider_device_id, port_id, bucket_start),
    CONSTRAINT chk_rollup_pv_port_minute_provider_nonempty CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_rollup_pv_port_minute_provider_device_id_nonempty CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_rollup_pv_port_minute_port_id_nonempty CHECK (length(trim(port_id)) > 0),
    CONSTRAINT chk_rollup_pv_port_minute_port_label_nonempty CHECK (length(trim(port_label)) > 0),
    CONSTRAINT chk_rollup_pv_port_minute_sample_count_positive CHECK (sample_count > 0),
    CONSTRAINT chk_rollup_pv_port_minute_ts_order CHECK (first_ts_unix_ms <= last_ts_unix_ms)
);

SELECT create_hypertable(
    'telemetry_rollup_pv_port_minute',
    'bucket_start',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day'
);

CREATE INDEX IF NOT EXISTS idx_rollup_pv_port_minute_device_bucket
    ON telemetry_rollup_pv_port_minute (device_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_pv_port_minute_device_port_bucket
    ON telemetry_rollup_pv_port_minute (device_id, port_id, bucket_start DESC);

CREATE TABLE IF NOT EXISTS telemetry_rollup_pv_port_hour (
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    device_id UUID NOT NULL,
    port_id TEXT NOT NULL,
    port_label TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL,
    first_ts_unix_ms BIGINT NOT NULL,
    last_ts_unix_ms BIGINT NOT NULL,
    max_observed_volts DOUBLE PRECISION,
    max_observed_amps DOUBLE PRECISION,
    max_observed_watts DOUBLE PRECISION,
    last_observed_volts DOUBLE PRECISION,
    last_observed_amps DOUBLE PRECISION,
    last_observed_watts DOUBLE PRECISION,
    last_observed_at_unix_ms BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_rollup_pv_port_hour PRIMARY KEY (provider, provider_device_id, port_id, bucket_start),
    CONSTRAINT chk_rollup_pv_port_hour_provider_nonempty CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_rollup_pv_port_hour_provider_device_id_nonempty CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_rollup_pv_port_hour_port_id_nonempty CHECK (length(trim(port_id)) > 0),
    CONSTRAINT chk_rollup_pv_port_hour_port_label_nonempty CHECK (length(trim(port_label)) > 0),
    CONSTRAINT chk_rollup_pv_port_hour_sample_count_positive CHECK (sample_count > 0),
    CONSTRAINT chk_rollup_pv_port_hour_ts_order CHECK (first_ts_unix_ms <= last_ts_unix_ms)
);

SELECT create_hypertable(
    'telemetry_rollup_pv_port_hour',
    'bucket_start',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '7 days'
);

CREATE INDEX IF NOT EXISTS idx_rollup_pv_port_hour_device_bucket
    ON telemetry_rollup_pv_port_hour (device_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_pv_port_hour_device_port_bucket
    ON telemetry_rollup_pv_port_hour (device_id, port_id, bucket_start DESC);

CREATE TABLE IF NOT EXISTS telemetry_rollup_pv_port_day (
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    device_id UUID NOT NULL,
    port_id TEXT NOT NULL,
    port_label TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    sample_count INTEGER NOT NULL,
    first_ts_unix_ms BIGINT NOT NULL,
    last_ts_unix_ms BIGINT NOT NULL,
    max_observed_volts DOUBLE PRECISION,
    max_observed_amps DOUBLE PRECISION,
    max_observed_watts DOUBLE PRECISION,
    last_observed_volts DOUBLE PRECISION,
    last_observed_amps DOUBLE PRECISION,
    last_observed_watts DOUBLE PRECISION,
    last_observed_at_unix_ms BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT pk_rollup_pv_port_day PRIMARY KEY (provider, provider_device_id, port_id, bucket_start),
    CONSTRAINT chk_rollup_pv_port_day_provider_nonempty CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_rollup_pv_port_day_provider_device_id_nonempty CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_rollup_pv_port_day_port_id_nonempty CHECK (length(trim(port_id)) > 0),
    CONSTRAINT chk_rollup_pv_port_day_port_label_nonempty CHECK (length(trim(port_label)) > 0),
    CONSTRAINT chk_rollup_pv_port_day_sample_count_positive CHECK (sample_count > 0),
    CONSTRAINT chk_rollup_pv_port_day_ts_order CHECK (first_ts_unix_ms <= last_ts_unix_ms)
);

SELECT create_hypertable(
    'telemetry_rollup_pv_port_day',
    'bucket_start',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '30 days'
);

CREATE INDEX IF NOT EXISTS idx_rollup_pv_port_day_device_bucket
    ON telemetry_rollup_pv_port_day (device_id, bucket_start DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_pv_port_day_device_port_bucket
    ON telemetry_rollup_pv_port_day (device_id, port_id, bucket_start DESC);

SELECT remove_retention_policy('public.telemetry_rollup_pv_port_minute', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_pv_port_hour', if_exists => TRUE);
SELECT remove_retention_policy('public.telemetry_rollup_pv_port_day', if_exists => TRUE);

SELECT add_retention_policy(
    'public.telemetry_rollup_pv_port_minute',
    drop_after => INTERVAL '90 days',
    if_not_exists => TRUE,
    schedule_interval => INTERVAL '1 day'
);

SELECT add_retention_policy(
    'public.telemetry_rollup_pv_port_hour',
    drop_after => INTERVAL '3 years',
    if_not_exists => TRUE,
    schedule_interval => INTERVAL '1 day'
);

SELECT add_retention_policy(
    'public.telemetry_rollup_pv_port_day',
    drop_after => INTERVAL '3 years',
    if_not_exists => TRUE,
    schedule_interval => INTERVAL '1 day'
);
