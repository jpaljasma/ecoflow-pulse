# Config 06 - Raspberry Pi 5 Appliance

This config defines the local appliance target for running Pulse on a Raspberry
Pi 5 instead of hosted Google Cloud runtime. It is intentionally conservative:
single-node durability, low operational overhead, and clear failure behavior
matter more than preserving the hosted HA topology.

## Target Hardware

- Raspberry Pi 5, 8GB RAM.
- Official Raspberry Pi 27W USB-C power supply or an equivalent 5V/5A supply.
- Argon NEO 5 M.2 NVMe PCIe case.
- SanDisk Optimus GX 7100 500GB M.2 2280 NVMe SSD.
- Active cooling and NVMe thermal pad/heatsink installed.
- Ethernet preferred for appliance installs.

The 500GB decimal SSD is about 465GiB usable. Keep steady-state Pulse allocation
below 250GiB and warn when free space drops below 80GiB.

## Host Install

Use a temporary microSD bootstrap once, then boot from NVMe.

1. Flash the final NVMe with Raspberry Pi Imager:
   - device: Raspberry Pi 5
   - OS: Raspberry Pi OS Lite 64-bit
   - hostname: `pulse`
   - SSH: public-key auth enabled
   - network: Ethernet preferred
   - user/timezone: install-specific
2. Ensure the NVMe boot partition has this in `/boot/firmware/config.txt`:

   ```ini
   dtparam=pciex1
   # Appliance default: do not set dtparam=pciex1_gen=3.
   ```

3. From the temporary microSD boot, update EEPROM and set:

   ```ini
   BOOT_ORDER=0xf416
   PCIE_PROBE=1
   ```

4. Assemble the Argon case, remove the microSD card, and boot from NVMe.

5. Install baseline packages on the NVMe system:

   ```bash
   sudo apt update
   sudo apt full-upgrade -y
   sudo apt install -y ca-certificates curl gnupg jq unzip openssl \
     pciutils nvme-cli smartmontools bluez bluetooth rfkill \
     avahi-daemon chrony zram-tools
   sudo systemctl enable --now bluetooth avahi-daemon chrony fstrim.timer
   sudo rfkill unblock bluetooth
   ```

### First Hardware Bring-Up Learnings

The first real Pi 5 appliance install proved these checks save time:

- Prefer wired Ethernet for first boot. WiFi or stale local DNS can make SSH
  failures look like boot failures.
- If SSH resets or closes before authentication, confirm whether the Pi has
  reached the local login prompt before re-imaging. A micro-HDMI display is the
  fastest way to distinguish boot success from SSH/customisation failure.
- `Permission denied (publickey)` means the network and SSH daemon are alive;
  fix the authorised key or re-image with the exact public key. It is different
  from `kex_exchange_identification` resets, which happen before
  authentication.
- Give a fresh image 5-10 minutes on first boot before power-cycling. First
  boot may resize the filesystem, generate SSH host keys, apply Imager
  customisation, and settle time sync.
- When using an Argon/non-HAT NVMe carrier, keep `/boot/firmware/config.txt`
  explicit with `dtparam=pciex1` and keep PCIe Gen 2 as the appliance default.
- PCIe Gen 3 can be a useful lab experiment on the Argon NEO 5 and SanDisk
  Optimus GX 7100 path. On the first appliance hardware, adding
  `dtparam=pciex1_gen=3` changed `sudo hdparm -t /dev/nvme0n1` from about
  `449 MB/sec` to about `882 MB/sec`. Keep Gen 2 as the shipped default until
  Gen 3 also passes reboot, thermal, SMART, and unsafe-shutdown checks on the
  target appliance.
- `PCIE_PROBE=1` should be present in EEPROM config. A boot order that already
  boots NVMe is acceptable if NVMe root is mounted and EEPROM is current.
- Treat `dphys-swapfile.service does not exist` as harmless on current
  Raspberry Pi OS images when `swapon --show` confirms zram-only swap.
- K3s needs memory cgroups enabled on Raspberry Pi OS. If install logs report
  `Failed to find memory cgroup`, append `cgroup_memory=1
  cgroup_enable=memory` to `/boot/firmware/cmdline.txt`, keeping the file as a
  single line, then reboot.
- Install host iptables tools before or immediately after K3s if the installer
  reports missing `iptables-save` or `iptables-restore`.
