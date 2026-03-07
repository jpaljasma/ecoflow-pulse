# ADR-0015: Quota archive replayability and safe rollup regeneration

**Status:** Accepted  
**Date:** 2026-03-07  
**Owners:** Jaan  
**Related:** ADR-0006, ADR-0014  
**Supersedes:** ADR-0014 (quota archive exclusion rule only)

---

## Context
EcoFlow Pulse currently uses two paths for solar-relevant telemetry:
- raw MQTT payloads
- normalized quota refresh/bootstrap payloads (`source=quota`, `payload_type=ecoflow.quota.normalized`)

The accepted M2 implementation intentionally excluded normalized quota frames from the raw archive while still letting them affect projection, rollups, and read models.

That tradeoff turned out to be incorrect for solar validation and replay correctness:
- some DPU solar behavior is only visible in quota-derived payloads,
- replaying only archived MQTT frames undercounts solar relative to live history,
- local `dev-regen-data` used delete-first replay into additive rollup upserts, which can leave empty charts during rebuild and is not a safe overwrite strategy.

### Requirements / Goals
- Replay and rebuild paths must be able to reconstruct the same solar signal used by live rollups as closely as practical.
- Historical regeneration must not create a chart-emptying window for users.
- Rebuilds must replace affected buckets safely, in bounded transactions/chunks.

### Non-goals
- Exactly-once historical reconstruction.
- Instant zero-downtime rebuild for every query consumer during long-running backfills.

---

## Options considered
### Option A: Keep excluding quota frames and keep replay-via-NATS regeneration
**Pros**
- Minimal code changes.
- Preserves prior archive volume assumptions.

**Cons**
- Replay remains structurally unable to reproduce live DPU solar totals.
- Delete-first rebuilds create visible empty-chart windows.
- Additive rollup upserts are not a safe overwrite model.

### Option B: Archive quota frames but still regenerate through NATS replay
**Pros**
- Replay input becomes more complete.
- Small archive-side change.

**Cons**
- Regeneration still depends on asynchronous workers.
- Delete-first / additive-write behavior remains unsafe.

### Option C: Archive quota frames and rebuild rollups directly from archive with transactional replacement (chosen)
**Pros**
- Replayable archive now contains quota-derived solar signals.
- Rebuilds compute final bucket values first, then replace rows safely.
- Avoids delete-first empty-chart windows.

**Cons**
- Adds dedicated rebuild code path.
- Slightly increases archive object volume.

---

## Decision
- We will archive normalized quota envelopes instead of skipping them.
- We will treat quota-derived solar frames as replayable inputs for historical rebuilds.
- We will regenerate rollups through a direct archive-to-rollup rebuild path, not by replaying into ingest/NATS for overwrite scenarios.
- We will replace rebuilt rollup rows in bounded transactional chunks using upsert semantics with final values, without deleting the requested range first.

---

## Rationale
This is the only option that addresses both correctness and UX:
- correctness: replay must include the same quota-derived solar data that live rollups see,
- UX: regeneration must not blank out charts before backfill completes,
- operations: direct rebuilds are deterministic and do not depend on consumer lag or additive rollup math.

---

## Consequences
### Positive
- DPU solar rebuilds can include quota-derived frames.
- Rollup regeneration becomes safer for local/dev and future operational tooling.
- Proof output can validate rebuilt solar more honestly.

### Negative / Tradeoffs
- Archive size increases.
- Two historical paths now exist:
  - replay-to-ingest for operational event reflow,
  - direct rebuild for safe rollup replacement.

### Risks & mitigations
- **Risk:** quota archive increases replay/object volume.  
  **Mitigation:** keep payload framing/compression unchanged and validate manifest/object growth operationally.
- **Risk:** direct rebuild logic diverges from live rollup extraction.  
  **Mitigation:** reuse `rollupworker.SampleFromEnvelope` for rebuild aggregation and keep dedicated tests.

---

## Implementation plan
1. Stop skipping `ecoflow.quota.normalized` envelopes in archive worker.
2. Add a direct archive-to-rollup rebuild package and CLI.
3. Update `dev-regen-data` to use direct rebuild instead of delete-first replay through NATS.
4. Update proof output/docs to reflect derived solar validation semantics.

### Rollout / Migration
- Existing archived windows still lack old skipped quota frames; rebuild accuracy improves only for newly archived windows after this change.
- Existing rollup tables remain schema-compatible; replacement happens row-by-row in transactions.

### Observability
- logs:
  - direct rebuild start/completion with object/message/bucket counts
  - chunk replacement counts per resolution
- proof:
  - `derived_solar_generated_wh` remains the primary local/dev rebuild proof field

### Security / Compliance
- No new secret classes.
- Rebuild uses existing MinIO/Postgres credentials already required for replay tooling.

---

## Acceptance criteria
- Quota envelopes are written to archive objects and manifest-backed replay can observe them.
- `dev-regen-data` no longer deletes rollup ranges before rebuild.
- Rebuild writes happen in bounded transactions/chunks and complete without emptying charts first.

---

## Follow-ups
- [ ] Measure archive growth after quota-frame inclusion.
- [ ] Add bucket-diff reporting between pre-rebuild and post-rebuild local windows.
