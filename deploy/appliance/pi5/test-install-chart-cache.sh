#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp_parent="${TMPDIR:-/tmp}"
tmp_parent="${tmp_parent%/}"
fixture_root="$(mktemp -d "$tmp_parent/pulse-pi-chart-cache.XXXXXX")"
trap 'rm -rf "$fixture_root"' EXIT

mkdir -p \
  "$fixture_root/deploy/charts/pulse-platform/charts" \
  "$fixture_root/deploy/charts/pulse-services" \
  "$fixture_root/deploy/env/pi"

cat >"$fixture_root/deploy/charts/pulse-platform/Chart.yaml" <<'YAML'
apiVersion: v2
name: pulse-platform
version: 0.1.0
dependencies:
  - name: nats
    version: 2.12.4
    repository: https://nats-io.github.io/k8s/helm/charts/
YAML

cat >"$fixture_root/deploy/charts/pulse-platform/Chart.lock" <<'YAML'
dependencies:
- name: nats
  repository: https://nats-io.github.io/k8s/helm/charts/
  version: 2.12.4
digest: sha256:test
generated: "2026-06-11T00:00:00Z"
YAML

cat >"$fixture_root/deploy/charts/pulse-services/Chart.yaml" <<'YAML'
apiVersion: v2
name: pulse-services
version: 0.1.0
YAML

cat >"$fixture_root/deploy/charts/pulse-services/Chart.lock" <<'YAML'
dependencies: []
digest: sha256:test
generated: "2026-06-11T00:00:00Z"
YAML

cat >"$fixture_root/deploy/env/pi/values.platform.yaml" <<'YAML'
runtime: {}
YAML
cat >"$fixture_root/deploy/env/pi/values.services.yaml" <<'YAML'
runtime: {}
YAML
touch "$fixture_root/deploy/charts/pulse-platform/charts/nats-2.12.4.tgz"

output="$("$script_dir/pulse-appliance-install.sh" install \
  --repo-root "$fixture_root" \
  --dry-run \
  --skip-host-prepare \
  --skip-k3s-install \
  --skip-wait)"

require_output() {
  local needle="$1"
  if ! grep -Fq -- "$needle" <<<"$output"; then
    echo "expected dry-run output to contain: $needle" >&2
    echo "--- dry-run output ---" >&2
    printf '%s\n' "$output" >&2
    exit 1
  fi
}

reject_output() {
  local needle="$1"
  if grep -Fq -- "$needle" <<<"$output"; then
    echo "expected dry-run output not to contain: $needle" >&2
    echo "--- dry-run output ---" >&2
    printf '%s\n' "$output" >&2
    exit 1
  fi
}

require_output "chart dependencies already cached for $fixture_root/deploy/charts/pulse-platform; skipping helm dependency build"
require_output "chart dependencies already cached for $fixture_root/deploy/charts/pulse-services; skipping helm dependency build"
reject_output "dependency build --skip-refresh $fixture_root/deploy/charts/pulse-platform"

rm -f "$fixture_root/deploy/charts/pulse-platform/charts/nats-2.12.4.tgz"
output="$("$script_dir/pulse-appliance-install.sh" install \
  --repo-root "$fixture_root" \
  --dry-run \
  --skip-host-prepare \
  --skip-k3s-install \
  --skip-wait)"

require_output "chart dependency archives missing for $fixture_root/deploy/charts/pulse-platform; running helm dependency build --skip-refresh"
require_output "dependency build --skip-refresh $fixture_root/deploy/charts/pulse-platform"

echo "pulse appliance install chart cache test passed"
