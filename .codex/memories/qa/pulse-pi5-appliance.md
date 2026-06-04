# QA Memory - Pulse Pi 5 Appliance

## Current focus

- Define validation gates for the plan-only PR and later appliance slices.

## Files to inspect first

- `.codex/tasks/pulse-pi5-appliance.md`
- `docs/architecture/config-06-pi5-appliance.md`
- `Makefile`
- Existing edge collector, archive worker, and deploy tests for touched slices.

## Decisions made

- Plan-only PR validation is Markdown lint.
- 2026-06-04: `make lint` passed for the plan-only PR.
- Runtime slices need targeted tests close to the changed packages.
- Worker shutdown, ingest retry, archive outbox, and BLE retry changes need race
  or failure-mode coverage.

## Open risks

- Full appliance acceptance requires real Pi hardware; CI can only validate
  render, unit, and script behavior.
- Capacity burn-in criteria must be recorded from the target 8GB Pi, not from
  desktop k3d.

## Next step

- For PR closeout, confirm no product/runtime code changed and the PR body
  renders correctly.
