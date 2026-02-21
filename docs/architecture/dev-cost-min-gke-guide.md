# EcoFlow Pulse — Dev Cost-Min Guide (GKE + Local k3d)

**Status:** ✅ Adopted guidance (applies immediately)

This guide is intentionally opinionated. It exists to keep **costs super low** while preserving **high environment parity** and realistic failure behavior.

---

## The Prime Directive (non-negotiable)

> **Daily development happens on k3d (local).**  
> **GKE dev is for integration testing + realistic cloud behaviors** (OAuth redirects on real devices, ingress/TLS, workload identity, autoscaling, node failures).  
> **When you’re not actively using GKE dev, scale it down.**

If a task can be done locally, it should be done locally.

---

## What “daily development” means (local-first)

### Default workflow (local)
You should be able to do 90% of the work with:

- `make dev-up` (k3d + platform + services)
- iterate (code changes, tests, UI work)
- `make dev-down` (optional)

**Local runs the same stack shape**:
- NATS JetStream
- CloudNativePG + Postgres/Timescale
- Valkey replication + Sentinel
- Keycloak
- MinIO (raw archive)

This ensures you “feel” HA behavior locally:
- CNPG failover
- Valkey primary change / replica promotion
- NATS pod restarts / consumer rebalance

---

## When GKE dev is worth using (cloud-only validation)

Use GKE dev **only** when you need one of these:

1. **OAuth + social login end-to-end** with real redirect URIs (mobile devices)
2. **Ingress/TLS** (cert-manager, real domains, HTTPS)
3. **Workload Identity + External Secrets** behavior
4. **Cloud storage** (GCS) & lifecycle retention validation
5. **Autoscaling and node lifecycle** behavior (preemptions, rescheduling)
6. **“Realistic cloud feel” integration tests** (Argo CD sync, upgrades)

Everything else stays local.

---

## GKE dev “cost-min” profile (pick-for-me)

### Cluster
- **ONE** GKE dev cluster (use namespaces for dev/stage early)
- **Standard mode**
- **Zonal** (cheapest) in **us-east1-b**
- Namespaces:
  - `pulse-dev`
  - `pulse-staging`
  - `platform-*` (ingress, observability, operators)

**Why one cluster?**  
Multiple clusters multiply fixed costs and operational overhead. One cluster + namespaces is the best ROI early.

### Node pools (small + burstable)
**Pool A — on-demand baseline (everything can run here):**
- machine: `e2-medium`
- autoscaling: **min 2, max 4**

**Pool B — optional Spot for stateless only (big savings):**
- machine: `e2-medium` (Spot)
- autoscaling: **min 0, max 4**
- taint: `spot=true:NoSchedule`
- only tolerate for:
  - Node BFF
  - WS Gateway
  - Query API
  - extra projection workers
- never schedule:
  - Postgres/CNPG
  - NATS JetStream
  - Valkey
  - Keycloak DB

> If you don’t want Spot complexity yet, skip Pool B. Pool A alone is enough for ~1k devices.

---

## Your #1 dev cost killer: LoadBalancers & log volume

### Rule: don’t create multiple external LoadBalancers
- Default: **no public ingress**
- Use `kubectl port-forward` for most dev work
- Create a public ingress only when needed (OAuth testing, demos)

### Rule: keep observability “lite” in dev
- Prometheus + Grafana: OK, short retention (2–7 days)
- Loki/Tempo: optional until you need them
- Avoid verbose per-message telemetry logs in cloud dev

---

## Scale-down policy (mandatory)

When you’re not using GKE dev:
- scale **stateless** deployments to 0 or 1
- reduce node pool minimums

### Quick scale-down checklist
1) Scale down stateless workloads (example):
```bash
kubectl -n pulse-dev scale deploy/node-bff --replicas=0
kubectl -n pulse-dev scale deploy/ws-gateway --replicas=0
kubectl -n pulse-dev scale deploy/query-api --replicas=0
kubectl -n pulse-dev scale deploy/projection --replicas=0
```

2) Keep stateful platform minimal:
- CNPG instances: 2 (or 1 if fully parked)
- NATS: 1–3 (1 to park, 3 for realism tests)
- Valkey: 1–3 (1 to park, 3 for sentinel/failover tests)
- Keycloak: 0–2 (0 if parked; 2 for auth testing)

3) Reduce node pool minimums (examples):
```bash
# Baseline pool: drop to min=1 when parked (still supports system pods)
gcloud container clusters update pulse-dev --zone us-east1-b \
  --node-pool baseline-pool \
  --enable-autoscaling --min-nodes 1 --max-nodes 4

# Spot pool: keep min at 0 always
gcloud container clusters update pulse-dev --zone us-east1-b \
  --node-pool spot-pool \
  --enable-autoscaling --min-nodes 0 --max-nodes 4
```

> In practice: keep baseline min=2 when actively testing HA; drop to min=1 when parked.

---

## Namespace guardrails (required)

To prevent “accidental bills” and resource fights, apply quotas in `pulse-dev`.

### ResourceQuota (example)
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: pulse-dev-quota
  namespace: pulse-dev
spec:
  hard:
    requests.cpu: "4"
    requests.memory: 8Gi
    limits.cpu: "8"
    limits.memory: 16Gi
    pods: "60"
    persistentvolumeclaims: "20"
```

### LimitRange (example)
```yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: pulse-dev-limits
  namespace: pulse-dev
spec:
  limits:
    - type: Container
      defaultRequest:
        cpu: 100m
        memory: 256Mi
      default:
        cpu: 500m
        memory: 512Mi
```

---

## Deployment rules (GitOps-friendly)

- **Argo CD** manages GKE dev (truth from git)
- Local k3d is Helm-driven directly (simplest), or optionally Argo for parity

**Rule:** If it’s in GKE dev, it must be in GitOps (no click-ops).

---

## “Feel like prod” without paying prod prices

When you *are* testing realism on GKE dev:
- run platform in **small HA mode**:
  - CNPG: 2
  - NATS: 3
  - Valkey: 3 (sentinel)
  - Keycloak: 2
- run services at 2 replicas (BFF/WS/query/ingest/projection)

When you’re *not* doing realism tests:
- park it:
  - keep only what you need for the next test
  - scale the rest to 0 and reduce node mins

---

## Ready-to-apply checklist (paste into requirements)

- [ ] Daily dev uses **k3d local** (not GKE)
- [ ] GKE dev used only for cloud-only validation items
- [ ] GKE dev is **one zonal Standard cluster** in **us-east1-b**
- [ ] Baseline pool uses small burstable nodes (`e2-medium`), min 2 for HA tests
- [ ] Optional Spot pool for stateless (min 0)
- [ ] Default dev: no public ingress, port-forward instead
- [ ] Observability is “lite” by default
- [ ] Quotas + LimitRanges applied to `pulse-dev`
- [ ] When idle, scale down workloads and reduce node pool mins

---

## Notes / future upgrades (when device count grows)
- >10k devices: consider regional WS gateways + CDN for content
- Timescale → ClickHouse only when you feel real pain (keep the seam clean)
