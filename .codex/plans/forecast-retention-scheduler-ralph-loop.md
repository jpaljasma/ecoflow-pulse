# Forecast Retention and Scheduler Ralph-Loop Plan

Status: In progress  
Last updated: 2026-04-21

## Goal

Ship a production-safe forecast retention and scheduling patch set that:

- prunes existing weather and solar forecast persistence safely
- collapses weather storage to latest-only per canonical location
- collapses solar persistence to canonical site-level scheduled runs only
- moves weather refresh and canonical solar generation to a `4h` cadence
- aligns client freshness to that `4h` cadence without breaking existing product behavior
- introduces a reusable background scheduling platform for future recurring workloads

## Locked defaults

- Treat this as a production patch, not a greenfield redesign.
- Request-serving `go-grpc-api` stays traffic-only for recurring maintenance work.
- Weather and solar client freshness must align to the new `4h` backend cadence.
- Do not add client polling for weather or solar.
- Keep stale-while-refresh behavior and placeholder reuse so the UI does not flash empty states.
- Keep location, scope, and device changes immediately reactive through query invalidation and query keys.
- Use one canonical weather snapshot row per canonical location.
- Use one canonical solar modeling key per physical site or location.
- Kubernetes `CronJob` is not the main application scheduler for high-cardinality workloads.

## Workstreams

1. Ralph-loop bootstrap and architecture tracking
2. Reusable scheduler and planner substrate
3. Weather storage compaction and `4h` canonical refresh
4. Solar canonical scheduled persistence and hot retention
5. Client freshness alignment and regression protection
6. Validation, rollout evidence, and data-savings proof

## Validation targets

- targeted Go tests for weather, solar, scheduler, and worker packages
- `npm run -w apps/pulse-platform test -- weather_routes.test.ts solarForecastClient.test.ts`
- `npm run -w apps/pulse-platform typecheck`
- `npm run -w apps/universal test -- src/features/weather/api.test.ts src/features/weather/hooks.test.ts src/features/weather/model.test.ts src/features/profile/hooks.test.ts`
- `npm run -w apps/universal typecheck`
- `npm run -w apps/universal e2e:web -- profile-weather.spec.ts`
- `helm lint deploy/charts/pulse-services -f deploy/env/local/values.services.yaml`
- `helm lint deploy/charts/pulse-services -f deploy/env/dev/values.services.yaml`

## Notes

- Weather hot storage should move from append-only snapshots to latest-only canonical-location rows.
- Solar hot persistence should move from request-triggered writes to canonical scheduled site runs.
- The scheduler should be reusable for future recurring workloads rather than feature-specific ticker loops.
