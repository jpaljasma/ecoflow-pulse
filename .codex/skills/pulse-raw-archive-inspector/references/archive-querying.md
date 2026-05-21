# Archive Querying Reference

## Data Flow

- `archive_object_manifest` stores durable object references and aggregate
  metadata for archived telemetry frames.
- Archive objects are zstd-compressed varint-framed protobuf
  `TelemetryEnvelope` records.
- The inspector uses the manifest window first, then filters decoded envelopes by
  provider, selected canonical device UUIDs, payload type, and timestamp.
- Provider device IDs and archive object keys are used internally only and must
  not be copied into user-facing output.

## Payload Types

- `ecoflow.mqtt.raw`: raw EcoFlow MQTT payload bytes stored in the envelope.
- `ecoflow.quota.normalized`: normalized EcoFlow quota parameter payload.
- `provider.params.normalized`: normalized provider parameter payload used by
  Pecron and Anker Solix provider runners.

For Pecron investigations, "raw archive storage" usually means durable archive
objects containing normalized MQTT-derived parameter frames, not original Pecron
vendor JSON. Report that distinction if the user asks for raw vendor payloads.

## Useful Commands

Hosted cloud GCS, with DSN already exported:

```bash
go run .codex/skills/pulse-raw-archive-inspector/scripts/pulse_raw_archive_inspect.go \
  --preset cloud-gcs \
  --provider pecron \
  --family E1000LFP \
  --search fan,cool,heat \
  --hours 12
```

Load runtime values from a Kubernetes context:

```bash
go run .codex/skills/pulse-raw-archive-inspector/scripts/pulse_raw_archive_inspect.go \
  --load-runtime \
  --kube-context <context> \
  --preset cloud-gcs \
  --provider pecron \
  --family E1000LFP \
  --search fan,cool,heat \
  --hours 12
```

Bounded UTC incident window:

```bash
go run .codex/skills/pulse-raw-archive-inspector/scripts/pulse_raw_archive_inspect.go \
  --preset cloud-gcs \
  --provider pecron \
  --family E1000LFP \
  --from 2026-05-21T00:00:00Z \
  --to 2026-05-21T12:00:00Z \
  --search fan,cool,heat,ups,ac,dc,pv
```

## Output Interpretation

- `archive_objects_matched`: manifest objects selected for the provider/device
  filter and time range.
- `frames_decoded`: all envelope frames decoded from selected objects.
- `frames_matched`: frames that survived provider/device/time/payload filters.
- `payload_types`: the envelope payload families actually inspected.
- `search_terms payload_hits`: number of matched frames whose JSON payload text
  included any search term.
- `field_hits`: matching flattened field names and hit counts.
- `field[...] transitions`: number of observed value changes when that field was
  present in the inspected sequence.
- `nonzero`: numeric nonzero observations, useful for active signals such as
  AC/DC state, PV charging, fan level, or heater state.
- `matched_devices=0` with a `--family` filter means the family filter matched
  no canonical provider devices, so the script intentionally skipped provider-wide
  archive reads.

If `payload_hits=0` and `field_hits=(none)` for `fan,cool`, the inspected window
does not contain an explicit fan-like signal in the archived payloads. It may
still be possible to infer behavior from temperature, load, charge/discharge
state, or power fields, but label that as inference.

## Security Rules

- Do not print DSNs, access keys, secret keys, tokens, provider device IDs,
  canonical serials, MQTT topics, object keys, or physical locations.
- Prefer "matched_devices=1" or "provider family E1000LFP" over device-specific
  labels.
- If you must quote output, remove any sensitive accidental stderr from
  `kubectl`, `gcloud`, or object-store errors before sharing.
