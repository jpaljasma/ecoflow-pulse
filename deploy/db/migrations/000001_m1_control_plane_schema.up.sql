-- M1 control-plane schema (v1)
-- Scope: identity + ownership authorization primitives
-- Requires: PostgreSQL 18+ (uuidv7() built-in)
-- Tables:
--   users
--   devices
--   user_devices (viewer/admin linkage)
--
-- Timestamp convention (locked):
--   - created_at/updated_at are UTC, application-managed.
--   - No database defaults/triggers for timestamp writes.

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    keycloak_subject TEXT NOT NULL UNIQUE,
    email TEXT,
    display_name TEXT,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_users_keycloak_subject_nonempty
        CHECK (length(trim(keycloak_subject)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    ecoflow_sn TEXT NOT NULL UNIQUE,
    product_name TEXT,
    model TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_devices_ecoflow_sn_nonempty
        CHECK (length(trim(ecoflow_sn)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_devices_model ON devices (model);

CREATE TABLE IF NOT EXISTS user_devices (
    user_id UUID NOT NULL,
    device_id UUID NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, device_id),
    CONSTRAINT fk_user_devices_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_devices_device
        FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE,
    CONSTRAINT chk_user_devices_role
        CHECK (role IN ('viewer', 'admin'))
);

CREATE INDEX IF NOT EXISTS idx_user_devices_device ON user_devices (device_id);
CREATE INDEX IF NOT EXISTS idx_user_devices_role ON user_devices (role);
