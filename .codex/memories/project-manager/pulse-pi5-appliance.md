# Project Manager Memory - Pulse Pi 5 Appliance

## Current focus

- Land the plan-only PR that makes Ralph Loop the execution method for the
  appliance migration.
- Keep implementation explicitly blocked until this plan PR merges.
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

## Open risks

- The full plan is broad; later implementation branches must resist bundling
  host install, BLE retry, archive outbox, and workload merging into one PR.
- Architecture tracking can drift if task board state is not updated per slice.

## Next step

- Create and merge the plan-only PR, then begin Phase 1 from updated `main`.
