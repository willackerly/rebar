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

failed=0

echo "Pre-commit: checking TODO tracking..."
if [ -x "$BASE/check-todos.sh" ]; then
  "$BASE/check-todos.sh" || failed=$((failed + 1))
fi

echo ""
echo "Pre-commit: checking contract references..."
if [ -x "$BASE/check-contract-refs.sh" ]; then
  "$BASE/check-contract-refs.sh" || failed=$((failed + 1))
fi

# Optional: verify numeric claims haven't drifted (fast — just find/grep/wc)
if [ -x "$BASE/check-ground-truth.sh" ]; then
  echo ""
  echo "Pre-commit: checking ground truth metrics..."
  "$BASE/check-ground-truth.sh" || failed=$((failed + 1))
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
