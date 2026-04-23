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
