CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_credentials_provider_access_key_hash
    ON provider_credentials (provider, access_key_hash);
