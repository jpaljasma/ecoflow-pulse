# QA Memory - Pulse Pi 5 Appliance

## Current focus

- Validate the first Phase 2 BLE direct-gRPC transport slice.

## Files to inspect first

- `.codex/tasks/pulse-pi5-appliance.md`
- `docs/architecture/config-06-pi5-appliance.md`
- `Makefile`
- Existing edge collector, archive worker, and deploy tests for touched slices.
- `cmd/pulse-edge-collector/main_test.go`
- `deploy/pulse-edge/pulse-edge-collector.service`
- `docs/how-to/run-pulse-edge-collector.md`
- `deploy/appliance/pi5/test-host-prepare.sh`
- `deploy/appliance/pi5/test-install-dry-run.sh`
- `deploy/env/pi/values.platform.yaml`
- `deploy/env/pi/values.services.yaml`

## Decisions made

- Plan-only PR validation is Markdown lint.
- 2026-06-04: `make lint` passed for the plan-only PR.
- Runtime slices need targeted tests close to the changed packages.
- Worker shutdown, ingest retry, archive outbox, and BLE retry changes need race
  or failure-mode coverage.
- 2026-06-04: `make appliance-pi-validate` passed for shellcheck, host fstab
  fixture coverage, and Pi Helm lint.
- 2026-06-04: Installer dry-run coverage proves the appliance command path
  reaches host/K3s skips, Helm dependency builds, the Keycloak bootstrap pass,
  runtime-secret preflight, and services Helm apply without needing Pi
  hardware.

## Open risks

- Full appliance acceptance requires real Pi hardware; CI can only validate
  render, fixture, unit, and script behavior.
- Capacity burn-in criteria must be recorded from the target 8GB Pi, not from
  desktop k3d.
- Host script tests cover fstab option merging but not real EEPROM, PCIe, or
  systemd behavior.
- Installer tests are dry-run only; real K3s install, CNPG readiness, Keycloak
  login, and hostPort reachability need appliance hardware.
- Direct gRPC transport needs targeted collector tests and a Pi hardware check
  after the loopback hostPort service is installed.

## Next step

- Targeted checks passed for `go test ./cmd/pulse-edge-collector -count=1`,
  `go test ./cmd/ecoflow-grpc-api ./internal/edgecollector -count=1`,
  `make pulse-edge-pi5-bundle`, `make lint`, and
  `make appliance-pi-validate`.
- The latest bundle rebuild showed the expected Pi 5 compile environment:
  `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOARM64=v8.2`.
- Next: run duplicate backup scan and sensitive-text scan before PR closeout.
