# ADR-0018: Rollups Persist Explicit Energy Buckets for Energy Dashboard and Historical Analytics

**Status:** Superseded  
**Date:** 2026-03-11  
**Owners:** Platform / Data Plane  
**Superseded by:** ADR-0019 (service-boundary non-goal only)  
**Related:** [ADR-0005-databases-postgres-timescaledb-for-control-plane-rollups.md](./ADR-0005-databases-postgres-timescaledb-for-control-plane-rollups.md), [ADR-0006-replay-raw-archive-in-object-storage-protobuf-zstd-as-source-of-truth.md](./ADR-0006-replay-raw-archive-in-object-storage-protobuf-zstd-as-source-of-truth.md), [ADR-0015-quota-archive-replayability-and-safe-rollup-regeneration.md](./ADR-0015-quota-archive-replayability-and-safe-rollup-regeneration.md), [../README.md](../README.md)

---

## Context
The Energy dashboard shipped as a truthful v1 using a mixed methodology:
- `solar_generated_wh` already exists as a persisted rollup bucket,
- several other energy-series values are derived at query time from average-power rollup fields and local-calendar bucket widths,
- historical per-port PV observations are enriched from archive reads instead of native rollup columns.

That v1 approach is acceptable for product review and local deployment, but it is not the correct long-term architecture for energy analytics.

Query-time derivation creates avoidable ambiguity:
- fleet aggregation can be correct only if every energy-relevant power field is merged consistently,
- historical charts depend on bucket-width math instead of persisted energy facts,
- new UI/API consumers must relearn which values are measured and which are reconstructed,
- backfill/rebuild correctness is harder to reason about because the persisted rollup does not fully represent the intended energy model.

The platform already has the right structural pieces for a better model:
- Timescale-backed rollups are the authoritative historical query path,
- object archive is the replay source of truth,
- archive-to-rollup rebuilds are already a supported operational pattern,
- quota-derived normalized telemetry is replayable and can contribute to rebuild accuracy.

### Requirements / Goals
- Persist first-class energy buckets in rollups so historical energy APIs query stored energy facts instead of deriving them from average power.
- Keep the Energy dashboard and future analytics consistent across single-device and fleet views.
- Preserve replay/backfill correctness through archive-driven rollup regeneration.
- Make energy semantics explicit in schemas, APIs, tests, and documentation.

### Non-goals
- This ADR does not introduce a separate Energy service or change the Node BFF / Go gRPC / Expo architecture.
- This ADR does not require historical per-port PV observations to move out of archive enrichment immediately.
- This ADR does not change retention windows or the raw archive source-of-truth model.

---

## Options considered
### Option A: Keep long-term query-time derived energy buckets
**Pros**
- No rollup schema changes.
- Fastest short-term implementation path.

**Cons**
- Semantics remain ambiguous and fragile.
- Fleet and comparison logic keep depending on query-layer reconstruction.
- Backfills cannot materialize the final energy model directly into rollups.
- Product/UI additions will keep reopening methodology questions.

### Option B: Persist explicit energy buckets in rollups and rebuild historical windows from archive
**Pros**
- Energy history becomes a first-class stored fact model.
- Query logic becomes simpler, more consistent, and easier to validate.
- Fleet merges and calendar-window comparisons operate on persisted energy values.
- Replay and rebuild semantics align with the raw archive source of truth.

**Cons**
- Requires rollup schema changes, worker changes, and backfill work.
- Increases rollup storage footprint.
- Needs careful transition logic while old windows are still partially derived.

### Option C: Create a dedicated energy-materialization store outside rollups
**Pros**
- Could optimize specifically for energy analytics and UI needs.
- Avoids modifying existing rollup tables.

**Cons**
- Adds a second historical truth path.
- Increases operational complexity and drift risk.
- Conflicts with the existing architecture direction that rollups are the primary history query model.

---

## Decision
- We will persist explicit energy buckets in the rollup schema instead of treating query-time derivation as the long-term method.
- We will keep the current derived-v1 approach only as a temporary compatibility layer until rollup persistence and rebuild/backfill are complete.
- We will prefer persisted explicit energy buckets in Go query paths and API responses as soon as those columns are available for the requested window.
- We will rebuild historical windows from the archive source of truth so persisted energy buckets are populated with replay-consistent values.
- We will not create a separate energy history datastore for this problem.

The target explicit rollup model must include, at minimum:
- `solar_generated_wh`
- `ac_input_energy_wh`
- `ac_output_energy_wh`
- `dc_output_energy_wh`
- `load_energy_wh`
- `battery_charge_energy_wh`
- `battery_discharge_energy_wh`

---

