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

Phase 1 implementation files:

- `deploy/appliance/pi5/pulse-appliance-host-prepare.sh`
- `deploy/appliance/pi5/pulse-appliance-status.sh`
- `deploy/appliance/pi5/k3s-config.yaml`
- `deploy/env/pi/values.platform.yaml`
- `deploy/env/pi/values.services.yaml`

Validation command:

```bash
make appliance-pi-validate
```

## Appliance Workload Defaults

- Postgres/CNPG: singleton, Timescale enabled, `max_connections=40`,
  `shared_buffers=256MB`, `work_mem=4MB`, WAL compression on, 64Gi PVC cap.
- NATS: singleton JetStream, file storage, stream replicas `1`, max age `24h`,
  max bytes `8Gi`, 16Gi PVC cap.
- Valkey: singleton with AOF, memory cap 256-384Mi, 2Gi PVC cap.
- Keycloak: singleton, memory limit 768Mi, local username/password auth
  required, social identity providers optional.
- Pulse core: merged Go runtime only where it reduces overhead without
  weakening graceful shutdown or idempotency.
- Public web and realtime gateway: separate singleton workloads unless
  measurement proves a safe merge.
- Observability lite: optional, 24h retention, no OpenTelemetry collector by
  default.

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
Environment=PULSE_EDGE_OUTBOX_DIR=/var/lib/pulse-edge/outbox
Environment=PULSE_EDGE_OUTBOX_MAX_AGE=168h
Environment=PULSE_EDGE_OUTBOX_MAX_BYTES=2GiB
MemoryMax=256M
Restart=always
RestartSec=5s
```

BLE retries must be idempotent. Add a stable client sample identity to edge
telemetry before enabling durable retry.

## Archive And Cloud Shutdown

GCS remains the raw object store. Hosted compute, databases, and caches are
turned off after migration and validation.

Appliance archive behavior:

- write compressed local archive batches to SSD first;
- ACK the local pipeline only after local fsync and outbox record creation;
- upload to GCS asynchronously;
- record GCS manifest rows for remote uploaded objects only;
- block archive-backed rebuilds while local-only outbox entries are pending.

Use `ARCHIVE_OBJECT_PROVIDER=gcs` and an appliance-specific
`ARCHIVE_WRITER_ID`, with service-account credentials mounted as a Kubernetes
secret.

## Acceptance

- Fresh NVMe install boots without microSD and survives 10 reboot cycles.
- `pulse-appliance status` checks throttling, NVMe SMART, free disk, K3s,
  Helm releases, Keycloak login, GCS write, gRPC loopback, BLE heartbeat, BLE
  outbox, and NATS stream limits.
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
