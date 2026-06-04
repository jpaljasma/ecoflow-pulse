# How-To: Back Up And Cut Over Pulse Pi 5 Appliance

This runbook covers a planned Pulse Pi 5 appliance cutover where the local Pi
becomes the active runtime and Google Cloud resources are turned off except for
GCS object storage.

Use it after the Pi appliance host, K3s, Helm releases, Keycloak, GCS archive
writer, and BLE collector are already installed and healthy.

## Recovery Model

The appliance has three durable recovery sources:

- the `pulse-platform-core` CNPG database, which stores Pulse app data and the
  local Keycloak database;
- the GCS raw archive bucket, which remains the authoritative remote object
  store for uploaded archive objects;
- local SSD upload outboxes, which must be empty before planned cutover,
  backup, rebuild, or hosted shutdown work is considered complete.

Valkey is treated as rebuildable cache. NATS JetStream is required for local
ingest restart safety, but it is not the long-term recovery source after all
messages have drained into Postgres and the archive pipeline.

## Preconditions

1. Run from the Pi host or from an operator workstation that can reach the Pi
   over the LAN.
2. Install local client tools on the machine running the commands:

```bash
sudo apt install -y postgresql-client jq
```

3. Use the K3s kubeconfig explicitly:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
```

4. Confirm the appliance status is clean:

```bash
pulse-appliance status
kubectl --kubeconfig "$KUBECONFIG" get nodes -o wide
kubectl --kubeconfig "$KUBECONFIG" get pods -A
helm --kubeconfig "$KUBECONFIG" -n pulse-platform status pulse-platform
helm --kubeconfig "$KUBECONFIG" -n pulse-services status pulse-services
```

5. Confirm the archive upload outbox is empty:

```bash
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services \
  exec deploy/pulse-services-go-archive -- \
  /app/ecoflow-archive-outbox-status --fail-on-pending
```

Do not proceed if this command reports pending entries. Restore GCS
connectivity, let the archive worker flush the outbox, and rerun the check.

## Create A Backup Bundle

Create an operator-controlled directory that is not committed to Git:

```bash
backup_name="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="$HOME/pulse-appliance-backups/$backup_name"
mkdir -p "$backup_dir"
chmod 700 "$backup_dir"
```

Record install state and Kubernetes state:

```bash
{
  date -u
  hostnamectl
  uname -a
  vcgencmd get_throttled
  vcgencmd measure_temp
  lsblk -o NAME,MODEL,SIZE,FSTYPE,MOUNTPOINTS
  sudo rpi-eeprom-config | grep -E 'BOOT_ORDER|PCIE_PROBE'
} > "$backup_dir/host-state.txt"

kubectl --kubeconfig "$KUBECONFIG" get nodes -o wide \
  > "$backup_dir/k8s-nodes.txt"
kubectl --kubeconfig "$KUBECONFIG" get pods -A -o wide \
  > "$backup_dir/k8s-pods.txt"
helm --kubeconfig "$KUBECONFIG" -n pulse-platform list \
  > "$backup_dir/helm-pulse-platform.txt"
helm --kubeconfig "$KUBECONFIG" -n pulse-services list \
  > "$backup_dir/helm-pulse-services.txt"
```

Dump the shared CNPG database. This captures Pulse control-plane data, rollups,
archive manifest rows, and Keycloak state because the Pi values configure
Keycloak to use `pulse-platform-core-rw`.

```bash
db_user="$(kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  get secret pulse-platform-core-app \
  -o jsonpath='{.data.username}' | base64 -d)"
db_password="$(kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  get secret pulse-platform-core-app \
  -o jsonpath='{.data.password}' | base64 -d)"

kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  port-forward svc/pulse-platform-core-rw 15432:5432 &
pf_pid="$!"
trap 'kill "$pf_pid" 2>/dev/null || true' EXIT
sleep 3

PGPASSWORD="$db_password" pg_dump \
  --host 127.0.0.1 \
  --port 15432 \
  --username "$db_user" \
  --dbname pulse \
  --format custom \
  --no-owner \
  --no-acl \
  --file "$backup_dir/pulse-platform-core.dump"

kill "$pf_pid"
trap - EXIT
unset db_password
```

Store secret material separately from the database dump. The runtime secret,
GCS service account secret, social IdP secrets, and any provider credentials are
sensitive and must stay in an encrypted password manager or encrypted offline
backup. Do not paste them into GitHub issues, PRs, logs, or docs.

Record a GCS archive sanity marker without copying raw archive objects back to
the Pi:

```bash
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services \
  exec deploy/pulse-services-go-archive -- \
  /app/ecoflow-archive-outbox-status --fail-on-pending \
  > "$backup_dir/archive-outbox-status.txt"
```

If your operator workstation has authenticated `gcloud` access to the archive
bucket, also record a bucket metadata snapshot:

```bash
gcloud storage buckets describe gs://<pulse-archive-bucket> \
  > "$backup_dir/gcs-bucket.txt"
