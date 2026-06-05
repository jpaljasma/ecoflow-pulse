#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../.." && pwd)"
release_values="$(mktemp "${TMPDIR:-/tmp}/pulse-pi-release-values.XXXXXX.yaml")"
trap 'rm -f "$release_values"' EXIT

cat >"$release_values" <<'YAML'
runtime: {}
YAML

output="$("$script_dir/pulse-appliance-install.sh" install \
  --repo-root "$repo_root" \
  --release-values "$release_values" \
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

require_output "skipping host preparation"
require_output "skipping K3s install"
require_output "helm --kubeconfig /etc/rancher/k3s/k3s.yaml"
require_output "dependency build --skip-refresh"
require_output "kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml"
require_output "upgrade --install pulse-platform"
require_output "deploy/env/pi/values.platform.yaml"
require_output "$release_values"
require_output "running platform bootstrap pass without Keycloak"
require_output "upgrade --install pulse-services"
require_output "deploy/env/pi/values.services.yaml"
require_output "-n pulse-services get secret pulse-services-runtime-secret"

echo "pulse appliance install dry-run test passed"
