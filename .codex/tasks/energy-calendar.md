# Energy Calendar Task Board

Status: `PROGRESS`
Plan: `.codex/plans/energy-calendar-ralph-loop.md`
Branch: `codex/energy-calendar`
Base commit: `f96baa6`

## Assumptions

- Calendar money saved uses existing local `gridPricePerKwh` and `currency`.
- Selected-month totals exclude muted adjacent-grid days.
- Calendar values come from server day rollups, not client-side parallel daily dashboard calls.
- The new Pulse mark is a reusable in-app UI component, not a replacement app icon asset set.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | backend-go | Energy calendar proto, selected-date windows, service aggregation | branch setup | `go test ./internal/energydashboard ./internal/inference ./cmd/ecoflow-grpc-api -count=1` |
| DONE | bff-node | `/api/v1/energy/calendar` route and selected-date dashboard params | backend contract shape | `npm run -w apps/pulse-platform test -- history_routes.test.ts telemetry_client.test.ts`; `npm run -w apps/pulse-platform typecheck`; `npm run -w apps/pulse-platform lint` |
| DONE | frontend-universal | Calendar API/model/hooks/page/nav/date picker/device link/brand mark | BFF schema | targeted Vitest, `npm run -w apps/universal typecheck`, `npm run -w apps/universal lint`, `npm run -w apps/universal e2e:web -- energy-calendar.spec.ts` |
| DONE | qa | Unit/e2e/browser visual QA matrix | implementation | desktop/light/mobile screenshots; targeted Go/BFF/universal tests; `typecheck`; `lint`; E2E; `make lint`; `git diff --check` |
| DONE | product-review | Mockup fidelity, accessibility, docs | UI implementation | fixed seven-column CSS math, tightened Pulse/Apple grid sizing, removed rejected copy, verified light/dark/mobile renders |

## Decisions

- 2026-05-09: Add Calendar as primary nav after Energy.
- 2026-05-09: Use hybrid visual direction: B grid, Pulse rounded materials, A prefetch glyph and selected blue dot.
- 2026-05-09: Future dates show unavailable state and do not deep-link.
- 2026-05-09: Device page gets a Calendar hero action beside `Open Energy`.

## Blockers

- None.

## Next Actions

1. Stage focused branch changes.
2. Commit, push, and open PR.
