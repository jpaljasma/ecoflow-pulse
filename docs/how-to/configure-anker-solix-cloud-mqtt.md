# Configure Anker SOLIX Cloud MQTT

Anker SOLIX Cloud MQTT support is unofficial and reverse-engineered. V1 is
read-only from a user-control perspective and targets SOLIX portable power
stations plus home-battery systems that publish cloud MQTT telemetry.

I found no public official Anker cloud MQTT API. Anker's official Home Assistant
integration is local Modbus TCP, not cloud MQTT. Pulse's cloud behavior follows
the community [`thomluther/anker-solix-api`](https://github.com/thomluther/anker-solix-api)
and [`thomluther/ha-anker-solix`](https://github.com/thomluther/ha-anker-solix)
implementations.

## Supported Scope

- Provider: `anker_solix`
- UI label: `Anker SOLIX Cloud MQTT`
- Credential config: `{"server":"com","country":"US"}` by default
- Cloud servers: `com`, `eu`
- Account country: ISO-2 country code, such as `US` or `DE`
- Telemetry transport: MQTT over TLS on port `8883`
- REST use: login, discovery, and MQTT certificate bootstrap only
- Supported families:
  - SOLIX portable power stations with mapped MQTT telemetry
  - SOLIX Solarbank and home-battery systems
  - SOLIX F3800/E10/X1 home-backup systems where MQTT descriptors are mapped
- Out of scope:
  - standalone smart plugs and standalone meters
  - Prime chargers, alternator chargers, EV chargers, coolers
  - non-SOLIX Anker power banks

Discovered models that do not have a tested MQTT descriptor are shown as
unsupported or needing samples before enablement.

## Add an Anker SOLIX Connection

1. Open `Settings` -> `Integrations`.
2. Select `Anker SOLIX Cloud MQTT`.
3. Select the cloud server assigned to the Anker account.
4. Enter the two-letter account country.
5. Enter the Anker account email and password.
6. Save with `Validate and activate`.
7. Open the available-device panel, discover devices, test MQTT, and enable only
   supported SOLIX devices.

The email and password are provider credential material. API clients receive
only masked account metadata plus non-secret config, such as
`{"server":"com","country":"US"}`.

Use a dedicated Anker account for Pulse when possible. Recent community testing
suggests parallel tokens may work with current app/cloud behavior, but the cloud
API is unofficial and can drift without notice.

## Runtime Behavior

Pulse uses REST to authenticate, discover owned devices, and request the MQTT
connection material. Live telemetry comes from the Anker cloud MQTT broker over
TLS on port `8883`.

Some Anker SOLIX devices only publish useful MQTT telemetry after a status
request or realtime trigger. Pulse treats those MQTT publishes as telemetry
transport triggers, not user-facing control support. User-facing controls,
setpoints, schedules, and output toggles remain out of scope for V1.

Telemetry is decoded at the Anker provider boundary and converted into canonical
Pulse telemetry params before entering the shared ingest publisher path.
Projection, archive, rollup, realtime, and UI code should stay provider-neutral.

## Rate Guidance

Avoid periodic REST telemetry polling. Use REST for login, discovery, MQTT cert
bootstrap, and low-frequency metadata refresh only.

The community Anker tools default API endpoint throttling around 10 requests per
minute and warn that realtime MQTT triggers can create extra cloud traffic. Pulse
uses a balanced trigger cadence by default: a `300s` trigger timeout refreshed at
`270s`, with runtime overrides documented in configuration reference.

## Troubleshooting

- If login succeeds but discovery is empty, verify both cloud server and account
  country. Anker accounts can appear routed while returning no devices.
- If discovery works but MQTT validation does not, confirm the device is owned
  by the same account and online in the Anker app.
- If a model is shown as needing samples, collect MQTT fixtures before adding it
  to the supported normalization registry.
- If decoded values look wrong, fix the Anker decoder tests and provider
  normalization first. Do not add provider branches downstream.

## References Reviewed

- [`anker-charging/ha-anker-solix-official`](https://github.com/anker-charging/ha-anker-solix-official):
  official Anker Home Assistant integration using local Modbus TCP.
- [`thomluther/anker-solix-api`](https://github.com/thomluther/anker-solix-api):
  Anker cloud auth, device discovery, MQTT certificate bootstrap, MQTT maps, and
  monitor tooling.
- [`thomluther/ha-anker-solix`](https://github.com/thomluther/ha-anker-solix):
  Home Assistant cloud integration notes for MQTT behavior, trigger/status
  behavior, account ownership, and decode drift.
