#!/usr/bin/env bash
# ci-check.sh — Atomic CI entrypoint that runs all contract/doc checks
# rebar-scripts: 2026.03.20
#
# Usage: ./scripts/ci-check.sh [--strict]
#
# Runs each check script and reports a summary. In strict mode (default),
# any failure fails the CI job. In non-strict mode, failures are warnings.
#
# Individual checks can be skipped with environment variables:
#   SKIP_CONTRACT_HEADERS=1  — skip contract header check
#   SKIP_CONTRACT_REFS=1     — skip contract reference check
#   SKIP_TODOS=1             — skip TODO tracking check
#   SKIP_FRESHNESS=1         — skip freshness check
#   SKIP_REGISTRY=1          — skip registry consistency check
#   SKIP_GROUND_TRUTH=1     — skip ground truth metric verification
#   SKIP_COMPLIANCE=1       — skip rebar compliance check
#   SKIP_STEWARD=1          — skip steward health scan
#
# Exit code: 0 = all pass, 1 = failures in strict mode

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
STRICT="${1:---strict}"

passed=0
failed=0
skipped=0
results=()

run_check() {
  local name="$1"
  local skip_var="$2"
  local script="$3"
  shift 3

  # Check skip flag
  if [ "${!skip_var:-0}" = "1" ]; then
    results+=("SKIP  $name")
    skipped=$((skipped + 1))
    return
  fi

  # Check script exists
  if [ ! -x "$script" ]; then
    results+=("SKIP  $name (script not found or not executable)")
    skipped=$((skipped + 1))
    return
  fi

  echo ""
  echo "━━━ $name ━━━"
  if "$script" "$@"; then
    results+=("PASS  $name")
    passed=$((passed + 1))
  else
    results+=("FAIL  $name")
    failed=$((failed + 1))
  fi
}

echo "Running contract and documentation checks..."

run_check "Contract Headers"    SKIP_CONTRACT_HEADERS "$SCRIPT_DIR/check-contract-headers.sh"
run_check "Contract References" SKIP_CONTRACT_REFS    "$SCRIPT_DIR/check-contract-refs.sh"
run_check "TODO Tracking"       SKIP_TODOS            "$SCRIPT_DIR/check-todos.sh"
run_check "Doc Freshness"       SKIP_FRESHNESS        "$SCRIPT_DIR/check-freshness.sh"
run_check "Registry Consistency" SKIP_REGISTRY         "$SCRIPT_DIR/compute-registry.sh" --check
run_check "Ground Truth"        SKIP_GROUND_TRUTH     "$SCRIPT_DIR/check-ground-truth.sh"
run_check "Rebar Compliance"    SKIP_COMPLIANCE       "$SCRIPT_DIR/check-compliance.sh"
run_check "Steward"             SKIP_STEWARD          "$SCRIPT_DIR/steward.sh"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for result in "${results[@]}"; do
  echo "  $result"
done
echo ""
echo "  $passed passed, $failed failed, $skipped skipped"

if [ "$failed" -gt 0 ] && [ "$STRICT" = "--strict" ]; then
  echo ""
  echo "CI check failed. Fix the issues above or skip individual checks"
  echo "with environment variables (e.g., SKIP_FRESHNESS=1)."
  exit 1
fi

exit 0
