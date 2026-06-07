#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../.." && pwd)"

command_name="install"
dry_run=0
skip_host_prepare=0
skip_k3s_install=0
skip_platform=0
skip_services=0
skip_wait=0
kubeconfig="${PULSE_APPLIANCE_KUBECONFIG:-${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}}"
kube_context="${KUBECONFIG_CONTEXT:-}"
k3s_version="${PULSE_APPLIANCE_K3S_VERSION:-}"
k3s_channel="${PULSE_APPLIANCE_K3S_CHANNEL:-stable}"
wait_timeout="${WAIT_TIMEOUT:-1800s}"

helm_bin="${HELM:-helm}"
kubectl_bin="${KUBECTL:-kubectl}"
curl_bin="${CURL:-curl}"

platform_release="${PLATFORM_RELEASE:-pulse-platform}"
services_release="${SERVICES_RELEASE:-pulse-services}"
platform_namespace="${PLATFORM_NAMESPACE:-pulse-platform}"
services_namespace="${SERVICES_NAMESPACE:-pulse-services}"
platform_chart="${PLATFORM_CHART:-deploy/charts/pulse-platform}"
services_chart="${SERVICES_CHART:-deploy/charts/pulse-services}"
platform_values="${PI_PLATFORM_VALUES:-deploy/env/pi/values.platform.yaml}"
services_values="${PI_SERVICES_VALUES:-deploy/env/pi/values.services.yaml}"
runtime_secret="${PULSE_SERVICES_RUNTIME_SECRET:-pulse-services-runtime-secret}"
platform_extra_values=()
services_extra_values=()

if [ -n "${PULSE_APPLIANCE_RELEASE_VALUES:-}" ]; then
  platform_extra_values+=("$PULSE_APPLIANCE_RELEASE_VALUES")
  services_extra_values+=("$PULSE_APPLIANCE_RELEASE_VALUES")
fi
if [ -n "${PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES:-}" ]; then
  platform_extra_values+=("$PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES")
fi
if [ -n "${PULSE_APPLIANCE_SERVICES_EXTRA_VALUES:-}" ]; then
  services_extra_values+=("$PULSE_APPLIANCE_SERVICES_EXTRA_VALUES")
fi

usage() {
  cat <<'USAGE'
Usage: pulse-appliance-install.sh [install|upgrade|wait|status] [options]

Installs or upgrades the Pulse Raspberry Pi 5 appliance stack on local K3s.

Options:
  --dry-run             Print the commands that would run.
  --repo-root DIR       Repository root containing deploy/charts and deploy/env.
  --kubeconfig PATH     Kubeconfig path for kubectl and Helm.
  --kube-context NAME   Use an explicit Kubernetes context.
  --k3s-version VER     Install a pinned K3s version instead of a channel.
  --k3s-channel NAME    Install from a K3s channel when no version is set.
  --release-values FILE Apply an install-specific values file to both charts.
  --platform-extra-values FILE
                         Apply an additional install-specific platform values file.
  --services-extra-values FILE
                         Apply an additional install-specific services values file.
  --skip-host-prepare   Do not run Pi host tuning before install.
  --skip-k3s-install    Do not install or upgrade K3s.
  --skip-platform       Do not apply the platform Helm release.
  --skip-services       Do not apply the services Helm release.
  --skip-wait           Do not wait for releases after apply.
  -h, --help            Show this help.

Environment overrides:
  HELM, KUBECTL, CURL, WAIT_TIMEOUT, PLATFORM_RELEASE, SERVICES_RELEASE,
  PLATFORM_NAMESPACE, SERVICES_NAMESPACE, PI_PLATFORM_VALUES, PI_SERVICES_VALUES,
  PULSE_SERVICES_RUNTIME_SECRET, PULSE_APPLIANCE_KUBECONFIG,
  PULSE_APPLIANCE_K3S_VERSION, PULSE_APPLIANCE_K3S_CHANNEL,
  PULSE_APPLIANCE_RELEASE_VALUES, PULSE_APPLIANCE_PLATFORM_EXTRA_VALUES,
  PULSE_APPLIANCE_SERVICES_EXTRA_VALUES
USAGE
}

if [ "$#" -gt 0 ]; then
  case "$1" in
    install|upgrade|wait|status)
      command_name="$1"
      shift
      ;;
  esac