gcloud storage ls --recursive gs://<pulse-archive-bucket>/<prefix>/ \
  | tail -n 200 \
  > "$backup_dir/gcs-recent-objects.txt"
```

Replace the bucket and prefix placeholders with the install-specific values.
Do not store service-account JSON inside the repository.

## Planned Cloud Shutdown Cutover

Use this sequence when the Pi is already ingesting locally and the hosted cloud
runtime is no longer needed.

1. Confirm the Pi is healthy:

```bash
pulse-appliance status
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services \
  exec deploy/pulse-services-go-archive -- \
  /app/ecoflow-archive-outbox-status --fail-on-pending
vcgencmd get_throttled
df -h /
```

2. Confirm local user access:

- `https://pulse.home.arpa` resolves on the LAN.
- Local Keycloak username/password login works.
- Social login works only if this install configured install-specific IdPs.
- BLE heartbeat and provider MQTT ingest are visible in appliance status or
  service logs.

3. Take a fresh backup bundle with the commands above.

4. Freeze or park the hosted runtime:

- stop hosted provider-ingest workers first;
- stop hosted projection, rollup, archive, inference, scheduler, and API
  workloads after hosted queues drain;
- scale hosted web/realtime edges down after local login and realtime are
  verified;
- park or delete hosted database/cache compute only after the final backup is
  captured and the Pi has passed a burn-in window;
- keep the GCS archive bucket, lifecycle policy, IAM binding, and service
  account needed by the Pi.

5. Keep DNS local-only. The default appliance endpoint remains
   `https://pulse.home.arpa`. If using a real domain, use split-horizon or
   private DNS only; do not expose public ingress to the Pi.

6. After shutdown, rerun:

```bash
pulse-appliance status
kubectl --kubeconfig "$KUBECONFIG" get pods -A
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services logs \
  deploy/pulse-services-go-archive --tail=100
```

## Restore To The Same Pi

Use this path for a controlled restore from a known-good appliance backup.

1. Stop app traffic while keeping the platform database reachable:

```bash
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services \
  scale deploy --all --replicas=0
kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  scale deploy pulse-platform-public-app --replicas=0
kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  scale deploy pulse-platform-realtime-gateway --replicas=0
kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  scale statefulset pulse-platform-keycloak --replicas=0
```

2. Restore the database dump:

```bash
restore_dir="$HOME/pulse-appliance-backups/<backup-name>"
db_user="$(kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  get secret pulse-platform-core-app \
  -o jsonpath='{.data.username}' | base64 -d)"
db_password="$(kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  get secret pulse-platform-core-app \
  -o jsonpath='{.data.password}' | base64 -d)"

kubectl --kubeconfig "$KUBECONFIG" -n pulse-platform \
  port-forward svc/pulse-platform-core-rw 15432:5432 &
pf_pid="$!"
trap 'kill "$pf_pid" 2>/dev/null || true' EXIT
sleep 3

PGPASSWORD="$db_password" pg_restore \
  --host 127.0.0.1 \
  --port 15432 \
  --username "$db_user" \
  --dbname pulse \
  --clean \
  --if-exists \
  --no-owner \
  --no-acl \
  --single-transaction \
  "$restore_dir/pulse-platform-core.dump"

kill "$pf_pid"
trap - EXIT
unset db_password
```

3. Re-apply the appliance release bundle so migrations, ConfigMaps, Secrets,
   and workload definitions match the restore target:

```bash
make appliance-pi-upgrade
```

4. Wait for convergence and validate:

```bash
kubectl --kubeconfig "$KUBECONFIG" get pods -A
pulse-appliance status
kubectl --kubeconfig "$KUBECONFIG" -n pulse-services \
  exec deploy/pulse-services-go-archive -- \
  /app/ecoflow-archive-outbox-status --fail-on-pending
```

5. Verify local Keycloak login, BLE heartbeat, provider MQTT ingest, GCS write,
   and a recent device dashboard update before declaring the restore complete.

## Restore To Replacement Hardware

1. Install Raspberry Pi OS Lite 64-bit, host tuning, K3s, and the same Pulse
   appliance release on the replacement Pi.
2. Restore the encrypted secret bundle outside Git.
3. Restore the CNPG database dump using the same restore procedure.
4. Keep the same GCS bucket and prefix unless deliberately moving archives.
5. Do not run archive-backed rebuilds until the archive outbox status check is
   clean and GCS object/manifest coverage has been audited.

## Failure Rules

- If the archive upload outbox has pending entries, do not run archive-backed
  rebuilds, do not declare a backup complete, and do not turn off the last
  hosted recovery path.
- If GCS is unavailable, keep local ingest running and wait for the outbox to
  flush after connectivity returns.
- If database restore fails, keep hosted resources parked rather than deleted
  until the Pi restore is fixed or the last known-good backup is restored.
- Never delete the GCS archive bucket as part of cost shutdown.
- Never expose the appliance through public ingress as a shortcut for remote
  access; use VPN or private DNS patterns instead.
