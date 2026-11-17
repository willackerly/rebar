#!/usr/bin/env bash
# Full QA flow: steward + enforcement
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REBAR="$REPO_ROOT/bin/rebar"; [ -x "$REBAR" ] || REBAR=rebar

echo "=== QA Flow ==="
echo ""

echo "Step 1: Steward scan..."
"$REBAR" --repo-root "$REPO_ROOT" run steward

echo ""
echo "Step 2: Enforcement checks..."
"$REBAR" --repo-root "$REPO_ROOT" check --strict

echo ""
echo "QA complete. Review STEWARD_REPORT.md for action items."
