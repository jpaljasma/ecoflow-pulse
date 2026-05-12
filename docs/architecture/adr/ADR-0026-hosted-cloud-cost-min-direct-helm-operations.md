# ADR-0026: Hosted Cloud Cost-Min Direct Helm Operations

**Status:** Accepted  
**Date:** 2026-05-12  
**Owners:** Pulse platform team  
**Related:** ADR-0002, ADR-0014, ADR-0025

---

## Context
The hosted `pulse-cloud` cluster is being narrowed to the cheapest reliable
cloud data plane: ingest, archive, rollup, CNPG/Timescale, NATS JetStream,
Valkey/Sentinel, Secret Manager integration, and GCS raw archive storage.

Argo CD has been useful for GitOps realism, but in this cost-min hosted mode it
adds pods, memory pressure, and another reconciliation loop. The operator is
comfortable with explicit manual deploy commands as long as the commands are
repeatable, gated, and easy to audit.

### Requirements / Goals
- Keep the default rollout no-planned-outage for critical ingest/storage paths.
- Avoid accidental removal of public/auth services before local cloud-Postgres
  app mode is proven.
- Provide a direct Helm deploy path that can replace Argo after a clean no-op
  apply.
- Keep stateful durability on PVC-backed CNPG, NATS, and Valkey.

### Non-goals
- Guaranteeing every MQTT provider message during all reconnect windows. The
  EcoFlow subscribe path currently uses QoS `0`.
- Removing local/dev Argo workflows in this decision.
- Moving CNPG/NATS/Valkey to cheaper but less appropriate storage.

---

## Decision
- Hosted cost-min cloud operations will use direct Helm commands from the repo
  after the cutover from Argo is proven.
- Argo must remain installed until direct Helm can apply the current platform and
  services charts cleanly and health gates pass.
- Default cloud values may only carry safe ingest hardening. Destructive cost-min
  reductions live in explicit opt-in cloud overlay files.
- `go-ingest` runs with two replicas in hosted cloud; `go-archive` and
  `go-rollup` remain singleton durable-consumer workers.
- Singleton worker PDBs for ingest/archive/rollup use `minAvailable: 1`, not
  `maxUnavailable: 1`.

---

## Rationale
Direct Helm removes Argo's standing resource cost while preserving a deterministic
apply path. Keeping the aggressive reduction in opt-in overlays prevents a merge
from unexpectedly removing public app, realtime, auth, ingress, or load balancer
resources before the local app path has been connected to cloud Postgres and
validated.

---

## Consequences
### Positive
- Lower steady-state cloud cost after Argo and public-path components are removed.
- Manual deploys are explicit and gated by cluster health checks.
- Ingest rollout safety improves before any infrastructure reduction.

### Negative / Tradeoffs
- Hosted cloud loses automatic Argo reconciliation after cutover.
- Operators must run the manual deploy target intentionally.
- Git remains source of truth, but reconciliation is no longer continuous.

### Risks & mitigations
- **Risk:** Direct Helm and Argo fight over the same resources.  
  **Mitigation:** Keep Argo until direct apply is proven, then remove Argo before
  relying on direct Helm as the normal path.
- **Risk:** Cost-min overlays remove public/auth paths too early.  
  **Mitigation:** Do not reference cost-min overlays from default cloud values or
  automatic Argo applications.
- **Risk:** MQTT reconnect windows can still miss provider messages.  
  **Mitigation:** Run two ingest replicas, preserve drain hooks, monitor drop
  logs, and keep NATS/archive/rollup replayability intact after publish.

---

## Implementation plan
1. Merge and deploy the safe cloud values: `go-ingest=2`, archive/rollup
   unchanged, and PDBs hardened for ingest/archive/rollup.
2. Validate local app services against cloud CNPG through a private
   `kubectl port-forward`.
3. Prove `make cloud-deploy` as a direct Helm no-op/apply path while Argo is
   still available for rollback.
4. Remove Argo and use the explicit cost-min overlays only after the above gates
   pass.
5. Delete unused public-path resources and app nodes only after pods are healthy
   on the intended stateful pools.

### Rollout / Migration
- Never drain more than one node at a time.
- Never delete Argo before direct Helm has applied successfully.
- Never delete the app pool while required workloads still schedule there.
- Abort on missing CNPG RW endpoint, NATS/Valkey readiness loss, worker
  readiness churn, or recent ingest/archive/rollup drop logs.

### Observability
- Watch ingest/archive/rollup logs for `dropping envelope`, drain failures, and
  publish failures.
- Verify NATS, Valkey, CNPG ready counts before and after each live step.
- Verify fresh telemetry reaches archive and rollup after each rollout.

---

## Acceptance criteria
- `go-ingest` is `2/2` ready in hosted cloud.
- `go-archive` and `go-rollup` remain ready.
- Local app services can read cloud Postgres through `127.0.0.1:25432`.
- Direct Helm deploy can apply the current hosted charts and pass health gates.
- Cost-min overlay removal leaves no Argo pods, no public load balancer, no
  unused app nodes, and no unattached disks.

---

## Follow-ups
- [ ] Cut over the default hosted deploy workflow from Argo mode to direct Helm
      mode after Argo is removed.
- [ ] Consider EcoFlow MQTT QoS or provider backfill options if strict
      per-message continuity becomes a product requirement.
