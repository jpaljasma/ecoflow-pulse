# ADR-0011: Dev Cost Policy — k3d Daily, GKE for Integration Only

**Status:** Accepted  
**Date:** 2026-02-20  
**Owners:** Jaan

---

## Context

EcoFlow Pulse is being built by a very small team (initially a single developer/operator). Uncontrolled cloud usage can quickly dominate costs and slow iteration.

At the same time, we need **high parity** across environments and we must validate cloud-only behaviors:
- OAuth redirect flows on physical devices (real domains/TLS)
- ingress/TLS behavior, certificates, and real DNS
- workload identity / secret syncing
- object storage retention and lifecycle policies
- autoscaling and node lifecycle behavior

We also want a workflow that encourages fast iteration with minimal “it works on my machine” drift.

---

## Decision

### Prime Directive (non-negotiable)

> **Daily development happens on k3d (local).**  
> **GKE dev is for integration testing + realistic cloud behaviors.**  
> **When GKE dev is not actively used, it must be scaled down.**

### Environment policy

1) **Local (k3d) is the default**
- k3d runs the same platform dependencies in-cluster:
  - NATS JetStream
  - CloudNativePG + Postgres/Timescale
  - Valkey replication + Sentinel
  - Keycloak
  - MinIO (local raw archive)
- services run in-cluster by default for maximum parity
- one-command workflow is required (e.g., `make dev-up`, `make dev-down`)

2) **GKE dev is intentionally minimal and shared**
- run **one** GKE dev cluster and use namespaces for dev/stage early
- avoid spinning up multiple clusters unless there is a clear need

3) **GKE dev should be configured for low cost**
- cluster mode: **Standard**
- topology: **zonal** (dev)
- location: **us-east1-b**
- node pool baseline: small burstable nodes (e.g., `e2-medium`)
  - autoscaling: min **2** (for HA feel), max **4**
- optional: Spot pool for stateless services only (min 0)

4) **GKE dev usage rules**
GKE dev is used only for:
- OAuth/social login end-to-end tests (real redirect URIs)
- ingress/TLS validation (cert-manager, HTTPS)
- workload identity / external secrets behavior
- GCS retention/lifecycle and raw archive behavior
- autoscaling and node lifecycle behaviors
- Argo CD sync/upgrade and “cloud realism” integration tests

Everything else stays local.

5) **Scale-down is mandatory**
When not actively using GKE dev:
- scale stateless deployments to 0 or 1
- reduce node pool minimums (baseline min=1; Spot min=0)
- avoid leaving public ingress/LBs running unnecessarily

---

## Consequences

### Positive
- Cloud costs stay predictable and low during development
- High parity is preserved (k3d matches the Kubernetes deployment shape)
- Cloud-only features are validated without making the cloud the default dev loop
- “HA feel” can be tested locally and in cloud when needed

### Negative / Tradeoffs
- Some work requires waiting for cluster scale-up when returning to GKE dev
- Developers must follow discipline (local-first) rather than “always cloud”
- Requires a small amount of automation (Make targets and/or scripts)

---

## Implementation plan

1) Ensure local workflow exists and is simple:
- `make k3d-up`
- `make platform-up`
- `make services-up`
- `make dev-up` (composite)
- `make dev-down`

2) Add GKE dev scaling scripts/targets:
- `make gke-park` (scale stateless to 0; reduce node mins)
- `make gke-wake` (restore baseline mins; scale key services back up)

3) Enforce guardrails in `pulse-dev` namespace:
- `ResourceQuota`
- `LimitRange`

4) Document the “allowed reasons” to use GKE dev and include this ADR in `docs/architecture/adr`.

---

## Acceptance criteria
- New features can be developed end-to-end on k3d with one command
- GKE dev can be brought up for integration testing and parked afterwards
- Monthly dev cloud spend remains low and predictable
- Team can reproduce “failover feel” locally (CNPG/Valkey/NATS behavior)

---

## Follow-ups
- [x] Add `make gke-park` and `make gke-wake` targets
- [x] Add `pulse-dev` ResourceQuota + LimitRange manifests
- [x] Add a short “when to use GKE dev” section in the main architecture README
