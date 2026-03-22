#!/usr/bin/env bash
# Contract audit (architect's view)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REPORT="$REPO_ROOT/architecture/.state/steward-report.json"

# Ensure steward data exists
if [ ! -f "$REPORT" ]; then
  echo "Running steward scan..." >&2
  "$REPO_ROOT/scripts/steward.sh" > /dev/null 2>&1
fi

if ! command -v jq &>/dev/null; then
  echo "ask architect: jq required — brew install jq / apt install jq" >&2
  exit 4
fi

echo "=== Architect Audit ==="
echo ""

# DRAFT contracts (missing sections)
draft_output=$(jq -r '
  [.contracts[] | select(.lifecycle == "draft")]
  | if length > 0 then
      "Contracts in DRAFT (incomplete spec):",
      (.[] | "  \(.contract_id) v\(.version) — missing: \(
        [.spec_gate | to_entries[]
         | select(.value == false and .key != "completeness")
         | .key] | join(", "))")
    else empty end
' "$REPORT" 2>/dev/null || true)
if [ -n "$draft_output" ]; then
  echo "$draft_output"
  echo ""
fi

# DISPUTE discoveries
dispute_output=$(jq -r '
  [.contracts[].discoveries[]? | select(.type == "DISPUTE")]
  | if length > 0 then
      "Open DISPUTEs (contract needs updating):",
      (.[] | "  \(.contract // "none") — \(.description)")
    else empty end
' "$REPORT" 2>/dev/null || true)
if [ -n "$dispute_output" ]; then
  echo "$dispute_output"
  echo ""
fi

# Action items for architect
items=$(jq -r '.action_items.architect[]?' "$REPORT" 2>/dev/null || true)
if [ -n "$items" ]; then
  echo "Action items:"
  echo "$items" | sed 's/^/  /'
  echo ""
fi

# Summary line
total=$(jq -r '.summary.contracts.total' "$REPORT")
draft=$(jq -r '.summary.contracts.draft' "$REPORT")
active=$(jq -r '.summary.contracts.active' "$REPORT")
verified=$(jq -r '.summary.contracts.verified' "$REPORT")
echo "Contracts: $total total, $draft draft, $active active, $verified verified"