- A fresh Pi clone needs Helm repository definitions before vendoring platform
  chart dependencies. Add the NATS, CNPG, Bitnami, MinIO, ingress-nginx,
  jetstack, external-secrets, prometheus-community, and OpenTelemetry Helm
  repos, then run `helm repo update` before `make chart-deps-local
  CHART=deploy/charts/pulse-platform`.
- The Pi can download its own GitHub Actions release artifact after installing
  GitHub CLI from the official Debian/Raspberry Pi APT repository. Use
  `gh auth login --hostname github.com --web` on the headless Pi, complete the
  printed device-code flow in another browser, then use `gh run download` for
  the `pulse-pi-release-values` artifact.

## Host Tuning

Use ext4 with default journaling and no continuous discard. Enable weekly
`fstrim.timer` instead.

Root filesystem options should be merged into the existing root entry while
preserving the install-specific device or UUID. Example root entry:

```fstab
UUID=<root-filesystem-uuid> / ext4 defaults,noatime,errors=remount-ro 0 1
```

Phase 1 host scripts must edit only the options column for the existing `/`
mount and must not replace the device identifier, filesystem type, dump field,
or fsck order with placeholder values.

Disable disk swap and use zram:

```bash
sudo systemctl disable --now dphys-swapfile || true
```

`/etc/default/zramswap`:

```ini
ALGO=zstd
PERCENT=25
PRIORITY=100
```

`/etc/sysctl.d/90-pulse-appliance.conf`:

```ini
vm.swappiness=10
vm.dirty_background_ratio=5
vm.dirty_ratio=10
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=1024
```

`/etc/systemd/journald.conf.d/90-pulse.conf`:

```ini
[Journal]
Storage=persistent
SystemMaxUse=512M
RuntimeMaxUse=128M
SystemKeepFree=2G
MaxRetentionSec=14day
Compress=yes
```

### Optional SDRAM Tuning

The appliance default stays on stable Raspberry Pi OS firmware delivered by
APT and `rpi-eeprom-update`. Do not require `sudo rpi-update` in shipped
appliance setup: Raspberry Pi documents it as a pre-release firmware/kernel
path intended for testing, development, or specific bug fixes.

For one-off lab benchmarking on a locally recoverable Pi 5, the current SDRAM
timing experiment can be tested manually:

```bash
sudo apt update
sudo apt full-upgrade -y
sudo rpi-update
sudo rpi-eeprom-config --edit
```

Add this EEPROM setting:

```ini
SDRAM_BANKLOW=1
```

Then reboot and re-run the acceptance checks below. If boot, thermal, memory,
or K3s stability regresses, roll back to supported firmware:

```bash
sudo apt update
sudo apt install --reinstall raspi-firmware
sudo reboot
```

Do not combine this with appliance defaults that are already intentionally
conservative, such as PCIe Gen 2. PCIe Gen 3, CPU overclocking, and SDRAM
experiments are lab-only unless they survive the full reboot and burn-in suite
on the target hardware.

### Optional CPU Overclocking

The appliance default CPU clock remains the Raspberry Pi 5 stock `2.4GHz`.
Overclocking can improve bursty compile or local maintenance work, but it is
not a shipped appliance default because each board, case, PSU, ambient
temperature, and workload mix has different stability margins.

The first tested conservative candidate for this appliance hardware is
`arm_freq=2500` with `over_voltage_delta=10000`; it survived stress testing
with no throttling on the target Pi 5, Argon NEO 5, and NVMe assembly.

To test it, edit `/boot/firmware/config.txt`:

```bash
sudo nano /boot/firmware/config.txt
```

Add:

```ini
# Optional Pi 5 lab overclock; not an appliance default.
arm_freq=2500
over_voltage_delta=10000
```

Then reboot and verify:

```bash
sudo reboot
vcgencmd measure_clock arm
vcgencmd measure_temp
vcgencmd get_throttled
```

Acceptance for an appliance candidate:

- `vcgencmd get_throttled` remains `throttled=0x0` after boot, K3s startup,
  and sustained load.
- CPU temperature stays below throttling range with the Argon case assembled,
  NVMe thermal pad installed, and the appliance located where it will actually
  run.
- 10 reboot cycles, BLE startup, K3s convergence, and the capacity burn-in pass
  without crashes, filesystem errors, data loss, or clock/thermal throttling.

