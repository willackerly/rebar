#!/usr/bin/env bash
# pre-commit.sh — Pre-commit hook that runs fast contract/TODO checks
# rebar-scripts: 2026.03.20
#
# Install:
#   cp scripts/pre-commit.sh .git/hooks/pre-commit
#   chmod +x .git/hooks/pre-commit
#
# Or symlink:
#   ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
#
# This runs only the fast checks suitable for pre-commit (<5s total).
# The full suite (including freshness, registry) runs in CI via ci-check.sh.
#
# REBAR_PROJECT_ROOT=<dir> checks <dir> instead (`rebar check --pre-commit`).
#
# Skip with: git commit --no-verify (use sparingly)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# If installed as a git hook, scripts are at ../../scripts/
# If run directly, they're in the same directory
if [ -f "$SCRIPT_DIR/check-todos.sh" ]; then
  BASE="$SCRIPT_DIR"
elif [ -f "$SCRIPT_DIR/../../scripts/check-todos.sh" ]; then
  BASE="$SCRIPT_DIR/../../scripts"
else
  echo "Warning: Cannot find check scripts. Skipping pre-commit checks."
  exit 0
fi

# shellcheck source=scripts/_rebar-config.sh
source "$BASE/_rebar-config.sh" 2>/dev/null || true
# Under `REBAR_CHECK_IMPL=go rebar check` (the default) the CLI's native suite runs instead.
if type _rebar_native >/dev/null 2>&1; then _rebar_native pre-commit "$@"; fi
type _rebar_script >/dev/null 2>&1 || _rebar_script() { echo "$BASE/$1"; }
# Unknown .rebarrc checks_enable names are reported once here, not per check.
if type _rebar_checks_enable_warn >/dev/null 2>&1; then _rebar_checks_enable_warn; fi
if type _rebar_contract_namespace_warn >/dev/null 2>&1; then _rebar_contract_namespace_warn; fi
if [ -n "${REBAR_PROJECT_ROOT:-}" ]; then
  cd "$REBAR_PROJECT_ROOT" || exit 1
fi

failed=0

echo "Pre-commit: checking TODO tracking..."
check_todos="$(_rebar_script check-todos.sh)"
if [ -x "$check_todos" ]; then
  "$check_todos" || failed=$((failed + 1))
fi

echo ""
echo "Pre-commit: checking contract references..."
check_contract_refs="$(_rebar_script check-contract-refs.sh)"
if [ -x "$check_contract_refs" ]; then
  "$check_contract_refs" || failed=$((failed + 1))
fi

# Optional: verify numeric claims haven't drifted (fast — just find/grep/wc)
check_ground_truth="$(_rebar_script check-ground-truth.sh)"
if [ -x "$check_ground_truth" ]; then
  echo ""
  echo "Pre-commit: checking ground truth metrics..."
  "$check_ground_truth" || failed=$((failed + 1))
fi

if [ "$failed" -gt 0 ]; then
  echo ""
  echo "Pre-commit checks failed. Fix the issues above before committing."
  echo "To skip (not recommended): git commit --no-verify"
  exit 1
fi

echo ""
echo "Pre-commit checks passed."
exit 0
