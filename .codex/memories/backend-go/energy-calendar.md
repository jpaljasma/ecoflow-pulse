# backend-go Memory — Energy Calendar

## Current focus

Add selected-date window semantics and an Energy calendar aggregation contract.

## Files to inspect first

- `proto/pulse/telemetry/v1/telemetry.proto`
- `internal/energydashboard/windows.go`
- `cmd/ecoflow-grpc-api/energy_dashboard.go`
- `cmd/ecoflow-grpc-api/telemetry_service_test.go`

## Decisions made

- Historical selected dates are full local days.
- Current selected date is local midnight to service `now`.
- Calendar grid is Sunday-start and includes adjacent-month days.

## Open risks

- Codegen command may be required after proto changes.
- Avoid int narrowing without bounds checks.

## Next step

Write failing Go tests for selected-day windows and calendar aggregation.
