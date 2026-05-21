#!/usr/bin/env sh
set -eu

normalize_requested_mode() {
  case "${1:-auto}" in
    auto|"")
      printf '%s' auto
      ;;
    local)
      printf '%s' local
      ;;
    cloud|local-edge|local_edge)
      printf '%s' local-edge
      ;;
    *)
      echo "unsupported DEV_DEPLOY_DATA_MODE=$1 (expected auto, local, or local-edge)" >&2
      exit 2
      ;;
  esac
}

normalize_data_plane() {
  case "${1:-}" in
    cloud|local-edge|local_edge)
      printf '%s' local-edge
      ;;
    local)
      printf '%s' local
      ;;
    *)
      printf '%s' ''
      ;;
  esac
}

requested_mode="$(normalize_requested_mode "${DEV_DEPLOY_DATA_MODE:-auto}")"
if [ "$requested_mode" != "auto" ]; then
  printf '%s\n' "$requested_mode"
  exit 0
fi

expo_mode="$(normalize_data_plane "${EXPO_PUBLIC_LOCAL_DATA_PLANE:-}")"
if [ -n "$expo_mode" ]; then
  printf '%s\n' "$expo_mode"
  exit 0
fi

kubectl_bin="${KUBECTL:-kubectl}"
kube_context="${K3D_CONTEXT:-k3d-pulse-local}"
platform_namespace="${PLATFORM_NAMESPACE:-pulse-platform}"
services_namespace="${SERVICES_NAMESPACE:-pulse-services}"
platform_release="${PLATFORM_RELEASE:-pulse-platform}"
services_release="${SERVICES_RELEASE:-pulse-services}"
public_app_deploy="${PLATFORM_PUBLIC_APP_DEPLOYMENT:-${platform_release}-public-app}"
services_runtime_configmap="${SERVICES_RUNTIME_CONFIGMAP:-${services_release}-runtime-env}"

current_public_data_plane="$(
  "$kubectl_bin" --context "$kube_context" -n "$platform_namespace" \
    get "deploy/$public_app_deploy" \
    -o 'jsonpath={.spec.template.spec.containers[0].env[?(@.name=="PULSE_PLATFORM_DATA_PLANE")].value}' \
    2>/dev/null || true
)"
if [ "$(normalize_data_plane "$current_public_data_plane")" = "local-edge" ]; then
  printf '%s\n' local-edge
  exit 0
fi

current_projection_prefix="$(
  "$kubectl_bin" --context "$kube_context" -n "$services_namespace" \
    get "configmap/$services_runtime_configmap" \
    -o 'jsonpath={.data.PROJECTION_KEY_PREFIX}' \
    2>/dev/null || true
)"
case "$current_projection_prefix" in
  *cloud*)
    printf '%s\n' local-edge
    exit 0
    ;;
esac

printf '%s\n' local
