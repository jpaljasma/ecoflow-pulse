#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <label> nats|postgres|valkey <local-port> <command> [args...]" >&2
}

if [ "$#" -lt 4 ]; then
  usage
  exit 2
fi

label="$1"
probe="$2"
local_port="$3"
shift 3

probe_bin="${CLOUD_FORWARD_PROBE_BIN:-cloud-forward-probe.sh}"
interval_sec="${CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC:-10}"
restart_delay_sec="${CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC:-2}"
startup_grace_sec="${CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC:-5}"
failure_threshold="${CLOUD_FORWARD_SUPERVISOR_FAILURE_THRESHOLD:-1}"
max_cycles="${CLOUD_FORWARD_SUPERVISOR_MAX_CYCLES:-0}"
child_pid=""
child_started_at=0
failure_count=0

stop_child() {
  if [ -n "$child_pid" ] && kill -0 "$child_pid" >/dev/null 2>&1; then
    kill "$child_pid" >/dev/null 2>&1 || true
    wait "$child_pid" >/dev/null 2>&1 || true
  fi
  child_pid=""
}

cleanup() {
  stop_child
}

trap cleanup EXIT
trap 'cleanup; exit 0' INT TERM

start_child() {
  echo "$label supervisor starting child: $*"
  "$@" &
  child_pid="$!"
  child_started_at="$(date +%s)"
  failure_count=0
}

restart_child() {
  echo "$label supervisor restarting child"
  stop_child
  sleep "$restart_delay_sec"
  start_child "$@"
}

start_child "$@"

cycles=0
while :; do
  sleep "$interval_sec"
  cycles=$((cycles + 1))

  if [ -z "$child_pid" ] || ! kill -0 "$child_pid" >/dev/null 2>&1; then
    echo "$label supervisor child exited"
    restart_child "$@"
  elif "$probe_bin" "$probe" "$local_port"; then
    echo "$label supervisor probe ok"
    failure_count=0
  else
    now="$(date +%s)"
    if [ $((now - child_started_at)) -lt "$startup_grace_sec" ]; then
      echo "$label supervisor probe failed during startup grace for 127.0.0.1:$local_port"
      failure_count=0
    else
      failure_count=$((failure_count + 1))
      echo "$label supervisor probe failed for 127.0.0.1:$local_port ($failure_count/$failure_threshold)"
      if [ "$failure_count" -ge "$failure_threshold" ]; then
        restart_child "$@"
      fi
    fi
  fi

  if [ "$max_cycles" -gt 0 ] && [ "$cycles" -ge "$max_cycles" ]; then
    exit 0
  fi
done
