# ADR-0009: Local Development — k3d Kubernetes with One-Command Bringup

**Status:** Accepted  
**Date:** 2026-02-20

## Context
Local development must be simple and match the GKE deployment shape:
- Kubernetes locally
- same charts/manifests across environments
- minimal “it works on my machine” drift

## Decision
Use **k3d** for local Kubernetes and keep local dev workflow simple:
- platform dependencies run in-cluster (NATS, CNPG/Postgres/Timescale, Valkey, Keycloak, MinIO)
- services run in-cluster by default
- provide Make targets like `dev-up` / `dev-down` for one-command workflows
- optional: run one service locally with port-forwards for debugging (not required)

## Consequences
### Positive
- High parity with dev/prod environments
- Simple for a single developer
- Easy to validate HA behavior locally

### Tradeoffs
- Slightly heavier than pure local processes
- Requires some attention to resource requests to avoid thrash

### Follow-ups
- Add k3d config, Helm umbrella charts, and Make targets
- Decide whether Tilt is needed after M0
