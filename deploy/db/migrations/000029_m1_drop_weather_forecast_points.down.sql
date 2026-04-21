CREATE TABLE IF NOT EXISTS weather_forecast_points (
    snapshot_id UUID NOT NULL REFERENCES weather_forecast_snapshots(id) ON DELETE CASCADE,
    target_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (snapshot_id, target_time)
);

CREATE INDEX IF NOT EXISTS idx_weather_forecast_points_target_time
    ON weather_forecast_points (target_time DESC);
