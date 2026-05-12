# How-to: No-Planned-Outage Hosted Cloud Cost Rollout

Use this runbook to move hosted cloud toward ingest/storage-only mode without a
planned ingest outage. The rollout is intentionally staged: first harden ingest,
then prove local app access to cloud Postgres, then remove Argo and public-path
infrastructure.

## 1. Preflight Health Gate

Start from the cloud context and require the live data plane to be healthy:

```bash
make cloud-context
make cloud-health-gate
make cloud-status
```

Do not continue if the gate reports:
- CNPG has fewer than `2` ready instances,
- the CNPG RW endpoint has no ready address,
- NATS or Valkey has fewer than `3` ready pods,
- any non-completed pod is not running,
- recent ingest/archive/rollup logs contain drops or drain/publish failures.

## 2. Safe Ingest Rollout

The default cloud services values keep archive and rollup as singleton durable
workers and raise ingest to two replicas. Deploy this stage through the existing
Argo path while Argo is still installed:

```bash
make cloud-refresh
make argocd-wait-apps-cloud
kubectl -n pulse-services rollout status deploy/pulse-services-cloud-go-ingest --timeout=600s
kubectl -n pulse-services get deploy,pdb
make cloud-health-gate
```

Expected result:
- `pulse-services-cloud-go-ingest` has `2/2` ready pods,
- `pulse-services-cloud-go-archive` has `1/1`,
- `pulse-services-cloud-go-rollup` has `1/1`,
- ingest/archive/rollup PDBs use `minAvailable: 1`.

## 3. Local App Against Cloud Postgres

Generate the local environment file without printing credentials:

```bash
make cloud-db-env
```

In one terminal, keep the private cloud CNPG port-forward open:

```bash
make cloud-db-forward
```

In another terminal, source the generated file before running local Go services:

```bash
source .tmp/cloud-postgres.env
```

The generated DSNs point to `127.0.0.1:25432`. This does not expose Postgres
publicly; it only works while the authenticated `kubectl port-forward` is open.

Validate at least one local API or CLI path against cloud data before continuing.
For example, start the local Go API with the sourced env and verify device or
history reads through the local web app.

## 4. Prove Direct Helm Before Removing Argo

Run the direct Helm path against the current default cloud values first:

```bash
make cloud-deploy
make cloud-health-gate
make cloud-status
```

The direct path uses:
- release `pulse-platform-cloud` in namespace `pulse-platform`,
- release `pulse-services-cloud` in namespace `pulse-services`,
- `helm upgrade --install`,
- `--take-ownership`,
- `--server-side=true`.

Keep Argo installed until this direct apply succeeds and the health gate passes.

## 5. Cost-Min Overlay

Only after local app cloud-Postgres mode and direct Helm are proven, apply the
explicit cost-min overlays:

```bash
make cloud-cost-min-deploy
make cloud-health-gate
make cloud-status
```

The cost-min overlays keep:
- CNPG/Timescale at `2` instances,
- NATS JetStream at `3` brokers,
- Valkey/Sentinel at `3` nodes,
- `go-ingest` at `2`,
- `go-archive` at `1`,
- `go-rollup` at `1`,
- the migration job path,
- GCS raw archive storage,
- External Secrets for runtime secret sync.

The overlays disable:
- public app,
- realtime gateway,
- Keycloak,
- ingress-nginx,
- cert-manager,
- cloud public load balancer,
- unused Go public/API/background workers.

## 6. Argo Removal And Cleanup

After `make cloud-cost-min-deploy` is healthy, remove Argo deliberately:

```bash
kubectl -n argocd get applications.argoproj.io
helm -n argocd uninstall argocd
kubectl delete namespace argocd
```

Then verify cost stragglers:

```bash
gcloud container node-pools list \
  --cluster pulse-cloud \
  --region us-east1 \
  --project ecoflow-pulse-dev-260221-01

gcloud compute forwarding-rules list \
  --project ecoflow-pulse-dev-260221-01 \
  --format='table(name,region,IPAddress,target.basename(),loadBalancingScheme)'

gcloud compute disks list \
  --project ecoflow-pulse-dev-260221-01 \
  --filter='zone:(us-east1*)' \
  --format='table(name,location(),sizeGb,type.scope(diskTypes),status,users.basename())'
```

Do not delete the app pool until `make cloud-status` shows required workloads no
longer scheduling there and the health gate remains green.

## Rollback Defaults

- If ingest rollout fails, restore the previous services values and rerun
  `make cloud-refresh`.
- If direct Helm fails while Argo still exists, leave Argo installed and inspect
  the Helm error before retrying.
- If cost-min overlay causes scheduling pressure, scale the app pool back before
  retrying.
- If any health gate fails, stop the sequence and restore the previous stage.
