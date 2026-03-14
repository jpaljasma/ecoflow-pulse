ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_weather_location_source;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_display_name_source;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS display_name_source,
    DROP COLUMN IF EXISTS weather_longitude,
    DROP COLUMN IF EXISTS weather_latitude,
    DROP COLUMN IF EXISTS weather_location_label,
    DROP COLUMN IF EXISTS weather_location_source,
    DROP COLUMN IF EXISTS weather_location_enabled,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS locale,
    DROP COLUMN IF EXISTS family_name,
    DROP COLUMN IF EXISTS given_name,
    DROP COLUMN IF EXISTS email_verified;
