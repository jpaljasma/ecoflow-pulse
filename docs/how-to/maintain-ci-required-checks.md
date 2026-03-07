# Maintain Required CI Checks

This runbook keeps merge protection aligned with ADR-0016.

## Required checks on `main`

- `go-test`
- `go-test-race-critical`
- `frontend-ci`
- `proto-ci`
- `CodeQL`
- `db-migrations-ci`

These names are treated as an architecture contract. Do not rename checks casually.
The `frontend-ci`, `CodeQL`, and `db-migrations-ci` required checks may
aggregate multiple internal shard jobs, but the wrapper job names themselves
must stay stable.

## When workflows change

Use this sequence whenever you add/rename workflows, jobs, or path filters:

1. Update workflow YAML in `.github/workflows/`.
2. Verify expected check names in a PR run.
3. Update branch protection/ruleset required checks to match new names.
4. Confirm the PR is still blocked when required checks fail.
5. Update ADR/docs if the policy changed.
6. Prefer internal no-op gating and wrapper jobs over broad workflow-level
   filters when a required check name must remain present on the PR.

## Verify check names from CLI

```bash
gh pr checks <pr-number>
gh run list --limit 20
```

## Monitor CI duration and regression risk

Review CI performance on a regular cadence (weekly is enough for current scale):

1. Inspect median and p95 durations for `go-test`, `go-test-race-critical`, `frontend-ci`, `proto-ci`, `CodeQL`, and `db-migrations-ci`.
2. If developer latency increases, optimize in this order:
   - dependency/cache hit rates,
   - internal changed-files gating for required jobs,
   - path filters,
   - test splitting/parallelism,
   - flaky test remediation.
3. Re-check that required checks and workflow names are unchanged.

## Common failure mode

If a required check is missing after a workflow edit, merges can be blocked or unintentionally loosened depending on ruleset behavior. Always validate ruleset/check alignment in the same PR.
