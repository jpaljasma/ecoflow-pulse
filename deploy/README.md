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

Direct hosted cloud operations, used after the no-outage Argo cutover gates are
proven:

```bash
make cloud-health-gate
make cloud-deploy
make cloud-cost-min-deploy
make cloud-db-env
make cloud-db-forward
make cloud-db-forward-start
make cloud-realtime-forward-start
make cloud-realtime-forward-status
make dev-web-deploy-cloud-realtime
make dev-up-cloud-db
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
  - `.github/workflows/cloud-services-deploy.yml` for automatic hosted
    `pulse-services-cloud` deployment after a successful `Go Tests` workflow on
    `main`,
  - `make cloud-up` for the hosted Argo bootstrap/apply/wait path,
  - `make cloud-refresh` for re-applying hosted Argo apps after branch changes,
  - `make cloud-deploy` for the direct Helm hosted apply path once Argo cutover
    is being validated,
  - `make cloud-cost-min-deploy` for the explicit ingest/storage-only overlay
    after local cloud-Postgres app mode is proven,
  - `make cloud-db-env`, `make cloud-db-forward`, and
    `make cloud-db-forward-start` for host-side Go tools reading cloud CNPG
    through a loopback port-forward,
  - `make local-cloud-db-env`, `make services-up-cloud-db`, and
    `make dev-up-cloud-db` for local k3d API backend pods reading cloud CNPG
    through an explicitly started Docker-managed background forward while local
    side-effect workers are disabled; this cloud-data mode also points the local
    realtime gateway at cloud NATS/Valkey through Docker-managed forwards so
    live data comes from the hosted data plane,
  - `make cloud-realtime-forward-start`, `make cloud-realtime-forward-status`,
    `make cloud-realtime-forward-stop`, and
    `make dev-web-deploy-cloud-realtime` for refreshing only the local public
    app/realtime gateway while keeping websocket telemetry on the cloud
    NATS/Valkey path; these cloud-data local workflows build the web bundle
    with `EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud` so Settings labels the active
    source as Cloud even though the browser edge remains local, and the local
    build scrubs hosted `EXPO_PUBLIC_CLOUD_*` values so auth/HTTP stay local,
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
  - cost-reduction and node-pool right-sizing steps live in
    [`docs/how-to/reduce-hosted-gke-cost.md`](../docs/how-to/reduce-hosted-gke-cost.md).

## Cloud Profile

The hosted cloud profile is intentionally separate from the cost-min `dev`
overlay. Use it for the real shared data plane in project
`ecoflow-pulse-dev-260221-01`.

Cloud defaults in this branch:

- platform + services keep the same namespaces (`pulse-platform`,
  `pulse-services`) and topology as local,
- the checked-in cloud overlay now favors a low-cost hosted profile:
  public app `1`, realtime gateway `1`, gRPC API `1`, energy API `1`,
  ingest/inference/projection/archive `1`, rollup `1`, scheduler `1`,
  solar verification `1`,
- Argo CD cloud bootstrap keeps the application controller, repo server, and
  ApplicationSet controller single-replica until the direct Helm cutover in
  [ADR-0026](../docs/architecture/adr/ADR-0026-hosted-cloud-cost-min-direct-helm-operations.md)
  is proven,
- the low-cost node target keeps app workloads on `e2-standard-2` and runs
  the stateful quorum pools on shared-core `e2-medium` nodes:
  - keep `app-pool` on `e2-standard-2`,
  - keep `stateful-pool-medium`, `stateful-pool-ha-medium`, and
    `stateful-pool-quorum-medium`, each in its matching zone with `min=max=1`,
- the live cluster that prompted this profile had three app-pool
  `e2-standard-2` nodes plus one `n2-highmem-4` and two `n2d-standard-4`
  stateful nodes; live pod requests and observed memory did not justify the
  high-memory/general-purpose stateful shape,
- the expected steady low-cost shape is one or two `e2-standard-2` app nodes
  plus one `e2-medium` shared-core stateful node in each stateful zone,
- do not delete the final `app-pool` node in the current cost-min shape: it is
  the only untainted placement path for GKE-managed system pods and the
  required Pulse data-plane workers,
- the profile keeps the stateful application topology intact while right-sizing
  the nodes:
  - CNPG remains `2` instances with zone-aware anti-affinity,
  - NATS JetStream remains clustered `3`,
  - Valkey/Sentinel remains `3` nodes total with `sentinel.quorum=2`,
- CNPG custom resources are rendered directly in the chart without Helm
  `lookup` gating so Argo CD can create the database cluster during cloud
  bootstrap syncs,
- stateful persistence now uses CSI-backed `standard-rwo` (`pd-balanced`) PVCs
  sized `50Gi` for CNPG and `20Gi` each for JetStream and Valkey,
- SSD-backed PVCs are the intended place to spend disk performance budget; do
  not treat node boot disks as the durability path for CNPG/NATS/Valkey,
- hosted node boot disks are now intended to stay at `50Gi` `pd-balanced`,
  because CNPG/NATS/Valkey durability lives on their PVCs rather than on the
  node boot disk,
- this branch adds a GKE `standard-rwo-regional` storage class for future
  regional `pd-balanced` use, but does not force an in-place PVC migration for
  existing stateful claims,
- all live CNPG/NATS/Valkey PVCs are now converged on the intended
  `standard-rwo` footprint (`50Gi` for CNPG, `20Gi` for NATS/Valkey), and the
  superseded unattached disks from earlier migrations have been removed,
- for the current E2/N2 cloud mix, use that regional class selectively when a
  database-oriented recovery-time objective justifies the extra cost; do not
  default NATS/Valkey to regional disks before their application-level replica
  topology is in place,
- ingress-nginx stays enabled as a single controller behind the existing cloud
  load balancer, and external-secrets runs controller/webhook replicas `1/1`,
  while observability-lite remains disabled in the hosted profile,
- archive storage switches to provider-aware `ARCHIVE_OBJECT_PROVIDER=gcs`,
  bucket/prefix `pulse-telemetry-raw` / `raw`,
- services use a dedicated Kubernetes service account annotated for GKE
  Workload Identity and read runtime secrets from Secret Manager-backed
  `ExternalSecret`,
- ingest now runs as `2` cloud replicas; archive and rollup remain singleton
  durable-consumer workers,
- ingest/archive/rollup PDBs use `minAvailable: 1` in the cloud profile so
  voluntary disruption cannot evict the only available durable worker,
- Keycloak stays the auth system, with redirect/CORS allowances for both the
  cloud domain and localhost Expo web-dev origins; Google social login and the
  Keycloak config-cli import are temporarily held out of the bootstrap path
  until the base platform is healthy,
- Argo-managed cloud installs disable the vendored Helm admission/startup hook
  jobs for `ingress-nginx`, `cert-manager`, `kube-prometheus-stack`, and
  Keycloak so umbrella-chart syncs do not stall waiting on subchart hooks,
- cloud Argo applications avoid `Replace=true` so immutable Job resources do
  not deadlock bootstrap retries.

The explicit cost-min overlays in `deploy/env/cloud/*.cost-min.yaml` are the
hosted ingest/storage-only cloud shape after the no-outage cutover. Keep the
cloud Argo Applications and direct Helm target on these overlays until Argo is
removed; use `make cloud-cost-min-deploy` for manual direct applies. See
[`docs/how-to/no-planned-outage-cloud-cost-rollout.md`](../docs/how-to/no-planned-outage-cloud-cost-rollout.md).

Hosted rollout sequence for the multi-zone stateful HA target:

1. Sync the low-cost Helm overlay first so stateless replicas collapse before
   the cluster scales node pools.
2. Create replacement `e2-medium` stateful pools in the same zones as the
   existing PVC-backed pools; during a future migration, old and new pool names
   can be temporarily allowed in node affinity until the pods have moved.
3. Move one stateful ordinal or CNPG instance at a time, keeping database,
   JetStream, and Sentinel health green between moves.
4. Only after the replacement pods are healthy, delete the superseded stateful
   pools from the previous machine class.
5. Treat the `standard-rwo-regional` storage class as an opt-in recovery-time
   migration target, not as part of the cost-cutting path.

Do not treat the app pool as part of the stateful-pool cleanup. A 2026-05-14
live check found the three `e2-medium` stateful nodes already CPU-request tight
(`88%`, `88%`, and `67%` allocated), while the remaining
`go-ingest`/`go-archive`/`go-rollup` workers request about `600m` CPU and
GKE-managed system pods still run on the untainted app node. Server dry-run
drain also showed singleton `go-archive` and `go-rollup` PDBs block eviction.
Keep `app-pool` at `e2-standard-2`, autoscaling `min=1`, `max=2` until a
future design proves enough stateful-node headroom, safe system-pod placement,
and non-blocking worker disruption budgets.

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
- revisit multi-replica public/auth/realtime replicas only when the budget
  allows the larger HA posture again,
- decide later whether CNPG should migrate from zonal `standard-rwo` to the
  prepared `standard-rwo-regional` class; do not treat that as part of the
  first replica rollout,
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
