# Reference: Commands

## Go Commands

```bash
go test ./...
go run ./cmd/ecoflow-smoke
go run ./cmd/ecoflow-server
go run ./cmd/ecoflow-mqtt-sub
go run ./cmd/ecoflow-pv-fingerprint
go run ./cmd/ecoflow-panel-db-import
go run ./cmd/ecoflow-panel-csv-backfill
go run ./cmd/ecoflow-panel-select-train
```

## Helper Scripts

```bash
./scripts/regenerate_solar_panel_db.sh
./scripts/train_panel_select_model.sh
```

Optional link map override for panel DB regeneration:

```bash
SOLAR_PANEL_LINK_MAP=data/solar_panels/panel_purchase_links_v13.json ./scripts/regenerate_solar_panel_db.sh
```

## Make Targets

```bash
make lint
make test
make bench
make build
make smoke
make mqtt
make k3d-up
make platform-up
make platform-wait
make services-up
make services-wait
make dev-up
make dev-down
make db-migrate-up-local
make db-migrate-down-local
make db-migrate-verify-local
make db-migrate-cycle-local
make db-migrate-e2e-local
make auth-keycloak-verify-local
make gke-context
make gke-dev-guardrails
make gke-park
make gke-wake
make scale-down
make scale-up
make argocd-bootstrap-dev
make argocd-apps-dev
make argocd-wait-apps
make argocd-dev-up
make web-stop
make web
```

For required local tooling (for example `helm`, `k3d`, `kubectl`), see:
`docs/reference/local-dev-prerequisites.md`.

Notes:

- default `GOFLAGS` in `Makefile` include `-tags=moderncompress -mod=mod`,
- `make mqtt` exits cleanly on `q`/`Ctrl+C` and does not return non-zero on
  intentional stop.
- `make web` restarts Expo web by first stopping any process listening on
  `WEB_PORT` (default `8081`), then running:
  `npm run -w apps/universal web -- --port $(WEB_PORT) --clear`.
- `make k3d-up` creates or reuses local k3d cluster from `deploy/tilt/k3d-config.yaml`.
  Requires `k3d`, `kubectl`, and a running Docker daemon.
  It also switches `kubectl` context to `k3d-pulse-local`.
- `make platform-up` updates Helm deps and installs/upgrades `pulse-platform` using `deploy/env/local/values.platform.yaml`.
  Local Helm/Kubernetes operations are pinned to `k3d-pulse-local` (`--kube-context` / `--context`) so local targets cannot accidentally apply to GKE.
  It includes retry/backoff for transient CNPG webhook race conditions during initial install.
  It runs a CRD-safe reconcile flow for CloudNativePG:
  1) initial Helm install/upgrade,
  2) wait for CNPG operator deployment readiness,
  3) second Helm pass to apply CRD-backed resources (for example `Cluster`).
  Connection + bootstrap contract:
  - bootstrap app credentials are configured in `cloudnativepgCluster.bootstrap.*` and rendered to secret `pulse-platform-core-app`,
  - service-facing contract is exposed via configmap `pulse-platform-core-contract`,
  - DSN-style connection secret is exposed via `pulse-platform-core-connection`,
  - local Keycloak is configured to use CNPG (`keycloak.postgresql.enabled=false`, `keycloak.externalDatabase.*` -> `pulse-platform-core-rw`).
  Validation examples:
  - `kubectl get sts -n pulse-platform`
  - `kubectl get pods -n pulse-platform`
  - `kubectl get deploy -n pulse-platform`
  - `kubectl get nodes -o wide`
  - `kubectl get clusters.postgresql.cnpg.io -n pulse-platform`
  - `kubectl get configmap pulse-platform-core-contract -n pulse-platform -o yaml`
  - `kubectl get secret pulse-platform-core-connection -n pulse-platform -o jsonpath='{.data.url}' | base64 -d; echo`
  - `kubectl get cluster pulse-platform-core -n pulse-platform -o yaml | rg "imageName|shared_preload_libraries|postInitApplicationSQL"`
  - `CNPG_POD=$(kubectl get pod -n pulse-platform -l cnpg.io/cluster=pulse-platform-core -o jsonpath='{.items[0].metadata.name}')`
  - `kubectl exec -n pulse-platform "${CNPG_POD}" -- psql -U postgres -d pulse -tAc "SHOW shared_preload_libraries;"`
  - `kubectl exec -n pulse-platform "${CNPG_POD}" -- psql -U postgres -d pulse -tAc "SELECT extname FROM pg_extension WHERE extname='timescaledb';"`
  - `kubectl get pod pulse-platform-valkey-node-0 -n pulse-platform -o jsonpath='{.spec.containers[*].name}'`
  Recovery examples after Docker restart:
  - `docker restart k3d-pulse-local-agent-0`
  - `kubectl get nodes -o wide`
  - `kubectl get pods -n pulse-platform`
