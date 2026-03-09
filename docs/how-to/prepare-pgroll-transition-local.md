# How-to: Prepare the Local Database for a Future pgroll Transition

This repository now carries **minimal** `pgroll` adoption:

- local developer commands to run `pgroll` against the local CNPG primary
- an in-repo directory for future `pgroll` plans
- documentation for the transition path
- runtime support for an explicit Postgres `search_path` cutover via
  `DB_SCHEMA_SEARCH_PATH`

It does **not** switch the runtime over to `pgroll` yet. The active rollout
path remains the forward-only SQL hook job (`ecoflow-db-migrate-job`).

## Prerequisites

- local k3d stack is up
- `pgroll` is installed locally
- the local CNPG primary is reachable through `kubectl port-forward`
- the raw SQL migration validation path is already green:
  - `make db-migrate-cycle-local`
  - `make db-migrate-e2e-local`
  - `make test-db-migrations-ci`

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
- The environment rollout path is still the raw SQL migration hook job by
  default; `pgroll` plan execution remains an explicit operator action.
- To run two application versions against different schemas during a pgroll
  transition, set `DB_SCHEMA_SEARCH_PATH` per runtime deployment revision.
- Continue treating those local/CI commands as the promotion gate before any
  environment rollout automation is enabled.

## Runtime cutover note

The services chart now exposes `runtime.env.dbSchemaSearchPath`, which maps to
`DB_SCHEMA_SEARCH_PATH` in the runtime containers. During a future `pgroll`
transition, that is the explicit knob used to move old and new runtime
deployments onto different schema search paths without changing the base DSN.
