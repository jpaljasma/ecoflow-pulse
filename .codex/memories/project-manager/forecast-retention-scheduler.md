# Project Manager Memory: Forecast Retention and Scheduler

## Current focus

- Keep the new forecast-retention loop aligned with the approved production-safe scope.
- Sequence bootstrap, scheduler, storage compaction, client freshness alignment, and validation in that order.
- Keep worker cost low by delegating only explicit, bounded slices.

## Decisions so far

- This work gets a dedicated Ralph-loop instead of being folded into `weatherd` or `solar-forecastd`.
- The canonical task board is `.codex/tasks/forecast-retention-scheduler.md`.
- The scheduler outcome must be reusable for future recurring workloads.

## Open risks

- Client freshness changes can accidentally reintroduce request-path upstream fetches or stale UI bugs.
- Weather snapshot compaction must not break yesterday verification fallback behavior.
- Solar canonicalization must not regress device-scoped serving behavior.

## Next step

- Land the architecture row, then implement the scheduler and storage changes with targeted regression coverage.
