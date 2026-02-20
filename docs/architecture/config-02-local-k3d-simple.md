# Config 02 — Local dev (k3d) kept very simple

Goal: local dev feels like dev/prod, but stays one-command simple.

---

## Local cluster
Use **k3d**.

Minimal config (1 server + 2 agents), disable traefik so ingress-nginx matches GKE:

```yaml
apiVersion: k3d.io/v1alpha5
kind: Simple
metadata:
  name: pulse-local
servers: 1
agents: 2
ports:
  - port: 80:80
    nodeFilters:
      - loadbalancer
  - port: 443:443
    nodeFilters:
      - loadbalancer
options:
  k3s:
    extraArgs:
      - arg: --disable=traefik
        nodeFilters:
          - server:*
```

Create cluster:
```bash
k3d cluster create --config deploy/tilt/k3d-config.yaml
kubectl get nodes
```

---

## One-command workflow (recommended Make targets)
- `make dev-up`:
  1) create k3d cluster (if missing)
  2) `helm upgrade --install pulse-platform ... -f env/local/values.platform.yaml`
  3) `helm upgrade --install pulse-services ... -f env/local/values.services.yaml`
- `make dev-down`:
  - uninstall charts (optional)
  - optionally delete k3d cluster

**Default rule:** services run in-cluster (parity > cleverness)

Optional debugging mode:
- port-forward deps (Postgres, NATS, Valkey)
- run one service locally for step debugging

---

## Local platform deps
- NATS JetStream
- CloudNativePG + Postgres/Timescale
- Valkey replication + Sentinel
- Keycloak
- MinIO (raw archive)

Keep observability “lite” locally.
