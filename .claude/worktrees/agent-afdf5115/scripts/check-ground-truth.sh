#!/usr/bin/env bash
# check-ground-truth.sh — Verify METRICS file matches codebase reality.
#
# Computes project metrics from code and compares against claims in the
# METRICS file. Catches "silent success" drift where everything works but
# documented numbers describe a different reality.
#
# CUSTOMIZATION REQUIRED: Define your project's metrics in compute_metrics().
#
# Usage: ./scripts/check-ground-truth.sh
# Exit: 0 = all claims match, 1 = drift detected

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
METRICS_FILE="$REPO_ROOT/METRICS"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
exit_code=0

# ── CUSTOMIZE: Define your project's metrics ──
#
# Each metric is a key=value pair echoed to stdout.
# Keys must match entries in the METRICS file.
#
# By convention, these locations are reliable and countable:
#   tests/          — all test files live here
#   architecture/   — all contracts live here
#   src/            — source code with CONTRACT: headers
#
compute_metrics() {
  # Examples (uncomment and adjust for your project):
  #
  # echo "unit_test_files=$(find tests/ -name '*.test.ts' -o -name '*.test.tsx' 2>/dev/null | wc -l | tr -d ' ')"
  # echo "e2e_spec_files=$(find tests/e2e/ -name '*.spec.ts' 2>/dev/null | wc -l | tr -d ' ')"
  # echo "contracts=$(ls architecture/CONTRACT-*.md 2>/dev/null | wc -l | tr -d ' ')"
  # echo "api_route_modules=$(find src/routes/ -name '*.ts' -not -name 'index.ts' -not -name '*.test.ts' 2>/dev/null | wc -l | tr -d ' ')"

  :  # no-op — remove this line when adding metrics
}

# ── Verification engine (do not modify below this line) ──

verify() {
  if [ ! -f "$METRICS_FILE" ]; then
    echo -e "${YELLOW}SKIP${NC}: No METRICS file found"
    echo "  Create a METRICS file with key = value pairs."
    echo "  See scripts/check-ground-truth.sh for examples."
    return 0
  fi

  local computed
  computed=$(compute_metrics)

  if [ -z "$computed" ]; then
    echo "No metrics defined. Customize compute_metrics() in this script."
    return 0
  fi

  while IFS='=' read -r key value; do
    # Skip empty lines and comments
    [ -z "$key" ] && continue
    echo "$key" | grep -q '^[[:space:]]*#' && continue

    key=$(echo "$key" | xargs)
    value=$(echo "$value" | xargs)

    # Look up key in METRICS file
    local documented
    documented=$(grep "^[[:space:]]*${key}[[:space:]]*=" "$METRICS_FILE" 2>/dev/null \
      | head -1 | cut -d'=' -f2 | xargs) || true

    if [ -z "$documented" ]; then
      echo -e "${YELLOW}NEW${NC}:   $key = $value  (not in METRICS file)"
    elif [ "$value" = "$documented" ]; then
      echo -e "${GREEN}OK${NC}:    $key = $value"
    else
      echo -e "${RED}DRIFT${NC}: $key — METRICS says $documented, code says $value"
      exit_code=1
    fi
  done <<< "$computed"
}

echo "=== Ground Truth Verification ==="
echo ""
verify
echo ""

if [ $exit_code -eq 0 ]; then
  echo -e "${GREEN}All documented metrics match codebase reality${NC}"
else
  echo -e "${RED}Metric drift detected — update METRICS to match reality${NC}"
fi

exit $exit_code
