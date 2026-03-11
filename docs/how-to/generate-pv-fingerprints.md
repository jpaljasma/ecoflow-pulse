# How-To: Generate PV Fingerprints

Use the PV fingerprint command to summarize solar telemetry by device and PV
port with features suitable for panel identification.

## Input

- telemetry CSV: `logs/telemetry_training.csv`
- optional panel hint map: JSON file (see schema below)

## Basic Usage

```bash
go run ./cmd/ecoflow-pv-fingerprint
```

Default output:

- `logs/pv_fingerprint.csv`

## Custom Input and Output

```bash
go run ./cmd/ecoflow-pv-fingerprint \
  -csv logs/telemetry_training.csv \
  -out logs/pv_fingerprint.csv
```

Write to stdout:

```bash
go run ./cmd/ecoflow-pv-fingerprint -out -
```

## Panel Hint Map (Optional)

Pass a JSON file to label known panel setups per device/port.

```bash
go run ./cmd/ecoflow-pv-fingerprint -panel-map panel_map.json
```

JSON schema:

```json
[
  {
    "device_sn": "DEMOD2M00001057",
    "product_name": "DELTA 2 Max",
    "port": "high",
    "panel_setup": "4x125W EcoFlow Bifacial Modular",
    "panel_count": 4,
    "nominal_total_w": 500
  }
]
```

Notes:

- `device_sn` is preferred for matching.
- `product_name` is used as fallback when `device_sn` does not match.
- `port` supports `low` or `high`.

## Output Columns

The output includes:

- identity: `sn`, `name`, `port`
- panel hint metadata: `panel_setup`, `panel_count`, `nominal_total_w`
- power features: `avg_w`, `median_w`, `avg_active_w`, `median_active_w`, `max_w`
- electrical features: voltage and current averages/medians/max
- capability comparison: `cap_w`, `max_w_util_pct`, `cap_v_range`, `cap_a`, `cap_headroom_w`
- state distribution: `(empty)`, `idle`, `charging` percentages
- window span and energy estimate: `duration_h`, `est_wh`
