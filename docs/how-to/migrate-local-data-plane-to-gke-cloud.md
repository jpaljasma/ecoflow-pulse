# How To Migrate the Local Data Plane to GKE Cloud

This runbook moves the authoritative local Pulse data plane into the hosted
Google Cloud profile in project `ecoflow-pulse-dev-260221-01`.

Scope:

- bootstrap the hosted `cloud` deploy profile on the regional GKE cluster
  `pulse-cloud` in `us-east1`,
- converge the hosted cluster onto the current steady-state node pools:
  `app-pool` (`e2-standard-2`, public/stateless, `us-east1-d`) and
  `stateful-pool` (`n2-highmem-4`, stateful anchor, `us-east1-c`),
- restore the local control-plane/history database into cloud CNPG,
- copy raw archive objects from local MinIO into cloud GCS without changing
  keys or prefixes,
- reconcile the restored user row onto the hosted Keycloak subject,
- import the `pulsemqtt` emulator device as inactive so history is available in
  cloud while live emulator ingest stays paused,
- switch local web-app sessions between `Local` and `Cloud` using the app’s new
  connection-profile setting.

## 1. Prerequisites

- local source-of-truth environment is healthy:
  - `make dev-up`
  - `make platform-wait services-wait`
- hosted cluster exists:
  - project: `ecoflow-pulse-dev-260221-01`
  - cluster: `pulse-cloud`
  - region: `us-east1`
  - expected node pools after hosted cutover:
    - `app-pool`
    - `stateful-pool`
- cloud domain, TLS issuer, Artifact Registry image repos, and Secret Manager
  secret names have been set in `deploy/env/cloud/*.yaml`
- Workload Identity bindings exist for:
  - `pulse-platform-gcp-secrets`
  - `pulse-services-runtime`
- GCS bucket exists for the raw archive:
  - bucket: `pulse-telemetry-raw`
  - prefix: `raw`

## 2. Bootstrap Argo CD and Hosted Apps

Fetch credentials and bootstrap the hosted app set:

```bash
make gke-cloud-context
make argocd-bootstrap-cloud
make argocd-apps-cloud
make argocd-wait-apps-cloud
```

Verify the hosted namespaces and core workloads:

```bash
kubectl get ns pulse-platform pulse-services
kubectl -n pulse-platform get deploy,sts,svc
kubectl -n pulse-services get deploy,svc
kubectl -n argocd get applications
```

Expected steady-state cloud topology from this branch:

- `pulse-platform-public-app`: `2`
- `pulse-platform-realtime-gateway`: `2`
- `pulse-services-go-grpc-api`: `2`
- `pulse-services-go-energy-api`: `2`
- `pulse-services-go-ingest`: `1`
- `pulse-services-go-inference`: `1`
- `pulse-services-go-projection`: `1`
- `pulse-services-go-archive`: `1`
- `pulse-services-go-rollup`: `1`
- `pulse-services-go-solar-verification`: `1`
- `pulse-platform-core-1`: `1`
- `pulse-platform-cloud-nats-0`: `1`
- `pulse-platform-cloud-valkey-node-0`: `1`

Current hosted storage profile after the cutover:

- node pools use `50Gi` `pd-balanced` boot disks,
- CNPG/NATS/Valkey now store real data on CSI-backed `standard-rwo`
  (`pd-balanced`) PVCs sized `50Gi`, `20Gi`, and `20Gi`,
- SSD performance budget should stay on those PVCs first rather than on large
  node boot disks,
- the live cluster now runs a cheaper safe idle posture of `1` `app-pool`
  node plus `1` `stateful-pool` node; scale `app-pool` back to `2` before
  larger rollouts so the public/stateless path regains surge headroom,
- CNPG does not need a large node boot disk to retain data; the durable
  history/metadata path is the PVC, not the node root disk,
- the current stateful PVCs are still zonal `us-east1-c` disks, so a different
  zone is not a simple reschedule target for the existing claims; future
  resiliency work needs multi-zone replicas and, where justified, regional
  disks.

## 3. Restore the Local Control-Plane DB into Cloud CNPG

Create a local logical backup from the authoritative local CNPG service:

```bash
LOCAL_DSN='postgres://pulse:pulse-local-dev-password@127.0.0.1:15432/pulse?sslmode=disable'
pg_dump --format=custom --no-owner --no-privileges --dbname "${LOCAL_DSN}" > /tmp/pulse-local.dump
```

Port-forward the hosted CNPG write service and restore into cloud:

```bash
kubectl -n pulse-platform port-forward svc/pulse-platform-core-rw 25432:5432
PGPASSWORD='replace-me-cloud-db-password' \
  pg_restore \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    --dbname 'postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' \
    /tmp/pulse-local.dump
```

Verify the expected control-plane tables exist and contain data:

```bash
psql 'postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' -c '\dt'
psql 'postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' -c 'select count(*) from users;'
psql 'postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' -c 'select count(*) from provider_devices;'
```

