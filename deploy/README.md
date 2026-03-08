# Deploy Scaffold (Milestone 0)

This directory contains the Kubernetes deployment layout for EcoFlow Pulse.

Current state: **platform + telemetry worker runtime in local k3d**.

## Layout

- `charts/pulse-platform/`: umbrella chart for platform dependencies
  (NATS, CloudNativePG/Postgres, Valkey, Keycloak, MinIO, ingress-nginx,
  cert-manager, External Secrets, observability-lite).
- `charts/pulse-services/`: Pulse runtime services chart.
  - currently deploys Go telemetry workers:
    - `go-ingest`
    - `go-projection`
    - `go-archive`
  - Node BFF/WS gateway/query API remain staged for later milestones.
- `env/local/`: local values overrides.
- `env/dev/`: dev values overrides.
- `env/dev/values.argocd.yaml`: Argo CD bootstrap values for GKE dev.
- `env/dev/guardrails/`: dev namespace guardrails (`ResourceQuota`, `LimitRange`).
- `env/dev/recommended/`: recommended runtime policies not auto-applied by scaffold.
  - `pulse-services-go-ingest-hpa.recommended.yaml`: ingest worker HPA baseline.
- `argocd/apps/`: direct Argo CD Applications:
  - `pulse-platform`
  - `pulse-services`
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
make dev-up
```

Expanded commands:

```bash
make k3d-up
make platform-up
make services-image-build-local
make services-image-import-local
make services-up
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
```

Defaults:
- local values are read from `deploy/env/local/*.yaml`,
- `dev-down` keeps the k3d cluster unless `DELETE_CLUSTER=1`.
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
- current local services defaults enable containerized telemetry workers
  (`go-ingest`, `go-projection`, `go-archive`) using image
  `ecoflow-pulse/services:local`.
- local/dev MinIO credentials are intentionally pinned for deterministic
  bring-up and service compatibility:
  - platform chart uses `minio.rootUser` / `minio.rootPassword`
    (MinIO chart `5.4.0` top-level keys),
  - services runtime secret uses matching
    `ARCHIVE_OBJECT_ACCESS_KEY` / `ARCHIVE_OBJECT_SECRET_KEY`
    (`minio` / `minio123`).
- local keeps `external-secrets` and `observability-lite` disabled by default.
- dev values enable `ingress-nginx` + `cert-manager` and expose the public app
  ingress host as `pulse.dev.local` (TLS remains opt-in until a real issuer is
  configured).
- dev values also enable `external-secrets` + `observability-lite` using
  low-footprint resource limits.
- GKE Argo CD bootstrap uses `deploy/env/dev/values.argocd.yaml`.

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
