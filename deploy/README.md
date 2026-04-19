# Deploy Scaffold (Milestone 0)

This directory contains the Kubernetes deployment layout for EcoFlow Pulse.

Current state: **platform + telemetry worker runtime in local k3d**.

## Layout

- `charts/pulse-platform/`: umbrella chart for platform dependencies
  (NATS, CloudNativePG/Postgres, Valkey, Keycloak, MinIO, ingress-nginx,
  cert-manager, External Secrets, observability-lite).
- `charts/pulse-services/`: Pulse runtime services chart.
  - currently deploys Go telemetry workers:
    - `db-migrate` hook job (disabled by default; forward-only rollout path)
    - `go-ingest`
    - `go-inference`
    - `go-projection`
    - `go-archive`
  - Node BFF/WS gateway/query API remain staged for later milestones.
- `env/local/`: local values overrides.
- `env/dev/`: dev values overrides.
- `env/cloud/`: hosted GKE cloud values overrides (`pulse-cloud`, `us-east1`).
- `env/dev/values.argocd.yaml`: Argo CD bootstrap values for GKE dev.
- `env/cloud/values.argocd.yaml`: Argo CD bootstrap values for the hosted cloud cluster.
- `env/dev/guardrails/`: dev namespace guardrails (`ResourceQuota`, `LimitRange`).
- `env/dev/recommended/`: recommended runtime policies not auto-applied by scaffold.
  - `pulse-services-go-ingest-hpa.recommended.yaml`: ingest worker HPA baseline.
  - `pulse-services-go-ingest-keda.recommended.yaml`: KEDA `ScaledObject` baseline using Prometheus-backed ingest autoscaling metrics.
  - `pulse-services-db-migrations.recommended.yaml`: dev rollout migration hook values.
- `argocd/apps/`: direct Argo CD Applications:
  - `pulse-platform`
  - `pulse-services`
  - `pulse-platform-cloud`
  - `pulse-services-cloud`
- `tilt/k3d-config.yaml`: local k3d cluster config used by architecture docs.

## Namespaces

- Platform namespace: `pulse-platform`
- Services namespace: `pulse-services`

## Iteration Plan

1. Platform chart dependencies complete for local/dev baseline.
2. Containerized telemetry worker Deployments active in local.
3. Add query/API/BFF services in later milestones.
4. Expand autoscaling and production hardening policies.

## Local Bringup (Make)

Default local workflow:

```bash
make local-up
```

Hosted cloud workflow:

```bash
make cloud-up
make cloud-status
```

Expanded commands:

```bash
make k3d-up
make platform-up
make services-image-build-local
make services-image-import-local
make services-image-build-cloud SERVICES_CLOUD_IMAGE_TAG=<tag>
make services-image-push-cloud SERVICES_CLOUD_IMAGE_TAG=<tag>
make services-up
make local-up
make local-deploy
make local-status
make dev-down
make dev-down DELETE_CLUSTER=1
make gke-context GKE_PROJECT_ID=<project>
make gke-dev-guardrails GKE_PROJECT_ID=<project>
make gke-park GKE_PROJECT_ID=<project>
make gke-wake GKE_PROJECT_ID=<project>
make argocd-bootstrap-dev GKE_PROJECT_ID=<project>
make argocd-apps-dev GKE_PROJECT_ID=<project>
make argocd-wait-apps GKE_PROJECT_ID=<project>
make argocd-dev-up GKE_PROJECT_ID=<project>
make gke-cloud-context
make cloud-context
make argocd-bootstrap-cloud
make argocd-apps-cloud
make argocd-wait-apps-cloud
make argocd-cloud-up
make cloud-up
make cloud-refresh
make cloud-status
```