fi

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      dry_run=1
      ;;
    --repo-root)
      repo_root="${2:?--repo-root requires a directory}"
      shift
      ;;
    --kubeconfig)
      kubeconfig="${2:?--kubeconfig requires a path}"
      shift
      ;;
    --kube-context)
      kube_context="${2:?--kube-context requires a value}"
      shift
      ;;
    --k3s-version)
      k3s_version="${2:?--k3s-version requires a value}"
      shift
      ;;
    --k3s-channel)
      k3s_channel="${2:?--k3s-channel requires a value}"
      shift
      ;;
    --release-values)
      platform_extra_values+=("${2:?--release-values requires a file}")
      services_extra_values+=("${2:?--release-values requires a file}")
      shift
      ;;
    --platform-extra-values)
      platform_extra_values+=("${2:?--platform-extra-values requires a file}")
      shift
      ;;
    --services-extra-values)
      services_extra_values+=("${2:?--services-extra-values requires a file}")
      shift
      ;;
    --skip-host-prepare)
      skip_host_prepare=1
      ;;
    --skip-k3s-install)
      skip_k3s_install=1
      ;;
    --skip-platform)
      skip_platform=1
      ;;
    --skip-services)
      skip_services=1
      ;;
    --skip-wait)
      skip_wait=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

repo_root="$(cd "$repo_root" && pwd)"
platform_chart="$repo_root/$platform_chart"
services_chart="$repo_root/$services_chart"
platform_values="$repo_root/$platform_values"
services_values="$repo_root/$services_values"

