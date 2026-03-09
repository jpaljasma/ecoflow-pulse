# How-to: Prepare the Local Database for a Future pgroll Transition

This repository now carries **minimal** `pgroll` adoption:

- local developer commands to run `pgroll` against the local CNPG primary
- an in-repo directory for future `pgroll` plans
- documentation for the transition path

It does **not** switch the runtime over to `pgroll` yet. The active rollout
path remains the forward-only SQL hook job (`ecoflow-db-migrate-job`).

## Prerequisites

- local k3d stack is up
- `pgroll` is installed locally
- the local CNPG primary is reachable through `kubectl port-forward`

Official install/usage reference:

- `pgroll` GitHub README: [xataio/pgroll](https://github.com/xataio/pgroll)

## Local commands

Initialize `pgroll` metadata in the local database:

```bash
make pgroll-init-local
```

Inspect current `pgroll` migration status locally:

```bash
make pgroll-status-local
```

Start a future `pgroll` plan file from this repo:

```bash
make pgroll-start-local PGROLL_PLAN=deploy/db/pgroll/plans/<plan-file>
```

Complete or roll back the active `pgroll` rollout locally:

```bash
make pgroll-complete-local
make pgroll-rollback-local
```

## Important limitations

- These commands are for local experimentation and planning only.
- The application does not yet set version-aware schema search paths, so this
  repo is **not** using `pgroll`’s simultaneous multi-schema serving mode in
  production paths.
- Until that runtime work lands, keep environment rollout on the raw SQL
  migration job and continue validating schema changes with:
  - `make db-migrate-cycle-local`
  - `make db-migrate-e2e-local`
  - `make test-db-migrations-ci`
