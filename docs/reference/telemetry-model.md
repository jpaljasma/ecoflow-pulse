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
- status flags and mode pills
  (AC/DC/USB/12V/EV/fan/solar/preconditioning/X-Boost/solar-priority/
  passthrough/transfer/AC auto-on or always-on/energy management),
- device-detail diagnostics entries for model-specific troubleshooting states,
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

History and Energy API reads now treat persisted energy buckets as
authoritative. If `solar_generated_wh` or other explicit Wh columns are missing
for a rollup bucket, the read path leaves that field unset instead of deriving
Wh from average power. Historical repair must happen through the archive-to-
rollup rebuild path, not query-time synthesis.

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
  replay envelopes,
- non-EcoFlow provider payloads must be decoded into canonical params before
  entering the shared ingest publisher path. Pecron E1000LFP REST/MQTT `kv`
  payloads and Anker SOLIX Cloud MQTT payloads use the generic
  `provider.params.normalized` envelope payload type.

## Live Detail Derivation

For websocket device-detail rendering, the realtime gateway also derives a
small live-detail envelope from the merged raw snapshot:

- `detail.signals`:
  current AC/DC/USB/12V/EV/fan/solar/preconditioning booleans derived from the
  same merged live raw metrics used for projected watts.
- `detail.solarPorts`:
  current per-port PV state/volts/amps/watts.
  Canonical port IDs must support both of the current EcoFlow field families:
  - numbered MPPT families as `pv-1`, `pv-2`, ..., `pv-N`,
  - dual-MPPT low/high families as `pv-low` and `pv-high`.
  Pecron E1000LFP uses the same numbered shape; its generic cloud
  `dc_input_power` field is decoded at the Pecron boundary as `pv-1` live watts,
  with static product limits attached as device capability/detail metadata when
  those limits are not reported in MQTT or REST snapshots.
  Do not assume a fixed count of two PV ports in the live detail, history, or
  Energy API paths; supported devices may expose one, two, or more numbered PV
  inputs.

- energy PV-port history:
  persisted minute/hour/day PV-port rollup facts (`max_observed_*`,
  `last_observed_*`, `sample_count`) are the steady-state source for
  `EnergyService/GetEnergyPvPortHistory`; archive scans remain a fallback and
  rebuild source, not the intended hot read path.
  Numbered-port history rows must use canonical `pv-N` IDs so UI matching works
  across one-port, two-port, and future multi-port products even when older
  frames used alternate aliases such as `pv1`, `pv2`, `pv-low`, or `pv-high`.

Frontend rule:

- on `/device/{id}`, `System Signals` and `Solar Inputs` must prefer websocket
  `detail.*` when present and only fall back to REST `device.details.*` before
  the first live detail snapshot arrives.
- slower-changing device modes such as X-Boost, solar-priority/solar-only,
  passthrough/transfer mode, AC auto-on/always-on, and energy management are
  REST-backed `device.details.*` fields rendered alongside live booleans in the
  `System Signals` section.
- model-specific troubleshooting states are exposed as
  `device.details.diagnostics[]` and rendered under a collapsed `Diagnostics`
  panel so the primary device summary stays concise.

Frontend rule:

- solar history UI consumes `metrics.solarGeneratedWh` only; it does not derive
  solar energy from `pvAvgW`.
- solar history charts on `/devices` and `/device/{id}` overlay the previous
  day's bucket series as a thin dotted orange step line and show `Yesterday so
  far`, `Today so far`, and `Yesterday total` legend values in the chart corner.
- solar history chart buckets are absolute local time-of-day buckets anchored
  to the displayed window start. If a payload has fewer buckets than the
  displayed daylight window, pad missing buckets at the end rather than
  right-aligning the payload; otherwise hover labels and visible values can
  drift into future clock times.
- solar history legends suppress percentage deltas when the previous-day
  baseline is below `24 Wh`; in that case they show absolute change instead,
  and use `new activity today` when yesterday is zero.
- solar history view models reuse one fetched payload for today's total,
  yesterday's total, yesterday-through-the-current-elapsed-time, delta, and
  both chart series; the query is day-scoped, fetches the full previous local
  day for totals, and refreshes again just after local midnight so the
  comparison rolls forward automatically even on spring-forward and fall-back
  days.
- solar history compare bounds are computed in the client using local calendar
  day math and sent explicitly to the BFF; do not infer "yesterday" by
  subtracting the current elapsed duration on the server.
- solar history charts prefer weather-provided sunrise/sunset when a configured
  weather location/timezone is available, round those bounds to surrounding
  half-hour marks, render 10-minute buckets as step blocks, and expose
  per-bucket inspection with hover (web) and tap (native) using a
  crosshair/tooltip overlay for `Today` and `Yesterday`.
- `Energy Impact` on `/devices` and `/device/{id}` is derived from the same
  measured `todayWh` solar total; it currently estimates avoided `CO2e`, `NOx`,
  and `SO2` for "today so far" using default `NYUP` factors. Methodology and
  constants live in [`solar-avoided-emissions.md`](solar-avoided-emissions.md).
- `/energy` reuses the same impact methodology, but binds it to the active
  Energy dashboard scope/window so the card reflects the currently selected
  device/fleet range instead of exposing its own independent period controls.
- the Energy dashboard's `Today` window is a partial local-calendar day
  (`local midnight -> now`), and its previous comparison is yesterday over the
  same local clock span. The `Last 24h` preset is the rolling elapsed-time
  comparison (`now-24h -> now` against the preceding 24 hours).
- `date=YYYY-MM-DD` on the `Today` Energy preset turns the dashboard into a
  selected local-day view: past dates use the full local day, the current date
  remains partial through `now`, and the previous series follows the prior
  local-day window.
- `/api/v1/energy/calendar` exposes a Sunday-start visible month for fleet or
  canonical UUID device scope. It includes muted adjacent-month cells, per-day
  solar/value totals, `hasData`, future flags, and selected-month-only totals.
- Public Energy requests resolve local-calendar timezone from the current user
  profile rather than appending timezone query parameters. Calendar current-day
  cells are backed by live current-day rollups so today matches the homepage
  solar total. Non-current Calendar months are cacheable as complete
  user-visible month responses; the current profile-local month stays on the
  live path so today is not hidden behind a stale month cache.
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