Defaults:
- local values are read from `deploy/env/local/*.yaml`,
- `dev-down` keeps the k3d cluster unless `DELETE_CLUSTER=1`.
- recommended operator-facing shortcuts are now:
  - `make local-up` for full local bring-up,
  - `make local-deploy` for incremental local rollouts,
  - `make services-image-push-cloud SERVICES_CLOUD_IMAGE_TAG=<tag>` for a
    repeatable `linux/amd64` cloud services image build + push with Artifact
    Registry auth refreshed from the current `gcloud` session,
  - `make cloud-up` for the hosted Argo bootstrap/apply/wait path,
  - `make cloud-refresh` for re-applying hosted Argo apps after branch changes,
  - `make cloud-status` for a quick hosted health snapshot.
- current local platform defaults enable core dependencies (`nats`,
  `cloudnativepg` + `timescaledb`, `valkey`, `keycloak`, `minio`).
- local edge defaults now also enable:
  - `ingress-nginx`,
  - `cert-manager`,
  - `https://localhost` ingress to `pulse-platform-public-app`,
  - a cert-manager-generated localhost TLS secret signed by a local in-cluster
    CA issuer.
  - on macOS, `make platform-up` auto-trusts the current localhost certificate
    authority in the login keychain by default (`LOCAL_PLATFORM_AUTO_TRUST_TLS=1`).
  - optional HTTP/3/QUIC at the edge via:
    - UDP 443 service `pulse-platform-public-edge-http3`,
    - ingress `server-snippet` QUIC listeners,
    - `Alt-Svc` response advertising `h3`.
- current local public app defaults keep the user-facing stack multi-replica for
  round-robin validation:
  - `pulse-platform-public-app`: `2` replicas
  - `pulse-platform-realtime-gateway`: `2` replicas
  - `pulse-services-go-grpc-api`: `3` replicas
  - `pulse-services-go-energy-api`: `3` replicas
- current local services defaults enable containerized telemetry workers
  (`go-ingest`, `go-inference`, `go-projection`, `go-rollup`, `go-archive`,
  `go-grpc-api`, `go-energy-api`, `go-solar-verification`) using image
  `ecoflow-pulse/services:local`; the request-serving gRPC/API workloads stay
  at `3` replicas each while `go-solar-verification` runs with `2` local
  replicas that cooperatively claim pending verification work from Postgres and
  are required to run one-per-host across the non-control-plane agent nodes,
  with one-at-a-time rollouts so the background verifier load scales out
  without competing with request serving or collapsing back onto a single hot
  node during updates. Local verifier defaults also use a shorter `1m` loop and
  a `3072` row batch limit so large local backlogs drain fast enough to profile.
  Worker Deployments also use metrics-backed drain hooks: Kubernetes calls
  `/drainz` on the metrics port during `preStop`, which flips readiness to
  draining before `SIGTERM` so rolling updates hand work off cleanly.
- local/dev MinIO credentials are intentionally pinned for deterministic
  bring-up and service compatibility:
  - platform chart uses `minio.rootUser` / `minio.rootPassword`
    (MinIO chart `5.4.0` top-level keys),
  - services runtime secret uses matching
    `ARCHIVE_OBJECT_ACCESS_KEY` / `ARCHIVE_OBJECT_SECRET_KEY`
    (`minio` / `minio123`).
- local MinIO now serves as the authoritative raw replay archive for k3d, so
  local values must keep `minio.persistence.enabled=true`; ephemeral MinIO is
  not acceptable when replay/rebuild trust matters.
- local values also pre-create the `pulse-telemetry-raw` bucket so archive
  workers and rebuild tooling do not depend on manual bucket bootstrap after a
  fresh PVC comes online.
- local keeps `external-secrets` disabled by default, but now enables
  `observability-lite` by default so Prometheus, Grafana, and the
  OpenTelemetry collector are available in the standard k3d stack.
