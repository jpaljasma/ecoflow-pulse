#!/usr/bin/env bash
set -euo pipefail

free_warn_gib=80
kube_context="${KUBECONFIG_CONTEXT:-}"
platform_ns="${PULSE_PLATFORM_NAMESPACE:-pulse-platform}"
services_ns="${PULSE_SERVICES_NAMESPACE:-pulse-services}"
failures=0
warnings=0

usage() {
  cat <<'USAGE'
Usage: pulse-appliance-status.sh [--kube-context CONTEXT] [--free-warn-gib N]

Runs conservative host and K3s health checks for a Pulse Raspberry Pi appliance.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --kube-context)
      kube_context="${2:?--kube-context requires a value}"
      shift
      ;;
    --free-warn-gib)
      free_warn_gib="${2:?--free-warn-gib requires a value}"
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
  if [ -n "$kube_context" ]; then
    kubectl --context "$kube_context" "$@"
  else
    kubectl "$@"
  fi
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

check_throttling
check_root_disk
check_nvme
check_k3s
check_helm_release pulse-platform "$platform_ns"
check_helm_release pulse-services "$services_ns"
check_loopback_grpc

printf 'summary: %d failure(s), %d warning(s)\n' "$failures" "$warnings"
[ "$failures" -eq 0 ]
