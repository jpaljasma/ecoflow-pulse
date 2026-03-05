#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
FLOW_FILE="${SCRIPT_DIR}/maestro/smoke.yaml"

# Maestro installer places binaries here; include it so non-interactive shells work.
if [ -d "${HOME}/.maestro/bin" ]; then
  PATH="${PATH}:${HOME}/.maestro/bin"
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

echo "running Maestro mobile smoke flow"
echo "  app id: ${app_id}"
echo "  expo url: ${expo_url}"

MAESTRO_APP_ID="${app_id}" MAESTRO_EXPO_URL="${expo_url}" maestro test "${FLOW_FILE}"
