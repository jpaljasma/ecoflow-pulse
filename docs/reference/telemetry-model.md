# Reference: Telemetry Model

## MQTT Topic

Primary device-to-app telemetry topic:

- `/open/{certificateAccount}/{sn}/quota`

## Snapshot Domains

The dashboard snapshot aggregates telemetry into these domains:

- device summary (SOC, in/out/net, state, updated),
- meta guardrails (SOC window min/max and backup reserve when reported),
- channels (AC in, PV low/high/total, AC out, DC out, XT150 in/out),
- pack-level battery data (up to 5 packs for DPU),
- status flags (AC/DC/USB/EV/passthrough/grounded/fan/preconditioning),
- ETA and ML estimate outputs.
- solar panel recommendations (per PV port):
  - detected setup from runtime panel model,
  - runtime panel model uses adaptive prediction cadence per port:
    - low/collecting confidence: every 3rd sample,
    - medium confidence: every 5th sample,
    - high confidence: every 10th sample,
    while still ingesting every sample into tracker windows,
  - add-panels upsell recommendation (headroom to device PV limit),
  - if a port is undetected/idle while the peer PV port has a detected setup,
    add-panels can mirror peer per-panel sizing to local MPPT limits,
  - upgrade recommendation selected from all safe panel DB candidates (not only best/alt metadata),
  - cold-weather Voc safety is enforced when selecting panel candidates/series
    counts (conservative +20% Voc rise factor) to avoid unsafe MPPT voltage spikes,
  - upgrade suggestions include series/parallel layouts (`xSxP`) and enforce MPPT
    voltage/current limits,
  - recommendation ranking applies a stronger shoulder-hours uplift term to
    account for oversizing benefit under real solar curves (not only flat
    max-watt clipping),
  - recommendation ranking applies a complexity score so lower-complexity
    topologies (fewer panels, fewer parallel branches, and less mixed S+P wiring)
    are preferred when energy gain is marginal,
  - near PV-port saturation, clipped (overpaneled) topologies are preferred
    when outcomes are close, especially when they reduce panel/cable complexity,
  - recommendation ranking applies a modest panel-efficiency boost using
    `module_efficiency_pct`; inferred values (`estimated_*`) are down-weighted
    relative to `reported`/`notes`,
  - each panel DB candidate now carries `purchase_link` metadata (seeded from
    CSV, curated link map, or domain-aware fallback), and recommendation options
    retain the source link for downstream web/UI integrations,
  - EcoFlow `125W Bifacial Modular` layouts get a reduced complexity weight in
    ranking (1-4P setups are treated as easier to deploy),
  - if upgrade `#1` is clipping, upgrade `#2` is forced to a non-clipping
    alternative when one exists,
  - if a port has no viable primary recommendation but another port has a
    detected panel setup, recommendation `#1` falls back to that detected panel
    model and auto-sizes layout to the current MPPT limits,
  - alternates prefer panel-model diversity; same-model alternates are only used
    when clipping behavior materially changes,
  - detailed candidate matrix renders every safe panel candidate per PV input,
    including electrical data (Voc/Vmp/Imp/Isc, safe series range, best layout,
    complexity score, and expected clipped/non-clipped potential),
  - projected battery charge ETA impact (primary and second-best upgrade paths),
    using the same shoulder-hours uplift model as recommendation ranking (instead
    of flat clipped max-watt assumptions),
  - "Best Upgrade Path" summary row that picks the shortest resulting charge ETA
    across mixed per-port scenarios (`add`, `upgrade #1`, `upgrade #2`) and renders actionable steps:
    what to buy, how to install by PV input, and resulting ETA impact,
  - recommendation plan selection is cached and only recomputed when detected
    panel signatures change (setup/count/nominal), reducing UI render overhead.
  - conservative bifacial ETA adjustment (+15% ETA-effective PV watts when the
    detected/recommended panel is bifacial),
  - all-ports combined ETA impact summary rows when multiple PV ports are present.

## PV Derivation Precedence

For live snapshot/read-model derivation (`pulse-platform`, realtime gateway, and
rollup extraction), PV watts follow this precedence:

- DPU MPPT power fields first: `params.inLvMpptPwr + params.inHvMpptPwr` (or
  `param.powGetPvL + param.powGetPvH`),
- then D2M per-port watts: `params.pv1ChargeWatts + params.pv2ChargeWatts`,
- top-level `pvW` only as fallback when canonical fields are absent.

