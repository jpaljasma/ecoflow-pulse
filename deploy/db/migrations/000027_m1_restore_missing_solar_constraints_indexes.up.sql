DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_runs'::regclass
          AND conname = 'solar_forecast_runs_pkey'
    ) THEN
        ALTER TABLE solar_forecast_runs
            ADD CONSTRAINT solar_forecast_runs_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_runs'::regclass
          AND conname = 'uq_solar_forecast_runs_site_issued_version'
    ) AND NOT EXISTS (
        SELECT 1
        FROM pg_class
        WHERE relkind = 'i'
          AND relname = 'uq_solar_forecast_runs_site_issued_version'
    ) THEN
        ALTER TABLE solar_forecast_runs
            ADD CONSTRAINT uq_solar_forecast_runs_site_issued_version UNIQUE (site_key, issued_at, forecast_version);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_solar_forecast_runs_location_issued_at
    ON solar_forecast_runs (canonical_location_key, issued_at DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_runs_scope_issued_at
    ON solar_forecast_runs (scope_kind, issued_at DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_runs_site_local_date_issued
    ON solar_forecast_runs (site_key, issue_local_date DESC, issued_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_hourly_training_records'::regclass
          AND conname = 'solar_forecast_hourly_training_records_pkey'
    ) THEN
        ALTER TABLE solar_forecast_hourly_training_records
            ADD CONSTRAINT solar_forecast_hourly_training_records_pkey PRIMARY KEY (run_id, target_time);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_site_target_time
    ON solar_forecast_hourly_training_records (site_key, target_time DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_verified_at
    ON solar_forecast_hourly_training_records (verified_at DESC NULLS LAST);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_pending_lookup
    ON solar_forecast_hourly_training_records (verification_status, target_time ASC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_verified_site_local_date
    ON solar_forecast_hourly_training_records (site_key, verification_status, target_local_date DESC, target_time DESC, issued_at DESC);

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_pending_claim_lookup
    ON solar_forecast_hourly_training_records (target_time, run_id)
    WHERE verification_status = 'pending';

CREATE INDEX IF NOT EXISTS idx_solar_forecast_hourly_rollup_cover
    ON solar_forecast_hourly_training_records (site_key, target_local_date, target_time, run_id)
    INCLUDE (
        device_id,
        horizon_bucket,
        forecast_generation_wh,
        baseline_forecast_generation_wh,
        actual_generation_wh,
        verification_status,
        absolute_error_wh,
        squared_error_wh2
    );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_verification_daily'::regclass
          AND conname = 'solar_forecast_verification_daily_pkey'
    ) THEN
        ALTER TABLE solar_forecast_verification_daily
            ADD CONSTRAINT solar_forecast_verification_daily_pkey PRIMARY KEY (site_key, verification_local_date, forecast_version, served_variant, horizon_bucket);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_solar_forecast_verification_daily_date
    ON solar_forecast_verification_daily (verification_local_date DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_calibration_state'::regclass
          AND conname = 'solar_forecast_calibration_state_pkey'
    ) THEN
        ALTER TABLE solar_forecast_calibration_state
            ADD CONSTRAINT solar_forecast_calibration_state_pkey PRIMARY KEY (site_key, forecast_version, horizon_bucket, hour_of_day);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_solar_forecast_calibration_updated_at
    ON solar_forecast_calibration_state (updated_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_site_serving_state'::regclass
          AND conname = 'solar_forecast_site_serving_state_pkey'
    ) THEN
        ALTER TABLE solar_forecast_site_serving_state
            ADD CONSTRAINT solar_forecast_site_serving_state_pkey PRIMARY KEY (site_key, forecast_version);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_solar_forecast_serving_state_updated_at
    ON solar_forecast_site_serving_state (updated_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'solar_forecast_verification_daily_run_rollup'::regclass
          AND conname = 'solar_forecast_verification_daily_run_rollup_pkey'
    ) THEN
        ALTER TABLE solar_forecast_verification_daily_run_rollup
            ADD CONSTRAINT solar_forecast_verification_daily_run_rollup_pkey PRIMARY KEY (run_id, verification_local_date, horizon_bucket);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_solar_forecast_run_daily_rollup_site_date
    ON solar_forecast_verification_daily_run_rollup (site_key, verification_local_date, forecast_version, served_variant, horizon_bucket);
