-- M2 archive manifest index schema (v1)
-- Scope:
--   archive_object_manifest
--
-- Conventions:
--   - UUIDv7 primary keys
--   - created_at/updated_at are UTC semantics and app-managed
--   - object key (bucket+path) is unique for idempotent writes

CREATE TABLE IF NOT EXISTS archive_object_manifest (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    provider TEXT NOT NULL,
    shard INTEGER NOT NULL,
    shard_count INTEGER NOT NULL,
    partition_hour TIMESTAMPTZ NOT NULL,
    ts_min_unix_ms BIGINT NOT NULL,
    ts_max_unix_ms BIGINT NOT NULL,
    record_count INTEGER NOT NULL,
    object_bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    object_size_bytes BIGINT NOT NULL,
    content_type TEXT NOT NULL,
    compression TEXT NOT NULL,
    checksum_crc32 TEXT NOT NULL,
    writer_id TEXT NOT NULL,
    device_ids TEXT[] NOT NULL DEFAULT '{}',
    provider_device_ids TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_archive_manifest_provider_nonempty
        CHECK (length(trim(provider)) > 0),
    CONSTRAINT chk_archive_manifest_shard_nonnegative
        CHECK (shard >= 0),
    CONSTRAINT chk_archive_manifest_shard_count_positive
        CHECK (shard_count > 0),
    CONSTRAINT chk_archive_manifest_record_count_positive
        CHECK (record_count > 0),
    CONSTRAINT chk_archive_manifest_object_bucket_nonempty
        CHECK (length(trim(object_bucket)) > 0),
    CONSTRAINT chk_archive_manifest_object_key_nonempty
        CHECK (length(trim(object_key)) > 0),
    CONSTRAINT chk_archive_manifest_object_size_positive
        CHECK (object_size_bytes > 0),
    CONSTRAINT chk_archive_manifest_content_type_nonempty
        CHECK (length(trim(content_type)) > 0),
    CONSTRAINT chk_archive_manifest_compression_nonempty
        CHECK (length(trim(compression)) > 0),
    CONSTRAINT chk_archive_manifest_writer_id_nonempty
        CHECK (length(trim(writer_id)) > 0),
    CONSTRAINT chk_archive_manifest_ts_order
        CHECK (ts_min_unix_ms <= ts_max_unix_ms),
    CONSTRAINT uq_archive_manifest_bucket_key
        UNIQUE (object_bucket, object_key)
);

CREATE INDEX IF NOT EXISTS idx_archive_manifest_partition_shard
    ON archive_object_manifest (partition_hour, shard);

CREATE INDEX IF NOT EXISTS idx_archive_manifest_ts_window
    ON archive_object_manifest (ts_min_unix_ms, ts_max_unix_ms);

CREATE INDEX IF NOT EXISTS idx_archive_manifest_provider_partition
    ON archive_object_manifest (provider, partition_hour);

CREATE INDEX IF NOT EXISTS idx_archive_manifest_created_at
    ON archive_object_manifest (created_at);

CREATE INDEX IF NOT EXISTS idx_archive_manifest_device_ids_gin
    ON archive_object_manifest
    USING GIN (device_ids);

CREATE INDEX IF NOT EXISTS idx_archive_manifest_provider_device_ids_gin
    ON archive_object_manifest
    USING GIN (provider_device_ids);
