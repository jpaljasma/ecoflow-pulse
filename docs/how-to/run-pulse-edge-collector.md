# Run Pulse Edge Collector on Raspberry Pi

Pulse Edge Collector is the local bridge for BLE telemetry. In v1 it supervises
the EcoFlow BLE probe, reads its structured JSONL output, and posts discovery
and telemetry batches to Pulse through `/api/v1/edge/*`.

## CI Artifact

Pull requests that touch the edge collector run a dedicated Raspberry Pi 5
check. It builds and uploads `pulse-edge-pi5-linux-arm64.tar.gz`, containing:

- `bin/pulse-edge-collector`
- `bin/ecoflow-ble-discover`
- `config/config.yaml`
- `systemd/pulse-edge-collector.service`
- `docs/run-pulse-edge-collector.md`

Build the same bundle locally with:

```bash
make pulse-edge-pi5-bundle
```

The binaries are built for `linux/arm64` with stripped symbols. The collector
also applies Raspberry Pi 5 friendly runtime defaults when the equivalent Go
runtime environment variables are not set:

- `GOMAXPROCS=4`
- `GOMEMLIMIT=512MiB`
- `GOGC=100`

The packaged `systemd` unit sets those values explicitly and also caps the
service at `MemoryMax=768M`, which is intentionally conservative for a
Raspberry Pi 5 with 8 GB RAM.

## Install

Copy the bundle to the Pi, then install the binaries, template config, and unit:

```bash
tar -xzf pulse-edge-pi5-linux-arm64.tar.gz
cd pulse-edge-pi5-linux-arm64

sudo install -m 0755 bin/pulse-edge-collector /usr/local/bin/pulse-edge-collector
sudo install -m 0755 bin/ecoflow-ble-discover /usr/local/bin/ecoflow-ble-discover
sudo install -d -m 0750 /etc/pulse-edge
sudo install -m 0640 config/config.yaml /etc/pulse-edge/config.yaml
sudo install -m 0644 systemd/pulse-edge-collector.service /etc/systemd/system/pulse-edge-collector.service
```

Edit `/etc/pulse-edge/config.yaml` for the target Pulse endpoint:

```yaml
profile: local
targets:
  local:
    base_url: https://localhost
  hosted:
    base_url: https://pulse.example.com
ble:
  discover_binary: /usr/local/bin/ecoflow-ble-discover
  raw_output_path: /run/pulse-edge/ecoflow-ble-raw.jsonl
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

Protect the secret file:

```bash
sudo chmod 0640 /etc/pulse-edge/secret.env
```

## systemd

Start the collector:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pulse-edge-collector
sudo systemctl status pulse-edge-collector
```

The collector exits cleanly on `SIGTERM`/`Ctrl+C`, signals the BLE probe, and
stops posting telemetry. The raw JSONL file is truncated on collector startup,
so restarts do not replay stale probe events. The Pi package keeps that file in
`/run/pulse-edge`, a tmpfs-backed runtime directory, to avoid unnecessary SD
card writes.

It keeps only live retry state in memory. If Pulse is unavailable, samples that
fail to post are dropped and the next probe refresh continues from live BLE
data. If the BLE probe exits unexpectedly, the collector restarts it with capped
exponential backoff. If BLE authentication fails, the collector exits with
status `10`; the packaged `systemd` unit treats that as a non-restartable
configuration failure.

## Update

For a new bundle, stop the service, replace the two binaries and the unit file,
then restart:

```bash
sudo systemctl stop pulse-edge-collector
sudo install -m 0755 bin/pulse-edge-collector /usr/local/bin/pulse-edge-collector
sudo install -m 0755 bin/ecoflow-ble-discover /usr/local/bin/ecoflow-ble-discover
sudo install -m 0644 systemd/pulse-edge-collector.service /etc/systemd/system/pulse-edge-collector.service
sudo systemctl daemon-reload
sudo systemctl start pulse-edge-collector
```
