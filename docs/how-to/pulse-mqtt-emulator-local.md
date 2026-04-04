# How To: Run the Pulse MQTT Emulator Locally

This guide describes the local-only Pulse MQTT emulator provider that mimics the
EcoFlow signed REST + MQTT handshake used by the current ingest stack.

## What the emulator implements

- Signed REST discovery at `/iot-open/sign/device/list`
- Signed REST MQTT certification at `/iot-open/sign/certification`
- Signed REST quota bootstrap at `/iot-open/sign/device/quota/all`
- MQTT 3.1.1 over TLS with username/password from the certification payload
- EcoFlow-style topic routing: `/open/<certificateAccount>/<UPPERCASE_SN>/quota`
- DPU-style JSON frames for a seeded `DELTA Pro Ultra X` device with `4` battery
  packs (`24.576 kWh`, `6.144 kWh` per pack)
- Default DPU-X telemetry profile with a high-voltage PV string modeled after
  `7 x 590 W` modules in series on `PV High`, steady AC household demand
  (`450 W`-`1.2 kW` scenes), and a small auxiliary DC load (`5 W`-`15 W`)
- DPU-X solar telemetry follows a local time-of-day curve (sunrise ramp, midday
  plateau with cloud variation, sunset taper) instead of a short repeating
  spike loop
- Battery charge/discharge is derived from live PV minus load, with per-pack
  power spread kept near-even and slightly SOC-biased to mimic managed pack
  balancing
- The DPU-X provider path exposes both PV inputs at the Ultra X envelope:
  `5000 W / 500 V / 15 A` each
- The DPU-X capacity model follows the EcoFlow X-series per-inverter ceiling:
  up to `10` smart extra batteries (`61.44 kWh` total)
- Short pack/system preconditioning bursts are emitted in `30`-`45` second
  windows so battery-heating signals can be validated end to end

## Local cluster workflow

Deploy the updated services chart:

```bash
make services-image-build-local
make services-image-import-local
make dev-deploy
```

The local services chart enables the in-cluster emulator Deployment by default.

## Seed a local emulator credential + device

1. Find the real Keycloak subject for the signed-in local user.

Example query against the local CNPG primary:

```bash
kubectl -n pulse-platform exec pulse-platform-core-1 -- \
  psql "postgres://pulse:pulse-local-dev-password@pulse-platform-core-rw.pulse-platform.svc.cluster.local:5432/pulse?sslmode=disable" \
  -At -F '|' \
  -c "select keycloak_subject,email from users where lower(email)=lower('<your-email>') order by updated_at desc;"
```

2. Seed the emulator-backed provider credential and DPU-X device against that
   subject.

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse:pulse-local-dev-password@127.0.0.1:15432/pulse?sslmode=disable' \
ECOFLOW_DEV_PROVIDER='pulsemqtt' \
ECOFLOW_DEV_ACCESS_KEY='pulse-mqtt-local-ak' \
ECOFLOW_DEV_SECRET_KEY='pulse-mqtt-local-sk' \
ECOFLOW_DEV_USER_SUBJECT='<real-keycloak-subject>' \
ECOFLOW_DEV_USER_EMAIL='<your-email>' \
ECOFLOW_DEV_SEED_SNS='PULSEDPUX24K001' \
go run ./cmd/ecoflow-dev-seed
```

Notes:

- The local chart defaults the emulator REST/MQTT credentials to
  `pulse-mqtt-local-ak` / `pulse-mqtt-local-sk`.
- The seeded device name is `DPU-X 24 kWh`.
- The seeded model is `DELTA Pro Ultra X`.

## Validate the provider end to end

1. Confirm the emulator pod is healthy:

```bash
kubectl -n pulse-services get pods -l app.kubernetes.io/component=pulse-mqtt-emulator
kubectl -n pulse-services get svc pulse-services-pulse-mqtt-emulator
```

2. Confirm the grpc-api and ingest worker are running the updated chart:

```bash
kubectl -n pulse-services rollout status deploy/pulse-services-go-grpc-api
kubectl -n pulse-services rollout status deploy/pulse-services-go-ingest
```

3. Validate that the provider device is present and probeable through the
   control-plane path.

Suggested checks:

- `GET /api/v1/integrations?provider=pulsemqtt`
- `GET /api/v1/devices/available`
- `POST /api/v1/devices/available/test-mqtt`

4. Watch the ingest worker logs for the emulator-backed device:

```bash
kubectl -n pulse-services logs deploy/pulse-services-go-ingest --since=10m | rg 'pulsemqtt|PULSEDPUX24K001'
```

Successful end-to-end behavior looks like:

- discovery succeeds with the `pulsemqtt` provider
- MQTT probe receives a quota-frame sample on the `/open/.../quota` topic
- ingest logs show a connected session for `provider=pulsemqtt`
- the seeded `DELTA Pro Ultra X` device appears in the product under the target user

## Replay a bounded MQTT history window

The emulator exposes a local-only replay endpoint that republishes historical
quota frames over the same MQTT topic used by the live ingest session.

Example:

```bash
curl -X POST \
  "http://127.0.0.1:18080/replay?from=2026-04-03T00:00:00%2B03:00&to=2026-04-03T21:30:00%2B03:00&step=1m"
```

Notes:

- the replayed MQTT payloads now include stable `id` and `time` fields
- `pulsemqtt` ingest preserves the payload `time` as the envelope
  `observed_time_unix_ms`, while keeping `ingested_time_unix_ms` at the actual
  receive time
- repeated replays of the same emulator payload keep the same deterministic
  envelope id, so downstream archive/rollup dedup paths can treat them as the
  same logical sample

## Repair a bad history window safely

For already-written flat or incorrect minute buckets, use the bounded backfill
CLI instead of patching SQL rows directly. It replays the emulator's own MQTT
frames through the live `pulsemqtt` ingest session, waits for those historical
envelopes to land in the raw archive, and then runs the standard archive-to-
rollup rebuild flow for the requested device window.

If you are running the command from your laptop, forward the emulator and MinIO
first:

```bash
kubectl --context k3d-pulse-local -n pulse-services port-forward svc/pulse-services-pulse-mqtt-emulator 18080:8080
kubectl --context k3d-pulse-local -n pulse-platform port-forward svc/pulse-platform-minio 9000:9000
```

```bash
CONTROL_PLANE_DB_DSN='postgres://pulse:pulse-local-dev-password@127.0.0.1:15432/pulse?sslmode=disable' \
go run ./cmd/pulse-mqtt-history-backfill \
  -provider pulsemqtt \
  -provider-device-id PULSEDPUX24K001 \
  -emulator-url 'http://127.0.0.1:18080' \
  -from '2026-04-03T00:00:00+03:00' \
  -to '2026-04-03T21:30:00+03:00'
```

The command fails closed if replay publishes zero MQTT frames or if the
historical envelopes do not appear in the archive before rebuild.

When replay-only rollup windows still lack explicit `*_energy_wh` buckets, the
platform history/dashboard read path derives bucket energy from the stored
bucket-average power and bucket duration so solar/energy charts stay truthful
without mutating archived raw frames or patching rollup SQL rows manually.
