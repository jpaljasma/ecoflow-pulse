# Energy Dashboard Task Board

Status: `DONE`  
Plan: `.codex/plans/energy-dashboard-ralph-loop.md`  
Spec: `/Users/jpaljasma/Downloads/energy-dashboard.md`  
Branch: `codex/energy-dashboard-loop-plan`  
Base commit: `8939db7`

## Assumptions

- `/energy` will be a first-class tab route in the universal app.
- Go will add `GetEnergyDashboard` to the existing telemetry service surface.
- `grid_price_per_kwh` will be client-side persisted for v1 unless an existing low-cost server setting path is discovered.
- Frontend implementation does not start until the backend gap audit says the required data path is real.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | `project-manager` | Bootstrap `.codex` plan/tasks/memory scaffolding | none | scaffolding created |
| DONE | `project-manager` | Add architecture tracking entry and sub-steps | bootstrap | `docs/architecture/README.md` updated |
| DONE | `backend-go` | Backend gap audit for energy metrics and per-port PV history | architecture tracking | findings recorded 2026-03-11 |
| DONE | `backend-go` | Go energy dashboard contract, aggregation, and query layer | backend gap audit | `go test ./internal/energydashboard ./internal/telemetryquery ./internal/grpcmw ./cmd/ecoflow-grpc-api -count=1` |
| DONE | `bff-node` | Node BFF `/api/v1/energy/dashboard` route and tests | Go contract shape stable | `npm run -w apps/pulse-platform typecheck` and `npm run -w apps/pulse-platform test -- history_routes.test.ts` |
| DONE | `frontend-universal` | Universal `/energy` route, URL-state sync, charts, and sections | BFF contract fixture stable | `npm run -w apps/universal typecheck` and `npm run -w apps/universal e2e:web -- devices.spec.ts energy.spec.ts` |
| DONE | `qa` | Regression coverage and review gates | per workstream | targeted Go + Node + universal web E2E validations recorded |
| DONE | `product-review` | Final spec audit and closeout | QA green | localhost review accepted 2026-03-11 |

## Decisions

- 2026-03-11: Start with a backend-first audit because the current rollup schema only guarantees `solar_generated_wh` plus power/SOC/temp metrics, while the spec requires additional energy buckets and per-port history.
- 2026-03-11: Keep implementation on one feature branch and use `.codex/memories/<agent>/memory.md` to avoid repeated full-repo scans.
- 2026-03-11: The current historical path does not expose explicit energy buckets or per-port historical maxima. Backend implementation must expand rollup/query storage before BFF/UI work can be truthful.
- 2026-03-11: Reuse the existing fleet authz pattern from `InferenceService.resolveFleetDeviceIDs` for all-device Energy scope instead of inventing a new authorization path.
- 2026-03-11: Start the backend implementation by isolating Energy-specific helpers in `internal/energydashboard` so window logic, scope normalization, and summary math are tested before RPC wiring begins.
- 2026-03-11: Expose the first BFF surface as `GET /api/v1/energy/dashboard` inside the existing history routes plugin to reuse shared auth, rate limiting, and gRPC error mapping.
- 2026-03-11: Parse boolean query flags explicitly in the BFF; `z.coerce.boolean()` is unsafe for URL strings like `?includeComparison=false` because non-empty strings coerce to `true`.
- 2026-03-11: Start the universal app with an honest vertical slice: real summary/battery data, scope/preset URL state, and placeholder-aware chart sections rather than inventing unsupported energy bucket history.
- 2026-03-11: Persist `grid_price_per_kwh` and currency locally in the universal app via a small zustand+AsyncStorage store, then forward those settings to the BFF request.
- 2026-03-11: Populate `currentPowerPoints`/`previousPowerPoints` from the Go Energy response using preset-appropriate power resolutions, while keeping the energy-bucket chart empty until explicit energy rollups exist.
- 2026-03-11: Fill more of the Energy spec on the frontend with real currently-available data: compare toggle, six-card hero summary, derived insight cards, and a PV operating envelope section based on live port metadata/capabilities.
- 2026-03-11: Derive explicit energy bucket fields in the query layer from existing average-power buckets as an interim contract step, return `currentEnergyPoints`/`previousEnergyPoints` from Go, and render a real energy-history chart in the universal app without waiting for a schema migration.
- 2026-03-11: Build a tested internal extractor for historical per-port PV observations from archived normalized quota envelopes so future API wiring can reuse canonical port parsing instead of inventing another provider-specific path.
- 2026-03-11: Reuse `replaycli` manifest/object readers inside `TelemetryService` so `GetEnergyDashboard` can return archive-backed per-device PV port history without introducing a second archive client stack.
- 2026-03-11: Align the universal route model with the public spec params (`device`, `preset`, `compare`, `tz`) while continuing to accept legacy internal aliases for compatibility.
- 2026-03-11: Make the browser API mock scope-aware and preset-aware so `/energy` E2E can validate that device/preset route changes reach the rendered dashboard state rather than only the URL bar.
- 2026-03-11: Add the required clipping / bottleneck insight card to the Energy insights section, sourced from the existing PV envelope diagnostics instead of leaving a spec gap at final review.
- 2026-03-11: Extend the hero energy chart to expose additional optional series already present in the contract (`AC input`, `DC output`, `Battery charge`, `Battery discharge`) while leaving `AC output` as an explicit backend gap.
- 2026-03-11: Add a compact battery flow strip and SOC band visual to the `/energy` page so the battery section is no longer summary-cards-only.
- 2026-03-11: Promote battery power to a first-class series on the secondary power chart so the chart default set is closer to the Energy spec without requiring another backend change.
- 2026-03-11: Make previous-period comparison explicit on the secondary power chart via lighter overlay series and matching point-count/status text instead of leaving comparison treatment implicit.
- 2026-03-11: Promote `ac_output_*` to a first-class derived backend metric in the telemetry query/gRPC contract and wire it through the universal Energy chart, closing the last major backend data gap called out in product review.

