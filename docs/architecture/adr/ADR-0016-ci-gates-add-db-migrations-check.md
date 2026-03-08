# ADR-0016: CI Gates — Add `db-migrations-ci` as a Required Merge Check

**Status:** Accepted  
**Date:** 2026-03-07  
**Owners:** Jaan  
**Related:** ADR-0010  
**Supersedes:** ADR-0010

---

## Context

ADR-0010 locked the initial required GitHub Actions checks for merges to `main`.
Since then, the repository has accumulated a non-trivial PostgreSQL/TimescaleDB
schema surface:
- control-plane tables (`users`, `devices`, `user_devices`)
- provider integration tables
- archive manifest tables
- rollup hypertables and retention policies

The architecture README already carries an explicit deferred follow-up to add a
dedicated migration CI workflow that validates the schema path end to end:
- apply all up migrations
- verify expected schema state
- apply all down migrations
- apply all up migrations again
- run end-to-end data/constraint checks

Without a required check for that path, migration regressions can still merge
even when the general Go and proto checks stay green.

### Requirements / Goals
- Keep the merge gate name stable as `db-migrations-ci`.
- Validate migrations against PostgreSQL 18 + TimescaleDB in CI.
- Reuse the same logical contract documented for local migration validation.

### Non-goals
- Replace the later `pgroll` adoption follow-up.
- Define environment rollout sequencing (`dev -> staging -> prod`) in this ADR.

---

## Options considered
### Option A: Rely on local-only migration commands
**Pros**
- No new CI time or infrastructure.

**Cons**
- Schema regressions remain easy to miss before merge.
- Required checks still do not reflect the documented migration contract.

### Option B: Add a GitHub Actions migration check backed by containerized Postgres/Timescale (chosen)
**Pros**
- Reproducible validation path in CI and locally.
- Directly exercises the migration files and schema expectations.
- Keeps required-check behavior auditable and explicit.

**Cons**
- Adds CI runtime and Docker/Testcontainers dependency.
- Requires a new required check name in branch protection.

---

## Decision

- We will add `db-migrations-ci` as a required merge check for `main`.
- We will validate migrations in CI with PostgreSQL 18 + TimescaleDB using the
  repository’s Go integration test harness.
- We will keep the final required check name stable via a wrapper job, even when
  internal execution is skipped for unrelated changes.

---

## Rationale

This keeps the merge gate aligned with the current schema risk:
- migrations now affect control-plane authz, replay metadata, and historical
  rollups, so they deserve an explicit gate,
- containerized validation is close enough to local CNPG/Timescale behavior for
  schema correctness checks,
- a wrapper job avoids the missing-check failure mode documented in the CI
  maintenance runbook.

---

## Consequences
### Positive
- Schema regressions are blocked before merge.
- Required-check policy matches the architecture README follow-up.
- Developers get a local, CI-aligned command for migration validation.

### Negative / Tradeoffs
- CI duration increases modestly.
- Docker/Testcontainers becomes part of the migration validation path.

### Risks & mitigations
- **Risk:** required check drift breaks merge protection.  
  **Mitigation:** keep the wrapper job name fixed as `db-migrations-ci` and
  update the required-check runbook in the same change.
- **Risk:** local and CI migration verification diverge.  
  **Mitigation:** mirror the local `up -> verify -> down -> up -> e2e` contract
  inside a dedicated integration test and keep both docs paths aligned.

---

## Implementation plan
1. Add a containerized migration cycle + e2e integration test.
2. Add a local `make test-db-migrations-ci` wrapper.
3. Add a GitHub Actions workflow with stable wrapper check naming.
4. Update architecture/docs and branch protection/ruleset required checks.

### Rollout / Migration
- Add `db-migrations-ci` to the repository ruleset required checks when the
  workflow lands.
- Keep existing required checks unchanged.

### Observability
- logs:
  - migration apply/down/verify/e2e test output in GitHub Actions

### Security / Compliance
- No new secrets are required.
- CI uses ephemeral containerized Postgres/Timescale only.

---

## Acceptance criteria
- PRs receive a stable `db-migrations-ci` check.
- The check validates `up -> verify -> down -> up -> e2e`.
- A local command exists to run the same migration validation path.

---

## Follow-ups
- [ ] Update GitHub branch protection / rulesets so `db-migrations-ci` is required alongside the other locked checks.
- [ ] Revisit this workflow after `pgroll` adoption to decide whether the gate should validate both raw SQL and `pgroll` rollout plans.
