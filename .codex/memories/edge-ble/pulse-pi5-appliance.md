# Edge BLE Memory - Pulse Pi 5 Appliance

## Current focus

- Implement the second Phase 2 edge slice on
  `codex/pi5-appliance-ble-outbox`: durable local outbox and stable sample
  identity for the existing Pi BLE collector.

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
- 2026-06-04: Direct gRPC transport merged in PR #248; durable outbox now owns
  restart-safe retry and sample idempotency.

## Open risks

- Current collector behavior drops failed telemetry posts after bounded REST
  retries; appliance mode needs a durable local outbox.
- Outbox files must not persist collector secrets; add the secret only when
  sending.
- `client_sample_id` needs to reach the server envelope `message_id` path so
  retry duplicates dedupe downstream.
- BlueZ/dbus host dependencies must stay simple and diagnosable.

## Next step

- Durable outbox is implemented with secret-free JSON entries, fsync before
  send, replay on startup/successful heartbeat, ACK removal, stable
  `client_sample_id`, and deterministic edge envelope ids.
- Next slice can move to archive/GCS outbox or Pi hardware validation.
