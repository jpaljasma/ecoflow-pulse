# Backend Go Memory: Forecast Retention and Scheduler

## Current focus

- Add reusable scheduling primitives and worker wiring.
- Compact weather storage to latest-only canonical-location snapshots.
- Convert solar persistence from request-triggered writes to canonical scheduled site runs.

## Files to inspect first

- `internal/weatherd/service.go`
- `internal/weatherd/store/*`
- `internal/solarforecastd/*`
- `cmd/ecoflow-grpc-api/weather_runtime.go`
- `cmd/ecoflow-solar-verifier/*`
- `deploy/charts/pulse-services/templates/workers.yaml`

## Known gaps

- Weather snapshot persistence is append-only and still writes `weather_forecast_points`.
- Weather refresh currently relies on in-process ticker loops.
- Solar persistence still uses device-set-specific `site_key` construction and request-triggered training writes.
- There is no reusable generic scheduler worker yet.

## Next step

- Add the scheduling schema and runtime seams first, then refactor weather and solar persistence around them.
