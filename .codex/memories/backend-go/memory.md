# Backend Go Memory

## Current focus

- Keep the first vertical-slice Go contract usable while tracking the remaining data-model gaps that still block full spec parity.

## Files to inspect first

- `deploy/db/migrations/000004_m3_rollups_hypertables_schema.up.sql`
- `internal/rollupworker/*`
- `internal/telemetryquery/*`
- `proto/pulse/telemetry/v1/telemetry.proto`
- `cmd/ecoflow-grpc-api/*`

## Known gap to verify

- Current rollups clearly include power, SOC, temp, and `solar_generated_wh`.
- The spec also needs `ac_output_energy_wh`, `dc_output_energy_wh`, `load_energy_wh`, `ac_input_energy_wh`, `battery_charge_energy_wh`, `battery_discharge_energy_wh`, and per-port historical PV envelope data.

## Findings

- `deploy/db/migrations/000004_m3_rollups_hypertables_schema.up.sql` only persists aggregate power/SOC/temp metrics plus `solar_generated_wh`.
- `internal/rollupworker/types.go` only models `SOC`, `ACIn`, `PV`, `DC`, `Load`, `Net`, `Battery`, `Temp`, and `SolarGeneratedWh`.
- `internal/rollupworker/metrics.go` extracts instantaneous/aggregate power metrics and battery power, but not the spec's explicit window energy buckets.
- `internal/telemetryquery/postgres.go` and `proto/pulse/telemetry/v1/telemetry.proto` remain single-device and do not support Energy dashboard fleet aggregation.
- Live/provider metadata already exposes PV input capability metadata and current per-port volts/amps/watts through `controlplane.ListProviderDevices` and `providerDeviceMapper`, but historical per-port observed maxima are not stored in the rollup/query path.
- `docs/reference/telemetry-model.md` describes minute-bucket energy fields that are not present in the current rollup migration or query code, so docs and implementation are currently out of sync.

## Implemented foundation

- Added `internal/energydashboard/windows.go` with DST-sensitive preset window resolution for:
  - `today`
  - `yesterday`
  - `last7d`
  - `thisWeek`
  - `previousWeek`
  - `thisMonth`
  - `last12m`
- Added `internal/energydashboard/series.go` with server-side summary math for:
  - total solar/load/AC input/DC output/AC output energy
  - battery charge/discharge/net energy
  - self-sufficiency
  - estimated value and AC-input cost
- Added `internal/energydashboard/scope.go` to normalize `device=all|<uuid>` against visible device ids.
- Added `internal/energydashboard/model.go` to build spec-oriented summary and battery response blocks from current/previous rollup series.
- Validation: `go test ./internal/energydashboard -count=1` passed.

## Implemented service slice

- Added `GetEnergyDashboard` to `proto/pulse/telemetry/v1/telemetry.proto` and regenerated `gen/`.
- Added `cmd/ecoflow-grpc-api/energy_dashboard.go` with:
  - preset validation,
  - timezone-aware local window resolution,
  - authz-aware scope resolution for `device` and `all`,
  - per-device rollup fanout with merged aggregate series,
  - spec-oriented summary and battery response builders,
  - preset-aware power chart series via `currentPowerPoints` / `previousPowerPoints`,
  - real energy chart series via `currentEnergyPoints` / `previousEnergyPoints`.
- Added an interim query-layer expansion:
  - `RollupMetrics` now carries `ac_input_energy_wh`, `dc_output_energy_wh`, `load_energy_wh`, `battery_charge_energy_wh`, and `battery_discharge_energy_wh`
  - `internal/telemetryquery/postgres.go` derives those energy fields from stored average-power buckets and bucket duration
  - fleet merge logic now preserves and sums those energy fields across devices
- Validation: `go test ./internal/telemetryquery ./internal/energydashboard ./cmd/ecoflow-grpc-api -count=1` passed.

## Historical PV foundation

- Added `internal/energydashboard/pvhistory.go` as a tested foundation for historical per-port PV observations from archived normalized quota envelopes.
- The extractor:
  - accepts archived `TelemetryEnvelope` frames with `payload_type=ecoflow.quota.normalized`
  - decodes JSON `params`
  - maps `inLvMppt*` / `pv1ChargeWatts` to `PV Low`
  - maps `inHvMppt*` / `pv2ChargeWatts` to `PV High`
  - falls back to derived `volts * amps` when explicit watts are missing
  - aggregates max observed volts/amps/watts plus latest observation timestamp per device+port
- Wired that foundation into `TelemetryService`:
  - new optional archive deps use `replaycli.ManifestStore` + `replaycli.ObjectReader`
  - `cmd/ecoflow-grpc-api/main.go` now enables archive readers from existing `CONTROL_PLANE_DB_DSN` + `ARCHIVE_OBJECT_*` env vars when available
  - `GetEnergyDashboard` now queries current-window archive objects for visible provider-device ids and returns `pvPortHistory`
- Validation: `go test ./cmd/ecoflow-grpc-api ./internal/energydashboard -count=1` passed.

## Next step

- Decide whether the current on-demand archive scan is sufficient for v1 or whether historical PV observations need persisted per-port rollups for performance/cost reasons.

## Side findings worth retaining

- `backend-go/storm-guard-dpu.md` records a March 11, 2026 archive-backed DPU finding:
  - provider-device `Y711ZABA9H2P0294` emitted explicit Storm Guard fields
  - the payload exposed active/open state and an end time, but not a richer weather-cause field
