# Config 01 — GKE (us-east1) recommendations (low cost + small HA feel)

This config aims to keep costs low while still producing realistic failover behavior in dev.

---

## Cluster choice
- **GKE Standard, Regional**, region **us-east1**
- Release channel: **Regular**
- GitOps: **Argo CD**

---

## Dev node sizing (cheap)
Use burstable E2 shared-core where feasible.

**Dev baseline**
- Node type: `e2-medium` (small and stable enough for k8s overhead)
- Nodes: min **2**, max **6**

**Rationale**
- 2 nodes is the cheapest “HA feel” (pods reschedule, leaders move)
- Soft spreading rules will *try* to distribute replicas across nodes but won’t block scheduling.

---

## Staging / Prod
Start with the same layout but increase capacity as needed.

- Staging: min 2–3 nodes
- Prod: separate node pools recommended:
  - `app-pool` for stateless
  - `stateful-pool` for Postgres/NATS/Valkey/Keycloak DB

Hosted cloud note:

- the current shared cloud data plane on `pulse-cloud` is now running this
  split explicitly with:
  - `primary-pool`: `e2-standard-4` for public/stateless traffic,
  - `stateful-pool`: `e2-standard-4` for CNPG/NATS/Valkey in `us-east1-c`,
- the next optimization target is a finer-grained split:
  - general/public app pool on `e2-standard-2` so autoscaling can add smaller,
    cheaper increments of CPU/RAM,
  - dedicated CNPG pool on `e2-standard-4` so Postgres keeps stable failover
    and restore headroom,
  - NATS/Valkey should keep SSD-backed PVCs and only stay on larger nodes when
    real memory/latency evidence justifies it,
- the cheaper safe idle shape is `1` primary node + `1` stateful node; scale
  the primary pool back up before larger rollouts so public traffic keeps surge
  headroom,
- the live cloud storage profile now uses:
  - CNPG on `standard-rwo` (`pd-balanced`) at roughly `50Gi`
  - NATS JetStream on `standard-rwo` at roughly `20Gi`
  - Valkey on `standard-rwo` at roughly `20Gi`
- SSDs should stay focused on those stateful PVCs first; node boot disks only
  need enough room for images, logs, and kubelet overhead,
- the next storage optimization is future boot disks reduced toward `50Gi`,
  because CNPG durability now sits on the database PVC rather than on node
  boot storage.

---

## Ingress + TLS
- `ingress-nginx`
- `cert-manager` (Let’s Encrypt)

Browser delivery policy:
- prefer HTTP/2 at the TLS ingress/public edge,
- keep static asset compression and cache policy at the ingress/public edge,
- use preload / `103 Early Hints` and optional cross-origin `preconnect` where
  they materially improve first render,
- avoid HTTP/2 server push,
- enable HTTP/3 only when the chosen ingress/controller runtime exposes a
  stable QUIC/HTTP/3 path and the edge also exposes UDP 443 alongside TLS.

Keep ingress portable to EKS.

---

## Secrets
- Staging/Prod: External Secrets Operator + GCP Secret Manager
- Dev/Local: SOPS (age) or local secrets (minimal)

---

## Cost levers (optional)
- Add a Spot node pool for stateless workloads later (BFF/WS/query), keep stateful on regular nodes.
