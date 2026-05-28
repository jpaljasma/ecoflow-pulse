-- M1 edge collector schema (v1)
-- Scope: user-owned local collectors and manually approved local device sources.

CREATE TABLE IF NOT EXISTS edge_collectors (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL,
    display_name TEXT NOT NULL,
    setup_token_hash TEXT NOT NULL,
    collector_secret_hash TEXT,
    is_active BOOLEAN NOT NULL,
    collector_version TEXT,
    hostname TEXT,
    last_heartbeat_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_edge_collectors_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT chk_edge_collectors_display_name_nonempty
        CHECK (length(trim(display_name)) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_edge_collectors_setup_token_hash
    ON edge_collectors (setup_token_hash)
    WHERE setup_token_hash <> '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_edge_collectors_secret_hash
    ON edge_collectors (collector_secret_hash)
    WHERE collector_secret_hash IS NOT NULL AND collector_secret_hash <> '';

CREATE INDEX IF NOT EXISTS idx_edge_collectors_user_active
    ON edge_collectors (user_id, is_active);

CREATE TABLE IF NOT EXISTS edge_device_sources (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    collector_id UUID NOT NULL,
    user_id UUID NOT NULL,
    provider TEXT NOT NULL,
    transport TEXT NOT NULL,
    provider_device_id TEXT NOT NULL,
    display_name TEXT,
    model TEXT,
    address_hash TEXT,
    rssi_dbm INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    linked_device_id UUID,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_edge_device_sources_collector
        FOREIGN KEY (collector_id) REFERENCES edge_collectors (id) ON DELETE CASCADE,
    CONSTRAINT fk_edge_device_sources_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_edge_device_sources_linked_device
        FOREIGN KEY (linked_device_id) REFERENCES devices (id) ON DELETE SET NULL,
    CONSTRAINT chk_edge_device_sources_provider_nonempty
        CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_edge_device_sources_transport_nonempty
        CHECK (length(trim(transport)) > 0),
    CONSTRAINT chk_edge_device_sources_provider_device_id_nonempty
        CHECK (length(trim(provider_device_id)) > 0),
    CONSTRAINT chk_edge_device_sources_status
        CHECK (status IN ('pending', 'linked', 'ignored')),
    CONSTRAINT uq_edge_device_sources_collector_identity
        UNIQUE (collector_id, provider, transport, provider_device_id)
);

CREATE INDEX IF NOT EXISTS idx_edge_device_sources_user_status
    ON edge_device_sources (user_id, status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_edge_device_sources_linked_device
    ON edge_device_sources (linked_device_id)
    WHERE linked_device_id IS NOT NULL;
