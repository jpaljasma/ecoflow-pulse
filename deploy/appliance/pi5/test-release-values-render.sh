#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
helm_bin="${HELM:-helm}"
tmp_values="$(mktemp "${TMPDIR:-/tmp}/pulse-pi-release-values.XXXXXX.yaml")"
trap 'rm -f "$tmp_values"' EXIT

PULSE_PI_IMAGE_REGISTRY=registry.example.test \
PULSE_PI_IMAGE_NAMESPACE=pulse \
PULSE_PI_IMAGE_PULL_SECRET=ghcr-pull-secret \
PULSE_PI_PUBLIC_APP_IMAGE_DIGEST=sha256:1111111111111111111111111111111111111111111111111111111111111111 \
PULSE_PI_REALTIME_GATEWAY_IMAGE_DIGEST=sha256:2222222222222222222222222222222222222222222222222222222222222222 \
PULSE_PI_SERVICES_IMAGE_DIGEST=sha256:3333333333333333333333333333333333333333333333333333333333333333 \
  bash "$repo_root/deploy/appliance/pi5/pulse-appliance-render-release-values.sh" \
    --output "$tmp_values"

platform_render="$("$helm_bin" template pulse-platform "$repo_root/deploy/charts/pulse-platform" \
  -f "$repo_root/deploy/env/pi/values.platform.yaml" \
  -f "$tmp_values")"
services_render="$("$helm_bin" template pulse-services "$repo_root/deploy/charts/pulse-services" \
  -f "$repo_root/deploy/env/pi/values.services.yaml" \
  -f "$tmp_values")"

combined_render="${platform_render}"$'\n'"${services_render}"
if grep -q "pi-placeholder" <<<"$combined_render"; then
  echo "release values render still contains pi-placeholder" >&2
  exit 1
fi

require_rendered() {
  local rendered="$1"
  local pattern="$2"
  if ! grep -Fq "$pattern" <<<"$rendered"; then
    echo "missing rendered pattern: $pattern" >&2
    exit 1
  fi
}

require_rendered_block() {
  local rendered="$1"
  local first_pattern="$2"
  local second_pattern="$3"

  if ! awk \
    -v first_pattern="$first_pattern" \
    -v second_pattern="$second_pattern" '
      $0 ~ first_pattern {
        in_block = 1
      }
      in_block && $0 ~ second_pattern {
        found = 1
      }
      in_block && /^            - name:/ && $0 !~ first_pattern {
        in_block = 0
      }
      END {
        exit(found ? 0 : 1)
      }
    ' <<<"$rendered"; then
    echo "missing rendered block patterns: $first_pattern / $second_pattern" >&2
    exit 1
  fi
}

require_rendered "$platform_render" 'image: "registry.example.test/pulse/pulse-platform@sha256:1111111111111111111111111111111111111111111111111111111111111111"'
require_rendered "$platform_render" 'image: "registry.example.test/pulse/pulse-realtime-gateway@sha256:2222222222222222222222222222222222222222222222222222222222222222"'
require_rendered "$platform_render" 'name: ghcr-pull-secret'
require_rendered "$platform_render" 'image: docker.io/bitnamilegacy/keycloak:'
require_rendered "$platform_render" 'image: docker.io/bitnamilegacy/keycloak-config-cli:'
require_rendered_block "$platform_render" 'name: PULSE_PLATFORM_DATA_PLANE' 'value: "local"'
if grep -Fq 'name: pulse-platform-nats-box' <<<"$platform_render"; then
  echo "Pi appliance render should disable the NATS toolbox pod" >&2
  exit 1
fi
require_rendered "$services_render" 'image: "registry.example.test/pulse/services@sha256:3333333333333333333333333333333333333333333333333333333333333333"'
require_rendered "$services_render" 'name: ghcr-pull-secret'
require_rendered "$services_render" 'secretName: "pulse-services-gcs-credentials"'
require_rendered "$services_render" 'mountPath: "/var/run/pulse-gcs"'

echo "pulse appliance release values render test passed"
