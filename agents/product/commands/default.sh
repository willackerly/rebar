#!/usr/bin/env bash
# Gap analysis (product's view)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REBAR="$REPO_ROOT/bin/rebar"; [ -x "$REBAR" ] || REBAR=rebar
# Contracts location: .rebarrc contracts_dir (env REBAR_CONTRACTS_DIR), default architecture.
CONTRACTS_DIR="$(sed -n 's/^contracts_dir[[:space:]]*=[[:space:]]*//p' "$REPO_ROOT/.rebarrc" 2>/dev/null \
  | sed 's/[[:space:]]*#.*$//; s/[[:space:]]*$//; s:/$::' | head -1)" || true
CONTRACTS_DIR="${REBAR_CONTRACTS_DIR:-${CONTRACTS_DIR:-architecture}}"
REPORT="$REPO_ROOT/$CONTRACTS_DIR/.state/steward-report.json"

# Ensure steward data exists
if [ ! -f "$REPORT" ]; then
  echo "Running steward scan..." >&2
  "$REBAR" --repo-root "$REPO_ROOT" run steward > /dev/null 2>&1
fi

if ! command -v jq &>/dev/null; then
  echo "ask product: jq required — brew install jq / apt install jq" >&2
  exit 4
fi

echo "=== Product: Gaps & Discoveries ==="
echo ""

# DISCOVERY type items (behavior exists, no contract covers it)
disc_output=$(jq -r '
  [.contracts[].discoveries[]? | select(.type == "DISCOVERY")]
  | if length > 0 then
      "Uncovered behavior (needs contract):",
      (.[] | "  \(.contract // "none") — \(.description)")
    else empty end
' "$REPORT" 2>/dev/null || true)
if [ -n "$disc_output" ]; then
  echo "$disc_output"
  echo ""
fi

# Action items for product
items=$(jq -r '.action_items.product[]?' "$REPORT" 2>/dev/null || true)
if [ -n "$items" ]; then
  echo "Action items:"
  echo "$items" | sed 's/^/  /'
  echo ""
fi

# Contracts missing BDD source reference
echo "Contracts without BDD source reference:"
found_missing=false
for f in "$REPO_ROOT/$CONTRACTS_DIR"/CONTRACT-*.md; do
  [ -f "$f" ] || continue
  # Skip registry and template
  case "$(basename "$f")" in
    CONTRACT-REGISTRY*|CONTRACT-TEMPLATE*) continue ;;
  esac
  if ! grep -q '^\*\*Source:\*\*.*[a-zA-Z]' "$f" 2>/dev/null; then
    echo "  $(basename "$f" .md)"
    found_missing=true
  fi
done
if [ "$found_missing" = false ]; then
  echo "  (all contracts have BDD source references)"
fi
echo ""

# Summary
total=$(jq -r '.summary.contracts.total' "$REPORT")
discoveries=$(jq -r '.summary.open_discoveries' "$REPORT")
echo "Contracts: $total total, $discoveries open discoveries"
