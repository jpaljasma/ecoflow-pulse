# Run Pulse Edge Collector on Raspberry Pi

Pulse Edge Collector is the local bridge for BLE telemetry. In v1 it supervises
the EcoFlow BLE probe, reads its structured JSONL output, and posts discovery
and telemetry batches to Pulse through `/api/v1/edge/*`.

## Build

```bash
GOOS=linux GOARCH=arm64 go build -o bin/pulse-edge-collector ./cmd/pulse-edge-collector
GOOS=linux GOARCH=arm64 go build -o bin/ecoflow-ble-discover ./cmd/ecoflow-ble-discover
```

Copy both binaries to the Pi, for example under `/usr/local/bin`.

## Configure

Create `/etc/pulse-edge/config.yaml`:

```yaml
profile: local
targets:
  local:
    base_url: https://localhost
  hosted:
    base_url: https://pulse.example.com
ble:
  discover_binary: /usr/local/bin/ecoflow-ble-discover
  raw_output_path: /tmp/pulse-edge/ecoflow-ble-raw.jsonl
  args:
    - -duration=20s
    - -probe-timeout=11m
    - -listen-duration=0
    - -active-probe=auto
    - -ble-transport=rfcomm
```

Use `PULSE_EDGE_PROFILE=hosted` to switch targets.

## Enroll

Create a collector setup token through the Pulse edge collector API/UI, then
exchange it on the Pi:

```bash
PULSE_EDGE_PROFILE=local pulse-edge-collector -enroll-token "$SETUP_TOKEN"
```

Store the printed secret in `/etc/pulse-edge/secret.env`:

```bash
PULSE_EDGE_COLLECTOR_SECRET=...
PULSE_EDGE_PROFILE=local
```

## systemd

```ini
[Unit]
Description=Pulse Edge Collector
After=network-online.target bluetooth.target
Wants=network-online.target bluetooth.target

[Service]
EnvironmentFile=/etc/pulse-edge/secret.env
ExecStart=/usr/local/bin/pulse-edge-collector -config /etc/pulse-edge/config.yaml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

The collector exits cleanly on `SIGTERM`/`Ctrl+C`, signals the BLE probe, and
stops posting telemetry. The raw JSONL file is truncated on collector startup,
so restarts do not replay stale probe events. It keeps only live retry state in
memory; if Pulse is unavailable, samples that fail to post are dropped and the
next probe refresh continues from live BLE data.
