# Project Manager Memory - Pulse Pi 5 Appliance

## Current focus

- Keep the appliance implementation moving in small Ralph-loop PRs after the
  Phase 1 foundation merged.
- Current branch: `codex/pi5-workload-consolidation`.
- Maintain one canonical task board at `.codex/tasks/pulse-pi5-appliance.md`.

## Files to inspect first

- `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
- `.codex/tasks/pulse-pi5-appliance.md`
- `docs/architecture/README.md`
- `docs/architecture/config-06-pi5-appliance.md`

## Decisions made

- Implementation will be split into small Ralph-loop PRs after the plan PR.
- Plan-only PRs may update `.codex/` and docs, but not runtime code.
- Progress updates use the standard Ralph-loop `Progress` format.
- 2026-06-04: Phase 1 is complete after adding installer/upgrade/wait/status
  orchestration and dry-run coverage.
- 2026-06-04: Phase 3 runbook slice is docs-only and should define the planned
  backup/restore plus hosted-cloud shutdown gates before workload
  consolidation starts.
- 2026-06-04: Phase 4 starts with conservative runtime caps and a hardware
  burn-in runbook; do not merge processes until the target Pi produces capacity
  evidence.

## Open risks

- The full plan is broad; later implementation branches must resist bundling
  BLE retry, archive outbox, and workload merging into one PR.
- Architecture tracking can drift if task board state is not updated per slice.

## Next step

- Validate and open the Phase 4 runtime-cap and capacity burn-in PR.
