#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../.." && pwd)"

output="$("$script_dir/pulse-appliance-install.sh" install \
  --repo-root "$repo_root" \
  --dry-run \
  --skip-host-prepare \
  --skip-k3s-install \
  --skip-wait)"

require_output() {
  local needle="$1"
  if ! grep -Fq "$needle" <<<"$output"; then
    echo "expected dry-run output to contain: $needle" >&2
    echo "--- dry-run output ---" >&2
    printf '%s\n' "$output" >&2
    exit 1
  fi
}

require_output "skipping host preparation"
require_output "skipping K3s install"
require_output "helm dependency build --skip-refresh"
require_output "upgrade --install pulse-platform"
require_output "deploy/env/pi/values.platform.yaml"
require_output "running platform bootstrap pass without Keycloak"
require_output "upgrade --install pulse-services"
require_output "deploy/env/pi/values.services.yaml"
require_output "kubectl -n pulse-services get secret pulse-services-runtime-secret"

echo "pulse appliance install dry-run test passed"
