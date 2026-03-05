#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
FLOW_FILE="${SCRIPT_DIR}/maestro/smoke.yaml"

# Maestro installer places binaries here; include it so non-interactive shells work.
if [ -d "${HOME}/.maestro/bin" ]; then
  PATH="${PATH}:${HOME}/.maestro/bin"
fi
if [ -d "/opt/homebrew/opt/openjdk@17/bin" ]; then
  PATH="/opt/homebrew/opt/openjdk@17/bin:${PATH}"
fi

if ! command -v maestro >/dev/null 2>&1; then
  echo "maestro CLI not found." >&2
  echo "Install it with: curl -Ls \"https://get.maestro.mobile.dev\" | bash" >&2
  exit 1
fi

if ! command -v java >/dev/null 2>&1 || ! java -version >/dev/null 2>&1; then
  echo "Java runtime not found. Maestro requires Java 17+." >&2
  echo "Install a JDK and ensure 'java -version' works before rerunning." >&2
  exit 1
fi

app_id="${MAESTRO_APP_ID:-host.exp.Exponent}"
expo_url="${MAESTRO_EXPO_URL:-exp://127.0.0.1:8081}"
rendered_flow="$(mktemp)"
mock_api_pid=""
mock_ws_pid=""

is_port_listening() {
  local port="$1"
  if command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "${port}" >/dev/null 2>&1
    return
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
    return
  fi
  return 1
}

cleanup() {
  rm -f "${rendered_flow}"
  if [ -n "${mock_api_pid}" ]; then
    kill "${mock_api_pid}" >/dev/null 2>&1 || true
  fi
  if [ -n "${mock_ws_pid}" ]; then
    kill "${mock_ws_pid}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "running Maestro mobile smoke flow"
echo "  app id: ${app_id}"
echo "  expo url: ${expo_url}"

if ! is_port_listening 18081; then
  echo "  starting local mock API on 127.0.0.1:18081"
  node "${SCRIPT_DIR}/mock_api_server.js" >/tmp/pulse-maestro-mock-api.log 2>&1 &
  mock_api_pid="$!"
  sleep 1
  if ! kill -0 "${mock_api_pid}" >/dev/null 2>&1; then
    echo "mock API failed to start. See /tmp/pulse-maestro-mock-api.log" >&2
    exit 1
  fi
fi

if ! is_port_listening 8082; then
  echo "  starting local mock WS gateway on 127.0.0.1:8082"
  node "${SCRIPT_DIR}/mock_ws_server.js" >/tmp/pulse-maestro-mock-ws.log 2>&1 &
  mock_ws_pid="$!"
  sleep 1
  if ! kill -0 "${mock_ws_pid}" >/dev/null 2>&1; then
    echo "mock WS server failed to start. See /tmp/pulse-maestro-mock-ws.log" >&2
    exit 1
  fi
fi

if command -v xcrun >/dev/null 2>&1; then
  if xcrun simctl list devices | grep -q "Booted"; then
    echo "  priming booted iOS simulator via simctl openurl"
    xcrun simctl openurl booted "${expo_url}" >/dev/null 2>&1 || true
    sleep 2
  fi
fi

sed \
  -e "s|__MAESTRO_APP_ID__|${app_id}|g" \
  -e "s|__MAESTRO_EXPO_URL__|${expo_url}|g" \
  "${FLOW_FILE}" > "${rendered_flow}"

maestro test "${rendered_flow}"
