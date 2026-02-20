# Deploy Scaffold (Milestone 0)

This directory contains the Kubernetes deployment layout for EcoFlow Pulse.

Current state: **scaffold only** (Milestone 0, task #1 in progress).

## Layout

- `charts/pulse-platform/`: umbrella chart for platform dependencies
  (NATS, CloudNativePG/Postgres, Valkey, Keycloak, MinIO, observability).
- `charts/pulse-services/`: umbrella chart for Pulse runtime services
  (Node BFF, WS gateway, Go ingest/projection/query).
- `env/local/`: local values overrides.
- `env/dev/`: dev values overrides.
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
