#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." >/dev/null 2>&1 && pwd)"

DEFAULT_CSV="/Users/jpaljasma/Downloads/solar_panel_specs_with_ecoflow_compat_cold_voc_and_safety_margins_v13.csv"
CSV_PATH="${1:-${SOLAR_PANEL_CSV:-${DEFAULT_CSV}}}"
OUT_DIR="${2:-${SOLAR_PANEL_OUT_DIR:-${REPO_ROOT}/data/solar_panels}}"

OUT_JSON="${OUT_DIR}/solar_panel_specs_v13.json"
SUMMARY_JSON="${OUT_DIR}/solar_panel_specs_v13.summary.json"
INDEX_JSON="${OUT_DIR}/solar_panel_specs_v13.index.json"

if [[ ! -f "${CSV_PATH}" ]]; then
  echo "error: csv file not found: ${CSV_PATH}" >&2
  echo "usage: $0 [csv_path] [output_dir]" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"

cd "${REPO_ROOT}"
go run ./cmd/ecoflow-panel-db-import \
  -csv "${CSV_PATH}" \
  -out "${OUT_JSON}" \
  -summary-out "${SUMMARY_JSON}" \
  -index-out "${INDEX_JSON}"

echo
echo "Regenerated solar panel artifacts:"
echo "  - ${OUT_JSON}"
echo "  - ${SUMMARY_JSON}"
echo "  - ${INDEX_JSON}"
