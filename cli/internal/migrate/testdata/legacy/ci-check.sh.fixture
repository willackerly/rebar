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
#   SKIP_JTBD=1             — skip JTBD-presence contract check
#   SKIP_PREFIX_UNIQUENESS=1 — skip contract prefix-number uniqueness check
#   SKIP_GROUND_TRUTH=1     — skip ground truth metric verification
#   SKIP_COMPLIANCE=1       — skip rebar compliance check
#   SKIP_STEWARD=1          — skip steward health scan
#   SKIP_DOC_REFS=1         — skip cross-doc reference check
#   SKIP_DECAY_PATTERNS=1   — skip soft-hardening decay pattern check
#   SKIP_BOOTSTRAP_SYNC=1   — skip the thin templates/project-bootstrap/scripts check
#   SKIP_SCRIPT_TESTS=1     — skip scripts/tests/*.test.sh (rebar source repo only)
#   SKIP_FIX_COMMIT=1       — skip Gate G (HEAD's fix:/regression: must have Reproduced on:)
#   SKIP_BYPASS_FLAGS=1     — skip Gate I (HEAD's bypass mentions must have Bypass tickets:)
#
# REBAR_PROJECT_ROOT=<dir> checks <dir> with these scripts instead of the
# repo they live in; `rebar check` sets it so adopters need no local copies.
#
# Exit code: 0 = all pass, 1 = failures in strict mode

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
STRICT="${1:---strict}"

# shellcheck source=scripts/_rebar-config.sh
source "$SCRIPT_DIR/_rebar-config.sh" 2>/dev/null || true
type _rebar_script >/dev/null 2>&1 || _rebar_script() { echo "$SCRIPT_DIR/$1"; }
# Unknown .rebarrc checks_enable names are reported once here, not per check.
if type _rebar_checks_enable_warn >/dev/null 2>&1; then _rebar_checks_enable_warn; fi
if type _rebar_contract_namespace_warn >/dev/null 2>&1; then _rebar_contract_namespace_warn; fi
if [ -n "${REBAR_PROJECT_ROOT:-}" ]; then
  cd "$REBAR_PROJECT_ROOT" || exit 1
fi

passed=0
failed=0
skipped=0
results=()

run_check() {
  local name="$1"
  local skip_var="$2"
  local script
  script="$(_rebar_script "$3")"
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

run_check "Contract Headers"    SKIP_CONTRACT_HEADERS check-contract-headers.sh
run_check "Contract References" SKIP_CONTRACT_REFS    check-contract-refs.sh
run_check "Doc References"      SKIP_DOC_REFS         check-doc-refs.sh
run_check "TODO Tracking"       SKIP_TODOS            check-todos.sh
run_check "Doc Freshness"       SKIP_FRESHNESS        check-freshness.sh
run_check "Registry Consistency" SKIP_REGISTRY         compute-registry.sh --check
run_check "JTBD Presence"       SKIP_JTBD             check-jtbd-presence.sh
run_check "Prefix Uniqueness"   SKIP_PREFIX_UNIQUENESS check-prefix-uniqueness.sh
run_check "Ground Truth"        SKIP_GROUND_TRUTH     check-ground-truth.sh
run_check "Rebar Compliance"    SKIP_COMPLIANCE       check-compliance.sh
run_check "Decay Patterns"      SKIP_DECAY_PATTERNS   check-decay-patterns.sh
run_check "Bootstrap Sync"      SKIP_BOOTSTRAP_SYNC   sync-bootstrap.sh --check
run_check "Script Tests"        SKIP_SCRIPT_TESTS     tests/run.sh
run_check "Fix Commit Gate"     SKIP_FIX_COMMIT       check-fix-commit.sh
run_check "Bypass Flags Gate"   SKIP_BYPASS_FLAGS     check-bypass-flags.sh
run_check "Steward"             SKIP_STEWARD          steward.sh

# NOTE: Tag-to-CI coverage check (Node.js, project-specific) lives at
# templates/scripts/check-tag-ci-coverage.mjs. Adopters with Playwright
# or @-tagged spec files should copy it into their own scripts/ and wire
# it into their CI workflow directly, not via this universal ci-check.

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
