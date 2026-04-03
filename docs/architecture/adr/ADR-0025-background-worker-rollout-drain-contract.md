# ADR-0025: Background Worker Rollout Drain Contract

**Status:** Accepted  
**Date:** 2026-03-30  
**Owners:** Pulse platform team  
**Related:** ADR-0009, ADR-0013, ADR-0020

---

## Context
Pulse now runs critical background workers in Kubernetes by default for local and
environment validation. Those workers are part of the 99.99% availability path:
ingest, projection, rollup, archive, inference, and solar verification all
affect user-visible freshness, history correctness, or deploy safety.

Recent local rollouts exposed a gap between process-level graceful shutdown and
Kubernetes lifecycle behavior. Worker processes already listened for `SIGTERM`
and attempted clean shutdown, but Kubernetes had no explicit pre-stop drain
signal for the distroless worker image. During a bad interaction with leaked
Postgres transactions, that left rollouts vulnerable to deadlock: new replicas
could not acquire dependencies cleanly, while old replicas remained the only
available pods.

### Requirements / Goals
- Make worker rollouts resilient under routine rolling updates.
- Ensure terminating workers stop advertising readiness before process exit.
- Keep the rollout contract compatible with distroless images and metrics-backed
  health endpoints.
- Keep local and environment behavior aligned.

### Non-goals
- Replacing RollingUpdate with destructive recreate behavior for normal worker deployments.
- Adding shell-based lifecycle scripts to distroless images.

---

## Options considered
### Option A: Signal-only shutdown
**Pros**
- Minimal implementation work.

**Cons**
- Kubelet has no explicit drain step before `SIGTERM`.
- Readiness transitions depend entirely on process timing and probe cadence.

### Option B: Metrics-backed drain endpoint plus Kubernetes `preStop`
**Pros**
- Works with distroless images.
- Gives Kubernetes an explicit `go unready, then terminate` step.
- Aligns with the existing `/livez` and `/readyz` contract.

**Cons**
- Requires every metrics-backed worker to expose and maintain the drain endpoint.

### Option C: Sidecar or external drain coordinator
**Pros**
- Could centralize lifecycle behavior outside each worker binary.

**Cons**
- Adds operational complexity and more moving parts than needed for the current stack.

---

## Decision
- We will standardize background-worker rollout drain on metrics-backed `/drainz`
  endpoints plus Kubernetes `preStop` hooks.
- We will keep `/livez` healthy during drain and flip `/readyz` to `503` as soon
  as drain starts.
- We will use in-process drain delay rather than shell-based `sleep` hooks so the
  contract works on distroless images.
- We will keep the local incremental deploy path dependency-aware and verifier-first
  when a worker class has known startup pressure on shared dependencies.

---

## Rationale
This approach gives Kubernetes an explicit readiness handoff before termination
without introducing shell dependencies or sidecars. It matches the existing
worker metrics pattern, preserves RollingUpdate semantics, and reduces the chance
that routine deploys create ingest gaps, duplicated work, or rollout deadlocks
under transient dependency pressure.

---

## Consequences
### Positive
- Worker pods become unready before termination during rolling updates.
- Distroless service images keep a clean native rollout contract.
- Shutdown behavior is easier to reason about and test.

### Negative / Tradeoffs
- Metrics-backed workers now own one more lifecycle endpoint.
- Misconfigured probe/lifecycle templates can still undermine the contract if
  template and runtime drift.

### Risks & mitigations
- **Risk:** A worker exposes `/drainz` but the Deployment forgets to call it.  
  **Mitigation:** Keep rollout lifecycle behavior in shared Helm templates and document it.
- **Risk:** Drain hooks return too quickly for kubelet endpoint removal.  
  **Mitigation:** Keep a short in-process drain hold period before the hook returns.

---

## Implementation plan
1. Add `/drainz` support to metrics-backed worker HTTP servers.
2. Wire Kubernetes `preStop` hooks to `/drainz` in `pulse-services` worker Deployments.
3. Keep rollout docs and local deploy tooling aligned with the new contract.
4. Add regression tests covering readiness transitions during explicit drain.

### Rollout / Migration
- Roll out via normal Helm updates; no schema migration is required.
- Existing workers without metrics-backed readiness should not silently claim drain safety until their runtime is updated.

### Observability
- metrics:
  - readiness status via `/readyz`
  - liveness via `/livez`
  - worker-specific operational metrics already exposed on the metrics port
- logs:
  - startup dependency retries
  - shutdown and drain warnings
- alerts:
  - rollout timeouts or pods stuck unavailable beyond the configured deployment timeout

### Security / Compliance
- `/drainz` is cluster-internal lifecycle plumbing on the worker metrics port and does not change user-facing auth or data handling.

---

## Acceptance criteria
- Metrics-backed workers expose `/drainz` and flip `/readyz` to draining before exit.
- `pulse-services` worker Deployments use `preStop` drain hooks in chart templates.
- Local incremental deploys complete without the previously observed ingest deadlock caused by stale verifier DB pressure.

---

## Follow-ups
- [ ] Consider unifying ingest autoscale metrics serving with `internal/workermetrics` to remove duplicated drain logic.
- [ ] Evaluate whether any remaining non-metrics worker runtimes need the same drain contract.
