# ADR-0010: CI Gates — Required GitHub Actions Checks for Merge

**Status:** Superseded  
**Date:** 2026-02-20
**Superseded by:** ADR-0016

## Context
EcoFlow Pulse now has a split stack (Go backend/runtime + Expo universal client) and increasing architecture rigor via ADRs. The repository already enforces PR-based delivery and code scanning controls, but merge quality can still drift if frontend validation is optional.

To keep throughput high without regressions, CI must reliably gate both runtime domains:
- Go services/tooling
- Expo TypeScript app (web/iOS/Android code paths)

## Decision
Add and enforce required GitHub Actions status checks for merges to `main`:
- `go-test`
- `go-test-race-critical`
- `frontend-ci`
- `proto-ci`
- `CodeQL`

CI workflow intent:
- `go-test` validates backend/runtime contracts.
- `go-test-race-critical` validates race-safety on critical concurrent service paths.
- `frontend-ci` validates `typecheck`, `lint`, tests, and Expo web build sanity.
- `proto-ci` validates protobuf contract hygiene (`buf lint`) for `proto/`.
- `CodeQL` remains the security/code-scanning gate.

Ruleset/branch protection must treat these checks as mandatory (no optional merge when checks fail).

## Consequences
### Positive
- Prevents frontend regressions from bypassing merge gates.
- Aligns delivery quality across backend and universal app.
- Makes CI policy explicit and auditable as architecture governance.

### Tradeoffs
- Longer PR feedback cycle due to additional checks.
- Occasional friction when CI infra or flaky tests fail.

### Risks & mitigations
- **Risk:** Workflow/check name drift breaks required checks.  
  **Mitigation:** Keep stable check names and document them as locked CI contract.
- **Risk:** Path-filtered frontend CI may not run for indirect frontend-impacting changes.  
  **Mitigation:** include shared package/manifests/workflow paths in frontend-ci triggers; review periodically.

## Follow-ups
- [x] Add this CI-gate policy to architecture summary docs (`docs/architecture/README.md`).
- [x] Add a short runbook for maintaining required checks/rulesets.
- [x] Monitor CI duration and optimize caching/test split if developer latency increases.