This preserves explicit zero PV states from canonical fields and prevents stale
top-level `pvW` values from showing false solar input in live trends/history.

History/query paths do not derive solar Wh from `pvAvgW`. If persisted energy
buckets are missing, the backend must repair them through archive-to-rollup
rebuilds rather than synthesizing Wh at read time.

Rollup writes are envelope-idempotent at the storage boundary. Replayed or
redelivered envelopes must not increment `sample_count`, `solar_generated_wh`,
or other persisted energy buckets more than once for the same canonical
envelope/message identity.

For `/api/devices` summaries, when per-port solar telemetry is explicitly
reported (including `0 W` on all ports), that per-port reading is authoritative
over top-level snapshot `pvW` to avoid stale carry-over in device cards.

## DC Derivation Precedence

For live snapshot/read-model derivation, DC watts remain a summed output bucket:

- explicit USB/USB-C/12V/24V/paralleling power fields are summed directly,
- DPU Anderson output prefers backend electrical telemetry
  `params.outAdsVol * params.outAdsAmp` when `params.outAdsPwr` is absent or
  spuriously `0`,
- `params.outAdsPwr` remains the fallback when backend Anderson volts/amps are
  not available.

This keeps DPU DC trends aligned with continuous backend Anderson telemetry
instead of sparse appshow-only power pulses.

## Live Metric Derivation

For live snapshot/read-model derivation (`pulse-platform`, realtime gateway, and
rollup extraction), the main projected watts metrics are derived as follows:

- `soc`:
  prefer aggregate display fields such as `params.f32LcdShowSoc`,
  `params.f32ShowSoc`, `params.cmsBattSoc`, then plain `params.soc`.
- `pvW`:
  use the PV precedence described above.
- `acW`:
  prefer explicit AC input channels `params.inAcC20Pwr + params.inAc5p8Pwr`,
  then `params.invInWatts`, then `params.wattsInSum - pvW` clamped at `0`.
- `dcW`:
  sum explicit DC-output channels:
  `carWatts`, `wireWatts`, USB, USB-C, and parallel-output fields.
  For DPU Anderson output, use backend `params.outAdsVol * params.outAdsAmp`
  when `params.outAdsPwr` is absent or spuriously `0`; otherwise use
  `params.outAdsPwr`.
- `loadW`:
  prefer `params.wattsOutSum`, then sum explicit output channels
  (`outAc*`, USB, USB-C, `outPrPwr`, `outAdsPwr`), then fall back to
  `params.invOutWatts`.
- `batteryW`:
  prefer direct battery fields
  `params.bmsInputWatts/inputWatts - params.bmsOutputWatts/outputWatts`,
  then `params.batAmp * params.batVol`, then fall back to `acW + pvW - loadW`.
- `solarGeneratedWh`:
  backend-owned authoritative history energy metric.
  Query/read paths populate it from persisted rollup/query results and may
  perform backend-side minute gap fill using persisted PV history so the UI
  never derives solar energy client-side.

General derivation rules:

- explicit zero values are preserved so stale positive readings do not linger,
- capped/canonical fields are preferred over obviously broken fallback values,
- derived channels are intended to be source-agnostic across MQTT, quota, and
  replay envelopes.

## Live Detail Derivation

For websocket device-detail rendering, the realtime gateway also derives a
small live-detail envelope from the merged raw snapshot:

- `detail.signals`:
  current AC/DC/USB/12V/EV/fan/solar/preconditioning booleans derived from the
  same merged live raw metrics used for projected watts.
- `detail.solarPorts`:
  current per-port PV state/volts/amps/watts for D2M and DPU-style port pairs.

- energy PV-port history:
  persisted minute/hour/day PV-port rollup facts (`max_observed_*`,
  `last_observed_*`, `sample_count`) are the steady-state source for
  `EnergyService/GetEnergyPvPortHistory`; archive scans remain a fallback and
  rebuild source, not the intended hot read path.

Frontend rule:

- on `/device/{id}`, `System Signals` and `Solar Inputs` must prefer websocket
  `detail.*` when present and only fall back to REST `device.details.*` before
  the first live detail snapshot arrives.

Frontend rule:

- solar history UI consumes `metrics.solarGeneratedWh` only; it does not derive
  solar energy from `pvAvgW`.
- solar history charts on `/devices` and `/device/{id}` overlay the previous
  day's bucket series as a thin dotted orange comparison line and show
  `Yesterday` / `Today` legend totals in the chart corner.
