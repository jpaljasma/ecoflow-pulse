# Architecture Decision Records (ADR)

> This folder contains accepted decisions for EcoFlow Pulse. Each ADR is immutable once accepted; new ADRs supersede prior ones when changes are made.

## Index

- [ADR-0001-architecture-universal-client-node-rest-bff-go-grpc-data-plane.md](./ADR-0001-architecture-universal-client-node-rest-bff-go-grpc-data-plane.md)
- [ADR-0002-infrastructure-gke-us-east1-first-portable-to-eks-later.md](./ADR-0002-infrastructure-gke-us-east1-first-portable-to-eks-later.md)
- [ADR-0003-messaging-nats-jetstream-for-streaming-replay.md](./ADR-0003-messaging-nats-jetstream-for-streaming-replay.md)
- [ADR-0004-cache-valkey-redis-compatible-with-replication-sentinel.md](./ADR-0004-cache-valkey-redis-compatible-with-replication-sentinel.md)
- [ADR-0005-databases-postgres-timescaledb-for-control-plane-rollups.md](./ADR-0005-databases-postgres-timescaledb-for-control-plane-rollups.md)
- [ADR-0006-replay-raw-archive-in-object-storage-protobuf-zstd-as-source-of-truth.md](./ADR-0006-replay-raw-archive-in-object-storage-protobuf-zstd-as-source-of-truth.md)
- [ADR-0007-authentication-keycloak-oidc-with-social-login-jwt-validated-at-every-boundary.md](./ADR-0007-authentication-keycloak-oidc-with-social-login-jwt-validated-at-every-boundary.md)
- [ADR-0008-realtime-delivery-websockets-gateway-with-backpressure-downsampling.md](./ADR-0008-realtime-delivery-websockets-gateway-with-backpressure-downsampling.md)
- [ADR-0009-local-development-k3d-kubernetes-with-one-command-bringup.md](./ADR-0009-local-development-k3d-kubernetes-with-one-command-bringup.md)
---

## How to add an ADR

1. Copy the template: [TEMPLATE.md](./TEMPLATE.md)
2. Pick the next number (zero-padded): `ADR-0010`, `ADR-0011`, …
3. Name the file: `ADR-<NNNN>-<kebab-case-title>.md`
4. Start as **Status: Proposed** (or **Accepted** if you’re merging the decision immediately).
5. Add the new ADR to the **Index** above (keep it sorted by number).

> Rule of thumb: if the decision affects architecture, data, security, scalability, reliability, or operations, it deserves an ADR.

---

## Status lifecycle

- **Proposed** → under discussion / pending a decision
- **Accepted** → the decision is made and should be implemented
- **Deprecated** → no longer recommended, but may still exist in the system
- **Superseded** → replaced by a newer ADR (preferred to “editing history”)

---

## Superseding an ADR (how to change a decision)

When a decision changes:
1. Create a **new** ADR describing the new decision.
2. In the new ADR, add a line under the header like:
   - **Supersedes:** ADR-000X
3. In the old ADR, update only the header:
   - **Status:** Superseded  
   - **Superseded by:** ADR-00YY
4. Update this README index if needed.

**Do not rewrite accepted ADRs** other than the minimal status/superseded pointers above.

---

## Editing rules

- Accepted ADRs are **immutable** (except: `Status` + superseded pointers).
- Keep ADRs short, concrete, and actionable:
  - what we decided
  - why
  - impact / tradeoffs
  - follow-ups

---

## Naming & numbering

- Numbers are sequential, 4 digits, zero-padded: `0001`, `0002`, …
- Use kebab-case titles in filenames (lowercase, dashes).
- The ADR ID in the filename should match the ID in the document title.

