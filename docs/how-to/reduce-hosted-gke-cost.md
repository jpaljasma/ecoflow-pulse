# Reduce Hosted GKE Cost

This runbook captures the low-cost hosted posture for `pulse-cloud` in
`ecoflow-pulse-dev-260221-01`.

## Current Cost Driver

The hosted cluster is GKE Standard, so the dominant cost follows the VM node
pools. On 2026-05-01, the live cluster had:

- `app-pool`: three `e2-standard-2` nodes
- `stateful-pool`: one `n2-highmem-4` node
- `stateful-pool-ha`: one `n2d-standard-4` node
- `stateful-pool-quorum`: one `n2d-standard-4` node

Live `kubectl top nodes` and node allocation showed the stateful nodes were
using only a small slice of their requested high-memory/general-purpose
capacity. The low-cost target is to keep the same stateful application topology
but run it on small nodes.

## Low-Cost Target

- App pool: `e2-standard-2`, autoscaling `min=1`, `max=2`
- Stateful pools: one `e2-standard-2` per stateful zone, `min=max=1`
- Platform replicas:
  - public app `1`
  - realtime gateway `1`
  - Keycloak `1`
  - ingress-nginx controller `1`
  - external-secrets controller/webhook `1/1`
- Service replicas:
  - gRPC API `1`
  - energy API `1`
  - ingest/inference/projection/rollup/archive/scheduler/solar-verification `1`
- Stateful topology:
  - CNPG remains `2` instances
  - NATS remains `3` brokers
  - Valkey/Sentinel remains `3` nodes with quorum `2`

Using the public Compute Engine SKU model returned by the Cloud Billing Catalog
API for Americas on 2026-05-01, the live node mix is roughly `$0.80/hour` before
disks, load balancers, GKE management fees, logging, or traffic. Four
`e2-standard-2` nodes are roughly `$0.27/hour` and five are roughly
`$0.34/hour`, so the low-cost target cuts VM spend by about 58-66% depending on
whether the app pool needs one or two nodes.

## Apply the Overlay

Merge and sync the low-cost values first. That reduces stateless scheduling
pressure before node-pool migration.

```bash
make cloud-context
kubectl -n argocd get applications.argoproj.io pulse-platform-cloud pulse-services-cloud
```

After the branch lands on `main`, wait for Argo:

```bash
kubectl -n argocd get applications.argoproj.io pulse-platform-cloud pulse-services-cloud -w
kubectl -n pulse-platform get deploy,sts
kubectl -n pulse-services get deploy
```

## Right-Size Node Pools

GKE node-pool machine sizing is a node-pool migration. Create replacement pools,
move workloads, then remove the oversized pools after health is green.

First reduce app-pool autoscaling:

```bash
gcloud container node-pools update app-pool \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01 \
  --enable-autoscaling \
  --min-nodes 1 \
  --max-nodes 2
```

Create one low-cost replacement stateful pool per existing stateful zone:

```bash
gcloud container node-pools create stateful-pool-e2 \
  --cluster pulse-cloud \
  --region us-east1 \
  --node-locations us-east1-c \
  --project ecoflow-pulse-dev-260221-01 \
  --machine-type e2-standard-2 \
  --disk-type pd-balanced \
  --disk-size 50 \
  --num-nodes 1 \
  --enable-autoscaling \
  --total-min-nodes 1 \
  --total-max-nodes 1 \
  --node-taints workload=stateful:NoSchedule

gcloud container node-pools create stateful-pool-ha-e2 \
  --cluster pulse-cloud \
  --region us-east1 \
  --node-locations us-east1-d \
  --project ecoflow-pulse-dev-260221-01 \
  --machine-type e2-standard-2 \
  --disk-type pd-balanced \
  --disk-size 50 \
  --num-nodes 1 \
  --enable-autoscaling \
  --total-min-nodes 1 \
  --total-max-nodes 1 \
  --node-taints workload=stateful:NoSchedule

gcloud container node-pools create stateful-pool-quorum-e2 \
  --cluster pulse-cloud \
  --region us-east1 \
  --node-locations us-east1-b \
  --project ecoflow-pulse-dev-260221-01 \
  --machine-type e2-standard-2 \
  --disk-type pd-balanced \
  --disk-size 50 \
  --num-nodes 1 \
  --enable-autoscaling \
  --total-min-nodes 1 \
  --total-max-nodes 1 \
  --node-taints workload=stateful:NoSchedule
```

Then migrate one old node at a time. Keep health green between each drain.

```bash
kubectl get nodes -L cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type,topology.kubernetes.io/zone
kubectl -n pulse-platform get pods -o wide

kubectl cordon <old-stateful-node>
kubectl drain <old-stateful-node> --ignore-daemonsets --delete-emptydir-data --timeout=20m
kubectl -n pulse-platform get pods -o wide
kubectl -n pulse-platform get cluster,pod,pvc
```

After all PVC-backed pods are healthy on the low-cost pools, delete the old
oversized pools:

```bash
gcloud container node-pools delete stateful-pool \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01

gcloud container node-pools delete stateful-pool-ha \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01

gcloud container node-pools delete stateful-pool-quorum \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01
```

## Cleanup Round

After the cluster has stabilized on the low-cost pools, do one cleanup pass so
there are no dangling edges or stragglers.

1. Confirm no workloads still reference or run on old pools:

```bash
for pool in stateful-pool stateful-pool-ha stateful-pool-quorum; do
  kubectl get nodes -l "cloud.google.com/gke-nodepool=${pool}" -o wide
done
kubectl get nodes -L cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type
```

2. Remove legacy oversized node-pool names from cloud Helm affinity:

- `stateful-pool`
- `stateful-pool-ha`
- `stateful-pool-quorum`

Keep only:

- `stateful-pool-e2`
- `stateful-pool-ha-e2`
- `stateful-pool-quorum-e2`

3. Check for orphaned storage or load-balancer resources:

```bash
gcloud compute disks list \
  --project ecoflow-pulse-dev-260221-01 \
  --filter='zone:(us-east1*)' \
  --format='table(name,location(),sizeGb,type.scope(diskTypes),status,users.basename())'

gcloud compute forwarding-rules list \
  --project ecoflow-pulse-dev-260221-01 \
  --format='table(name,region,IPAddress,target.basename(),loadBalancingScheme)'
```

4. Commit the cleanup as a small follow-up PR once live verification proves the
   old pools are gone.

## Verify

```bash
kubectl get nodes -L cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type,topology.kubernetes.io/zone
kubectl top nodes
kubectl -n pulse-platform get deploy,sts,pods
kubectl -n pulse-services get deploy,pods
kubectl -n argocd get applications.argoproj.io pulse-platform-cloud pulse-services-cloud
```

The steady state should have no `n2-highmem-4` or `n2d-standard-4` nodes unless
there is an intentional temporary migration window.
