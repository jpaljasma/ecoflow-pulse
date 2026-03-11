# Energy Dashboard Ralph-Loop Plan

Status: Draft for review only  
Last updated: 2026-03-11  
Do not start implementation until the user approves this plan.

## Goal

Build the Energy page described in `/Users/jpaljasma/Downloads/energy-dashboard.md` across the existing stack:

- Go gRPC query layer
- Node REST BFF
- Expo universal app
- QA + final product review

The result must ship `/energy`, support `device=all|<uuid>`, calendar-based presets, previous-period comparison, server-side authz, and a spec-matching UI on web and mobile.

## Source Of Truth

Use these inputs in this order:

1. `/Users/jpaljasma/Downloads/energy-dashboard.md`
2. `/Users/jpaljasma/Library/Mobile Documents/com~apple~CloudDocs/Projects/ecoflow-api-playground/AGENTS.md`
3. `/Users/jpaljasma/Library/Mobile Documents/com~apple~CloudDocs/Projects/ecoflow-api-playground/docs/architecture/README.md`
4. Relevant locked architecture docs and rules already enforced in this repo:
   - time handling
   - universal UI data rules
   - auth boundary rules
   - UUID-only route/query param rules

## Repo-Specific Non-Negotiables

- Stay on one feature branch for the implementation task. Do not commit to `main`.
- Before the first product-code chunk, update `docs/architecture/README.md` to add or mark the Energy Dashboard task `PROGRESS` and add checkbox sub-steps.
- Keep docs in the same branch as code changes.
- User-facing time windows must use local calendar math first, then UTC transport.
- Universal UI route/query params must use canonical UUID device IDs only.
- The Energy page body must be scrollable on web and mobile.
- Node validates JWT, forwards JWT to Go, and Go validates again plus device authz.
- Do not silently drop spec items because data is inconvenient. If telemetry gaps exist, make them explicit and solve them or escalate.

## Implementation Bias

Default choices for v1 unless the user asks to change them:

- Add a dedicated `GetEnergyDashboard` RPC to the existing `TelemetryService` instead of creating a brand-new gRPC service.
- Add a page-specific Go aggregation package, likely `internal/energydashboard`, instead of pushing analytics logic into the BFF or frontend.
- Add a dedicated BFF route `GET /api/v1/energy/dashboard`.
- Add the page at `apps/universal/app/(tabs)/energy.tsx` so `/energy` is first-class and discoverable.
- Store `grid_price_per_kwh` and optional currency as a client-side persisted preference keyed by auth subject for v1 unless a server-side settings path already exists and is cheap to reuse.

## Critical Risk To Resolve First

The current rollup schema only guarantees:

- `soc_*`
- `ac_in_*`
- `pv_*`
- `dc_*`
- `load_*`
- `battery_*`
- `temp_*`
- `solar_generated_wh`

The spec also requires explicit energy buckets and per-PV-port history. The first engineering task must confirm whether these can be derived from current ingest/rollup inputs or whether new rollup fields and backfill-safe migrations are required. Do not start frontend implementation against guessed backend data.

## Agent Roster

Use six agents, but cap active implementation agents to three at a time to control cost.

| Agent | Primary scope | Memory file |
|---|---|---|
| `project-manager` | orchestration, task board, dependency ordering, progress/cost reporting, blocker handling | `.codex/memories/project-manager/memory.md` |
| `backend-go` | proto, Go service, query aggregation, rollup/schema changes, Go tests | `.codex/memories/backend-go/memory.md` |
| `bff-node` | Node route, grpc client wiring, DTO/view-model mapping, Node tests | `.codex/memories/bff-node/memory.md` |
| `frontend-universal` | Expo route, URL state, feature hooks/components, persisted price setting, UI tests | `.codex/memories/frontend-universal/memory.md` |
| `qa` | review every chunk, add/expand tests, run regression matrix, reject incomplete slices | `.codex/memories/qa/memory.md` |
| `product-review` | final spec audit, UX correctness, end-to-end acceptance check | `.codex/memories/product-review/memory.md` |

Rules:

- `project-manager` does not write product code unless a worker is blocked and reassignment is cheaper than waiting.
- `qa` can add tests and small correctness fixes, but should not become a second feature engineer.
- `product-review` only engages after a usable vertical slice exists and again at final signoff.

## Shared State Files

The loop must keep these files current:

