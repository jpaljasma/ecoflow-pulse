#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 nats|postgres|valkey <local-port>" >&2
}

if [ "$#" -ne 2 ]; then
  usage
  exit 2
fi

probe="$1"
local_port="$2"
timeout_sec="${CLOUD_FORWARD_PROBE_TIMEOUT_SEC:-2}"

case "$probe" in
  nats)
    output="$(printf 'PING\r\n' | nc -w "$timeout_sec" 127.0.0.1 "$local_port" 2>/dev/null || true)"
    printf '%s\n' "$output" | grep -Eq '^(INFO|PONG)'
    ;;
  postgres)
    output="$(printf '\000\000\000\010\004\322\026\057' | nc -w "$timeout_sec" 127.0.0.1 "$local_port" 2>/dev/null | head -c 1 || true)"
    [ "$output" = "S" ] || [ "$output" = "N" ]
    ;;
  valkey)
    output="$(printf '*1\r\n$4\r\nPING\r\n' | nc -w "$timeout_sec" 127.0.0.1 "$local_port" 2>/dev/null || true)"
    printf '%s\n' "$output" | grep -q '^+PONG'
    ;;
  *)
    usage
    exit 2
    ;;
esac
