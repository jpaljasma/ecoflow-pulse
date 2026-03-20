# Weatherd Task Board

Status: `PROGRESS`  
Plan: `.codex/plans/weatherd-ralph-loop.md`  
Branch: `codex/weatherd-profile-forecast`

## Assumptions

- Public weather reads use the saved profile weather coordinates and never raw query coordinates.
- Public widget requests send fixed panel defaults `tilt=45`, `azimuth=0`.
- Internal gRPC keeps unit-system support, but the initial public BFF path stays metric-first.

## Workstreams

| Status | Owner | Workstream | Dependency | Latest validation |
|---|---|---|---|---|
| DONE | `project-manager` | Safeguard dirty worktree, switch to updated `main`, create feature branch | none | `git stash push -u`, `git checkout main`, `git pull --ff-only origin main`, `git checkout -b codex/weatherd-profile-forecast` |
| DONE | `project-manager` | Add architecture tracking entry and Ralph-loop scaffolding | branch ready | `.codex/plans/weatherd-ralph-loop.md`, `.codex/tasks/weatherd.md`, `docs/architecture/README.md` updated |
| PROGRESS | `backend-go` | Proto, Go domain core, stores, correction, and grpc-api wiring | tracking | in progress |
| PROGRESS | `bff-node` | Node weather grpc client, routes, and tests | proto shape settling | in progress |
| PROGRESS | `frontend-universal` | Profile weather widgets, hooks, tests, and E2E mocks | public route fixtures | in progress |
| TODO | `qa` | Targeted regression and coverage audit | implementation slices | pending |

## Decisions

- 2026-03-18: Implement weather in the existing `grpc-api` runtime instead of introducing a new binary.
- 2026-03-18: Public profile widget path uses fixed solar panel defaults while the internal service still accepts optional tilt and azimuth.
- 2026-03-18: Weather UI will use MaterialCommunityIcons for v1 instead of introducing a second icon asset system.

## Next Actions

1. Add `pulse.weather.v1` protobuf definitions and generate Go stubs.
2. Implement the Go weather service path and persistence/caching layers.
3. Integrate the Node and universal slices, then run targeted verification.
