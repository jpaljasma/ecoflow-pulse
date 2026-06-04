# Edge BLE Memory - Pulse Pi 5 Appliance

## Current focus

- Implement the first Phase 2 edge slice on
  `codex/pi5-appliance-ble-direct`: direct local gRPC transport for the
  existing Pi BLE collector.

## Files to inspect first

- `cmd/pulse-edge-collector/main.go`
- `cmd/pulse-edge-collector/main_test.go`
- `proto/pulse/edge/v1/edge.proto`
- `cmd/ecoflow-grpc-api/edge_service.go`
- `internal/edgecollector`
- `deploy/pulse-edge/pulse-edge-collector.service`
- `docs/how-to/run-pulse-edge-collector.md`

## Decisions made

- BLE stays on the host under systemd from day one.
- Appliance BLE uses direct loopback gRPC by default.
- REST transport remains supported for non-appliance edge deployments.
- Retry idempotency needs a `client_sample_id` or equivalent stable client
  sample identity.
- 2026-06-04: Keep direct gRPC transport separate from durable outbox so the
  transport path can merge with focused tests first.

## Open risks

- Current collector behavior drops failed telemetry posts after bounded REST
  retries; appliance mode needs a durable local outbox.
- Direct gRPC does not by itself make retries durable; the next slice still
  needs `client_sample_id` and local outbox work.
- BlueZ/dbus host dependencies must stay simple and diagnosable.

## Next step

- Direct gRPC transport is implemented and covered for config parsing plus
  enrollment, heartbeat, discovery, and telemetry request mapping.
- Next slice: add durable outbox storage and stable client sample identity for
  idempotent replay.
