# Solar Forecastd Task Board

Status: `PROGRESS`  
Plan: `.codex/plans/solar-forecastd-ralph-loop.md`  
Branch: `codex/solar-forecastd-baseline`

## Assumptions

- The current slice focuses on a deterministic baseline, not full inference models yet.
- Energy/history data is the source of truth for actual generation so far.
- Weather data is used to estimate the remaining horizon and future daily potential.
- Conservative observed-production calibration is preferable to optimistic panel-limit guesses.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | `project-manager` | Mark architecture tracking `PROGRESS`, add ADR-0024, and create Ralph-loop scaffolding | none | `docs/architecture/README.md`, `docs/architecture/adr/ADR-0024-...`, `.codex/plans/solar-forecastd-ralph-loop.md`, `.codex/tasks/solar-forecastd.md` |
| PROGRESS | `frontend-universal` | Wire fleet solar history into the profile weather experience and replace widget-only solar guesses with actual-so-far plus remaining forecast | weather/profile route ready | `npm run -w apps/universal test -- src/features/weather/hooks.test.ts src/features/weather/model.test.ts src/features/weather/api.test.ts`; `npm run -w apps/universal e2e:web -- profile-weather.spec.ts` |
| PROGRESS | `backend-go` | Define dedicated solar forecast contract and persistence runway for forecast-vs-actual records, then wire deterministic `SolarForecastService` into grpc-api | ADR-0024 | `buf generate`; `go test ./internal/solarforecastd/... ./cmd/ecoflow-grpc-api/... -count=1`; `go test ./internal/integrationtest -count=1` |
| DONE | `bff-node` | Add explicit public solar outlook contract and DRY current-user/weather-context helpers | backend contract | `npm run -w apps/pulse-platform typecheck`; `npm run -w apps/pulse-platform test -- weather_routes.test.ts` |
| PROGRESS | `qa` | Extend deterministic regression coverage and later verification-quality coverage | implementation slices | `npm run -w apps/universal typecheck`; `npm run -w apps/universal test -- src/features/weather/api.test.ts src/features/weather/hooks.test.ts src/features/weather/model.test.ts`; `npm run -w apps/universal e2e:web -- profile-weather.spec.ts` |

## Decisions

- 2026-03-18: Use energy/history truth for actual solar generated so far instead of relying on device input ceilings.
- 2026-03-18: Keep weather-driven solar math in shared model helpers, not duplicated inside multiple widgets.
- 2026-03-18: Use a conservative deterministic baseline now, and preserve clean seams for later ML inference models.

## Next Actions

1. Persist forecast-vs-actual training records from the live solar service path and add verification metrics/quality rollups.
2. Add solar service observability plus SLO panels for availability, freshness, and budget/quality burn.
3. Promote site-calibrated forecasting before any full ML rollout.
4. Replace the remaining placeholder/fallback client heuristics once the service owns all solar view-model shaping.
