#!/usr/bin/env bash
# Full QA flow: steward + enforcement
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"

echo "=== QA Flow ==="
echo ""

echo "Step 1: Steward scan..."
"$REPO_ROOT/scripts/steward.sh"

echo ""
echo "Step 2: Enforcement checks..."
"$REPO_ROOT/scripts/ci-check.sh" --strict

echo ""
echo "QA complete. Review STEWARD_REPORT.md for action items."