normalize_values_file() {
  local value_file
  value_file="$1"
  if [[ "$value_file" != /* ]]; then
    value_file="$repo_root/$value_file"
  fi
  printf '%s\n' "$value_file"
}

normalized_values=()
if [ "${#platform_extra_values[@]}" -gt 0 ]; then
  for values_file in "${platform_extra_values[@]}"; do
    normalized_values+=("$(normalize_values_file "$values_file")")
  done
  platform_extra_values=("${normalized_values[@]}")
else
  platform_extra_values=()
fi
normalized_values=()
if [ "${#services_extra_values[@]}" -gt 0 ]; then
  for values_file in "${services_extra_values[@]}"; do
    normalized_values+=("$(normalize_values_file "$values_file")")
  done
  services_extra_values=("${normalized_values[@]}")
else
  services_extra_values=()
fi
unset normalized_values

log() {
  printf '%s\n' "$*"
}

quote_cmd() {
  printf '%q' "$1"
  shift || true
  while [ "$#" -gt 0 ]; do
    printf ' %q' "$1"
    shift
  done
}

run_cmd() {
  if [ "$dry_run" -eq 1 ]; then
    printf '+ '
    quote_cmd "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

require_command() {
  if [ "$dry_run" -eq 1 ]; then
    return 0
  fi
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 not found; install it before running the appliance installer" >&2
    exit 1
  fi
}

kubectl_base() {
  local args=("$kubectl_bin")
  if [ -n "$kubeconfig" ]; then
    args+=(--kubeconfig "$kubeconfig")
  fi
  if [ -n "$kube_context" ]; then
    args+=(--context "$kube_context")
  fi
  printf '%s\0' "${args[@]}"
}

helm_base() {
  local args=("$helm_bin")
  if [ -n "$kubeconfig" ]; then
    args+=(--kubeconfig "$kubeconfig")
  fi
  if [ -n "$kube_context" ]; then
    args+=(--kube-context "$kube_context")
  fi
  printf '%s\0' "${args[@]}"
}

run_kubectl() {
  local base=()
  while IFS= read -r -d '' item; do
    base+=("$item")
  done < <(kubectl_base)
  run_cmd "${base[@]}" "$@"
}

run_helm() {
  local base=()
  while IFS= read -r -d '' item; do
    base+=("$item")
  done < <(helm_base)
  run_cmd "${base[@]}" "$@"
}

kubectl_capture() {
  local base=()
  while IFS= read -r -d '' item; do
    base+=("$item")
  done < <(kubectl_base)
  "${base[@]}" "$@"
}

run_as_root() {
  if [ "$dry_run" -eq 1 ]; then
    if [ "${EUID:-$(id -u)}" -eq 0 ]; then
      run_cmd "$@"
    else
      run_cmd sudo "$@"
    fi
    return 0
  fi
  if [ "${EUID:-$(id -u)}" -eq 0 ]; then
    "$@"
  else
    require_command sudo
    sudo "$@"
  fi
}

ensure_files() {
  local file
  for file in "$platform_values" "$services_values" "$platform_chart/Chart.yaml" "$services_chart/Chart.yaml"; do
    if [ ! -f "$file" ]; then
      echo "missing appliance input: $file" >&2
      exit 1
    fi
  done
  if [ "${#platform_extra_values[@]}" -gt 0 ]; then
    for file in "${platform_extra_values[@]}"; do
      if [ ! -f "$file" ]; then
        echo "missing appliance extra values file: $file" >&2
        exit 1
      fi
    done
  fi
  if [ "${#services_extra_values[@]}" -gt 0 ]; then
    for file in "${services_extra_values[@]}"; do
      if [ ! -f "$file" ]; then
        echo "missing appliance extra values file: $file" >&2
        exit 1
      fi
    done
  fi
}

prepare_host() {
  if [ "$skip_host_prepare" -eq 1 ]; then
    log "skipping host preparation"
    return
  fi
  log "applying host tuning"
  run_as_root "$script_dir/pulse-appliance-host-prepare.sh"
}

install_k3s() {
  if [ "$skip_k3s_install" -eq 1 ]; then
    log "skipping K3s install"
    return
  fi
  require_command "$curl_bin"
  local installer
  installer="${TMPDIR:-/tmp}/pulse-k3s-install.sh"
  log "installing or upgrading K3s"
  run_cmd "$curl_bin" -sfL -o "$installer" https://get.k3s.io
  if [ -n "$k3s_version" ]; then
    run_as_root env INSTALL_K3S_VERSION="$k3s_version" sh "$installer" server
  else
    run_as_root env INSTALL_K3S_CHANNEL="$k3s_channel" sh "$installer" server
  fi
}

ensure_namespace() {
  local namespace="$1"
  if [ "$dry_run" -eq 1 ]; then
    run_kubectl create namespace "$namespace"
    return
  fi
  if ! kubectl_capture get namespace "$namespace" >/dev/null 2>&1; then
    run_kubectl create namespace "$namespace"
  fi
}

build_chart_dependencies() {
  run_cmd "$helm_bin" dependency build --skip-refresh "$platform_chart"
  run_cmd "$helm_bin" dependency build --skip-refresh "$services_chart"
}

platform_keycloak_first_pass_needed() {
  if [ "$dry_run" -eq 1 ]; then
    return 0
  fi
  ! kubectl_capture -n "$platform_namespace" get statefulset "$platform_release-keycloak" >/dev/null 2>&1
}

apply_platform_once() {
  local args=(
    upgrade --install "$platform_release" "$platform_chart"
    --namespace "$platform_namespace"
    --create-namespace
    --wait
    --timeout "$wait_timeout"
    -f "$platform_values"
  )
  local values_file
  if [ "${#platform_extra_values[@]}" -gt 0 ]; then
    for values_file in "${platform_extra_values[@]}"; do
      args+=(-f "$values_file")
    done
  fi
  for values_file in "$@"; do
    args+=(-f "$values_file")
  done
  run_helm "${args[@]}"
}

render_helm_template() {
  local release="$1"
  local chart="$2"
  local namespace="$3"
  shift 3

  "$helm_bin" template "$release" "$chart" --namespace "$namespace" "$@"
}

fail_on_placeholder_images() {
  local release="$1"
  local chart="$2"
  local namespace="$3"
  shift 3
  if [ "$dry_run" -eq 1 ]; then
    return 0
  fi

  local rendered
  rendered="$(render_helm_template "$release" "$chart" "$namespace" "$@")"
  if grep -Eq 'image:[[:space:]]*"?[^"]*pi-placeholder' <<<"$rendered"; then
    cat >&2 <<EOF
refusing to apply $namespace/$release because one or more rendered images still
use the appliance placeholder tag "pi-placeholder".

Provide install-specific image tags or digests with --release-values,
--platform-extra-values, --services-extra-values, or their environment
equivalents before running the appliance install or upgrade.
EOF
    exit 1
  fi
}

wait_endpoint_required() {
  local namespace="$1"
  local name="$2"
  local label="$3"
  if [ "$dry_run" -eq 1 ]; then
    run_kubectl -n "$namespace" get endpoints "$name"
    return
  fi
  local endpoint_ips
  for _ in $(seq 1 36); do
    endpoint_ips="$(kubectl_capture -n "$namespace" get endpoints "$name" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true)"
    if [ -n "$endpoint_ips" ]; then
      log "$label endpoints ready: $endpoint_ips"
      return
    fi
    sleep 5
  done
  echo "$label endpoints did not become ready" >&2
  exit 1
}

wait_condition_if_exists() {
  local namespace="$1"
  local kind="$2"
  local name="$3"
  local condition="$4"
  local timeout="$5"
  if [ "$dry_run" -eq 1 ]; then
    run_kubectl -n "$namespace" wait --for="condition=$condition" "$kind/$name" --timeout="$timeout"
    return
  fi
  if kubectl_capture -n "$namespace" get "$kind" "$name" >/dev/null 2>&1; then
    run_kubectl -n "$namespace" wait --for="condition=$condition" "$kind/$name" --timeout="$timeout"
  fi
}

wait_rollout_if_exists() {
  local namespace="$1"
  local resource="$2"
  local timeout="$3"
  if [ "$dry_run" -eq 1 ]; then
    run_kubectl -n "$namespace" rollout status "$resource" --timeout="$timeout"
    return
  fi
  if kubectl_capture -n "$namespace" get "$resource" >/dev/null 2>&1; then
    run_kubectl -n "$namespace" rollout status "$resource" --timeout="$timeout"
  fi
}

wait_platform() {
  run_kubectl wait --for=condition=Ready node --all --timeout="$wait_timeout"
  wait_rollout_if_exists "$platform_namespace" "deployment/$platform_release-cloudnative-pg" 180s
  wait_condition_if_exists "$platform_namespace" cluster.postgresql.cnpg.io "$platform_release-core" Ready "$wait_timeout"
  wait_rollout_if_exists "$platform_namespace" "statefulset/$platform_release-nats" "$wait_timeout"
  wait_rollout_if_exists "$platform_namespace" "statefulset/$platform_release-valkey-primary" "$wait_timeout"
  wait_rollout_if_exists "$platform_namespace" "statefulset/$platform_release-keycloak" "$wait_timeout"
  wait_rollout_if_exists "$platform_namespace" "deployment/$platform_release-ingress-nginx-controller" 300s
  wait_rollout_if_exists "$platform_namespace" "deployment/$platform_release-cert-manager" 300s
  wait_rollout_if_exists "$platform_namespace" "deployment/$platform_release-cert-manager-webhook" 300s
  wait_rollout_if_exists "$platform_namespace" "deployment/$platform_release-cert-manager-cainjector" 300s
  wait_endpoint_required "$platform_namespace" "$platform_release-core-rw" "CNPG rw service"
  wait_endpoint_required "$platform_namespace" "$platform_release-nats" "NATS service"
  wait_endpoint_required "$platform_namespace" "$platform_release-valkey-primary" "Valkey service"
  wait_endpoint_required "$platform_namespace" "$platform_release-keycloak-headless" "Keycloak service"
}

apply_platform() {
  if [ "$skip_platform" -eq 1 ]; then
    log "skipping platform Helm release"
    return
  fi
  local platform_template_args=(-f "$platform_values")
  local values_file
  if [ "${#platform_extra_values[@]}" -gt 0 ]; then
    for values_file in "${platform_extra_values[@]}"; do
      platform_template_args+=(-f "$values_file")
    done
  fi
  fail_on_placeholder_images "$platform_release" "$platform_chart" "$platform_namespace" "${platform_template_args[@]}"
  ensure_namespace "$platform_namespace"
  if platform_keycloak_first_pass_needed; then
    local bootstrap_values
    bootstrap_values="$(mktemp "${TMPDIR:-/tmp}/pulse-pi-platform-bootstrap.XXXXXX.yaml")"
    cat >"$bootstrap_values" <<'YAML'
components:
  keycloak:
    enabled: false
keycloakRealm:
  enabled: false
YAML
    log "running platform bootstrap pass without Keycloak"
    apply_platform_once "$bootstrap_values"
    wait_condition_if_exists "$platform_namespace" cluster.postgresql.cnpg.io "$platform_release-core" Ready "$wait_timeout"
    wait_endpoint_required "$platform_namespace" "$platform_release-core-rw" "CNPG rw service"
    rm -f "$bootstrap_values"
  fi
  log "applying platform Helm release"
  apply_platform_once
}

runtime_secret_exists() {
  if [ "$dry_run" -eq 1 ]; then
    run_kubectl -n "$services_namespace" get secret "$runtime_secret"
    return 0
  fi
  kubectl_capture -n "$services_namespace" get secret "$runtime_secret" >/dev/null 2>&1
}

require_runtime_secret() {
  if runtime_secret_exists; then
    return
  fi
  cat >&2 <<EOF
missing Kubernetes secret $services_namespace/$runtime_secret

Create it before applying services. At minimum it must provide the runtime
secret keys expected by deploy/env/pi/values.services.yaml, including the GCS
archive settings and provider credentials for this appliance install.
EOF
  exit 1
}

apply_services() {
  if [ "$skip_services" -eq 1 ]; then
    log "skipping services Helm release"
    return
  fi
  local services_template_args=(-f "$services_values")
  local services_helm_args=()
  local values_file
  if [ "${#services_extra_values[@]}" -gt 0 ]; then
    for values_file in "${services_extra_values[@]}"; do
      services_template_args+=(-f "$values_file")
      services_helm_args+=(-f "$values_file")
    done
  fi
  fail_on_placeholder_images "$services_release" "$services_chart" "$services_namespace" "${services_template_args[@]}"
  ensure_namespace "$services_namespace"
  wait_endpoint_required "$platform_namespace" "$platform_release-core-rw" "CNPG rw service"
  wait_endpoint_required "$platform_namespace" "$platform_release-nats" "NATS service"
  wait_endpoint_required "$platform_namespace" "$platform_release-valkey-primary" "Valkey service"
  wait_endpoint_required "$platform_namespace" "$platform_release-keycloak-headless" "Keycloak service"
  require_runtime_secret
  log "applying services Helm release"
  local helm_apply_args=(
    upgrade --install "$services_release" "$services_chart"
    --namespace "$services_namespace"
    --create-namespace
    --wait
    --timeout "$wait_timeout"
    -f "$services_values"
  )
  if [ "${#services_helm_args[@]}" -gt 0 ]; then
    helm_apply_args+=("${services_helm_args[@]}")
  fi
  run_helm "${helm_apply_args[@]}"
}

wait_services() {
  if [ "$dry_run" -eq 1 ]; then
    run_kubectl -n "$services_namespace" rollout status "deployment/$services_release-go-grpc-api" --timeout="$wait_timeout"
    run_kubectl -n "$services_namespace" get deploy -l "app.kubernetes.io/instance=$services_release"
    return
  fi
  if ! kubectl_capture get namespace "$services_namespace" >/dev/null 2>&1; then
    log "namespace $services_namespace does not exist yet; skipping services wait"
    return
  fi
  local deployments
  deployments="$(kubectl_capture -n "$services_namespace" get deploy -l "app.kubernetes.io/instance=$services_release" -o name 2>/dev/null || true)"
  if [ -z "$deployments" ]; then
    log "no services deployments found for $services_release"
    return
  fi
  local deploy
  while IFS= read -r deploy; do
    [ -n "$deploy" ] || continue
    run_kubectl -n "$services_namespace" rollout status "$deploy" --timeout="$wait_timeout"
  done <<<"$deployments"
}

run_status() {
  local args=()
  if [ -n "$kubeconfig" ]; then
    args+=(--kubeconfig "$kubeconfig")
  fi
  if [ -n "$kube_context" ]; then
    args+=(--kube-context "$kube_context")
  fi
  if [ "$dry_run" -eq 1 ]; then
    run_cmd "$script_dir/pulse-appliance-status.sh" "${args[@]}"
    return
  fi
  "$script_dir/pulse-appliance-status.sh" "${args[@]}"
}

run_install_or_upgrade() {
  ensure_files
  require_command "$helm_bin"
  require_command "$kubectl_bin"
  prepare_host
  install_k3s
  build_chart_dependencies
  apply_platform
  apply_services
  if [ "$skip_wait" -eq 0 ]; then
    wait_platform
    wait_services
  fi
}

case "$command_name" in
  install|upgrade)
    run_install_or_upgrade
    ;;
  wait)
    wait_platform
    wait_services
    ;;
  status)
    run_status
    ;;
  *)
    echo "unknown command: $command_name" >&2
    usage >&2
    exit 2
    ;;
esac
