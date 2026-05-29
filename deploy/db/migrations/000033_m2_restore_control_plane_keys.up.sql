-- Restore control-plane key constraints that older data planes may have missed.
--
-- Several baseline migrations use CREATE TABLE IF NOT EXISTS. If a table already
-- existed without its inline primary or foreign keys, PostgreSQL kept the table
-- but did not repair the missing constraints. Later migrations that reference
-- users(id), devices(id), or provider_credentials(id) require those keys.

DO $$
BEGIN
    IF to_regclass('users') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'users'::regclass
             AND contype = 'p'
       ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('devices') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'devices'::regclass
             AND contype = 'p'
       ) THEN
        ALTER TABLE devices
            ADD CONSTRAINT devices_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('provider_credentials') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'provider_credentials'::regclass
             AND contype = 'p'
       ) THEN
        ALTER TABLE provider_credentials
            ADD CONSTRAINT provider_credentials_pkey PRIMARY KEY (id);
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('user_devices') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'user_devices'::regclass
             AND contype = 'p'
       ) THEN
        ALTER TABLE user_devices
            ADD CONSTRAINT user_devices_pkey PRIMARY KEY (user_id, device_id);
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('user_devices') IS NOT NULL
       AND to_regclass('users') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'user_devices'::regclass
             AND conname = 'fk_user_devices_user'
       ) THEN
        ALTER TABLE user_devices
            ADD CONSTRAINT fk_user_devices_user
            FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
            NOT VALID;
        ALTER TABLE user_devices VALIDATE CONSTRAINT fk_user_devices_user;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('user_devices') IS NOT NULL
       AND to_regclass('devices') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'user_devices'::regclass
             AND conname = 'fk_user_devices_device'
       ) THEN
        ALTER TABLE user_devices
            ADD CONSTRAINT fk_user_devices_device
            FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE
            NOT VALID;
        ALTER TABLE user_devices VALIDATE CONSTRAINT fk_user_devices_device;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('provider_credentials') IS NOT NULL
       AND to_regclass('users') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'provider_credentials'::regclass
             AND conname = 'fk_provider_credentials_user'
       ) THEN
        ALTER TABLE provider_credentials
            ADD CONSTRAINT fk_provider_credentials_user
            FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
            NOT VALID;
        ALTER TABLE provider_credentials VALIDATE CONSTRAINT fk_provider_credentials_user;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('provider_devices') IS NOT NULL
       AND to_regclass('devices') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'provider_devices'::regclass
             AND conname = 'fk_provider_devices_device'
       ) THEN
        ALTER TABLE provider_devices
            ADD CONSTRAINT fk_provider_devices_device
            FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE
            NOT VALID;
        ALTER TABLE provider_devices VALIDATE CONSTRAINT fk_provider_devices_device;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('provider_devices') IS NOT NULL
       AND to_regclass('provider_credentials') IS NOT NULL
       AND NOT EXISTS (
           SELECT 1
           FROM pg_constraint
           WHERE conrelid = 'provider_devices'::regclass
             AND conname = 'fk_provider_devices_credential'
       ) THEN
        ALTER TABLE provider_devices
            ADD CONSTRAINT fk_provider_devices_credential
            FOREIGN KEY (credential_id) REFERENCES provider_credentials (id) ON DELETE RESTRICT
            NOT VALID;
        ALTER TABLE provider_devices VALIDATE CONSTRAINT fk_provider_devices_credential;
    END IF;
END $$;
