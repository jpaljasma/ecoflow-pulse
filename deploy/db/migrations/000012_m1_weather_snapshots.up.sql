-- M1 weather forecast + verification persistence
-- Scope: Open-Meteo snapshots, verification summaries, bias state, refresh candidates

CREATE TABLE IF NOT EXISTS weather_forecast_snapshots (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    canonical_location_key TEXT NOT NULL,
    timezone TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    model_selection TEXT NOT NULL,
    actual_source TEXT NOT NULL,
    request_json JSONB NOT NULL,
    bundle_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_weather_forecast_snapshots_location_issued_at
    ON weather_forecast_snapshots (canonical_location_key, issued_at DESC);

CREATE TABLE IF NOT EXISTS weather_forecast_points (
    snapshot_id UUID NOT NULL REFERENCES weather_forecast_snapshots(id) ON DELETE CASCADE,
    target_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (snapshot_id, target_time)
);

CREATE INDEX IF NOT EXISTS idx_weather_forecast_points_target_time
    ON weather_forecast_points (target_time DESC);

CREATE TABLE IF NOT EXISTS weather_yesterday_verifications (
    canonical_location_key TEXT NOT NULL,
    verification_date TIMESTAMPTZ NOT NULL,
    verification_source TEXT NOT NULL,
    result_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (canonical_location_key, verification_date)
);

CREATE TABLE IF NOT EXISTS weather_bias_state (
    canonical_location_key TEXT NOT NULL,
    metric_key TEXT NOT NULL,
    hour_of_day SMALLINT NOT NULL,
    sample_count INTEGER NOT NULL,
    additive_bias DOUBLE PRECISION,
    multiplicative_ratio DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (canonical_location_key, metric_key, hour_of_day)
);

CREATE TABLE IF NOT EXISTS weather_refresh_candidates (
    canonical_location_key TEXT PRIMARY KEY,
    request_json JSONB NOT NULL,
    last_requested_at TIMESTAMPTZ NOT NULL,
    last_refreshed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_weather_refresh_candidates_last_requested_at
    ON weather_refresh_candidates (last_requested_at DESC);