- solar history view models reuse one fetched payload for today's totals,
  yesterday's totals, delta, and both chart series; the query is day-scoped,
  compares against the full previous local day, and refreshes again just after
  local midnight so the comparison rolls forward automatically even on
  spring-forward and fall-back days.
- solar history compare bounds are computed in the client using local calendar
  day math and sent explicitly to the BFF; do not infer "yesterday" by
  subtracting the current elapsed duration on the server.
- solar history charts render `06:00` -> `20:00` local time in 10-minute
  buckets and expose per-bucket inspection with hover (web) and tap (native)
  using a crosshair/tooltip overlay for `Today` and `Yesterday`.
- `Energy Impact` on `/devices` and `/device/{id}` is derived from the same
  measured `todayWh` solar total; it currently estimates avoided `CO2e`, `NOx`,
  and `SO2` for "today so far" using default `NYUP` factors. Methodology and
  constants live in [`solar-avoided-emissions.md`](solar-avoided-emissions.md).
- `/energy` reuses the same impact methodology, but binds it to the active
  Energy dashboard scope/window so the card reflects the currently selected
  device/fleet range instead of exposing its own independent period controls.
- the same card also exposes a conservative mature-tree equivalent using PV
  lifecycle `CO2e` benchmark math. Methodology and constants live in
  [`tree-equivalent.md`](tree-equivalent.md).
- the same card also exposes an EV driving-energy equivalent using a premium-EV
  median consumption baseline derived from the bundled U.S.+Europe EV dataset.
  Source report lives in [`ev-us-europe-database-report.md`](ev-us-europe-database-report.md).
- `Energy Impact` exposes two periods:
  - `Today so far`: backed by the live-updating solar history path,
  - `Past 12 months`: day-rollup query loaded only on user selection and then
    cached client-side as a non-realtime view.
- power-trend UI seeds the initial 5-minute chart window from recent minute
  rollup history, then replaces the right-hand side with live websocket
  sparkline coverage as fresh samples arrive.
- raw MQTT logs are debugging aids only and are never a user-facing source of
  truth for history views.
- local rollup regeneration may replay raw MQTT log capture, but solar energy is
  always derived from canonical quota/MPPT PV updates using the same
  interval-based integration logic in both live rollup append and rebuild paths.
- rebuild paths must deduplicate envelopes by canonical envelope/message
  identity before bucket aggregation so archive retry duplicates cannot inflate
  regenerated rollups.

## Minute History Buckets

Minute buckets aggregate many samples into one row.

Stored metrics:

- SOC average (percent),
- solar generated (Wh/min),
- AC input (Wh/min),
- AC output (Wh/min),
- DC output (Wh/min),
- battery charge energy (Wh/min),
- total input and total output (Wh/min),
- net (Wh/min).

Notes:

- rollup tables may contain `sample_count = 0` rows for solar-only carry-forward
  buckets when canonical PV input spans a bucket without any direct point sample
  inside that bucket.

## Training Telemetry Capture

ML training data is persisted as CSV for offline model tuning.

- file: `logs/telemetry_training.csv`
- source: captured or curated telemetry exported for offline model work.

PV fingerprint features can be generated from training telemetry:

- command: `go run ./cmd/ecoflow-pv-fingerprint`
- output file: `logs/pv_fingerprint.csv`
- scope: per `device_sn + product_name + port(low/high)`
- includes median and max-based power/voltage/current features for panel modeling.

Panel selection model can be trained from telemetry:

- command: `go run ./cmd/ecoflow-panel-select-train`
- output file: `data/solar_panels/panel_select_model.json`
- replay mode reports per-port prediction accuracy and confidence.
- trainer-side panel hint inference uses an irradiance-aware fit (volts/amps +
  shoulder-hours weighting + MPPT clipping/safety constraints) to reduce
  low-sun misclassification into undersized panel classes.

## Raw MQTT Replay Log

Historical raw MQTT capture logs remain useful for replay/training pipelines:

- runtime files: `logs/mqtt_payload_raw-YYYY-MM-DD.log`
- payload format: timestamped `topic=... payload_raw=...` lines
- use them as benchmark/replay inputs rather than as an active runtime output contract.

## Supported Device Behaviors

Validated mappings and dashboard behavior are currently maintained for:

- DELTA 2 (D2)
- DELTA 2 Max (D2M)
- DELTA Pro Ultra (DPU)
