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
6. Open the device onboarding panel, discover available devices, and enable the
   E1000LFP.

The email and password are stored as provider credential material. API clients
receive only the masked email plus non-secret config, such as `{"region":"us"}`.

## Runtime Behavior

Pecron REST and MQTT payloads are decoded at the provider boundary in
`pkg/pecron`. The ingest runner subscribes to Pecron MQTT topics, merges partial
`kv` packets, normalizes E1000LFP fields, and publishes canonical telemetry
envelopes through the same publisher path used by other providers.

Downstream projection, archive, rollup, realtime, and UI code should continue to
treat telemetry as provider-neutral canonical params. Pecron-specific logic
belongs in the provider adapter, Pecron decoder, or ingest session runner.

## Field Mapping

The E1000LFP mapping includes:

- battery SOC, voltage, current, temperature, and capacity metadata,
- total input/output watts,
- AC input/output watts plus AC output voltage/frequency/power factor,
- DC output watts,
- remaining charge/discharge time fields,
- UPS and AC/DC switch state as read-only params,
- PV/DC input ports with canonical numbered IDs such as `pv-1`.

Port handling must stay cardinality-safe. Do not assume every provider device has
exactly two PV inputs.

## Rate-Limit Guidance

Use MQTT for steady-state live telemetry. REST snapshots are for login,
discovery, bootstrap, and periodic state refresh. Keep manual discovery
operator-triggered and avoid tight REST polling loops because Pecron does not
publish a public rate-limit contract for these reverse-engineered endpoints.

## Troubleshooting

- If discovery fails, verify the selected region first.
- If discovery succeeds but MQTT validation fails, confirm the device is online
  in the Pecron app and wait for the next live telemetry packet.
- If fields disappear or change shape, isolate fixes in `pkg/pecron` tests before
  touching projection, archive, rollups, realtime, or UI code.