## Rationale
Option B matches the existing platform architecture with the least long-term ambiguity.

Persisting explicit energy buckets keeps the historical query contract aligned with the data-plane contract:
- rollups store what the UI and APIs actually mean,
- archive replay can regenerate the same facts deterministically,
- fleet aggregation becomes arithmetic over stored energy fields instead of repeated inference,
- local-calendar comparisons no longer depend on translating average power into energy at read time.

This also reduces product risk. The Energy dashboard already exposed where derivation becomes awkward:
- estimated value semantics were easier to reason about when tied directly to measured/generated energy,
- missing or omitted merged fields created user-visible chart errors,
- acceptance depended on a deliberate waiver for derived-v1 buckets.

Storage growth is the main tradeoff, but it is acceptable because:
- these are bounded additional numeric columns on already-existing rollup rows,
- the gain in correctness and simpler read-path semantics outweighs the extra storage,
- rebuilds from archive are already part of the platform's operational model.

---

## Consequences
### Positive
- Energy history becomes explicit and queryable without reconstruction.
- API/UI semantics become easier to document and test.
- Fleet aggregation, comparison windows, and cost/value calculations become less error-prone.
- Historical rebuilds can produce the same explicit energy facts used by live reads.

### Negative / Tradeoffs
- Rollup schemas and workers become wider.
- Historical backfill work is required before all windows are fully migrated.
- Query paths need temporary dual-read compatibility while old data is phased out.

### Risks & mitigations
- **Risk:** Rollup writer changes produce inconsistent bucket semantics across minute/hour/day tables.  
  **Mitigation:** Define explicit per-bucket formulas once, test them at the rollup layer, and propagate by aggregation rather than re-derivation.

- **Risk:** Backfill takes too long or creates holes during migration.  
  **Mitigation:** Use bounded archive-to-rollup regeneration windows, track coverage metrics, and keep derived fallback only while coverage is incomplete.

- **Risk:** Quota-derived frames and MQTT-derived frames disagree for some device classes.  
  **Mitigation:** Keep replay/rebuild sourcing on normalized archive envelopes and expand device-specific regression fixtures for DPU and D2M classes.

---

## Implementation plan
1. Extend minute/hour/day rollup schemas to add explicit energy bucket columns.
2. Update rollup writers to materialize those buckets from normalized telemetry inputs at ingest/aggregation time.
3. Update rollup aggregation logic so higher-order buckets sum persisted lower-order energy buckets instead of re-deriving from average power.
4. Update Go query/energy dashboard paths to prefer persisted explicit buckets and keep temporary fallback behavior only where coverage is incomplete.
5. Add archive-to-rollup rebuild support for the new columns and backfill historical windows.
6. Remove derived-v1 fallback logic after coverage and validation are complete.

### Rollout / Migration
- Add the new columns in a backward-compatible schema change.
- Deploy writers before switching readers to persisted-first behavior.
- Backfill historical windows from archive in bounded batches.
- Expose coverage/usage signals so read paths can report whether a response came from persisted energy buckets or temporary derived fallback.
- Remove fallback only after validation confirms persisted coverage for required retention windows.

### Observability
- metrics:
  - rollup write coverage for each explicit energy bucket
  - archive rebuild rows updated / window coverage
  - Energy API responses using persisted buckets vs fallback derivation
  - rebuild duration and failure counts
- logs:
  - rollup writer warnings when required source fields are missing
  - rebuild warnings for archive decode / object gaps
  - read-path fallback notices while migration is active
- alerts:
  - rebuild failure rate above agreed threshold
  - unexpected fallback usage after migration target dates

### Security / Compliance
- No auth-boundary changes.
- No retention-policy changes.
- Replay continues to use the existing archive source of truth and inherited retention controls.

---

## Acceptance criteria
- Rollup schemas persist the explicit energy bucket set defined in this ADR.
- Minute-to-hour and hour-to-day aggregation sums persisted energy buckets instead of deriving them from average power.
- Energy dashboard/history APIs prefer persisted explicit buckets for covered windows.
- Archive-to-rollup rebuilds can populate the new columns for historical windows without destructive replay shortcuts.
- Regression coverage exists for at least one DPU-class and one D2M-class device path.
- Documentation clearly distinguishes temporary fallback behavior from the accepted long-term architecture.

---

## Follow-ups
- [ ] Add schema migrations for explicit energy bucket columns in the rollup tables.
- [ ] Update rollup writers and aggregators to materialize the explicit energy buckets.
- [ ] Add rebuild/backfill commands and runbooks for historical population of the new columns.
- [ ] Remove derived-v1 energy fallback after persisted coverage is complete.
