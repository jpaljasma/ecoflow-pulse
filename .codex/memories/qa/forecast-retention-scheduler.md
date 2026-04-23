# QA Memory: Forecast Retention and Scheduler

## Current focus

- Protect `/profile`, shared weather widgets, lazy yesterday verification, and solar scope flows while cadence and storage behavior change.

## Regression priorities

- `apps/pulse-platform` weather and solar route tests
- `apps/universal` weather, profile, and solar hook tests
- Playwright `profile-weather.spec.ts`
- targeted Go tests for weather and solar store behavior
- Helm lint for worker topology changes

## Next step

- Review the implementation slices for stale-data regressions, blank-state flashes, and scheduler safety before the final acceptance run.
