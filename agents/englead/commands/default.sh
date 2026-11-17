#!/usr/bin/env bash
# Enforcement and QA status
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
  echo "ask englead: jq required — brew install jq / apt install jq" >&2
  exit 4
fi

echo "=== Eng Lead: Enforcement & QA ==="
echo ""

# Enforcement results
echo "Enforcement:"
jq -r '.enforcement | to_entries[] | "  \(if .value == "pass" then "PASS" else "FAIL" end)  \(.key)"' "$REPORT" 2>/dev/null || true
echo ""

# Contracts in TESTING (need test files)
testing_output=$(jq -r '
  [.contracts[] | select(.lifecycle == "testing")]
  | if length > 0 then
      "Contracts in TESTING (need test files):",
      (.[] | "  \(.contract_id) — \(.impl_gate.implementing_count) impl, \(.impl_gate.test_count) tests")
    else empty end
' "$REPORT" 2>/dev/null || true)
if [ -n "$testing_output" ]; then
  echo "$testing_output"
  echo ""
fi

# Action items for eng lead
items=$(jq -r '.action_items.englead[]?' "$REPORT" 2>/dev/null || true)
if [ -n "$items" ]; then
  echo "Action items:"
  echo "$items" | sed 's/^/  /'
  echo ""
fi

# Summary
passing=$(jq -r '.summary.enforcement.passing' "$REPORT")
enf_total=$(jq -r '.summary.enforcement.total' "$REPORT")
discoveries=$(jq -r '.summary.open_discoveries' "$REPORT")
echo "Enforcement: $passing/$enf_total passing, $discoveries open discoveries"
