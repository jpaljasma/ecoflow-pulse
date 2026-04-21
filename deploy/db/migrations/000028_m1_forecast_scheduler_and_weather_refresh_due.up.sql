ALTER TABLE weather_refresh_candidates
    ADD COLUMN IF NOT EXISTS next_refresh_at TIMESTAMPTZ;

UPDATE weather_refresh_candidates
SET next_refresh_at = COALESCE(next_refresh_at, last_requested_at)
WHERE next_refresh_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_weather_refresh_candidates_next_refresh_at
    ON weather_refresh_candidates (next_refresh_at ASC, last_requested_at DESC);

CREATE TABLE IF NOT EXISTS scheduled_jobs (
    job_key TEXT PRIMARY KEY,
    job_type TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    next_run_at TIMESTAMPTZ NOT NULL,
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_scheduled_jobs_job_key_nonempty CHECK (btrim(job_key) <> ''),
    CONSTRAINT chk_scheduled_jobs_job_type_nonempty CHECK (btrim(job_type) <> ''),
    CONSTRAINT chk_scheduled_jobs_interval_positive CHECK (interval_seconds > 0)
);

CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_due
    ON scheduled_jobs (enabled, next_run_at ASC);
