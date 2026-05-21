---
name: pulse-raw-archive-inspector
description: Use when investigating Pulse archived MQTT telemetry or normalized provider envelopes from raw archive storage instead of JetStream, especially to inspect the past N hours, filter by provider/device family/model, discover new payload fields, count field/value transitions, or check whether a signal such as fan, heat, UPS, AC/DC, PV, battery, or temperature exists in stored telemetry without exposing device identifiers or credentials.
---

# Pulse Raw Archive Inspector

Use this skill for read-only telemetry archaeology from the durable archive
objects behind `archive_object_manifest`. Prefer it over JetStream when the user
asks for "past 12 hours", historical MQTT payloads, field discovery, or pattern
checks after the NATS retention window may have rolled off.

## Guardrails

- Keep this read-only. Do not replay, mutate rollups, delete manifests, or write
  archive objects while using this skill.
- Redact provider device IDs, serials, topics, credentials, object keys, DSNs,
  access keys, email addresses, and locations in user-facing output.
- Report aggregate evidence: UTC window, matched device count, object/frame
  counts, payload types, field names, nonzero counts, min/max, transitions, and
  search hit counts.
- Distinguish durable archive storage from JetStream retention. If you inspect
  only JetStream, say so explicitly; otherwise use the script below.
- Remember provider differences:
  - EcoFlow raw MQTT archives normally contain `ecoflow.mqtt.raw` envelopes.
  - Pecron and Anker Solix provider runners may archive normalized provider
    parameter envelopes (`provider.params.normalized`) derived from MQTT state,
    not the original vendor JSON bytes.

## Quick Start

Run the bundled inspector from the repository root:

```bash
go run .codex/skills/pulse-raw-archive-inspector/scripts/pulse_raw_archive_inspect.go \
  --preset cloud-gcs \
  --provider pecron \
  --family E1000LFP \
  --search fan,cool,heat \
  --hours 12
```

For local-edge runs where the DB DSN comes from the local k3d runtime secret but
the archive objects are hosted cloud GCS, use:

```bash
go run .codex/skills/pulse-raw-archive-inspector/scripts/pulse_raw_archive_inspect.go \
  --load-runtime \
  --kube-context k3d-pulse-local \
  --preset local-edge-cloud-archive \
  --provider pecron \
  --family E1000LFP \
  --search fan,cool,heat \
  --hours 12
```

For true local MinIO archive inspection:

```bash
go run .codex/skills/pulse-raw-archive-inspector/scripts/pulse_raw_archive_inspect.go \
  --load-runtime \
  --kube-context k3d-pulse-local \
  --preset local-minio \
  --provider ecoflow \
  --family DPU \
  --search fan \
  --hours 2
```

## Workflow

1. Pick the archive target:
   - Hosted cloud archive: `--preset cloud-gcs`, plus a DB DSN from env or
     `--load-runtime --kube-context <cloud-context>`.
   - Local-edge cloud DB plus cloud archive: `--load-runtime --kube-context
     k3d-pulse-local --preset local-edge-cloud-archive`.
   - True local archive: `--preset local-minio` and a reachable MinIO endpoint.
2. Choose a tight UTC or relative window. Prefer `--hours 12` for "past 12
   hours"; use `--from`/`--to` with RFC3339 UTC timestamps for incident windows.
3. Filter by `--provider` and optional `--family`. Family matches model,
   product name, capabilities, and non-sensitive metadata, not raw IDs.
4. Add `--search` terms for suspected signals (`fan,cool`, `heat,ptc`,
   `ups,ac,dc`, etc.). The script also prints top changing/nonzero fields so
   unexpected fields are visible.
5. Summarize results without sensitive identifiers. Include "no fan-like fields"
   only when both payload hits and field hits are zero for the inspected window.

## Script Notes

- Script: `scripts/pulse_raw_archive_inspect.go`
- The script reads `archive_object_manifest`, opens GCS or MinIO archive
  objects through `internal/replaycli`, decodes zstd-framed protobuf envelopes,
  flattens JSON payload fields, and prints aggregate stats.
- It never prints raw object keys, provider device IDs, serials, or raw payloads.
- It accepts env values (`CONTROL_PLANE_DB_DSN`, `ARCHIVE_OBJECT_*`) or can load
  runtime values from Kubernetes with `--load-runtime`.
- Use `--max-objects` for a quick sampling pass, but do not treat sampled output
  as complete evidence.

## Reference

Read `references/archive-querying.md` when you need details on archive schema,
payload formats, presets, or output interpretation.
