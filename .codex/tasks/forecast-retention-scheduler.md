# Forecast Retention and Scheduler Task Board

Status: `PROGRESS`  
Plan: `.codex/plans/forecast-retention-scheduler-ralph-loop.md`  
Branch: `codex/forecast-retention-scheduler`

## Assumptions

- This work is a production-safe patch, not a greenfield rewrite.
- Weather and solar client freshness will be aligned to `4h` without adding client polling.
- Reusable scheduling is a first-class outcome, not an implementation detail.
- Request-serving paths remain traffic-only for recurring maintenance work.

## Baseline

- Weather hot Postgres data measured locally for one canonical location is about `109.5 MB` logical and should fall to about `69.8 KB`.
- Solar hot Postgres data measured locally for one physical location hot window is about `651 MB` logical and should fall to about `15-16 MB`.
- Scheduled weather refreshes should fall from `48/day/location` to `6/day/location`.
- Request-shaped solar writes must be replaced by canonical scheduled site runs.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | `project-manager` | Create feature branch from updated `main` | none | `git checkout -b codex/forecast-retention-scheduler` |
| DONE | `project-manager` | Add architecture tracking entry and Ralph-loop scaffolding | branch ready | `.codex/plans/forecast-retention-scheduler-ralph-loop.md`, `.codex/tasks/forecast-retention-scheduler.md`, `docs/architecture/README.md` updated |
| DONE | `backend-go` | Add reusable scheduler worker, due-time schema, and worker wiring | tracking | `go test ./internal/scheduler ./cmd/ecoflow-scheduler -count=1`; `helm lint deploy/charts/pulse-services -f deploy/env/local/values.services.yaml`; `helm lint deploy/charts/pulse-services -f deploy/env/dev/values.services.yaml` |
| DONE | `backend-go` | Refactor weather storage to latest-only canonical snapshots and `4h` refresh | scheduler substrate | `go test ./internal/weatherd/... ./cmd/ecoflow-grpc-api -count=1`; `npm run -w apps/pulse-platform test -- weather_routes.test.ts`; `npm run -w apps/universal test -- src/features/weather/hooks.test.ts src/features/weather/api.test.ts src/features/weather/model.test.ts` |
| DONE | `backend-go` | Refactor solar persistence to canonical issue bucketing and hot retention prune paths | scheduler substrate | `go test ./internal/solarforecastd/... ./cmd/ecoflow-solar-verifier -count=1`; `npm run -w apps/pulse-platform test -- solarForecastClient.test.ts` |
| DONE | `qa` | Run targeted regression, E2E, and rollout validation | implementation slices | `npm run -w apps/universal e2e:web -- profile-weather.spec.ts`; targeted Go, BFF, universal, and Helm validation succeeded |
| TODO | `product-review` | Audit scale fit, product correctness, and operator ergonomics | QA pass | pending |

## Decisions

- 2026-04-21: Track this as a new Ralph-loop instead of folding it into the existing `weatherd` or `solar-forecastd` boards.
- 2026-04-21: Client freshness will move to `4h` by stale-time alignment, not polling.
- 2026-04-21: Reusable scheduling is implemented as a dedicated Postgres-backed scheduler worker with due-time claims and pluggable job handlers.

## Next Actions

1. Run live local or cloud DB evidence queries after migration so the board can capture exact before or after retention deltas on the updated schema.
2. Decide whether future scheduler growth needs JetStream dispatch or whether the current dedicated worker is sufficient for the near-term recurring job set.
3. Complete product-review signoff for scale fit and operator ergonomics.
