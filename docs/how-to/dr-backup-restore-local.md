# How-To: Run Local Backup/Restore Drill (M5 DR-lite)

This runbook defines the local `k3d-pulse-local` disaster-recovery-lite drill for:

- Postgres (control-plane, rollups, archive manifest)
- MinIO raw archive objects (`pulse-telemetry-raw`)
- Valkey live snapshot cache as rebuildable state (not backup source-of-truth)

The drill uses Make targets that pin all Kubernetes actions to local context.

## Preconditions

1. Local platform and services are running:

```bash
make dev-up
make platform-wait
make services-wait
```

2. Local schema is applied and (optionally) seeded:

```bash
make db-migrate-up-local
make db-seed-dev-local
```

3. `kubectl` and Docker are installed.
4. If local port `19000` is already in use, override MinIO drill port:

```bash
DR_MINIO_LOCAL_PORT=19021 make dr-drill-local DR_BACKUP_NAME=2026-03-05T1800Z
```

## Backup Artifacts

Default backup location:

- `.tmp/dr-backups/latest/postgres.data.sql`
- `.tmp/dr-backups/latest/minio/<bucket>/...`
- `.tmp/dr-backups/latest/report.env`

Override the backup directory name with `DR_BACKUP_NAME`:

```bash
make dr-backup-local DR_BACKUP_NAME=2026-03-05T1800Z
```

## Run Backup Only

```bash
make dr-backup-local DR_BACKUP_NAME=2026-03-05T1800Z
```

What it does:

1. Dumps Postgres data (control-plane + archive manifest tables) from local CNPG primary to `postgres.data.sql`.
2. Mirrors MinIO archive bucket from cluster to local artifacts.
3. Writes `report.env` with baseline row/object counts used for restore validation.

## Restore From Existing Backup

```bash
make dr-restore-local DR_BACKUP_NAME=2026-03-05T1800Z
```

What it does:

1. Truncates managed control-plane + manifest tables and restores Postgres data from `postgres.data.sql` via `psql`.
2. Clears and restores MinIO bucket contents from local backup artifacts.

## Run Full DR Drill (Backup + Simulated Loss + Restore + Validation)

```bash
make dr-drill-local DR_BACKUP_NAME=2026-03-05T1800Z
```

What it does:

1. Runs `dr-backup-local`.
2. Simulates data loss by truncating key Postgres tables and removing MinIO bucket objects.
3. Runs `dr-restore-local`.
4. Runs `make db-migrate-verify-local`.
5. Validates restored DB/object counts are not below the backup baseline from
   `report.env` (`actual >= expected`). If live ingest writes new rows/objects
   during the drill, the command reports a non-fatal growth note.

Successful run ends with:

`drill validation passed (db + archive object counts restored; actual >= backup baseline)`

## Optional Replay Sanity Check After Restore

Use replay listing against restored manifest/object data:

```bash
make replay-cli ARGS='-mode list-devices -from 2026-02-26T00:00:00Z -to 2026-02-26T23:59:59Z'
```

## Notes

1. This runbook intentionally treats Valkey as rebuildable cache. Authoritative recovery sources remain Postgres + object archive.
2. Local MinIO is configured for non-persistent storage by default; backup artifacts on local filesystem are the recovery source during this drill.
