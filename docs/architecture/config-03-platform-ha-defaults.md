# Config 03 — Platform HA defaults (small, realistic, cheap)

These settings are the “small HA feel” defaults for local + dev.

---

## Stateful platform (replicas)
- **Postgres/Timescale (CloudNativePG):** 2 instances (primary + replica)
- **NATS JetStream:** 3 replicas (with persistence)
- **Valkey:** replication + Sentinel
  - 1 primary + 2 replicas (3 pods total)
- **Keycloak:** 2 replicas (backed by Postgres)

---

## Pulse services (replicas)
- Node REST BFF: 2
- WS Gateway: 2
- Go Ingest: 2
- Go Projection: 2
- Go Query API (gRPC): 2

---

## Scheduling rules (don’t force extra nodes)
Use soft spreading:
- `topologySpreadConstraints` with `whenUnsatisfiable: ScheduleAnyway`
- preferred pod anti-affinity

This encourages HA behavior without requiring 3+ nodes everywhere.

---

## Resource requests (starter)
Tune later; keep stable early.

### Valkey (per pod)
- requests: 100m CPU / 256Mi RAM
- limits: 500m CPU / 512Mi RAM

### NATS (per pod)
- requests: 100m CPU / 256Mi RAM
- limits: 1 CPU / 1GiB RAM

### Postgres (per instance)
- requests: 250m CPU / 1GiB RAM
- limits: 2 CPU / 4GiB RAM

### Keycloak (per pod)
- requests: 200m CPU / 512Mi RAM
- limits: 1 CPU / 2GiB RAM

---

## Observability (“lite”)
Dev/local:
- Prometheus + Grafana: ON (short retention)
- Loki: optional / low retention
- Tempo: optional until you need traces

---

## Valkey (recommended Helm knobs)
Using a Valkey chart that supports Sentinel:
- `architecture: replication`
- `replica.replicaCount: 2`
- `sentinel.enabled: true`
