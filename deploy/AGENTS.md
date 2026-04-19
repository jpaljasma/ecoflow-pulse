# AGENTS

## Scope
This file adds deploy/runtime guidance for `deploy/` work on top of the repository root `AGENTS.md`.

## Rollouts
1. Default to rolling, non-destructive updates.
2. Public and background Deployments must not cut traffic or work abruptly during routine restarts.
3. Prefer readiness-driven drain behavior:
   - accurate readiness probes,
   - `preStop` drain hooks or equivalent,
   - sufficient termination grace for in-flight shutdown.
4. Treat deploy-induced auth failures, websocket interruptions without recovery, ingest gaps, or duplicate side effects as availability bugs.

## Local k3d Workflows
1. Keep local bring-up and deploy paths dependency-aware:
   - platform dependencies first,
   - service rollouts only after CNPG/NATS/Valkey/MinIO are actually ready.
2. Local incremental deploys should be reproducible and safe with:
   - `make services-image-build-local`
   - `make services-image-import-local`
   - `make dev-deploy`
3. Do not introduce manual pod babysitting as the expected recovery path; if a rollout only works after deleting pods by hand, treat that as an incomplete fix.

## Helm and Values
1. Keep chart behavior aligned between templates and local/dev values.
2. When rollout behavior changes, update:
   - chart templates,
   - local/dev values if needed,
   - deploy docs and command docs in the same branch.
3. For distroless images, prefer HTTP or native-process lifecycle hooks over shell-based assumptions.

## Validation
1. Run the narrowest useful validation set for deploy changes:
   - `helm lint deploy/charts/pulse-services -f deploy/env/local/values.services.yaml`
   - `helm lint deploy/charts/pulse-services -f deploy/env/dev/values.services.yaml` when relevant
   - local `kubectl rollout status` / `make dev-deploy` when rollout behavior changed
2. Record operational evidence in docs when the change affects deploy safety or recovery behavior.

## Hosted GKE Learnings
1. When a cloud StatefulSet needs a new PVC template, do not expect an in-place patch to converge:
   - recreate the StatefulSet controller with orphaned pods,
   - migrate one ordinal at a time,
   - keep quorum/replica health green between ordinals.
2. For hosted zonal HA, prefer efficient app nodes first:
   - keep the public/stateless pool on `e2-standard-2` by default,
   - only move public/auth traffic onto spare stateful-zone capacity when zonal stockout blocks the cleaner dedicated app-pool plan,
   - document that compromise explicitly and revisit it later.
3. For hosted CNPG zonal HA on `WaitForFirstConsumer` storage, verify where the replacement replica PVC binds before trusting the zone move; if you must intervene, preserve the primary, keep the off-zone PVC/data path honest, and let CNPG finish the final attach/promote path.