Immediately re-apply the latest branch migrations after restore so any
idempotent post-restore schema repairs are present before worker traffic ramps
up:

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' \
DB_MIGRATIONS_DIR='deploy/db/migrations' \
go run ./cmd/ecoflow-db-migrate-job
```

The current branch includes repair migrations for restored schemas that are
missing primary keys, unique constraints, or supporting indexes on
`provider_devices`, weather forecast tables, Timescale rollup/PV-port tables,
solar forecast tables, rollup envelope dedup, or `archive_object_manifest`.

## 4. Copy Raw Archive Objects from Local MinIO to Cloud GCS

Expose local MinIO temporarily:

```bash
kubectl --context k3d-pulse-local -n pulse-platform port-forward svc/pulse-platform-minio 19000:9000
```

Mirror the bucket into GCS while preserving object keys:

```bash
mc alias set pulse-local http://127.0.0.1:19000 minio minio123
mc alias set pulse-cloud https://storage.googleapis.com "${GOOGLE_ACCESS_KEY_ID}" "${GOOGLE_SECRET_ACCESS_KEY}"
mc mirror --overwrite pulse-local/pulse-telemetry-raw/raw pulse-cloud/pulse-telemetry-raw/raw
```

If steady-state cloud access uses Workload Identity only, the HMAC credentials
above are just a temporary migration bootstrap path. Remove them after the
mirror completes.

## 5. Audit and Reconcile the Archive Before Any Rebuild

Run archive integrity checks against the hosted manifest/object store pair:

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_PROVIDER='gcs' \
ARCHIVE_OBJECT_BUCKET='pulse-telemetry-raw' \
ARCHIVE_OBJECT_PREFIX='raw' \
ARCHIVE_OBJECT_GCS_PROJECT_ID='ecoflow-pulse-dev-260221-01' \
go run ./cmd/ecoflow-archive-audit
```

If the audit reports manifest/object drift, reconcile first:

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' \
ARCHIVE_OBJECT_PROVIDER='gcs' \
ARCHIVE_OBJECT_BUCKET='pulse-telemetry-raw' \
ARCHIVE_OBJECT_PREFIX='raw' \
ARCHIVE_OBJECT_GCS_PROJECT_ID='ecoflow-pulse-dev-260221-01' \
go run ./cmd/ecoflow-archive-reconcile
```

Only run targeted rollup rebuilds for windows where the audit proves drift. Do
not do a destructive full replay as the default migration step.

## 6. Sign In Once to Hosted Keycloak and Reconcile the User Subject

After the hosted Keycloak realm is live and Google login is configured, sign in
once as `jpaljasma@gmail.com` so the hosted identity provider issues the new
subject.

Then remap the restored user row by verified email:

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' \
go run ./cmd/ecoflow-user-subject-reconcile \
  -email 'jpaljasma@gmail.com' \
  -user-subject '<hosted-keycloak-sub>'
```

The command fails closed if that hosted subject already belongs to a different
email address.

## 7. Import the Emulator Device as Inactive

Use the new inactive-import path so the emulator’s historical data can exist in
cloud while live ingest remains paused:

```bash
curl -X POST 'https://pulse.example.com/api/v1/devices/available/import' \
  -H 'Authorization: Bearer <hosted-access-token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "provider": "pulsemqtt",
    "credentialId": "<pulsemqtt-credential-id>",
    "providerDeviceId": "PULSEDPUX24K001",
    "isActive": false,
    "ingestDesiredState": "paused"
  }'
```

Validate the stored device state:

```bash
psql 'postgres://pulse:replace-me-cloud-db-password@127.0.0.1:25432/pulse?sslmode=disable' \
  -c "select provider, provider_device_id, is_active, ingest_desired_state from provider_devices where provider='pulsemqtt';"
```

## 8. Point the Local App at Cloud

Build or run the universal app with both profiles configured:

```bash
EXPO_PUBLIC_CLOUD_API_URL='https://pulse.example.com' \
EXPO_PUBLIC_CLOUD_WS_URL='wss://pulse.example.com/ws' \
EXPO_PUBLIC_CLOUD_OIDC_ISSUER_URL='https://pulse.example.com/realms/pulse' \
EXPO_PUBLIC_CLOUD_OIDC_CLIENT_ID='pulse-universal-app' \
EXPO_PUBLIC_CLOUD_OIDC_AUDIENCE='pulse-universal-app' \
npm run -w apps/universal web -- --clear
```

Then use `Settings -> Connection Profile` to switch between:

- `Local`
- `Cloud`

Expected profile-switch behavior:

- REST base URL changes with the selected profile,
- websocket reconnects to the selected `/ws` endpoint,
- auth/query cache is partitioned by profile,
- switching issuer/client forces a re-login.

## 9. Acceptance Checklist

- hosted Keycloak login works for `jpaljasma@gmail.com`
- associated devices appear under the hosted account
- emulator device is present but `is_active=false` and `ingest_desired_state=paused`
- historical charts load from the hosted archive/history path
- local web sessions can switch cleanly between `Local` and `Cloud`
- cloud ingest/storage continue operating with local persistent services turned off

## 10. Rollback Notes

If the hosted migration is not accepted:

- keep the local stack as the source of truth,
- pause or scale down the hosted `pulse-services` workloads,
- do not delete the local archive or CNPG volumes until archive audit and
  chart-level acceptance have both passed,
- rerun archive audit/reconcile before any second restore attempt.
