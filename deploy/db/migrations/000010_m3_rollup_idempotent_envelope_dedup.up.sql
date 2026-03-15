CREATE TABLE IF NOT EXISTS telemetry_rollup_applied_envelopes (
    dedup_key text PRIMARY KEY,
    envelope_id text,
    message_id text,
    provider text NOT NULL,
    provider_device_id text NOT NULL,
    device_id uuid NOT NULL,
    event_time timestamptz NOT NULL,
    applied_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rollup_applied_envelopes_applied_at
    ON telemetry_rollup_applied_envelopes (applied_at DESC);

CREATE INDEX IF NOT EXISTS idx_rollup_applied_envelopes_provider_device_event
    ON telemetry_rollup_applied_envelopes (provider, provider_device_id, event_time DESC);
