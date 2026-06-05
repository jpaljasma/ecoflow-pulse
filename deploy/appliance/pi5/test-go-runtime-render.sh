#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../.." && pwd)"
helm_bin="${HELM:-helm}"

render="$("$helm_bin" template pulse-services \
  "$repo_root/deploy/charts/pulse-services" \
  -f "$repo_root/deploy/env/pi/values.services.yaml")"

require_env() {
  local deployment="$1"
  local env_name="$2"
  local env_value="$3"

  if ! awk \
    -v deployment="name: $deployment" \
    -v env_name="name: $env_name" \
    -v env_value="value: \"$env_value\"" '
      /^---$/ {
        if (block ~ /kind: Deployment/ && block ~ deployment && block ~ env_name && block ~ env_value) {
          found = 1
        }
        block = ""
        next
      }
      { block = block $0 "\n" }
      END {
        if (block ~ /kind: Deployment/ && block ~ deployment && block ~ env_name && block ~ env_value) {
          found = 1
        }
        exit(found ? 0 : 1)
      }
    ' <<<"$render"; then
    echo "expected $deployment to render $env_name=$env_value" >&2
    exit 1
  fi
}

require_runtime() {
  local deployment="$1"
  local max_procs="$2"
  local mem_limit="$3"

  require_env "$deployment" GOMAXPROCS "$max_procs"
  require_env "$deployment" GOMEMLIMIT "$mem_limit"
}

require_runtime pulse-services-go-ingest 1 384MiB
require_runtime pulse-services-go-inference 1 160MiB
require_runtime pulse-services-go-projection 1 256MiB
require_runtime pulse-services-go-rollup 1 256MiB
require_runtime pulse-services-go-archive 1 256MiB
require_runtime pulse-services-go-grpc-api 2 512MiB
require_runtime pulse-services-go-energy-api 1 384MiB
require_runtime pulse-services-go-scheduler 1 160MiB

echo "pulse appliance Go runtime render test passed"
