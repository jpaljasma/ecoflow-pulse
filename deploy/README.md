# Deploy Scaffold (Milestone 0)

This directory contains the Kubernetes deployment layout for EcoFlow Pulse.

Current state: **scaffold only** (Milestone 0, task #1 in progress).

## Layout

- `charts/pulse-platform/`: umbrella chart for platform dependencies
  (NATS, CloudNativePG/Postgres, Valkey, Keycloak, MinIO, ingress-nginx,
  cert-manager, External Secrets, observability-lite).
- `charts/pulse-services/`: umbrella chart for Pulse runtime services
  (Node BFF, WS gateway, Go ingest/projection/query).
- `env/local/`: local values overrides.
- `env/dev/`: dev values overrides.
- `env/dev/values.argocd.yaml`: Argo CD bootstrap values for GKE dev.
- `env/dev/guardrails/`: dev namespace guardrails (`ResourceQuota`, `LimitRange`).
- `argocd/apps/`: direct Argo CD Applications:
  - `pulse-platform`
  - `pulse-services`
- `tilt/k3d-config.yaml`: local k3d cluster config used by architecture docs.

## Namespaces

- Platform namespace: `pulse-platform`
- Services namespace: `pulse-services`

## Iteration Plan

1. Scaffold complete (this step).
2. Add platform chart dependencies progressively.
3. Add service deployments progressively.
4. Wire Make targets and one-command local bringup.

## Local Bringup (Make)

Default local workflow:

```bash
make dev-up
```

Expanded commands:

```bash
make k3d-up
make platform-up
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
- local keeps `ingress-nginx`, `cert-manager`, `external-secrets`, and
  `observability-lite` disabled by default.
- dev values enable `ingress-nginx` + `cert-manager` with cost-min settings
  (`ClusterIP`, single replicas).
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
