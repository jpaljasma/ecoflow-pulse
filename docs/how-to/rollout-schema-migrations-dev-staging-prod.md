# How-to: Roll Out Schema Migrations Across Dev, Staging, and Prod

This runbook defines the repository-supported rollout path for SQL schema
migrations after local and CI validation already passed.

## Preconditions

- local schema validation is green:
  - `make db-migrate-cycle-local`
  - `make db-migrate-e2e-local`
- CI schema validation is green:
  - `db-migrations-ci`
- the target environment already has:
  - a reachable Postgres/Timescale primary service,
  - a Kubernetes secret with DB app credentials,
  - the `pulse-services` Argo/Helm release wired to the current branch or `main`

## Rollout shape

The rollout path is **forward-only**:

1. validate locally and in CI
2. enable the migration hook job for the target environment
3. let the migration job run before the `pulse-services` workload sync
4. deploy application changes that depend on the migrated schema

The in-cluster job is:

- command: `ecoflow-db-migrate-job`
- source of truth: `deploy/db/migrations/*.up.sql`
- tracking table: `schema_migration_rollouts`
- sequencing:
  - Argo CD `PreSync` hook
  - Helm `pre-install,pre-upgrade` hook

The hook intentionally ignores all `*.down.sql` files. Environment rollback is a
separate operational procedure; it is not part of the automated rollout path.

## Dev

Use the recommended overlay as the starting point:

- [pulse-services-db-migrations.recommended.yaml](../../deploy/env/dev/recommended/pulse-services-db-migrations.recommended.yaml)

In dev, backups are not required by policy:

```yaml
runtime:
  migrations:
    enabled: true
    policy:
      rolloutEnv: dev
      requireBackup: false
      backupRef: ""
      forwardOnly: true
```

## Staging

Staging should use the same hook path, but must record the backup or snapshot
reference that was taken immediately before rollout:

```yaml
runtime:
  migrations:
    enabled: true
    policy:
      rolloutEnv: staging
      requireBackup: true
      backupRef: "cnpg-backup-2026-03-08T23:00:00Z"
      forwardOnly: true
```

If `requireBackup=true` and `backupRef` is empty, the migration job fails
closed before applying any SQL.

## Prod

Prod uses the same forward-only job, but the backup gate is mandatory and the
backup reference should point to the approved production backup artifact or
change-ticket record:

```yaml
runtime:
  migrations:
    enabled: true
    policy:
      rolloutEnv: prod
      requireBackup: true
      backupRef: "cnpg-prod-backup-2026-03-08T23:00:00Z"
      forwardOnly: true
```

## Validation and troubleshooting

- Render the services chart with the target values before sync:
  - `helm lint deploy/charts/pulse-services -f <values file>`
- Inspect the hook job in the platform namespace:
  - `kubectl -n pulse-platform get jobs`
  - `kubectl -n pulse-platform logs job/pulse-services-db-migrate`
- Verify tracked versions after success:
  - `SELECT version, rollout_env, backup_ref, applied_at FROM schema_migration_rollouts ORDER BY version;`

## Notes

- Local-first remains the required development path. The hook job does not
  replace the manual local validation commands.
- `pgroll` adoption remains a separate follow-up. This rollout path only covers
  the current SQL migration contract.