- local observability access examples:
  - `kubectl -n pulse-platform port-forward svc/pulse-platform-kube-promet-prometheus 9090:9090`
  - `kubectl -n pulse-platform port-forward svc/pulse-platform-grafana 3000:80`
  - local Grafana dashboards now include:
    - `Pulse Pipeline Overview`
    - `Pulse Ingest Health`
    - `Pulse Storage & History Pipeline`
    - `Pulse gRPC SLOs`
    - `Pulse Platform Infra`
    - `Pulse Auth & Profile`
    - `Pulse Client REST SLOs`
    - `Pulse Client WebSocket SLOs`
  - public-edge auth/profile observability now rides the same stack:
    - `pulse-platform-public-app` exposes `/metrics` on the existing HTTP service and is scrapeable through a `ServiceMonitor` when `runtime.publicApp.metrics.serviceMonitor.enabled=true`
    - `pulse-platform-realtime-gateway` exposes `/metrics` on the existing HTTP service and is scrapeable through a `ServiceMonitor` when `runtime.realtimeGateway.metrics.serviceMonitor.enabled=true`
    - the auth/profile dashboard covers public auth/profile request rates and latency, `401` vs `403` denials, current-user gRPC outcomes, browser session recovery outcomes, and realtime websocket auth/session behavior
- dev values enable `ingress-nginx` + `cert-manager` and expose the public app
  ingress host as `pulse.dev.local` (TLS remains opt-in until a real issuer is
  configured).
- dev/GKE keeps HTTP/3 opt-in by default (`runtime.publicApp.ingress.http3.enabled=false`);
  only enable QUIC + UDP `443` there when the environment is intentionally
  exercising a real browser-facing TLS edge.
- dev values also enable `external-secrets` + `observability-lite` using
  low-footprint resource limits.
- GKE Argo CD bootstrap uses `deploy/env/dev/values.argocd.yaml`.
- cloud values target the hosted regional cluster path:
  - separate values overlays under `deploy/env/cloud`,
  - Argo app manifests `pulse-platform-cloud` + `pulse-services-cloud`,
  - regional GKE defaults `pulse-cloud` in `us-east1`,
  - native GCS archive access through Workload Identity instead of MinIO/HMAC,
  - Secret Manager-backed runtime secrets via `ExternalSecret`,
  - local/cloud browser clients reach the hosted stack through the public HTTPS
    BFF + `/ws`, while `go-grpc-api` remains internal-only.

## Cloud Profile

The hosted cloud profile is intentionally separate from the cost-min `dev`
overlay. Use it for the real shared data plane in project
`ecoflow-pulse-dev-260221-01`.

Cloud defaults in this branch:

- platform + services keep the same namespaces (`pulse-platform`,
  `pulse-services`) and topology as local,
- the checked-in cloud overlay is a conservative hosted profile that currently
  converges to:
  public app `2`, realtime gateway `2`, gRPC API `2`, energy API `2`,
  ingest/inference/projection/archive `1`, rollup `1`,
  solar verification `1`,
- the current hosted cluster layout uses two GKE Standard node pools:
  - `app-pool` in `us-east1-d` on `e2-standard-2` for public/stateless
    workloads,
  - `stateful-pool` in `us-east1-c` on `n2-highmem-4` for CNPG, NATS
    JetStream, and Valkey,
- the stateful anchor is intentionally larger than the public pool after the
  `2026-04-19` recovery, because `us-east1-c` could not place the desired
  `e2-standard-4` shape during the outage and the fastest safe recovery was a
  same-zone x86 replacement that could reattach the existing zonal PVCs,
- the live cloud cluster now idles safely at `1` `app-pool` node plus `1`
  `stateful-pool` node; before larger rollouts or noisy background catch-up,
  scale `app-pool` back to `2` nodes so public/stateless workloads regain
  deployment headroom,
- CNPG is currently single-instance (`1`) with Timescale enabled,
- CNPG pod disruption budgets are disabled in the cloud bootstrap profile so
  GKE maintenance is not blocked by a single primary-only PDB during the
  dev-stage hosted rollout,
