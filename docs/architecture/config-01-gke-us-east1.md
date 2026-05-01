# Config 01 — GKE (us-east1) recommendations (low cost + small HA feel)

This config aims to keep costs low while still producing realistic failover behavior in dev.

---

## Cluster choice
- **GKE Standard, Regional**, region **us-east1**
- Release channel: **Regular**
- GitOps: **Argo CD**

---

## Networking dataplane
- New GKE clusters should explicitly enable GKE Dataplane V2 with VPC-native
  alias IPs (`--enable-dataplane-v2`, `--enable-ip-alias`).
- Existing clusters keep their current dataplane; DPv2 is selected only when a
  cluster is created.
- Do not enable or disable legacy Kubernetes network policy flags on DPv2
  clusters. DPv2 includes Kubernetes NetworkPolicy enforcement, and GKE rejects
  separate network-policy toggles for that dataplane.
- Prefer testing Pulse on DPv2 rather than preserving DPv1 by default. Document
  any temporary DPv1 opt-out with the workload, reproduced failure, and planned
  removal condition.

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

- the recovered live cluster is currently:
  - `app-pool`: `e2-standard-2`, `50Gi` `pd-balanced`, public/stateless
    traffic, currently serving from `us-east1-d`
  - `stateful-pool`: `n2-highmem-4`, `50Gi` `pd-balanced`, current stateful
    anchor, currently serving from `us-east1-c`
- the next HA target from this branch is:
  - keep `app-pool` small and cheap for public/stateless capacity,
  - keep `stateful-pool` as the current `us-east1-c` anchor,
  - add `stateful-pool-ha` in `us-east1-d` so CNPG, NATS, and Valkey can place
    replicas outside the current anchor zone,
  - keep `stateful-pool-quorum` as the third stateful slot when 3-member
    consensus systems need rollout-safe placement instead of packing two
    members onto one zone,
- the cheaper safe idle shape for that target is `2` app nodes plus `1`
  stateful node in each active stateful zone; the recommended rollout-safe HA
  target is three total stateful slots when NATS/Valkey are both running with
  3 members,
- the live cloud storage profile now uses:
  - CNPG on `standard-rwo` (`pd-balanced`) at roughly `50Gi`
  - NATS JetStream on `standard-rwo` (`pd-balanced`) at roughly `20Gi`
  - Valkey on `standard-rwo` (`pd-balanced`) at roughly `20Gi`
- SSDs should stay focused on those stateful PVCs first; node boot disks only
  need enough room for images, logs, and kubelet overhead,
- this branch also defines `standard-rwo-regional` as an optional regional
  `pd-balanced` storage class for future DB-oriented migrations,
- for the current E2/N2 cloud mix, prefer regional persistent disk over
  Hyperdisk Balanced HA when disk-level zonal resilience is worth the extra
  cost; GKE currently recommends Hyperdisk Balanced HA for 3rd-generation
  machine series and regional PD for 2nd-generation series or older,
- do not confuse “replicas in two zones” with “perfect arbitrary zone-loss
  quorum” for every 3-member consensus system:
  - CNPG gets a meaningful HA win with two instances across two zones,
  - NATS JetStream `3` and Valkey/Sentinel `3` improve rollout safety and
    partial zonal resilience in two zones,
  - a third stateful slot/zone is the next step when we want the 3-member
    consensus systems to stay comfortably quorum-safe during rollouts and
    arbitrary single-zone loss.
- live runtime note from this branch:
  - CNPG is healthy at `2`,
  - NATS is live at `3`,
  - Valkey/Sentinel is live at `3`,
  - the one-time NATS/Valkey StatefulSet recreate is complete, so every live
    broker/replica PVC is now on the newer `standard-rwo` / `20Gi` target,
  - CNPG is now genuinely zonal with one instance in `us-east1-c` and one in
    `us-east1-d`,
  - the public/auth path is now genuinely zonal as well:
    public app, realtime, ingress, and Keycloak are split across
    `us-east1-d` and `us-east1-b`,
  - `app-pool` remains on efficient `e2-standard-2` nodes; because a dedicated
    `e2-standard-2` app pool in `us-east1-b` stocked out during the live move,
    the second public zone currently reuses spare `stateful-pool-quorum`
    capacity instead of adding more permanent app-node cost.

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
