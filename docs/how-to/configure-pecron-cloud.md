# Configure Pecron Cloud

Pecron support is an unofficial, reverse-engineered cloud integration. V1 is
read-only and starts with the E1000LFP product key `p11vxg`.

## Supported Scope

- Providers: `pecron`
- Regions: `us`, `eu`, `cn`
- Device model: E1000LFP
- Product key: `p11vxg`
- Telemetry sources:
  - REST login, device discovery, product TSL lookup, and current attribute snapshots
  - MQTT over WebSocket live `kv` telemetry
- Deferred:
  - AC/DC toggles and other `batchControlDevice` writes
  - high-frequency reporting controls
  - LAN TCP and BLE

## Add a Pecron Connection

1. Open `Settings` -> `Integrations`.
2. Select `Pecron`.
3. Select the same cloud region used by the Pecron mobile app.
4. Enter the Pecron account email and password.
5. Save with `Validate and activate`.
6. Open the device onboarding panel, discover available devices, then use
   `Enable and Activate` for the E1000LFP. That action runs a live MQTT probe
   and atomically adds the active device only after telemetry is observed.

The email and password are stored as provider credential material. API clients
receive only the masked email plus non-secret config, such as `{"region":"us"}`.

## Runtime Behavior

Pecron REST and MQTT payloads are decoded at the provider boundary in
`pkg/pecron`. The ingest runner subscribes to Pecron MQTT topics, merges partial
`kv` packets, normalizes E1000LFP fields, and publishes canonical telemetry
envelopes through the same publisher path used by other providers.

The decoder also carries known cloud quirks from `jsight/unofficial-pecron-api`
and `attractify-logan/pecron-monitor`: REST `customizeTslInfo` values may arrive
under multiple value keys, `remain_time` and `remain_charging_time` can carry the
same value, and MQTT packets may be partial. Pulse keeps last-good voltage values
across placeholder zero-voltage packets and derives missing total input/output
watts from observed AC/DC components instead of inventing provider-specific
downstream logic.

Downstream projection, archive, rollup, realtime, and UI code should continue to
treat telemetry as provider-neutral canonical params. Pecron-specific logic
belongs in the provider adapter, Pecron decoder, or ingest session runner.

## Field Mapping

The E1000LFP mapping includes:

- battery SOC, voltage, current, temperature, and live pack power from MQTT or
  REST `kv` fields,
- capacity and pack-count metadata from the E1000LFP static product profile
  when the cloud payload does not report those limits directly,
- total input/output watts from observed provider fields,
- AC input/output watts plus AC output voltage/frequency/power factor,
- DC output watts,
- remaining charge/discharge time fields,
- UPS and AC/DC switch state as read-only params,
- PV/DC input ports with canonical numbered IDs such as `pv-1`; E1000LFP
  generic `dc_input_power` is normalized as `pv1ChargeWatts`, while
  product-page-only limits fill `pv_input_max_watts`, `pv_input_max_volts`, and
  `pv_input_max_amps`.

Port handling must stay cardinality-safe. Do not assume every provider device has
exactly two PV inputs.

## Rate-Limit Guidance

Use MQTT for steady-state live telemetry. REST snapshots are for login,
discovery, bootstrap, and periodic state refresh. Keep manual discovery
operator-triggered and avoid tight REST polling loops.

Pecron MQTT sessions must use the cloud-issued `qu_*` client ID unchanged. Do
not apply the EcoFlow MQTT client-ID namespace transform to Pecron sessions; the
Pecron broker rejects that mutated identity even when the same credential passes
discovery and smoke-test MQTT.

`attractify-logan/pecron-monitor` documents Pecron cloud `code 4026` as a
per-account daily polling budget of roughly 1280 polls/day. Pulse therefore uses
a 70s Pecron REST snapshot refresh default and rejects Pecron snapshot refresh
intervals below the documented 63s floor. If `4026` appears in logs, raise
`INGEST_PECRON_SNAPSHOT_REFRESH_INTERVAL` above 70s and prefer MQTT for live
updates.

Pulse does not enable Pecron `high_frequency_reporting` in V1. It is a write
control, and the same quirks log notes that leaving it enabled can also burn the
cloud polling budget.

## Troubleshooting

- If discovery fails, verify the selected region first.
- If discovery succeeds but `Enable and Activate` fails, confirm the device is
  online in the Pecron app and wait for the next live telemetry packet.
- If fields disappear or change shape, isolate fixes in `pkg/pecron` tests before
  touching projection, archive, rollups, realtime, or UI code.

## Reference Implementations Reviewed

- [`jsight/unofficial-pecron-api`](https://github.com/jsight/unofficial-pecron-api):
  REST authentication, device listing, current attributes, TSL parsing, region
  endpoints, and known remaining-time behavior.
- [`attractify-logan/pecron-monitor`](https://github.com/attractify-logan/pecron-monitor):
  cloud MQTT topic behavior, partial packet merge behavior, REST fallback, EU
  MQTT broker alias, and the documented Pecron cloud rate-limit quirks.
