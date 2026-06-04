# QA Memory - Pulse Pi 5 Appliance

## Current focus

- Validate the Phase 1 appliance scaffold slice.

## Files to inspect first

- `.codex/tasks/pulse-pi5-appliance.md`
- `docs/architecture/config-06-pi5-appliance.md`
- `Makefile`
- Existing edge collector, archive worker, and deploy tests for touched slices.
- `deploy/appliance/pi5/test-host-prepare.sh`
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

## Open risks

- Full appliance acceptance requires real Pi hardware; CI can only validate
  render, fixture, unit, and script behavior.
- Capacity burn-in criteria must be recorded from the target 8GB Pi, not from
  desktop k3d.
- Host script tests cover fstab option merging but not real EEPROM, PCIe, or
  systemd behavior.

## Next step

- Run `make lint` and inspect staged changes for duplicate backup files and
  sensitive text before PR closeout.
