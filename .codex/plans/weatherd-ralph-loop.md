# Weatherd Ralph-Loop Plan

Status: In progress  
Last updated: 2026-03-18

## Goal

Build a weather forecast and yesterday-verification feature on top of Open-Meteo using:

- Go gRPC in the existing `cmd/ecoflow-grpc-api` runtime
- Node REST BFF adapters
- Expo universal profile widgets
- deterministic tests and browser mocks

## Locked defaults

- Weather service lives in the existing `grpc-api` runtime, not a separate binary.
- Public BFF routes never accept lat/lon directly; they resolve them from the logged-in user profile.
- Public profile weather requests send fixed panel defaults `tilt=45` and `azimuth=0`.
- Open-Meteo forecast remains the hot-path upstream, with Previous Runs fallback for missing snapshot verification and Historical Forecast reserved for backfills.
- Weather icons use `MaterialCommunityIcons` with a local WMO-code mapping helper.

## Workstreams

1. Backend Go
   - protobuf contract
   - Open-Meteo adapters
   - cache key, unit conversion, budget manager
   - Valkey hot cache + Postgres snapshot store
   - verification and bias correction
   - grpc-api wiring
2. BFF Node
   - weather gRPC client
   - `GET /api/v1/weather/forecast`
   - `GET /api/v1/weather/yesterday`
   - current-user profile coordinate resolution
3. Frontend Universal
   - compact current-conditions widget
   - 7-day forecast card
   - yesterday verification section
   - attribution and browser E2E mock coverage

## Validation targets

- `buf lint` and `buf generate`
- targeted Go unit + integration tests for `internal/weatherd/...` and `cmd/ecoflow-grpc-api`
- targeted `apps/pulse-platform` Vitest coverage
- targeted `apps/universal` Vitest coverage
- Playwright profile weather flow with deterministic mocks

## Notes

- Coverage target for new weather Go packages is `>90%`.
- Snapshot persistence is canonical-location keyed from Open-Meteo returned grid-cell coordinates and elevation, with tilt and azimuth bucketing.
- Weather “actuals” are still model-derived archived weather in v1 and may be replaced later with station truth when available.
