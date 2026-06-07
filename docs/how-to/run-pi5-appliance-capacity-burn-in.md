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

1. Publish the appliance images from GitHub Actions. The workflow builds the
   services, public app, and realtime gateway images for `linux/arm64`, pushes
   them to GHCR, and uploads a digest-pinned release values artifact.

For public GHCR packages, omit `image_pull_secret`:

```bash
gh workflow run pi-appliance-images.yml \
  -f image_tag=pi-$(git rev-parse --short=12 HEAD)
```

For private GHCR packages, choose the Kubernetes pull secret name up front so
the generated release artifact references it:

```bash
gh workflow run pi-appliance-images.yml \
  -f image_tag=pi-$(git rev-parse --short=12 HEAD) \
  -f image_pull_secret=ghcr-pull-secret
```

After the workflow finishes, download the `pulse-pi-release-values` artifact
and copy the generated `pulse-pi-release.yaml` to the Pi.

2. Create the install-local release values file outside Git:

```bash
sudo mkdir -p /etc/pulse-appliance
sudo install -m 0640 -o root -g "$(id -gn)" pulse-pi-release.yaml \
  /etc/pulse-appliance/release.yaml
sudo chmod 0755 /etc/pulse-appliance
```

The installer refuses to apply a chart that still renders `pi-placeholder`.

3. If GHCR packages are private, create the same pull secret in both appliance
   namespaces before applying workloads. Use a token that can read GHCR
   packages, and keep it out of Git:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
for namespace in pulse-platform pulse-services; do
  kubectl --kubeconfig "$KUBECONFIG" create namespace "$namespace" \
    --dry-run=client -o yaml | kubectl --kubeconfig "$KUBECONFIG" apply -f -
  kubectl --kubeconfig "$KUBECONFIG" -n "$namespace" \
    create secret docker-registry ghcr-pull-secret \
    --docker-server=ghcr.io \
    --docker-username=<github-user-or-org> \
    --docker-password=<ghcr-read-packages-token> \
    --dry-run=client -o yaml | kubectl --kubeconfig "$KUBECONFIG" apply -f -
done
```

4. Apply or upgrade the platform first, then create the runtime and GCS
   credentials secrets. The credentials file must stay local to the appliance:

```bash
APPLIANCE_PI_INSTALL_ARGS="--release-values /etc/pulse-appliance/release.yaml --skip-services" \
  make appliance-pi-upgrade

APPLIANCE_PI_RUNTIME_SECRET_ARGS="--gcs-credentials /path/to/gcs-service-account.json --gcs-project-id <gcs-project-id> --archive-writer-id pulse-pi5" \
  make appliance-pi-create-runtime-secret
```

If `/etc/pulse-appliance/release.yaml` customizes
`runtime.gcsCredentials.secretKey`, `fileName`, or `mountPath`, include the
matching `--gcs-secret-key`, `--gcs-file-name`, and `--gcs-mount-path` flags in
`APPLIANCE_PI_RUNTIME_SECRET_ARGS`.

5. Apply the services release and run status:

```bash
APPLIANCE_PI_INSTALL_ARGS="--release-values /etc/pulse-appliance/release.yaml --skip-host-prepare --skip-k3s-install" \
  make appliance-pi-upgrade
make appliance-pi-status
```

Run `sudo env KUBECONFIG="$KUBECONFIG" make appliance-pi-status` when a full
NVMe SMART read is required. Non-root status checks can still validate the
cluster and archive outbox but report a warning when SMART access needs root.

6. Confirm the Pi sees the expected Kubernetes limits and node allocatable:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl --kubeconfig "$KUBECONFIG" describe node pulse-pi5 \
  | grep -A12 -E 'Capacity|Allocatable'
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services get deploy
kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform get statefulset
```

7. Confirm metrics are available:

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
  deploy/pulse-services-go-grpc-api --timeout=600s
```

The Pi overlay intentionally rolls this singleton with `maxSurge: 0` and
`maxUnavailable: 1` because the deployment owns loopback hostPort `19090`. If
an older release leaves a replacement gRPC pod `Pending` with an event like
`didn't have free ports for the requested pod ports`, upgrade to a release that
contains the Pi hostPort rollout strategy before treating it as capacity
pressure.

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