- `make services-up` updates Helm deps and installs/upgrades `pulse-services` using `deploy/env/local/values.services.yaml`.
- `make platform-wait` blocks until critical platform dependencies are ready:
  - CNPG operator deployment,
  - CNPG cluster `pulse-platform-core` `Ready` condition,
  - `nats`, `valkey-node`, and `keycloak` statefulsets,
  - `minio` deployment,
  - optional `ingress-nginx` controller deployment,
  - optional `cert-manager` controller/webhook/cainjector deployments,
  - optional External Secrets deployments (`external-secrets`, webhook, cert-controller),
  - optional observability-lite deployments (`kube-promet-operator`, `grafana`, `opentelemetry-collector`).
- `make services-wait` blocks until `pulse-services` pods are `Ready` (if services workloads exist).
- `make dev-up` runs `k3d-up`, `platform-up`, `platform-wait`, `services-up`, then `services-wait`.
  This enforces startup order and returns only when dependencies are actually ready.
- `make dev-down` uninstalls `pulse-services` and `pulse-platform`; preserves cluster by default.
  Set `DELETE_CLUSTER=1` to also delete the local k3d cluster.
- `make db-migrate-up-local` applies all SQL up migrations from `deploy/db/migrations` to the local CNPG primary (`k3d-pulse-local`, namespace `pulse-platform`, service `pulse-platform-core-rw`).
- `make db-migrate-down-local` applies all SQL down migrations in reverse order from `deploy/db/migrations` to the same local CNPG primary.
- `make db-migrate-verify-local` runs schema verification checks for M1 control-plane tables and constraints:
  - `users`, `devices`, `user_devices` existence
  - `users.id` default expression (`uuidv7()`)
  - no DB default expression on `users.created_at`/`users.updated_at`
  - control-plane check constraints (`keycloak_subject`, `ecoflow_sn`, `role`)
- `make db-migrate-cycle-local` runs the full local validation cycle:
  - `up` -> `verify` -> `down` -> `up` -> `verify`
- `make db-migrate-e2e-local` runs data-path checks on top of applied schema:
  - inserts `users`/`devices`/`user_devices` linkage,
  - verifies ownership join shape (`keycloak_subject -> user_devices -> ecoflow_sn`),
  - verifies uniqueness guards for `keycloak_subject` and `ecoflow_sn`,
  - verifies role guard rejects invalid roles.
- `make auth-keycloak-verify-local` validates Keycloak realm bootstrap on local k3d:
  - authenticates with `kcadm` against running Keycloak pod,
  - verifies realm `$(KEYCLOAK_REALM_NAME)` exists (default `pulse`),
  - verifies social providers `google` and `facebook` are present in that realm.
- `make gke-context` fetches kube credentials for GKE dev.
  Required variables:
  - `GKE_PROJECT_ID` (required)
  Optional variables:
  - `GKE_CLUSTER_NAME` (default `pulse-dev`)
  - `GKE_CLUSTER_ZONE` (default `us-east1-b`)
  - `GKE_BASELINE_NODEPOOL` (default `baseline-pool`; use `default-pool` if cluster was created via raw `gcloud container clusters create`)
- `make gke-dev-guardrails` creates `pulse-dev` namespace if needed and applies:
  - `deploy/env/dev/guardrails/pulse-dev-resourcequota.yaml`
  - `deploy/env/dev/guardrails/pulse-dev-limitrange.yaml`
- `make gke-park` scales stateless app deployments down and reduces node-pool minimums for cost-min idle mode.
  Defaults:
  - `GKE_STATELESS_DEPLOYMENTS="node-bff ws-gateway query-api projection ingest"`
  - baseline pool min/max: `1/4`
  - spot pool min/max: `0/4`
- `make gke-wake` restores baseline autoscaling settings, reapplies guardrails, and scales stateless app deployments up.
  Defaults:
  - stateless replicas: `2`
  - baseline pool min/max: `2/4`
  - spot pool min/max: `0/4`
- `make scale-down` is a convenience alias for `make gke-park` (same required variables and behavior).
- `make scale-up` is a convenience alias for `make gke-wake` (same required variables and behavior).
- `make argocd-bootstrap-dev` installs/upgrades Argo CD in `argocd` namespace on GKE dev and waits for:
  - `crd/applications.argoproj.io` Established
  - `deploy/argocd-server` Ready
  - `deploy/argocd-repo-server` Ready
  - `sts/argocd-application-controller` Ready
  Defaults:
  - chart: `argo/argo-cd`
  - version: `9.4.3`
  - values: `deploy/env/dev/values.argocd.yaml`
- `make argocd-apps-dev` applies direct app manifests in `deploy/argocd/apps/` (`pulse-platform`, `pulse-services`).
- `make argocd-wait-apps` waits until each Argo Application reaches:
  - `status.sync.status=Synced`
  - `status.health.status=Healthy`
- `make argocd-dev-up` is the full bootstrap sequence:
  - Argo CD install/upgrade
  - direct app apply
  - app sync/health wait loop
- pre-merge GKE validation pattern (when Argo apps track `main` but you need to validate branch changes):
  - patch app `spec.source.targetRevision=<branch>` with `argocd.argoproj.io/refresh=hard`,
  - run `make argocd-wait-apps`,
  - verify platform deployments/CRDs,
  - restore `spec.source.targetRevision=main` and wait again.

For complete fresh-project bootstrap (project creation, billing, APIs, cluster create, Argo bootstrap), see:
`docs/how-to/setup-gke-dev-project.md`.
