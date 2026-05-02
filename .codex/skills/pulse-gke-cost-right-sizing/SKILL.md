---
name: pulse-gke-cost-right-sizing
description: Use when reducing or validating Pulse hosted GKE costs, changing cloud node pools or machine types, migrating stateful GKE workloads, checking Kubernetes CPU usage, estimating monthly savings, cleaning dangling cloud resources, or diagnosing Argo/CNPG rollout health in this repository.
---

# Pulse GKE Cost Right-Sizing

## Overview

Use this project skill for hosted `pulse-cloud` cost work in
`ecoflow-pulse-dev-260221-01`. Treat the work as a live infrastructure
migration: make GitOps accept the target shape first, move stateful workloads
deliberately, then prune old pools and verify cost, CPU, and stragglers.

## Guardrails

- Start from clean, up-to-date `main`, create a focused `codex/<topic>` branch,
  and follow the root plus `deploy/AGENTS.md` workflow.
- Do not drain or delete a node pool until Argo has synced the chart values that
  allow scheduling on the replacement pool.
- Keep app and stateful sizing decisions separate. App nodes are constrained by
  aggregate requests; stateful nodes are constrained by PVC placement, quorum,
  memory, and CNPG health.
- Never present pricing as all-in cloud spend. Label VM compute separately from
  disks, load balancers, GKE management fees, logging, and traffic.
- Before final handoff, close or finish all long-running CLI sessions.

## Baseline

Collect live state before changing anything:

```bash
make cloud-context
gcloud container node-pools list \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01 \
  --format='table(name,config.machineType,locations,status,autoscaling.enabled,autoscaling.totalMinNodeCount,autoscaling.totalMaxNodeCount)'
kubectl get nodes -L cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type,topology.kubernetes.io/zone -o wide
kubectl top nodes
kubectl -n pulse-platform get pods -o wide
kubectl -n pulse-services get pods -o wide
kubectl -n argocd get applications pulse-platform-cloud pulse-services-cloud
```

Also inspect requested CPU/memory per node before choosing a smaller type:

```bash
kubectl describe node <node-name>
```

Use `Allocated resources` for schedulability and `kubectl top nodes` for actual
usage. A node can be idle by actual CPU but still too full by requests.

## Cost Math

Refresh prices from a current source, preferably the Cloud Billing Catalog API.
Compute monthly VM-only spend as:

```text
monthly = hourly_rate * node_count * 730
saved = old_monthly - new_monthly
percent_saved = saved / old_monthly
```

Report the exact live node mix and target mix used for the calculation. In this
repo, the low-cost steady target is usually:

- app pool: `e2-standard-2`, autoscaling `min=1`, `max=2`
- stateful pools: one shared-core `e2-medium` in each stateful zone, `min=max=1`
- CNPG `2` instances, NATS `3` brokers, Valkey/Sentinel `3` nodes

## Migration Flow

1. Create a transition GitOps PR that allows both old and new stateful pool
   names in NATS, CNPG, and Valkey affinity.
2. Validate before pushing:

```bash
ruby -e 'require "yaml"; YAML.load_file("deploy/env/cloud/values.platform.yaml"); puts "values.platform.yaml ok"'
git diff --check
helm lint deploy/charts/pulse-platform -f deploy/env/cloud/values.platform.yaml
helm template pulse-platform-cloud deploy/charts/pulse-platform -f deploy/env/cloud/values.platform.yaml >/tmp/pulse-platform-cloud.yaml
make lint
```

3. Merge only after PR checks pass. Hard-refresh Argo and wait for both cloud
   apps to be `Synced` and `Healthy`.
4. Create replacement node pools one at a time. For stateful shared-core pools,
   keep the stateful taint and one node per zone:

```bash
gcloud container node-pools create stateful-pool-medium \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01 \
  --machine-type e2-medium \
  --disk-type pd-balanced \
  --disk-size 50 \
  --image-type COS_CONTAINERD \
  --node-locations us-east1-c \
  --num-nodes 1 \
  --enable-autoscaling \
  --total-min-nodes 1 \
  --total-max-nodes 1 \
  --node-taints workload=stateful:NoSchedule \
  --workload-metadata GKE_METADATA \
  --service-account default \
  --enable-autorepair \
  --enable-autoupgrade \
  --quiet
```

Repeat with `stateful-pool-ha-medium` in `us-east1-d` and
`stateful-pool-quorum-medium` in `us-east1-b`.

5. Drain old stateful nodes one at a time. Prefer non-primary CNPG nodes first:

```bash
kubectl -n pulse-platform get cluster pulse-platform-core -o jsonpath='{.status.phase}{" current="}{.status.currentPrimary}{" target="}{.status.targetPrimary}{" ready="}{.status.readyInstances}{"\n"}'
kubectl drain <old-node> --ignore-daemonsets --delete-emptydir-data --timeout=10m
```

6. Between every drain, wait for the health gate below before continuing.
7. Delete drained old pools with `gcloud container node-pools delete ...`.
8. Create a cleanup PR that removes retired pool names from Helm affinity and
   removes transition-only docs wording.

## Health Gate

Do not proceed to the next disruptive step until all are true:

```bash
kubectl -n argocd get applications pulse-platform-cloud pulse-services-cloud
kubectl -n pulse-platform get statefulset pulse-platform-cloud-nats -o jsonpath='{.status.readyReplicas}{"\n"}'
kubectl -n pulse-platform get statefulset pulse-platform-cloud-valkey-node -o jsonpath='{.status.readyReplicas}{"\n"}'
kubectl -n pulse-platform get cluster pulse-platform-core -o jsonpath='{.status.phase}{" ready="}{.status.readyInstances}{" current="}{.status.currentPrimary}{"\n"}'
kubectl -n pulse-platform get endpoints pulse-platform-core-rw
kubectl get pods -A --field-selector=status.phase!=Running -o wide
```

Expected state: Argo `Synced/Healthy`, NATS ready `3`, Valkey ready `3`, CNPG
`Cluster in healthy state` with `readyInstances=2`, and a non-empty RW endpoint.

## CNPG Handoff

During pod-spec-affinity changes, CNPG can report
`Primary instance is being restarted without a switchover` while the old primary
terminates and the RW endpoint is empty. If a healthy replica is already running,
refresh the desired target primary, including the timestamp:

```bash
ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
kubectl -n pulse-platform patch cluster pulse-platform-core \
  --type=merge \
  --subresource=status \
  -p "{\"status\":{\"targetPrimary\":\"<healthy-replica>\",\"targetPrimaryTimestamp\":\"${ts}\"}}"
```

Then watch CNPG until the current primary, RW endpoint, and ready instance count
settle. Assume `kubectl cnpg` may not be installed.

## Cleanup Checks

After the cluster stabilizes, check for dangling edges and stragglers:

```bash
gcloud container node-pools list --cluster pulse-cloud --region us-east1 --project ecoflow-pulse-dev-260221-01
gcloud compute instances list --project ecoflow-pulse-dev-260221-01 --filter='name~^gke-pulse-cloud'
gcloud compute disks list --project ecoflow-pulse-dev-260221-01 --filter='zone:(us-east1*)' --format='table(name,location(),sizeGb,type.scope(diskTypes),status,users.basename())'
gcloud compute forwarding-rules list --project ecoflow-pulse-dev-260221-01 --format='table(name,region,IPAddress,target.basename(),loadBalancingScheme)'
kubectl top nodes
```

Call out unattached disks, old node pools, old GKE instances, non-running pods,
or Argo drift explicitly.
