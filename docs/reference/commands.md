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
- `make platform-up` updates Helm deps and installs/upgrades `pulse-platform` using `deploy/env/local/values.platform.yaml`.
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
  - `kubectl get cluster pulse-platform-core -n pulse-platform -o yaml | rg "imageName|shared_preload_libraries|postInitSQL"`
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
  - `minio` deployment.
- `make services-wait` blocks until `pulse-services` pods are `Ready` (if services workloads exist).
- `make dev-up` runs `k3d-up`, `platform-up`, `platform-wait`, `services-up`, then `services-wait`.
  This enforces startup order and returns only when dependencies are actually ready.
- `make dev-down` uninstalls `pulse-services` and `pulse-platform`; preserves cluster by default.
  Set `DELETE_CLUSTER=1` to also delete the local k3d cluster.
