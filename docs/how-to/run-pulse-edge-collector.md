# Run Pulse Edge Collector on Raspberry Pi

Pulse Edge Collector is the local bridge for BLE telemetry. In v1 it supervises
the EcoFlow BLE probe and reads its structured JSONL output. Appliance installs
upload discovery and telemetry over direct loopback gRPC. Non-appliance edge
installs can keep using the REST `/api/v1/edge/*` transport.

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
bundle target also sets `GOARM64=v8.2` for the Raspberry Pi 5 Cortex-A76 CPU
profile and `CGO_ENABLED=0` for reproducible cross-builds. Override
`PULSE_EDGE_PI5_GOARM64`, `PULSE_EDGE_PI5_CGO_ENABLED`, or
`PULSE_EDGE_PI5_LDFLAGS` only when building for a different ARM64 target or
debugging symbols. At runtime, the collector applies Raspberry Pi 5 friendly
defaults when the equivalent Go runtime environment variables are not set:

- `GOMAXPROCS=4`
- `GOMEMLIMIT=512MiB`
- `GOGC=100`

The packaged appliance `systemd` unit tightens those values for the Pi 5
appliance profile and sends edge traffic to the K3s loopback gRPC service:

- `GOMAXPROCS=1`
- `GOMEMLIMIT=192MiB`
- `GOGC=100`
- `PULSE_EDGE_TRANSPORT=grpc`
- `PULSE_EDGE_GRPC_ADDR=127.0.0.1:19090`
- `PULSE_EDGE_STARTUP_WAIT=10m`
- `PULSE_EDGE_STARTUP_RETRY_DELAY=5s`
- `PULSE_EDGE_OUTBOX_DIR=/var/lib/pulse-edge/outbox`
- `PULSE_EDGE_OUTBOX_MAX_AGE=168h`
- `PULSE_EDGE_OUTBOX_MAX_BYTES=2GiB`
- `MemoryMax=256M`

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

Edit `/etc/pulse-edge/config.yaml` for the target Pulse endpoint. The
`base_url` value is used by the REST transport. Appliance gRPC mode ignores the
URL and uploads to `PULSE_EDGE_GRPC_ADDR`.

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

Use `PULSE_EDGE_PROFILE=hosted` to switch REST targets. Set
`PULSE_EDGE_TRANSPORT=rest` in `/etc/pulse-edge/secret.env` for hosted or other
non-appliance installs that do not expose the loopback gRPC service.

## Enroll

In Pulse, open Devices, choose **Find available devices**, and connect EcoFlow
BLE auth if the owner has devices that require authenticated BLE sessions. The
normal path asks for the EcoFlow app email/password once, derives the EcoFlow
BLE user ID on the Pulse backend, discards the password and temporary app token,
and stores only encrypted derived auth material. The manual BLE user ID fallback
is local setup/debug only and is blocked in the normal cloud-authenticated path.

Create a collector setup token through the same Devices flow, then exchange it
on the Pi:

```bash
tmp="$(mktemp)"
PULSE_EDGE_PROFILE=local pulse-edge-collector -enroll-token "$SETUP_TOKEN" > "$tmp"
printf 'PULSE_EDGE_PROFILE=local\n' >> "$tmp"
sudo install -o root -g root -m 0640 "$tmp" /etc/pulse-edge/secret.env
rm -f "$tmp"
```

Enrollment always prints the collector secret and prints `ECOFLOW_BLE_USER_ID`
only when BLE auth material is connected for the owner:

```bash
PULSE_EDGE_COLLECTOR_SECRET=...
ECOFLOW_BLE_USER_ID=...
PULSE_EDGE_PROFILE=local
```

## systemd

Start the collector:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pulse-edge-collector
sudo systemctl status pulse-edge-collector
```

The collector exits cleanly on `SIGTERM`/`Ctrl+C`, signals the BLE probe, and
stops uploading telemetry. The raw JSONL file is truncated on collector
startup, so restarts do not replay stale probe events. The Pi package keeps
that file in `/run/pulse-edge`, a tmpfs-backed runtime directory, to avoid
unnecessary SD card writes.

On appliance boot, `k3s.service` can become active before the in-cluster edge
gRPC API is reachable through the loopback hostPort. The packaged unit sets
`PULSE_EDGE_STARTUP_WAIT=10m`, so the collector retries the initial heartbeat
in-process before starting BLE instead of exiting repeatedly and hitting the
systemd start-limit window.

When `PULSE_EDGE_OUTBOX_DIR` is set, discovery and telemetry uploads are written
to a local JSON outbox and fsynced before send. Successful sends remove the
outbox entry; failed sends stay on disk and replay after collector restart and
after later successful heartbeats. Outbox files do not persist the collector
secret; the current secret is added only when sending. Telemetry samples carry a
stable `client_sample_id` for collector outbox identity, while the in-cluster
edge ingest service derives its own stable envelope `message_id` from normalized
sample content for downstream retry dedupe.

If the BLE probe exits unexpectedly, the collector restarts it with capped
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