## Blockers

- No human blocker. Technical blocker resolved: backend expansion is mandatory before downstream work.

## Validation Log

- 2026-03-11: Audit only. Confirmed by code inspection that:
  - `deploy/db/migrations/000004_m3_rollups_hypertables_schema.up.sql` stores aggregate power/SOC/temp and `solar_generated_wh`, not the spec's explicit energy buckets.
  - `internal/rollupworker/types.go` and `internal/rollupworker/metrics.go` do not define or extract `ac_output_energy_wh`, `dc_output_energy_wh`, `load_energy_wh`, `battery_charge_energy_wh`, or `battery_discharge_energy_wh`.
  - `internal/telemetryquery/postgres.go` and `proto/pulse/telemetry/v1/telemetry.proto` are single-device and do not model Energy dashboard aggregates.
  - provider metadata and BFF device mapping already expose live PV port capability/detail data, but not historical per-port observations.
- 2026-03-11: `go test ./internal/energydashboard -count=1` passed after adding:
  - preset window resolution with DST-aware tests,
  - server-side Energy summary/value formulas,
  - `device=all|<uuid>` scope normalization helpers.
  - summary and battery model builders for a future Go-side dashboard response.
- 2026-03-11: `buf generate` passed after adding `GetEnergyDashboard` and its response models to `proto/pulse/telemetry/v1/telemetry.proto`.
- 2026-03-11: `go test ./cmd/ecoflow-grpc-api -count=1` passed after wiring `TelemetryService.GetEnergyDashboard` with authz-aware device scope resolution and series aggregation.
- 2026-03-11: `go test ./cmd/ecoflow-grpc-api -count=1` passed again after adding preset-aware power chart series and `includeComparison=false` query skipping.
- 2026-03-11: `npm run -w apps/pulse-platform typecheck` passed after extending the Node gRPC telemetry client with `getEnergyDashboard`.
- 2026-03-11: `npm run -w apps/pulse-platform test -- history_routes.test.ts` passed after adding `GET /api/v1/energy/dashboard` route coverage for single-device scope, fleet scope, and validation failures.
- 2026-03-11: `npm run -w apps/universal typecheck` passed after adding the `/energy` tab, energy feature client/hooks/model, and persisted settings store.
- 2026-03-11: `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts` passed for route-state normalization, chart series mapping, and persisted price-setting behavior.
- 2026-03-11: `npm run -w apps/universal typecheck` and `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts` passed again after adding compare toggle, insight derivation, and PV envelope modeling.
- 2026-03-11: `go test ./internal/telemetryquery ./internal/energydashboard ./cmd/ecoflow-grpc-api -count=1` passed after deriving interim energy buckets from average-power rollups and returning `currentEnergyPoints` / `previousEnergyPoints`.
- 2026-03-11: `npm run -w apps/pulse-platform typecheck` and `npm run -w apps/pulse-platform test -- solar_view.test.ts history_routes.test.ts` passed after updating normalized rollup fixtures for the expanded metrics contract.
- 2026-03-11: `npm run -w apps/universal typecheck` and `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts` passed after wiring the `/energy` screen to a real energy-history chart.
- 2026-03-11: `npm run -w apps/universal e2e:web -- energy.spec.ts devices.spec.ts` passed after adding mocked browser coverage for the `/energy` route.
- 2026-03-11: `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding interactive `/energy` coverage for the compare-toggle route state and previous-series behavior.
- 2026-03-11: `go test ./internal/energydashboard -count=1` passed after adding archive-envelope PV-port history extraction and aggregation helpers.
- 2026-03-11: `go test ./cmd/ecoflow-grpc-api ./internal/energydashboard -count=1` passed after wiring archive-backed `pvPortHistory` into `GetEnergyDashboard`.
- 2026-03-11: `npm run -w apps/pulse-platform typecheck` and `npm run -w apps/pulse-platform test -- history_routes.test.ts` passed after extending the dashboard contract with `pvPortHistory`.
- 2026-03-11: `npm run -w apps/universal typecheck`, `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after rendering historical PV maxima in the `/energy` PV envelope section.
- 2026-03-11: `npm run -w apps/universal typecheck`, `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed again after switching the route model to spec-aligned deep-link params.
- 2026-03-11: `npm run -w apps/universal typecheck` and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding scope-switch and preset-switch E2E coverage backed by a query-aware energy dashboard mock.
- 2026-03-11: `npm run -w apps/universal typecheck`, `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding the clipping/bottleneck insight card and rerunning the browser suite against a fresh exported web bundle.
- 2026-03-11: `npm run -w apps/universal typecheck`, `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding optional energy-chart series controls for `AC input`, `DC output`, `Battery charge`, and `Battery discharge`.
- 2026-03-11: `npm run -w apps/universal typecheck` and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding the battery flow strip and SOC band visual to the `/energy` battery section.
- 2026-03-11: `npm run -w apps/universal typecheck`, `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding battery power to the secondary power chart and updating shared chart callers.
- 2026-03-11: `npm run -w apps/universal typecheck` and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding previous-period overlays and explicit current/previous point summaries to the secondary power chart.
- 2026-03-11: `buf generate`, `go test ./internal/telemetryquery ./internal/energydashboard ./cmd/ecoflow-grpc-api -count=1`, `npm run -w apps/pulse-platform typecheck`, `npm run -w apps/universal typecheck`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after adding first-class `ac_output_*` metrics to the backend contract and wiring `AC output` into the hero energy chart.
- 2026-03-11: `npm run -w apps/universal typecheck`, `npm run -w apps/universal test -- src/features/energy/model.test.ts src/features/energy/store.test.ts src/features/history/powerTrend.test.ts`, and `npm run -w apps/universal e2e:web -- energy.spec.ts` passed after switching all-devices PV rendering to a table-first layout and adding browser coverage for the table headers.
- 2026-03-11: `make lint` passed after fixing a protobuf copy-by-value in archive PV history handling and an ineffectual fallback assignment in AC-output derivation.
- 2026-03-11: Local k3d validation succeeded via `make dev-deploy`; `https://localhost/energy?tz=America%2FNew_York` rendered live fleet summary cards, charts, battery flow, and PV table after fixing noop all-device Energy scope by forwarding the dev user subject through the Node history route and local gRPC noop auth path.
- 2026-03-11: `go test ./internal/energydashboard ./cmd/ecoflow-grpc-api -count=1` and `npm run -w apps/universal typecheck` passed after aligning `Estimated value` with displayed-period solar generation value instead of the older self-supplied-load formula, fixing real-solar / `$0.00` today cases in review.
- 2026-03-11: `go test ./cmd/ecoflow-grpc-api ./internal/energydashboard -count=1` passed after fixing fleet `AC output` aggregation in `mergeMetrics`; all-device Energy charts were previously flattening AC-output because `ACOutputAvgW`, `ACOutputMaxW`, and `ACOutputEnergyWh` were omitted from the multi-device merge path.
- 2026-03-11: Final closeout validation passed on the accepted branch state:
  - `make lint`
  - `go test ./internal/energydashboard ./internal/telemetryquery ./internal/grpcmw ./cmd/ecoflow-grpc-api -count=1`
  - `npm run -w apps/universal e2e:web -- devices.spec.ts energy.spec.ts`
  - `make dev-deploy`
- 2026-03-11: Localhost review accepted the final Energy dashboard UX at `https://localhost`, including device/fleet deep links, compare behavior, updated chart palettes, responsive PV layout, and quick-home navigation.

## Cost Notes

- Initial loop run only created tracking state and architecture bookkeeping.
- Deliberately skipped repo-wide lint/test during planning bootstrap to avoid unnecessary spend before the backend audit.

## Next Actions

1. Commit the accepted branch state and push `codex/energy-dashboard-loop-plan`.
2. Create the PR to `main`, verify multiline PR body rendering, and attach the final validation evidence.
3. Track persisted explicit energy buckets and any PV-history performance work as follow-up enhancements rather than blockers.

## Product Review Decision

- Current recommendation: `DONE` as a truthful v1 ship candidate.
- Accepted for: merge workflow and stakeholder use on the current feature branch state.
- Accepted methodology note:
  - interim derived energy buckets are accepted for v1 and are no longer treated as a release blocker.
