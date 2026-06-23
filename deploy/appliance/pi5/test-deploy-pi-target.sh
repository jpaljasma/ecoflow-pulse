#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../.." && pwd)"
makefile="$repo_root/Makefile"

require_pattern() {
  local pattern="$1"
  local message="$2"
  if ! grep -Eq "$pattern" "$makefile"; then
    echo "missing deploy-pi target behavior: $message" >&2
    exit 1
  fi
}

require_pattern '^deploy-pi:' "target name"
require_pattern 'gh run list --workflow "\$\(PI_APPLIANCE_IMAGES_WORKFLOW\)" --branch main --status success' "latest successful workflow lookup"
# The patterns below intentionally match literal Makefile variable references.
# shellcheck disable=SC2016
require_pattern 'gh run download "\$\$run_id" --name "\$\(PI_APPLIANCE_RELEASE_ARTIFACT\)"' "release artifact download"
# shellcheck disable=SC2016
require_pattern 'sudo install -D -m 0644 "\$\$release_dir/pulse-pi-release\.yaml" "\$\(PI_APPLIANCE_RELEASE_VALUES\)"' "release values install"
require_pattern 'PI_APPLIANCE_PLATFORM_EXTRA_VALUES \?= /etc/pulse-appliance/platform-extra\.yaml' "default platform extra values path"
# shellcheck disable=SC2016
require_pattern 'export PULSE_APPLIANCE_RELEASE_VALUES="\$\(PI_APPLIANCE_RELEASE_VALUES\)"' "release values env handoff"
# shellcheck disable=SC2016
require_pattern 'export PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES="\$\(PI_APPLIANCE_PLATFORM_EXTRA_VALUES\)"' "platform extra values env handoff"
require_pattern 'WAIT_TIMEOUT="\$\(PI_APPLIANCE_WAIT_TIMEOUT\)" APPLIANCE_PI_INSTALL_ARGS="--skip-host-prepare --skip-k3s-install" \$\(MAKE\) appliance-pi-upgrade' "appliance upgrade invocation"
require_pattern 'sudo env KUBECONFIG="\$\$\{KUBECONFIG:-/etc/rancher/k3s/k3s\.yaml\}" \$\(MAKE\) appliance-pi-status' "post-upgrade status"
require_pattern 'pulse-platform-public-app pulse-platform-realtime-gateway' "post-upgrade image check"

echo "pulse appliance deploy-pi target test passed"
