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

The `Pi Appliance Images` workflow runs automatically after pushes to `main`,
including normal pull-request merges. Use that automatic run for the standard
post-merge appliance update. Manual dispatch remains useful for reruns, custom
image tags, or release artifacts that must include a private GHCR pull secret.

If your local checkout cannot switch to `main` because another worktree already
uses it, trigger the workflow from the current branch while explicitly using
`origin/main` as the source of truth:

```bash
git fetch origin main --prune
TAG="pi-$(git rev-parse --short=12 origin/main)"
```

For a manual public GHCR rerun, omit `image_pull_secret`:

```bash
gh workflow run pi-appliance-images.yml \
  --ref main \
  -f image_tag="$TAG"
```

For a manual private GHCR run, choose the Kubernetes pull secret name up front
so the generated release artifact references it:

```bash
gh workflow run pi-appliance-images.yml \
  --ref main \
  -f image_tag="$TAG" \
  -f image_pull_secret=ghcr-pull-secret
```

After the automatic or manual workflow finishes, download the
`pulse-pi-release-values` artifact. This can happen on the workstation and be
copied to the Pi, or directly on the Pi after installing GitHub CLI.

To install `gh` on Raspberry Pi OS / Debian with the
[official GitHub CLI APT repository](https://github.com/cli/cli/blob/trunk/docs/install_linux.md),
use the maintained signed-keyring flow:

```bash
sudo apt update
sudo apt install -y wget ca-certificates
sudo mkdir -p -m 755 /etc/apt/keyrings
wget -nv -O /tmp/githubcli-archive-keyring.gpg \
  https://cli.github.com/packages/githubcli-archive-keyring.gpg
sudo install -m 0644 /tmp/githubcli-archive-keyring.gpg \
  /etc/apt/keyrings/githubcli-archive-keyring.gpg
sudo mkdir -p -m 755 /etc/apt/sources.list.d
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
  | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null
sudo apt update
sudo apt install -y gh
gh --version
```

Then authenticate from the headless Pi. The web flow prints a device code and
URL that can be completed from another browser:

```bash
gh auth login --hostname github.com --web
gh auth status
gh auth setup-git
```

For routine post-merge updates after the first appliance setup is complete, run
the one-command deploy target from the cloned repo on the Pi:

```bash
make deploy-pi
```

`make deploy-pi` automatically includes
`/etc/pulse-appliance/platform-extra.yaml` when that file exists. Use that
Pi-local file for install-specific platform settings that must persist across
future appliance upgrades but must not be committed, such as Google broker
credentials:

```bash
sudo install -d -m 0755 /etc/pulse-appliance
tmp_values="$(mktemp)"
cat > "$tmp_values" <<'YAML'
keycloakRealm:
  google:
    enabled: true
    clientId: "<google-client-id>"
    clientSecret: "<google-client-secret>"
YAML
sudo install -m 0640 -o root -g "$(id -gn)" \
  "$tmp_values" /etc/pulse-appliance/platform-extra.yaml
rm -f "$tmp_values"
```

Also configure the Google OAuth web client with this authorized redirect URI:

```text
https://pulse.home.arpa/realms/pulse/broker/google/endpoint
```

For manual inspection, custom workflow runs, private GHCR pull-secret changes,
or recovery from a partial artifact download, download the successful workflow
artifact directly:

```bash
gh run list --workflow pi-appliance-images.yml --branch main --limit 5
RUN_ID=<successful-workflow-run-id>
rm -rf /tmp/pulse-pi-release-values
gh run download "$RUN_ID" \
  -n pulse-pi-release-values \
  -D /tmp/pulse-pi-release-values
```

2. Create the install-local release values file outside Git:

```bash
sudo mkdir -p /etc/pulse-appliance
sudo install -m 0640 -o root -g "$(id -gn)" \
  /tmp/pulse-pi-release-values/pulse-pi-release.yaml \
  /etc/pulse-appliance/release.yaml
sudo chmod 0755 /etc/pulse-appliance
```

The installer refuses to apply a chart that still renders `pi-placeholder`.
Operator-local platform overrides are expected for optional social providers
and other install-specific settings. Clear only temporary live-debug overrides
before applying a refreshed artifact:

```bash
unset PULSE_APPLIANCE_SERVICES_EXTRA_VALUES
```

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
PULSE_APPLIANCE_RELEASE_VALUES=/etc/pulse-appliance/release.yaml \
PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES=/etc/pulse-appliance/platform-extra.yaml \
APPLIANCE_PI_INSTALL_ARGS="--skip-services" \
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
PULSE_APPLIANCE_RELEASE_VALUES=/etc/pulse-appliance/release.yaml \
PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES=/etc/pulse-appliance/platform-extra.yaml \
APPLIANCE_PI_INSTALL_ARGS="--skip-host-prepare --skip-k3s-install" \
  make appliance-pi-upgrade
make appliance-pi-status
```

Run `sudo env KUBECONFIG="$KUBECONFIG" make appliance-pi-status` when a full
NVMe SMART read is required. Non-root status checks can still validate the
cluster and archive outbox but report a warning when SMART access needs root.

The installer runs `helm dependency build --skip-refresh` for the platform and
services charts only when the local chart archive cache is missing entries from
`Chart.lock`. To avoid downloading the large dependency set on every Pi
upgrade, keep the generated untracked chart archives under
`deploy/charts/*/charts/` on the appliance checkout. They are a local SSD cache
for the Pi and should not be committed. Set
`PULSE_APPLIANCE_FORCE_CHART_DEPENDENCY_BUILD=1` only when intentionally
refreshing that cache.

6. Confirm the Pi sees the expected Kubernetes limits and node allocatable:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl --kubeconfig "$KUBECONFIG" describe node pulse-pi5 \
  | grep -A12 -E 'Capacity|Allocatable'
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services get deploy
kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform get statefulset
```

`pulse-platform-nats-0` normally reports `2/2` containers. That is one NATS
server container plus the chart's config reloader sidecar, not two NATS
servers. The separate `nats-box` toolbox deployment is disabled in the Pi
overlay.

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

If the first platform apply fails with missing CNPG resource mappings, check
that Helm chart dependencies are present and rerun the appliance upgrade. Do
not hand-apply empty or partial CRD manifests as a normal recovery path. On a
fresh failed install where you know CNPG CRDs were hand-applied and
`kubectl get clusters.postgresql.cnpg.io -A` reports no resources, remove only
those orphaned hand-applied CNPG CRDs before rerunning. After any CNPG database
resource exists, do not delete CNPG CRDs because that can remove the custom
resources that describe the live database.

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
