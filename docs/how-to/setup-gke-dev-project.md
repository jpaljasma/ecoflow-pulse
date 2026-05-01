# How-to: Set Up a New GKE Dev Project and Bootstrap Argo CD

Use this guide when you need a fresh Google Cloud project for EcoFlow Pulse dev integration tests.

This runbook is intentionally cost-min aligned with:

- `docs/architecture/dev-cost-min-gke-guide.md`
- `docs/architecture/adr/ADR-0011-dev-cost-policy-k3d-daily-gke-for-integration-only.md`

## 1. Prerequisites

Install and authenticate:

```bash
gcloud auth login
gcloud auth application-default login
gcloud billing accounts list
```

Install local tooling used by the Make targets:

```bash
gcloud --version
kubectl version --client
helm version
```

## 2. Define project/cluster variables

```bash
export PROJECT_ID="ecoflow-pulse-dev-260221-01"
export PROJECT_NAME="EcoFlow Pulse Dev 2026-02-21"
export BILLING_ACCOUNT="012712-2416AD-2CA20D"
export GKE_CLUSTER_NAME="pulse-dev"
export GKE_CLUSTER_ZONE="us-east1-b"
```

If your `PROJECT_ID` is already taken, pick a new unique suffix.

## 3. Create project and link billing

```bash
gcloud projects create "${PROJECT_ID}" --name="${PROJECT_NAME}"
gcloud billing projects link "${PROJECT_ID}" --billing-account="${BILLING_ACCOUNT}"
gcloud config set project "${PROJECT_ID}"
gcloud auth application-default set-quota-project "${PROJECT_ID}"
```

## 4. Enable required APIs

```bash
gcloud services enable \
  container.googleapis.com \
  compute.googleapis.com \
  iam.googleapis.com \
  cloudresourcemanager.googleapis.com \
  serviceusage.googleapis.com \
  logging.googleapis.com \
  monitoring.googleapis.com \
  secretmanager.googleapis.com
```

## 5. Create the GKE dev cluster (cost-min profile)

Create new GKE dev clusters with GKE Dataplane V2 explicitly enabled. This
keeps the runbook stable before and after Google's 2027 default change for new
clusters, and exercises the networking stack future clusters will use by
default.

```bash
gcloud container clusters create "${GKE_CLUSTER_NAME}" \
  --project "${PROJECT_ID}" \
  --zone "${GKE_CLUSTER_ZONE}" \
  --release-channel regular \
  --enable-ip-alias \
  --enable-dataplane-v2 \
  --machine-type e2-medium \
  --num-nodes 2 \
  --enable-autoscaling --min-nodes 2 --max-nodes 4 \
  --workload-pool "${PROJECT_ID}.svc.id.goog"
```

Verify the dataplane before installing platform components:

```bash
gcloud container clusters describe "${GKE_CLUSTER_NAME}" \
  --project "${PROJECT_ID}" \
  --zone "${GKE_CLUSTER_ZONE}" \
  --format='value(networkConfig.datapathProvider)'

make gke-context GKE_PROJECT_ID="${PROJECT_ID}"
kubectl -n kube-system get pods -l k8s-app=cilium -o wide
```

Expected:

- datapath provider is `ADVANCED_DATAPATH`
- `anetd` pods are present in `kube-system`

Do not add a legacy dataplane opt-out unless a specific workload compatibility
failure has been reproduced and documented. If a temporary DPv1 compatibility
cluster is required after Google's 2027 default change, use the then-current
`gcloud` opt-out flag and record the exception in the validation notes.

## 6. Bootstrap Argo CD and direct apps

From repo root:

```bash
make argocd-dev-up GKE_PROJECT_ID="${PROJECT_ID}"
```

This executes:

- `make gke-context`
- `make argocd-bootstrap-dev`
- `make argocd-apps-dev`
- `make argocd-wait-apps`

## 7. Verify Argo app health

```bash
kubectl -n argocd get applications.argoproj.io -o wide
```

Expected:

- `pulse-platform` => `SYNC STATUS: Synced`, `HEALTH STATUS: Healthy`
- `pulse-services` => `SYNC STATUS: Synced`, `HEALTH STATUS: Healthy`

## 8. Park and wake (cost control)

If your cluster has `default-pool` (not `baseline-pool`), override nodepool name:

```bash
make gke-park \
  GKE_PROJECT_ID="${PROJECT_ID}" \
  GKE_BASELINE_NODEPOOL=default-pool

make gke-wake \
  GKE_PROJECT_ID="${PROJECT_ID}" \
  GKE_BASELINE_NODEPOOL=default-pool
```

When done testing, park again:

```bash
make gke-park \
  GKE_PROJECT_ID="${PROJECT_ID}" \
  GKE_BASELINE_NODEPOOL=default-pool
```

## 9. Troubleshooting

- Argo app stuck in `OutOfSync/Missing`:
  - ensure app manifests include `spec.syncPolicy.automated`.
- `GKE_BASELINE_NODEPOOL` not found:
  - use actual nodepool name (`default-pool` on raw `gcloud container clusters create`).
- Credentials mismatch warnings:
  - re-run `gcloud auth application-default set-quota-project "${PROJECT_ID}"`.