- Plan: `.codex/plans/energy-dashboard-ralph-loop.md`
- Canonical board: `.codex/tasks/energy-dashboard.md`
- Agent memories:
  - `.codex/memories/project-manager/memory.md`
  - `.codex/memories/backend-go/memory.md`
  - `.codex/memories/bff-node/memory.md`
  - `.codex/memories/frontend-universal/memory.md`
  - `.codex/memories/qa/memory.md`
  - `.codex/memories/product-review/memory.md`

The task board must include:

- current status for each workstream: `TODO`, `PROGRESS`, `REVIEW`, `BLOCKED`, `DONE`
- current owner
- dependency
- latest validation result
- open blockers
- cost notes

Each agent memory must include:

- current understanding
- files touched or planned
- decisions made
- open risks
- next step

## Progress Output Format

The loop must emit short progress updates after each meaningful batch, not a long transcript.

Use this format:

```text
Progress
- done:
- in flight:
- next:
- tests:
- blockers:
- cost note:
```

Rules:

- 6 lines max
- mention concrete files or commands
- call out when the loop is reusing existing contracts to save time/cost
- never hide a blocker behind vague language

## Work Breakdown

### Phase 0: Bootstrap And Guardrails

Owner: `project-manager`

1. Create or refresh the task board and all memory files.
2. Add the Energy Dashboard task to `docs/architecture/README.md` and mark it `PROGRESS` before product code starts.
3. Add checkbox sub-steps under that task so progress is visible in commits.
4. Record the branch name, current commit, and implementation assumptions in the task board.
5. Open only the minimum relevant files for each worker to reduce context cost.

Deliverable:

- no product code yet
- planning state files exist
- architecture README is ready for implementation tracking

### Phase 1: Backend Gap Audit

Owner: `backend-go`  
Reviewer: `qa`

Inspect these areas first:

- `deploy/db/migrations/000004_m3_rollups_hypertables_schema.up.sql`
- `internal/rollupworker/*`
- `internal/telemetryquery/*`
- `proto/pulse/telemetry/v1/telemetry.proto`
- `cmd/ecoflow-grpc-api/*`

Required outputs:

1. Confirm what energy metrics already exist.
2. Confirm whether per-port PV history can be read from current data.
3. Decide whether new rollup columns, read models, or query-side derivations are required.
4. Write the decision and missing fields into `.codex/memories/backend-go/memory.md`.
5. Update `.codex/tasks/energy-dashboard.md` with a clear go/no-go statement for frontend work.

Hard rule:

- if exact energy buckets cannot be produced from current telemetry, implement the backend data path first; do not fake those cards in UI.

### Phase 2: Parallel Implementation

Only start this phase after Phase 1 is explicitly green on the task board.

#### Workstream A: Go Contract And Query Layer

Owner: `backend-go`

Likely files:

- `proto/pulse/telemetry/v1/telemetry.proto`
- `gen/pulse/telemetry/v1/*`
- `cmd/ecoflow-grpc-api/telemetry_service.go`
- `cmd/ecoflow-grpc-api/telemetry_service_test.go`
- `internal/telemetryquery/*`
- `internal/energydashboard/*` if created
- `internal/rollupworker/*`
- `deploy/db/migrations/*`

Scope:

- add the Energy dashboard request/response contract
- resolve `all devices` scope inside Go with authz
- compute calendar-window current + previous windows server-side
- aggregate energy/power series by preset
- compute summary cards, battery section, PV envelope summaries, and insight heuristics
- keep `QueryRollupRange` and `CompareRollupRange` intact

#### Workstream B: Node BFF

Owner: `bff-node`

Likely files:

- `apps/pulse-platform/src/app.ts`
- `apps/pulse-platform/src/routes/energy.ts`
- `apps/pulse-platform/src/grpc/telemetryClient.ts` or `src/grpc/energyClient.ts`
- `apps/pulse-platform/test/*`

Scope:

- add `GET /api/v1/energy/dashboard`
- validate `device`, `preset`, `compare`, optional `from`, `to`, `tz`
- pass through auth header and request id
- normalize the gRPC response into a stable frontend view model
- return clean auth/error semantics

#### Workstream C: Universal Frontend

Owner: `frontend-universal`

Likely files:

- `apps/universal/app/(tabs)/_layout.tsx`
- `apps/universal/app/(tabs)/energy.tsx`
- `apps/universal/src/features/energy-dashboard/*`
- `apps/universal/src/shared/*` only when existing shared UI is worth reusing
- `apps/universal/test or feature-local test files`

Scope:

- add `/energy` route and tab entry
- sync URL state for device, preset, compare, optional custom range
- render hero cards, energy chart, power chart, PV section, battery section, insights, and price section
- use `ScrollView` or equivalent scroll container
- preserve layout height during loading/refetch
- reuse canonical UUID device ids only

