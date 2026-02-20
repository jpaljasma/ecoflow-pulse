# Config 04 — Data retention & replay (protobuf + zstd)

This is the core durability model for EcoFlow Pulse.

---

## Retention (locked)
- Raw archive: **30 days**
- Minute rollups: **90 days**
- Hourly rollups: **3 years**
- Daily rollups: **3 years**

---

## Raw archive (authoritative replay)
Store time-partitioned objects in object storage.

### Partitioning
- `raw/yyyy=YYYY/mm=MM/dd=DD/hh=HH/shard=SS/part-NNNNN.pb.zst`
- `shard = hash(device_id) % N`

### Format
- Protobuf `TelemetryEnvelope` (length-delimited or concatenated frames)
- Compressed with zstd

### File sizing
- Target 1–5 minute objects per shard (tune later)

---

## Manifest index (Postgres)
Maintain a table for fast object lookup:

Suggested columns:
- `object_key`
- `shard`
- `ts_min`, `ts_max`
- `record_count`
- `checksum`
- `created_at`

---

## Replay modes (required)
1) **Per-device replay**: compute shard → lookup objects → filter device IDs → publish to replay subject
2) **Fleet replay**: replay shard/time ranges to rebuild projections
3) **Gap repair**: detect holes → enqueue replay jobs for targeted windows

---

## JetStream retention (operational)
Use JetStream retention of 24–72 hours for quick recovery.
The raw archive remains the authoritative 30-day replay log.

---

## Drop tolerance
Given 0.1% acceptable drops:
- prefer backpressure + downsampling before dropping
- if dropping is necessary, emit metrics and alarms
- use gap repair to catch up when feasible
