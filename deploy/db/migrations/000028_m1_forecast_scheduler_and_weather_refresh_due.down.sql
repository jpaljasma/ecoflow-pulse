DROP INDEX IF EXISTS idx_scheduled_jobs_due;
DROP TABLE IF EXISTS scheduled_jobs;

DROP INDEX IF EXISTS idx_weather_refresh_candidates_next_refresh_at;
ALTER TABLE weather_refresh_candidates
    DROP COLUMN IF EXISTS next_refresh_at;
