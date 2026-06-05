#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
helm_bin="${HELM:-helm}"
tmp_values="$(mktemp "${TMPDIR:-/tmp}/pulse-pi-release-values.XXXXXX.yaml")"
trap 'rm -f "$tmp_values"' EXIT

cat >"$tmp_values" <<'YAML'
runtime:
  publicApp:
    image:
      repository: registry.example.test/pulse-platform
      tag: ""
      digest: sha256:1111111111111111111111111111111111111111111111111111111111111111
  realtimeGateway:
    image:
      repository: registry.example.test/pulse-realtime-gateway
      tag: ""
      digest: sha256:2222222222222222222222222222222222222222222222222222222222222222
  image:
    repository: registry.example.test/services
    tag: ""
    digest: sha256:3333333333333333333333333333333333333333333333333333333333333333
  gcsCredentials:
    enabled: true
    secretName: pulse-services-gcs-credentials
    secretKey: key.json
    fileName: key.json
    mountPath: /var/run/pulse-gcs
YAML

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

require_rendered "$platform_render" 'image: "registry.example.test/pulse-platform@sha256:1111111111111111111111111111111111111111111111111111111111111111"'
require_rendered "$platform_render" 'image: "registry.example.test/pulse-realtime-gateway@sha256:2222222222222222222222222222222222222222222222222222222222222222"'
require_rendered "$services_render" 'image: "registry.example.test/services@sha256:3333333333333333333333333333333333333333333333333333333333333333"'
require_rendered "$services_render" 'secretName: "pulse-services-gcs-credentials"'
require_rendered "$services_render" 'mountPath: "/var/run/pulse-gcs"'

echo "pulse appliance release values render test passed"
