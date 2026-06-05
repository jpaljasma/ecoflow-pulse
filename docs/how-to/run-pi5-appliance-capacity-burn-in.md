# How-To: Run Pulse Pi 5 Appliance Capacity Burn-In

Use this runbook after the Pi appliance is installed, ingesting from the target
devices, and upgraded to a release that includes the Pi Go runtime caps. The
goal is to prove the current singleton deployment layout fits comfortably on an
8 GB Raspberry Pi 5 before merging workloads into a future `pulse-core`
process.

## What This Proves

The burn-in should show:

- no Raspberry Pi throttling flags;
- steady RSS across host plus Kubernetes workloads below `4.8GiB`;
- at least `1GiB` host memory headroom after cache pressure settles;
- low or zero zram swap use during normal ingest;
- no sustained CPU saturation with 1-2 users and about 10 devices;
- archive upload outbox drains to GCS and stays below its configured cap.

## Pi Runtime Caps

The Pi services overlay sets per-workload Go runtime caps instead of one global
cap. This keeps the current deployment layout conservative while leaving room
for future process merging.

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

These caps are intentionally below the Kubernetes memory limits in
`deploy/env/pi/values.services.yaml`. They are not the final merged-core
budget; they are a safe first clamp for the separate singleton process layout.

## Preconditions

1. Create an install-local release values file outside Git. Start from
   `deploy/env/pi/release.example.yaml` and replace every image repository and
   digest placeholder with published `linux/arm64` appliance images:

```bash
sudo mkdir -p /etc/pulse-appliance
sudo install -m 0600 deploy/env/pi/release.example.yaml \
  /etc/pulse-appliance/release.yaml
sudo nano /etc/pulse-appliance/release.yaml
```

The installer refuses to apply a chart that still renders `pi-placeholder`.

2. Apply or upgrade the platform first, then create the runtime and GCS
   credentials secrets. The credentials file must stay local to the appliance:

```bash
APPLIANCE_PI_INSTALL_ARGS="--release-values /etc/pulse-appliance/release.yaml --skip-services" \
  make appliance-pi-upgrade

APPLIANCE_PI_RUNTIME_SECRET_ARGS="--gcs-credentials /path/to/gcs-service-account.json --gcs-project-id <gcs-project-id> --archive-writer-id pulse-pi5" \
  make appliance-pi-create-runtime-secret
```

3. Apply the services release and run status:

```bash
APPLIANCE_PI_INSTALL_ARGS="--release-values /etc/pulse-appliance/release.yaml --skip-host-prepare --skip-k3s-install" \
  make appliance-pi-upgrade
make appliance-pi-status
```

4. Confirm the Pi sees the expected Kubernetes limits and node allocatable:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl --kubeconfig "$KUBECONFIG" describe node pulse-pi5 \
  | grep -A12 -E 'Capacity|Allocatable'
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services get deploy
kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform get statefulset
```

5. Confirm metrics are available:

```bash
kubectl --kubeconfig "$KUBECONFIG" top node
kubectl --kubeconfig "$KUBECONFIG" top pods -A --containers
```

If `kubectl top` is unavailable, wait for the K3s metrics server to settle:

```bash
kubectl --kubeconfig "$KUBECONFIG" -n kube-system rollout status \
  deploy/metrics-server --timeout=120s
```

## Run A 24-Hour Burn-In

Create a local log directory outside Git:

```bash
burn_dir="$HOME/pulse-appliance-burnin/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$burn_dir"
```

Run the sampler for 24 hours:

```bash
duration_seconds=$((24 * 60 * 60))
interval_seconds=60
end_epoch=$(( $(date +%s) + duration_seconds ))

while [ "$(date +%s)" -lt "$end_epoch" ]; do
  {
    echo "=== sample_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
    vcgencmd get_throttled || true
    vcgencmd measure_temp || true
    free -h
    swapon --show
    df -h /
    kubectl --kubeconfig "$KUBECONFIG" top node || true
    kubectl --kubeconfig "$KUBECONFIG" top pods -A --containers || true
    kubectl --kubeconfig "$KUBECONFIG" get pods -A \
      -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name,READY:.status.containerStatuses[*].ready,RESTARTS:.status.containerStatuses[*].restartCount,PHASE:.status.phase'
    kubectl --kubeconfig "$KUBECONFIG" -n pulse-services exec \
      deploy/pulse-services-go-archive -- \
      /app/ecoflow-archive-outbox-status || true
  } >> "$burn_dir/samples.log" 2>&1
  sleep "$interval_seconds"
done
```

Keep normal local usage during the run:

- leave provider MQTT ingest enabled;
- leave BLE collector enabled;
- open the web UI periodically from 1-2 user profiles;
- avoid manual rebuilds unless the test is explicitly about rebuild load.

## Optional Stress Windows

Use short stress windows after the first steady burn-in passes:

1. Restart `pulse-services-go-grpc-api` while BLE is running:

```bash
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services rollout restart \
  deploy/pulse-services-go-grpc-api
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services rollout status \
  deploy/pulse-services-go-grpc-api --timeout=180s
```

2. Temporarily block GCS egress for a bounded test, then unblock and confirm
   the archive upload outbox returns to zero pending entries.
3. Reboot once after the burn-in and confirm `pulse-appliance status` passes.

## Pass Criteria

Pass the capacity burn-in when all are true:

- `vcgencmd get_throttled` remains `throttled=0x0`;
- no Pulse pod repeatedly restarts;
- `kubectl top node` does not show sustained CPU saturation;
- total observed Kubernetes workload memory plus host used memory stays under
  the `4.8GiB` steady target;
- `free -h` shows at least `1GiB` available after caches settle;
- zram swap does not grow steadily during normal ingest;
- archive upload outbox is usually empty and returns to empty after any
  deliberate GCS outage window;
- root filesystem free space stays above the `80GiB` warning threshold.

If any criterion fails, keep the current singleton layout and tune the workload
that exceeded its budget. Do not merge that workload into a combined process
until restart, outbox, and idempotency behavior are understood.

## What To Merge Later

Only consider a merged `pulse-core` after the separate-process burn-in passes.
Merge candidates should start with low-risk query/API and scheduler work.
Provider ingest, archive upload, and rollup should remain separate until their
shutdown, retry, and replay behavior has hardware evidence under restart and
GCS-outage tests.
