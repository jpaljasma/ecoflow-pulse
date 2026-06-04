#!/usr/bin/env bash
set -euo pipefail

free_warn_gib=80
kubeconfig="${PULSE_APPLIANCE_KUBECONFIG:-${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}}"
kube_context="${KUBECONFIG_CONTEXT:-}"
platform_ns="${PULSE_PLATFORM_NAMESPACE:-pulse-platform}"
services_ns="${PULSE_SERVICES_NAMESPACE:-pulse-services}"
archive_outbox_dir="${ARCHIVE_UPLOAD_OUTBOX_DIR:-/var/lib/pulse-archive/outbox}"
archive_outbox_status_bin="${ARCHIVE_OUTBOX_STATUS_BIN:-/app/ecoflow-archive-outbox-status}"
failures=0
warnings=0

usage() {
  cat <<'USAGE'
Usage: pulse-appliance-status.sh [--kubeconfig PATH] [--kube-context CONTEXT] [--free-warn-gib N] [--archive-outbox-dir DIR]

Runs conservative host and K3s health checks for a Pulse Raspberry Pi appliance.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --kubeconfig)
      kubeconfig="${2:?--kubeconfig requires a path}"
      shift
      ;;
    --kube-context)
      kube_context="${2:?--kube-context requires a value}"
      shift
      ;;
    --free-warn-gib)
      free_warn_gib="${2:?--free-warn-gib requires a value}"
      shift
      ;;
    --archive-outbox-dir)
      archive_outbox_dir="${2:?--archive-outbox-dir requires a value}"
      shift
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

ok() {
  printf 'ok: %s\n' "$*"
}

warn() {
  warnings=$((warnings + 1))
  printf 'warn: %s\n' "$*" >&2
}

fail() {
  failures=$((failures + 1))
  printf 'fail: %s\n' "$*" >&2
}

have() {
  command -v "$1" >/dev/null 2>&1
}

kubectl_cmd() {
  local args=(kubectl)
  if [ -n "$kubeconfig" ]; then
    args+=(--kubeconfig "$kubeconfig")
  fi
  if [ -n "$kube_context" ]; then
    args+=(--context "$kube_context")
  fi
  "${args[@]}" "$@"
}

check_throttling() {
  if ! have vcgencmd; then
    warn "vcgencmd not found; skipping Raspberry Pi throttling checks"
    return
  fi
  local throttled temp
  throttled="$(vcgencmd get_throttled || true)"
  temp="$(vcgencmd measure_temp || true)"
  case "$throttled" in
    *0x0*) ok "no Pi throttling reported ($throttled, $temp)" ;;
    *) fail "Pi throttling flags are set ($throttled, $temp)" ;;
  esac
}

check_root_disk() {
  local available_kib available_gib
  available_kib="$(df -Pk / | awk 'NR == 2 {print $4}')"
  available_gib=$((available_kib / 1024 / 1024))
  if [ "$available_gib" -lt "$free_warn_gib" ]; then
    warn "root filesystem free space is ${available_gib}GiB, below ${free_warn_gib}GiB warning threshold"
  else
    ok "root filesystem free space is ${available_gib}GiB"
  fi
}

check_nvme() {
  if ! have nvme; then
    warn "nvme CLI not found; skipping NVMe SMART checks"
    return
  fi
  if [ ! -e /dev/nvme0n1 ]; then
    fail "/dev/nvme0n1 not found"
    return
  fi
  local smart
  smart="$(nvme smart-log /dev/nvme0n1 2>/dev/null || true)"
  if [ -z "$smart" ]; then
    fail "unable to read NVMe SMART log"
    return
  fi
  printf '%s\n' "$smart" | awk '
    /critical_warning/ { warning=$3 }
    /temperature/ && temp == "" { temp=$3 " " $4 }
    /unsafe_shutdowns/ { unsafe=$3 }
    END {
      printf "ok: NVMe SMART critical_warning=%s temperature=%s unsafe_shutdowns=%s\n", warning, temp, unsafe
    }'
}

check_k3s() {
  if ! have systemctl; then
    warn "systemctl not found; skipping k3s service check"
  elif systemctl is-active --quiet k3s; then
    ok "k3s service is active"
  else
    fail "k3s service is not active"
  fi
  if ! have kubectl; then
    warn "kubectl not found; skipping Kubernetes checks"
    return
  fi
  if kubectl_cmd get nodes >/dev/null 2>&1; then
    ok "Kubernetes API is reachable"
  else
    fail "Kubernetes API is not reachable"
    return
  fi
  if kubectl_cmd wait --for=condition=Ready node --all --timeout=10s >/dev/null 2>&1; then
    ok "all Kubernetes nodes are Ready"
  else
    fail "not all Kubernetes nodes are Ready"
  fi
}

check_helm_release() {
  local release="$1"
  local namespace="$2"
  if ! have helm; then
    warn "helm not found; skipping Helm release checks"
    return
  fi
  local args=(status "$release" --namespace "$namespace")
  if [ -n "$kubeconfig" ]; then
    args=(--kubeconfig "$kubeconfig" "${args[@]}")
  fi
  if [ -n "$kube_context" ]; then
    args=(--kube-context "$kube_context" "${args[@]}")
  fi
  if helm "${args[@]}" >/dev/null 2>&1; then
    ok "Helm release $namespace/$release exists"
  else
    fail "Helm release $namespace/$release is missing"
  fi
}

check_loopback_grpc() {
  if have nc; then
    if nc -z 127.0.0.1 19090 >/dev/null 2>&1; then
      ok "loopback gRPC port 127.0.0.1:19090 is reachable"
    else
      warn "loopback gRPC port 127.0.0.1:19090 is not reachable yet"
    fi
  else
    warn "nc not found; skipping loopback gRPC port check"
  fi
}

check_archive_upload_outbox() {
  if ! have kubectl; then
    warn "kubectl not found; skipping archive upload outbox check"
    return
  fi
  local pod
  pod="$(kubectl_cmd -n "$services_ns" get pod \
    -l "app.kubernetes.io/instance=pulse-services,app.kubernetes.io/component=go-archive" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -z "$pod" ]; then
    warn "archive worker pod not found; skipping archive upload outbox check"
    return
  fi
  local output status
  set +e
  output="$(kubectl_cmd -n "$services_ns" exec "$pod" -- \
    "$archive_outbox_status_bin" --dir "$archive_outbox_dir" --fail-on-pending 2>&1)"
  status=$?
  set -e
  case "$status" in
    0)
      ok "archive upload outbox clear ($output)"
      ;;
    2)
      fail "archive upload outbox has pending local entries ($output)"
      ;;
    *)
      warn "unable to check archive upload outbox in pod $pod ($output)"
      ;;
  esac
}

check_throttling
check_root_disk
check_nvme
check_k3s
check_helm_release pulse-platform "$platform_ns"
check_helm_release pulse-services "$services_ns"
check_loopback_grpc
check_archive_upload_outbox

printf 'summary: %d failure(s), %d warning(s)\n' "$failures" "$warnings"
[ "$failures" -eq 0 ]
