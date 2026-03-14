-- M1 auth/profile follow-up (ADR-0020)
-- Scope: current-user profile preferences + trusted social claim hydration

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS given_name TEXT,
    ADD COLUMN IF NOT EXISTS family_name TEXT,
    ADD COLUMN IF NOT EXISTS locale TEXT,
    ADD COLUMN IF NOT EXISTS timezone TEXT,
    ADD COLUMN IF NOT EXISTS weather_location_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS weather_location_source TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS weather_location_label TEXT,
    ADD COLUMN IF NOT EXISTS weather_latitude DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS weather_longitude DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS display_name_source TEXT NOT NULL DEFAULT 'provider',
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_users_display_name_source'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT chk_users_display_name_source
                CHECK (display_name_source IN ('provider', 'pulse'));
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_users_weather_location_source'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT chk_users_weather_location_source
                CHECK (weather_location_source IN ('none', 'auto'));
    END IF;
END
$$;