- CNPG custom resources are rendered directly in the chart without Helm
  `lookup` gating so Argo CD can create the database cluster during cloud
  bootstrap syncs,
- stateful persistence now uses CSI-backed `standard-rwo` (`pd-balanced`) PVCs
  sized `50Gi` for CNPG and `20Gi` each for JetStream and Valkey,
- SSD-backed PVCs are the intended place to spend disk performance budget; do
  not treat node boot disks as the durability path for CNPG/NATS/Valkey,
- hosted node boot disks are now `50Gi` `pd-balanced`; CNPG/NATS/Valkey
  durability lives on their PVCs rather than on the node boot disk,
- existing hosted stateful PVCs remain zonal `us-east1-c` claims, so a
  different zone is not a simple reschedule path for the current data; future
  resiliency work needs multi-zone replicas and, where justified, regional
  disk-backed claims,
- ingress-nginx stays enabled at `2` replicas with a matching PDB, and
  external-secrets runs controller/webhook replicas `2/2` with matching PDBs,
  while observability-lite remains temporarily disabled during initial
  convergence,
- archive storage switches to provider-aware `ARCHIVE_OBJECT_PROVIDER=gcs`,
  bucket/prefix `pulse-telemetry-raw` / `raw`,
- services use a dedicated Kubernetes service account annotated for GKE
  Workload Identity and read runtime secrets from Secret Manager-backed
  `ExternalSecret`,
- single-replica worker deployments render `maxUnavailable: 1` PDBs in the
  cloud profile so their selectors are covered without producing
  zero-eviction maintenance warnings,
- Keycloak stays the auth system, with redirect/CORS allowances for both the
  cloud domain and localhost Expo web-dev origins; Google social login and the
  Keycloak config-cli import are temporarily held out of the bootstrap path
  until the base platform is healthy,
- Argo-managed cloud installs disable the vendored Helm admission/startup hook
  jobs for `ingress-nginx`, `cert-manager`, `kube-prometheus-stack`, and
  Keycloak so umbrella-chart syncs do not stall waiting on subchart hooks,
- cloud Argo applications avoid `Replace=true` so immutable Job resources do
  not deadlock bootstrap retries.

Expected follow-up after the current hosted cutover:

- replace placeholder cloud domain values (`pulse.example.com`),
- confirm the Artifact Registry image repositories/tags used in
  `deploy/env/cloud/*.yaml`,
- populate the referenced Secret Manager secret names,
- provision the matching GCP IAM service accounts and Workload Identity
  bindings,
- keep CNPG as the largest stateful PVC; database history retention and
  replay metadata belong on the CNPG PVC, while JetStream and Valkey can stay
  materially smaller,
- use future HA work to add multi-zone stateful replicas before spending more
  SSD quota on larger boot disks; the current `50Gi` node roots are already
  right-sized unless node-local image/log pressure proves otherwise,
- follow the migration runbook in
  `docs/how-to/migrate-local-data-plane-to-gke-cloud.md`.

## Helm Dependency Bootstrap

Dependencies are defined in `charts/pulse-platform/Chart.yaml` and are disabled
by default via `components.*.enabled: false`.

When you need to render/install enabled dependencies locally:

```bash
helm dependency update deploy/charts/pulse-platform
```

Repository policy:
- commit `Chart.lock` for reproducible resolution,
- do not commit vendored `charts/*.tgz` artifacts.

## Worker Image Build/Load (local)

Worker binaries are packaged into one local image via:

```bash
make services-image-build-local
make services-image-import-local
```

`make services-up` runs these automatically when
`SERVICES_AUTO_BUILD_IMAGE=1` (default).

On Apple silicon, local image builds now target the native `linux/arm64`
platform by default instead of forcing x86/Rosetta. The local image import +
Helm rollout path is PVC-safe: it replaces pods with rolling updates but does
not recreate the k3d cluster or delete Postgres/MinIO/NATS/Valkey volumes.
