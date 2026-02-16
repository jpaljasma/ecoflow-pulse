# Tutorial: Get Started Locally

This tutorial walks through a first local run of Ecoflow-Pulse with API checks
and MQTT telemetry.

## Outcome

By the end you will:

- run unit tests,
- run a smoke API check,
- launch the MQTT dashboard for a target device.

## Prerequisites

- Go installed (matching project `go.mod` toolchain/runtime support).
- EcoFlow API credentials.
- At least one online EcoFlow device on your account.

## 1. Configure Environment

Create `.env` in the repository root:

```bash
cat > .env <<'EOF'
ECOFLOW_ACCESS_KEY=your_access_key
ECOFLOW_SECRET_KEY=your_secret_key
ECOFLOW_ENV=prod
ECOFLOW_MQTT_SN=your_device_sn
EOF
```

## 2. Validate Build and Tests

```bash
go test ./...
```

Expected result: all tests pass.

## 3. Run API Smoke Check

```bash
go run ./cmd/ecoflow-smoke
```

Expected result: device list and API connectivity details print successfully.

## 4. Run MQTT Dashboard

```bash
make mqtt
```

Expected result: live dashboard updates with telemetry for the selected SN.

Controls:

- press `q` to quit gracefully,
- `Ctrl+C` also exits cleanly.

## 5. Review Logs

After running, inspect:

- `logs/mqtt.log` for raw and parsed telemetry,
- `logs/telemetry_history.jsonl` for minute buckets used by trends and ETA warm start.