More aggressive lab experiments, such as `arm_freq=2800` with
`over_voltage_delta=25000`, should stay behind the same acceptance checks and
should not replace the conservative candidate unless they survive sustained
stress on the target appliance hardware.

If the Pi fails to boot or shows instability, remove the two overclock lines
from `/boot/firmware/config.txt` using the microSD/NVMe boot partition from
another machine, or from a working recovery shell, then reboot. Do not set
`force_turbo=1` for appliance mode; keep DVFS enabled so idle power and heat
stay low.

References:

- [Raspberry Pi OS update docs](https://www.raspberrypi.com/documentation/computers/os.html):
  use APT for routine stable firmware/kernel updates; reserve `rpi-update` for
  pre-release testing or when Raspberry Pi engineers instruct it.
- [Raspberry Pi config.txt docs](https://www.raspberrypi.com/documentation/computers/config_txt.html):
  define `arm_freq`, `over_voltage_delta`, `force_turbo`, throttling/clock
  inspection, and overclock recovery guidance.
- [Jeff Geerling's Pi 5 SDRAM tuning note](https://www.jeffgeerling.com/blog/2024/raspberry-pi-boosts-pi-5-performance-sdram-tuning/):
  reports the `SDRAM_BANKLOW=1` test path and observed benchmark gains, while
  noting the tweak may become default in future firmware.
- [Raspberry.tips Pi 5 overclocking guide](https://raspberry.tips/en/raspberrypi-tutorials/overclock-raspberry-pi-5):
  recommends starting around `arm_freq=2800` with `over_voltage_delta=25000`
  and validating with clock, temperature, and throttling checks.

Pre-install validation:

```bash
vcgencmd get_throttled
vcgencmd measure_temp
lsblk -o NAME,MODEL,SIZE,FSTYPE,MOUNTPOINTS
sudo nvme smart-log /dev/nvme0n1
sudo lspci -vv | grep -A20 -i "Non-Volatile"
```

Acceptance: no throttling, NVMe root mounted, PCIe Gen 2 x1 link, stable SSD
temperature, and no unsafe-shutdown count growth during normal reboots.

## K3s Runtime

K3s is installed after host tuning. The appliance owns
`/etc/rancher/k3s/config.yaml`:

```yaml
write-kubeconfig-mode: "0644"
disable:
  - traefik
  - servicelb
node-name: pulse-pi5
node-label:
  - pulse.appliance/local=true
kubelet-arg:
  - "system-reserved=cpu=500m,memory=1Gi,ephemeral-storage=12Gi"
  - "kube-reserved=cpu=300m,memory=512Mi,ephemeral-storage=8Gi"
  - "eviction-hard=memory.available<500Mi,nodefs.available<10%,imagefs.available<10%"
  - "eviction-minimum-reclaim=memory.available=256Mi,nodefs.available=2Gi,imagefs.available=2Gi"
  - "image-gc-high-threshold=70"
  - "image-gc-low-threshold=55"
  - "container-log-max-size=10Mi"
  - "container-log-max-files=3"
```

The appliance bundle includes pinned GHCR image digests, `deploy/env/pi/`
values, host tuning scripts, BLE binaries, and status/upgrade commands.
The BLE host binaries are built with the Pi 5 bundle target for `linux/arm64`,
`GOARM64=v8.2`, `CGO_ENABLED=0`, `-trimpath`, and stripped symbols by default.

Phase 1 implementation files:

- `deploy/appliance/pi5/pulse-appliance-host-prepare.sh`
- `deploy/appliance/pi5/pulse-appliance-install.sh`
- `deploy/appliance/pi5/pulse-appliance-status.sh`
- `deploy/appliance/pi5/k3s-config.yaml`
- `deploy/env/pi/values.platform.yaml`
- `deploy/env/pi/values.services.yaml`

Installer commands:

```bash
make appliance-pi-install
make appliance-pi-upgrade
make appliance-pi-wait
make appliance-pi-status
```

Pass install-specific flags with `APPLIANCE_PI_INSTALL_ARGS`, for example
`APPLIANCE_PI_INSTALL_ARGS="--skip-k3s-install"`. Use
`--release-values /etc/pulse-appliance/release.yaml` or
`PULSE_APPLIANCE_RELEASE_VALUES` for install-local image digests and any other
release inputs that must not be committed. Use
`PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES` or `--platform-extra-values` for
install-local platform overrides such as social-login credentials. The
installer defaults `kubectl`, Helm, and status checks to
`/etc/rancher/k3s/k3s.yaml`; override with `--kubeconfig` or
`PULSE_APPLIANCE_KUBECONFIG` only when using a different client config.

The installer runs host preparation, installs or upgrades K3s, builds Helm
chart dependencies, rejects rendered `pi-placeholder` images, applies the
platform chart with a Keycloak bootstrap pass, checks the install-specific
services runtime secret, applies the services chart, and waits for the
appliance workloads. Services fail closed when
`pulse-services/pulse-services-runtime-secret` is absent so GCS credentials and
provider material stay install-specific.

Published appliance container images are produced by the `Pi Appliance Images`
GitHub Actions workflow. It runs automatically on pushes to `main`, pushes
`linux/arm64` services, public app, and realtime gateway images to GHCR, and
uploads a `pulse-pi-release-values` artifact containing immutable image digests
for `--release-values /etc/pulse-appliance/release.yaml`. The workflow can also
be started manually for reruns, custom image tags, or private GHCR packages. If
GHCR packages are private, run the workflow manually with an `image_pull_secret`
input and create the same docker-registry secret in both the `pulse-platform`
and `pulse-services` namespaces before applying the release.

Pi image builds should keep CPU-heavy compile/export work on the native GitHub
runner architecture and reserve arm64 execution for target-runtime install
steps that truly need target binaries. A June 2026 Docker build record for
`pulse-platform` showed only 1 of 38 steps cached and a 5,228 second build, with
4,860 seconds spent in the Expo web export step while building the arm64 image.
The Pi workflow therefore uses best-effort persistent BuildKit `type=gha` cache
scopes for each image, and Node image build stages use `$BUILDPLATFORM` while
final stages still emit `linux/arm64` runtime images. Cache export failures must
not block release artifact upload after images have built and pushed.

Use `docs/how-to/run-pi5-appliance-capacity-burn-in.md` for the current
operator sequence to install `gh` on Raspberry Pi OS, authenticate a headless
Pi, download the artifact by workflow run ID, and install
`/etc/pulse-appliance/release.yaml`.

Keep `/etc/pulse-appliance/release.yaml` readable by the appliance operator and
not world-readable. A practical default is `0640 root:<operator-group>` with
`/etc/pulse-appliance` mode `0755`; a root-only `0600` file blocks the
non-root Make/Helm wrapper from reading install-local release values.

Keep optional install-specific platform settings in
`/etc/pulse-appliance/platform-extra.yaml`. The standard `make deploy-pi` target
includes that file automatically when it exists, which lets the appliance keep
Google social-login settings across future upgrades without committing OAuth
client credentials. The Google broker requires:

```yaml
keycloakRealm:
  google:
    enabled: true
    clientId: "<google-client-id>"
    clientSecret: "<google-client-secret>"
```

The matching Google OAuth web client must allow:

```text
https://pulse.home.arpa/realms/pulse/broker/google/endpoint
```

Create those local secrets after the platform database is ready:

```bash
APPLIANCE_PI_RUNTIME_SECRET_ARGS="--gcs-credentials /path/to/gcs-service-account.json --gcs-project-id <gcs-project-id> --archive-writer-id pulse-pi5" \
  make appliance-pi-create-runtime-secret
```

When `runtime.gcsCredentials.enabled=true` in the install-local release values,
the services chart mounts `pulse-services-gcs-credentials` at
`/var/run/pulse-gcs` and the runtime secret points
`GOOGLE_APPLICATION_CREDENTIALS` at the mounted JSON file. If the local release
values change `runtime.gcsCredentials.secretKey`, `fileName`, or `mountPath`,
pass the matching `--gcs-secret-key`, `--gcs-file-name`, and
`--gcs-mount-path` flags to `make appliance-pi-create-runtime-secret`.

Validation command:

```bash
make appliance-pi-validate
```

For full NVMe SMART validation, run the status check with privileges:
`sudo env KUBECONFIG="$KUBECONFIG" make appliance-pi-status`. Non-root status
still checks K3s, Helm, loopback gRPC, and archive outbox health, but reports a
warning when the kernel requires root for `nvme smart-log`.

## Appliance Workload Defaults

- Postgres/CNPG: singleton, Timescale enabled, `max_connections=40`,
  `shared_buffers=256MB`, `work_mem=4MB`, WAL compression on, 64Gi PVC cap.
- NATS: singleton JetStream, file storage, stream replicas `1`, max age `24h`,
  max bytes `8Gi`, 16Gi PVC cap. The Pi overlay disables the upstream
  `nats-box` toolbox Deployment; the NATS StatefulSet still keeps its small
  config reloader sidecar.
- Valkey: singleton with AOF, memory cap 256-384Mi, 2Gi PVC cap.
- Keycloak: singleton, memory limit 768Mi, local username/password auth
  required, social identity providers optional.
- Pulse core: merged Go runtime only where it reduces overhead without
  weakening graceful shutdown or idempotency.
- Public web and realtime gateway: separate singleton workloads unless
  measurement proves a safe merge.
- Observability lite: optional, 24h retention, no OpenTelemetry collector by
  default.

Before a merged `pulse-core` binary lands, the Pi services overlay keeps the
singleton deployments separate and applies conservative per-workload Go runtime
caps:

| Workload | `GOMAXPROCS` | `GOMEMLIMIT` |
|---|---:|---:|
| `go-ingest` | `1` | `384MiB` |
| `go-inference` | `1` | `160MiB` |
| `go-projection` | `1` | `256MiB` |
| `go-rollup` | `1` | `256MiB` |
| `go-archive` | `1` | `256MiB` |
| `go-grpc-api` | `2` | `512MiB` |
| `go-energy-api` | `1` | `384MiB` |
| `go-scheduler` | `1` | `160MiB` |

The Pi `go-grpc-api` deployment binds `127.0.0.1:19090` with a Kubernetes
`hostPort` so the host BLE collector can write directly to the in-cluster API.
Because a single node cannot schedule two pods that both claim the same
hostPort, the Pi overlay sets that deployment to `maxSurge: 0` and
`maxUnavailable: 1`. Upgrades may briefly interrupt the loopback gRPC endpoint;
the host collector waits for startup and uses its durable outbox to retry.

Run
[`run-pi5-appliance-capacity-burn-in.md`](../how-to/run-pi5-appliance-capacity-burn-in.md)
on the real appliance before merging more roles into one process.

Target steady RSS is under 4.8GiB, leaving at least 1GiB for the host.

## Network And Auth

Default appliance URL:

```text
https://pulse.home.arpa
```

Plan B is a real domain with split-horizon/private DNS. The domain resolves
only locally and does not imply public ingress.

Keycloak remains authoritative. Social login can be configured per install, but
local username/password auth must be complete and tested because arbitrary local
appliance domains are not suitable for one shared social OAuth client.

## BLE And Local Ingest

BLE runs on the host through systemd, not in Kubernetes. It writes to the
in-cluster API over loopback gRPC:

```text
127.0.0.1:19090 -> pulse-core:9090
```

Default BLE service settings:

```ini
After=network-online.target bluetooth.target dbus.service k3s.service
Requires=bluetooth.service
Environment=GOMAXPROCS=1
Environment=GOMEMLIMIT=192MiB
Environment=PULSE_EDGE_TRANSPORT=grpc
Environment=PULSE_EDGE_GRPC_ADDR=127.0.0.1:19090
Environment=PULSE_EDGE_STARTUP_WAIT=10m
Environment=PULSE_EDGE_STARTUP_RETRY_DELAY=5s
Environment=PULSE_EDGE_OUTBOX_DIR=/var/lib/pulse-edge/outbox
Environment=PULSE_EDGE_OUTBOX_MAX_AGE=168h
Environment=PULSE_EDGE_OUTBOX_MAX_BYTES=2GiB
MemoryMax=256M
Restart=always
RestartSec=5s
```

BLE retries are file-backed on the host. The collector writes discovery and
telemetry uploads to `PULSE_EDGE_OUTBOX_DIR`, fsyncs before send, replays
pending entries after restart or a later successful heartbeat, and removes each
entry after ACK. Outbox files omit the collector secret; the current secret is
injected at send time. Telemetry samples include a stable `client_sample_id`
that the edge ingest service maps to envelope `message_id` for downstream
dedupe.

Phase 2 direct-transport implementation starts with the transport boundary:

- `PULSE_EDGE_TRANSPORT=grpc` switches the host collector from REST to direct
  `EdgeIngestService` gRPC for enrollment, heartbeat, discovery, and telemetry.
- `PULSE_EDGE_GRPC_ADDR` defaults to `127.0.0.1:19090`, matching the appliance
  services overlay loopback hostPort.
- REST remains the default for non-appliance edge deployments.
- The packaged appliance unit waits up to `10m` for the initial loopback gRPC
  heartbeat so cold K3s/pod startup does not exhaust systemd start limits
  before BLE ingest can recover.
- Durable outbox replay and stable client sample identity are enabled for the
  appliance collector; hardware validation still needs a Pi boot/restart test.

## Archive And Cloud Shutdown

GCS remains the raw object store. Hosted compute, databases, and caches are
turned off after migration and validation.

Appliance archive behavior:

- write compressed local archive batches to SSD first;
- ACK the local pipeline only after local fsync and outbox record creation;
- upload to GCS asynchronously;
- record GCS manifest rows for remote uploaded objects only;
- block archive-backed rebuilds while local-only outbox entries are pending.

The Pi services overlay enables the archive upload outbox at
`/var/lib/pulse-archive/outbox`, backs it with a `16Gi` ReadWriteOnce PVC, and
sets `ARCHIVE_UPLOAD_OUTBOX_MAX_BYTES=17179869184`. The outbox entry stores
both the compressed object body and the manifest record. Flushes fail closed if
the manifest store is unavailable, so a remote object cannot become
authoritative for rebuilds before the manifest row is recorded.

`pulse-appliance status` checks that outbox from the archive worker pod with
`/app/ecoflow-archive-outbox-status`. Any pending local entries are a status
failure because GCS is behind the local SSD and archive-backed rebuilds are not
safe yet. `cmd/ecoflow-rollup-rebuild` also fails closed when
`ARCHIVE_UPLOAD_OUTBOX_DIR` contains pending entries and the rebuild uses
archive objects. The only bypass is the explicit
`-allow-pending-archive-outbox` manual recovery flag; raw-log rebuild inputs do
not depend on object-storage archive completeness and skip this guard.
By default the status command lets the helper read the archive pod's configured
`ARCHIVE_UPLOAD_OUTBOX_DIR`; `--archive-outbox-dir` is only for an explicit
operator override.

Use `ARCHIVE_OBJECT_PROVIDER=gcs` and an appliance-specific
`ARCHIVE_WRITER_ID`, with service-account credentials mounted as a Kubernetes
secret.

The planned backup and hosted-cloud shutdown sequence is documented in
[`run-pi5-appliance-backup-cutover.md`](../how-to/run-pi5-appliance-backup-cutover.md).
That runbook treats the shared `pulse-platform-core` CNPG database as the local
app and Keycloak backup source, requires the archive upload outbox to be empty
before cutover or rebuild work, and keeps GCS online while hosted compute,
databases, and caches are parked.

## Acceptance

- Fresh NVMe install boots without microSD and survives 10 reboot cycles.
- `pulse-appliance status` checks throttling, NVMe SMART, free disk, K3s,
  Helm releases, Keycloak login, GCS write, gRPC loopback, BLE heartbeat, BLE
  outbox, and NATS stream limits.
- Planned cutover has a fresh CNPG dump, secret backup, GCS sanity marker, and
  empty archive upload outbox before hosted runtime is shut down.
- Restart `pulse-core`, K3s, and BLE without duplicate telemetry.
- Block GCS for 24h without local ingest loss.
- Capacity burn-in with 10 devices stays under 4.8GiB steady RSS, preserves
  host headroom, and shows no CPU throttling.
- Disk pressure warns below 80GiB free and refuses unsafe archive rebuilds.

## Ralph Loop Requirement

Implementation of this config uses the Pulse Pi 5 appliance Ralph Loop:

- plan: `.codex/plans/pulse-pi5-appliance-ralph-loop.md`
- task board: `.codex/tasks/pulse-pi5-appliance.md`
- role memories under `.codex/memories/*/pulse-pi5-appliance.md`

This config is plan-only until the corresponding PR merges. Product/runtime
work must start from a fresh branch after merge.
