# ADR-0003: Messaging — NATS JetStream for Streaming + Replay

**Status:** Accepted  
**Date:** 2026-02-20

## Context
Telemetry ingestion and projection require:
- Low-latency fan-in/fan-out
- Consumer groups and backpressure handling
- Replayability (at least operational replay)
- Operational simplicity for a small team

Kafka ecosystems are powerful but heavier to operate early.

## Decision
Use **NATS JetStream** as the streaming bus for:
- telemetry ingestion events
- projection consumers
- replay subjects/jobs and gap repair workflows

Keep JetStream retention modest (24–72 hours) for operational replay; use object storage as the authoritative replay log.

Subject taxonomy and shard routing are standardized as:
- `pulse.telemetry.ingest.sNNN`
- `pulse.telemetry.projection.sNNN`
- `pulse.telemetry.archive.sNNN`
- `pulse.telemetry.replay.sNNN`
- `pulse.telemetry.gaprepair.sNNN`

Where:
- `NNN` is a zero-padded shard index,
- shard is deterministic by device ID using `hash(device_id) % shard_count`,
- initial default shard count is 128 (tunable).

## Consequences
### Positive
- Simple operations relative to Kafka
- High throughput and low latency
- Supports consumer groups and replay semantics

### Tradeoffs
- If long-term ecosystem integrations require Kafka, migration may be needed

### Follow-ups
- [x] Define subject naming + sharding strategy
- Define replay subject conventions and idempotency strategy
