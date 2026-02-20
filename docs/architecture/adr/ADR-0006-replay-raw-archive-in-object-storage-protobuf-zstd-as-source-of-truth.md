# ADR-0006: Replay — Raw Archive in Object Storage (Protobuf + Zstd) as Source of Truth

**Status:** Accepted  
**Date:** 2026-02-20

## Context
Replay is a hard requirement: MQTT cannot provide historical telemetry. The system needs:
- per-device replay
- fleet/system replay (ranges/shards)
- gap repair for missing windows
Retention requirements:
- raw telemetry: **30 days**
- rollups: years

## Decision
Write a **raw telemetry archive** to object storage as the authoritative replay log:
- format: **protobuf + zstd**
- partitioning: time (hour) + shard
- local: MinIO; prod: GCS
- maintain a Postgres manifest/index for quick object lookup

JetStream retention (24–72h) is operational-only; authoritative replay reads from the archive.

## Consequences
### Positive
- Durable replay with predictable cost
- Efficient storage via compression
- Works for per-device and fleet replay patterns

### Tradeoffs
- Requires a manifest/index and replay tooling
- Replays must be throttled to protect projections and DB

### Follow-ups
- Implement archive writer + manifest table
- Implement replay CLI and gap detector
- Define idempotency via message IDs
