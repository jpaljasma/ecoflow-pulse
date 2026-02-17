# How-To: Add a Solar Panel to the Database

Add a new panel row to the source CSV and make it available in:

- runtime PV upgrade recommendations,
- panel DB candidate matrix,
- optional panel detection model training flow.

## Prerequisites

- Source CSV is available:
  - default: `/Users/jpaljasma/Downloads/solar_panel_specs_with_ecoflow_compat_cold_voc_and_safety_margins_v13.csv`
- Repo is up to date and tests pass.

## 1. Minimum Required Fields

Recommendation engine and detection engine have different minimum inputs.

### Recommendation Engine (panel DB) - hard minimum

These are the minimum fields needed for a panel to appear in recommendations:

- `Brand`
- `Model`
- `Pmax_STC_W` (> 0)
- `EcoFlow_compatibility` (must include valid device/port tags)

### Recommendation Engine (panel DB) - strongly recommended safety fields

These should be present so generated layouts are safe and accurate:

- `Voc_V` (cold-voltage safety / max-series cap)
- `Imp_A` or `Isc_A` (current-limit safety)
- `Vmp_V` (better MPPT-window/layout quality)
- `Type` and/or `Notes` (bifacial and efficiency inference quality)

### Detection Engine (`Detected` row) - separate requirements

Detection does not run directly from panel DB rows. It needs:

- telemetry samples in `logs/telemetry_training.csv`,
- panel hints (`panel_setup`, `panel_count`, `nominal_total_w`) by SN/product+port,
- retrained model artifact `data/solar_panels/panel_select_model.json`.

## 2. Add or Update a Panel Row in CSV

Recommended full column set for best recommendation quality:

- `Brand`
- `Model`
- `Type`
- `Pmax_STC_W`
- `Voc_V`
- `Vmp_V`
- `Imp_A`
- `Isc_A`
- `EcoFlow_compatibility`
- `Notes`
- `Purchase_link` (optional but recommended)

Compatibility format must include device tags in text form, for example:

- `D2/D2 Max 11–60V/15A: YES`
- `DPU Low 30–150V/15A: YES`
- `DPU-X High 80–500V/15A: needs ≥2S (max 8S)`

Notes:

- `module_efficiency_pct` is derived automatically:
  - from explicit columns when present,
  - then from `Notes` patterns (for example `Efficiency 25%`),
  - otherwise from conservative defaults by panel type:
    - bifacial: `22.5`
    - monofacial: `20.0`
- `purchase_link` is resolved in this order:
  - explicit CSV column (`Purchase_link`) when present,
  - curated override map `data/solar_panels/panel_purchase_links_v13.json`,
  - domain-aware derived fallback (official EcoFlow store search, or trusted solar vendors).

## 3. Regenerate Panel Artifacts

```bash
./scripts/regenerate_solar_panel_db.sh
```

This updates:

- `data/solar_panels/solar_panel_specs_v13.json`
- `data/solar_panels/solar_panel_specs_v13.summary.json`
- `data/solar_panels/solar_panel_specs_v13.index.json`
- `purchase_link` fields in both normalized and index artifacts.

## 4. Verify the New Panel Exists in Artifacts

Example checks:

```bash
jq '.records[] | select(.brand=="EcoFlow" and (.model | test("220W"; "i"))) | {id, brand, model, module_efficiency_pct, module_efficiency_source, ecoflow_compatibility_entries}' data/solar_panels/solar_panel_specs_v13.json
```

```bash
jq '.by_panel_key | to_entries[] | select(.value.brand=="EcoFlow" and (.value.model | test("220W"; "i"))) | {key: .key, tags: .value.compatibility_tags, eff: .value.module_efficiency_pct, eff_src: .value.module_efficiency_source}' data/solar_panels/solar_panel_specs_v13.index.json
```

To verify purchase links:

```bash
jq '.records[] | select(.brand=="EcoFlow" and (.model | test("220W"; "i"))) | {id, purchase_link, purchase_link_source}' data/solar_panels/solar_panel_specs_v13.json
```

```bash
jq '.by_panel_key | to_entries[] | select(.value.brand=="EcoFlow" and (.value.model | test("220W"; "i"))) | {key: .key, purchase_link: .value.purchase_link}' data/solar_panels/solar_panel_specs_v13.index.json
```

## 5. Verify Device Tag Mapping for Recommendations

Confirm your panel key is indexed under target device tag(s):

```bash
jq -r '.by_device_tag.d2_d2_max[]' data/solar_panels/solar_panel_specs_v13.index.json
jq -r '.by_device_tag.dpu_low[]' data/solar_panels/solar_panel_specs_v13.index.json
jq -r '.by_device_tag.dpu_high[]' data/solar_panels/solar_panel_specs_v13.index.json
```

If the panel key is not present, re-check `EcoFlow_compatibility` text formatting.

## 6. Run Validation

```bash
go test ./cmd/ecoflow-panel-db-import
go test ./cmd/ecoflow-mqtt-sub
go test ./...
make lint
```

## 7. Verify Runtime Recommendations

Run dashboard with candidate matrix enabled:

```bash
ECOFLOW_MQTT_SHOW_SOLAR_CANDIDATES=true make mqtt
```

Check in UI:

- `Solar Recommendations` table includes your panel in upgrade/add paths when safe.
- `Solar Candidate Matrix` lists your panel under the correct PV port.

## 8. Optional: Make Detection Model Recognize the New Panel Setup

This is only needed for the `Detected` row (panel identification), not for static DB recommendations.

1. Collect telemetry with the panel connected (`logs/telemetry_training.csv`).
2. Optionally provide panel labels via panel map JSON.
3. Train model:

```bash
./scripts/train_panel_select_model.sh
```

4. Re-run dashboard and verify `Detected` confidence/output improves.

## 9. Commit Required Files

Include in your branch:

- CSV source update (if tracked in your workflow),
- regenerated `data/solar_panels/*.json` artifacts,
- any model artifact updates if retrained (`data/solar_panels/panel_select_model.json`),
- relevant docs updates.
