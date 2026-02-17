#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." >/dev/null 2>&1 && pwd)"

CSV_PATH="${1:-${PANEL_SELECT_TRAINING_CSV:-${REPO_ROOT}/logs/telemetry_training.csv}}"
OUT_PATH="${2:-${PANEL_SELECT_MODEL_OUT:-${REPO_ROOT}/data/solar_panels/panel_select_model.json}}"
PANEL_MAP="${3:-${PANEL_SELECT_PANEL_MAP:-}}"

if [[ ! -f "${CSV_PATH}" ]]; then
  echo "error: telemetry csv not found: ${CSV_PATH}" >&2
  echo "usage: $0 [training_csv] [output_model_json] [optional_panel_map_json]" >&2
  exit 1
fi

cd "${REPO_ROOT}"
if [[ -n "${PANEL_MAP}" ]]; then
  go run ./cmd/ecoflow-panel-select-train \
    -csv "${CSV_PATH}" \
    -out "${OUT_PATH}" \
    -panel-map "${PANEL_MAP}" \
    -replay
else
  go run ./cmd/ecoflow-panel-select-train \
    -csv "${CSV_PATH}" \
    -out "${OUT_PATH}" \
    -replay
fi

echo
echo "Panel select model trained:"
echo "  - ${OUT_PATH}"
