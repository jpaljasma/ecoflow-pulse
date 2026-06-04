# Edge BLE Memory - Pulse Pi 5 Appliance

## Current focus

- Prepare the direct local gRPC and durable outbox slice for the existing Pi BLE
  collector.

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

## Open risks

- Current collector behavior drops failed telemetry posts after bounded REST
  retries; appliance mode needs a durable local outbox.
- BlueZ/dbus host dependencies must stay simple and diagnosable.

## Next step

- Add direct gRPC transport and failing tests for retry/idempotency behavior
  before changing collector upload logic.