Frontend rule:

- do not rebuild backend math in the client unless the spec explicitly calls for client-only behavior

### Phase 3: QA Hardening

Owner: `qa`

QA must review each workstream before it is marked `DONE`.

Required coverage:

- Go unit tests for window math, scope authz, aggregation, and insight heuristics
- Node tests for route validation, auth forwarding, compare behavior, and unauthorized scope
- Frontend tests for URL-state sync, empty states, loading stability, and summary rendering
- DST-sensitive tests for `today`, `yesterday`, `thisWeek`, `previousWeek`, `thisMonth`, and `last12m`
- Playwright web coverage for `/energy`

Optional only if navigation/layout changes require it:

- update Maestro mobile smoke

### Phase 4: Product Review And Closeout

Owner: `product-review`  
Coordinator: `project-manager`

Tasks:

1. Review the delivered page against the attached spec line by line.
2. Verify copy/tone and directional semantics on deltas.
3. Verify web and mobile layout quality.
4. Verify all-devices vs single-device behavior.
5. Verify price section empty-state and configured-state behavior.
6. Record any spec mismatches as blocking defects, not polish.
7. After green review, ensure docs and task board are updated with final validation evidence.

Closeout steps:

1. If new developer-facing commands were added or changed, update `docs/reference/commands.md`.
2. Push the feature branch to `origin`.
3. Create the PR with `gh pr create --body-file <file>`.
4. Verify the PR body with `gh pr view <number> --json url,title,body`.
5. Treat escaped newlines, missing validation notes, or missing doc references as defects and fix them immediately.

## Cost Controls

This loop must be persistent, but it must not be wasteful.

1. Maximum three active implementation agents at once: `backend-go`, `bff-node`, `frontend-universal`.
2. `qa` runs targeted tests first; run full repo gates only after a chunk is plausibly green.
3. Reuse one stable contract fixture between BFF and frontend to avoid duplicate mock design.
4. Do not run web export, Playwright, or mobile smoke on every patch.
5. Do not reread the full repo each loop. Each agent should keep its own memory and reopen only changed files.
6. If the same failure repeats three times, `project-manager` must change strategy instead of brute-forcing.
7. Progress updates must mention what was skipped intentionally to save cost.

## Required Validation Gates

Run the smallest relevant set per chunk, then the full set before signoff.

Backend chunk:

- `buf generate`
- targeted `go test` for touched Go packages
- `go test ./...` before handoff
- `make test-race` if rollupworker, grpc hot path, or async worker behavior changes materially

BFF chunk:

- `npm run -w apps/pulse-platform typecheck`
- `npm run -w apps/pulse-platform lint`
- `npm run -w apps/pulse-platform test`

Frontend chunk:

- `npm run -w apps/universal typecheck`
- `npm run -w apps/universal lint`
- `npm run -w apps/universal test`

Final signoff:

- `go test ./...`
- `npm run -w apps/pulse-platform typecheck`
- `npm run -w apps/pulse-platform lint`
- `npm run -w apps/pulse-platform test`
- `npm run -w apps/universal typecheck`
- `npm run -w apps/universal lint`
- `npm run -w apps/universal test`
- `npm run -w apps/universal e2e:web`
- `make lint`

## Stop Conditions

The loop should keep going through normal failures:

- failing tests
- bad patches
- incorrect assumptions that can be corrected from repo context
- rework needed after QA review

The loop must pause and ask the user only for hard blockers:

- required telemetry fields do not exist and cannot be derived safely from current data
- the user wants a different route placement than a tabbed `/energy`
- the user wants server-side price persistence instead of client-side v1 storage
- the implementation would violate locked architecture or accepted ADRs

## Definition Of Done

The task is done only when all of the following are true:

- `/energy` exists and is navigable
- `all devices` and single-device mode both work
- URL state reloads correctly
- all required presets work with correct previous-period comparison
- summary cards, energy chart, power chart, PV section, battery section, insights, and price section match the spec
- authz is enforced in BFF and Go
- QA has reviewed every workstream
- product review is green
- `docs/architecture/README.md` and any changed docs are updated
- validation evidence is recorded in the task board

## First Run Checklist For The Loop

1. Wait for user approval of this plan.
2. Create the task board and memory files.
3. Add the tracked Energy Dashboard task to `docs/architecture/README.md` and mark it `PROGRESS`.
4. Run Phase 1 backend gap audit before any frontend coding.
5. Keep updates short and cost-aware.
