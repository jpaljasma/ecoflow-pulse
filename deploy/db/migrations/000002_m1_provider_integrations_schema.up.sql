-- M1 provider integrations schema (v1)
-- Scope:
--   provider_credentials
--   provider_devices
--
-- Conventions (locked by ADR-0012/ADR-0014):
--   - UUIDv7 primary keys
--   - created_at/updated_at are UTC semantics and app-managed
--   - provider validation is app-layer; DB enforces non-empty values

CREATE TABLE IF NOT EXISTS provider_credentials (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL,
    provider TEXT NOT NULL,
    access_key_ciphertext BYTEA NOT NULL,
    secret_key_ciphertext BYTEA NOT NULL,
    access_key_hash BYTEA NOT NULL,
    access_key_mask TEXT NOT NULL,
    is_active BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_provider_credentials_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT chk_provider_credentials_provider_nonempty
        CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_provider_credentials_access_key_ciphertext_nonempty
        CHECK (octet_length(access_key_ciphertext) > 0),
    CONSTRAINT chk_provider_credentials_secret_key_ciphertext_nonempty
        CHECK (octet_length(secret_key_ciphertext) > 0),
    CONSTRAINT chk_provider_credentials_access_key_hash_nonempty
        CHECK (octet_length(access_key_hash) > 0),
    CONSTRAINT chk_provider_credentials_access_key_mask_nonempty
        CHECK (length(trim(access_key_mask)) > 0),
    CONSTRAINT uq_provider_credentials_user_provider_access_key_hash
        UNIQUE (user_id, provider, access_key_hash)
);

CREATE INDEX IF NOT EXISTS idx_provider_credentials_user_provider_active
    ON provider_credentials (user_id, provider, is_active);

CREATE INDEX IF NOT EXISTS idx_provider_credentials_provider_active
    ON provider_credentials (provider, is_active);

CREATE TABLE IF NOT EXISTS provider_devices (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    device_id UUID NOT NULL,
    provider TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    credential_id UUID NOT NULL,
    product_name TEXT,
    model TEXT,
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL,
    ingest_desired_state TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_provider_devices_device
        FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE,
    CONSTRAINT fk_provider_devices_credential
        FOREIGN KEY (credential_id) REFERENCES provider_credentials (id) ON DELETE RESTRICT,
    CONSTRAINT chk_provider_devices_provider_nonempty
        CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_provider_devices_provider_device_id_nonempty
        CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_provider_devices_ingest_desired_state
        CHECK (ingest_desired_state IN ('active', 'draining', 'paused')),
    CONSTRAINT uq_provider_devices_provider_device_id
        UNIQUE (provider, provider_device_id),
    CONSTRAINT uq_provider_devices_device_provider
        UNIQUE (device_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_provider_devices_provider_active
    ON provider_devices (provider, is_active);

CREATE INDEX IF NOT EXISTS idx_provider_devices_credential
    ON provider_devices (credential_id);

CREATE INDEX IF NOT EXISTS idx_provider_devices_ingest_desired_state
    ON provider_devices (ingest_desired_state);

CREATE INDEX IF NOT EXISTS idx_provider_devices_device_provider
    ON provider_devices (device_id, provider);
