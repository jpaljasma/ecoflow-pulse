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

CREATE TABLE IF NOT EXISTS weather_verification_forecast_anchors (
    canonical_location_key TEXT NOT NULL,
    verification_date TIMESTAMPTZ NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    bundle_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (canonical_location_key, verification_date)
);

CREATE INDEX IF NOT EXISTS idx_weather_verification_forecast_anchors_issued_at
    ON weather_verification_forecast_anchors (issued_at DESC);
