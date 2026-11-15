#!/usr/bin/env bash
# check-contract-refs.sh — Verify every CONTRACT: reference points to a real file
# rebar-scripts: 2026.03.20
#
# Usage: ./scripts/check-contract-refs.sh [architecture-dir]
# Default: architecture/
#
# Exit code: 0 = all refs valid, 1 = broken refs found

set -euo pipefail

ARCH_DIR="${1:-architecture}"

if [ ! -d "$ARCH_DIR" ]; then
  echo "Architecture directory '$ARCH_DIR' not found."
  exit 1
fi

# Load shared config (namespace, tier helpers).
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$SCRIPT_DIR/_rebar-config.sh" ] && source "$SCRIPT_DIR/_rebar-config.sh"
REBAR_NS="$(_rebar_namespace 2>/dev/null || true)"

broken=0
unnamespaced=0
total=0

# Regex matches both legacy (CONTRACT:<id>.<v>) and namespaced
# (CONTRACT:<ns>:<id>.<v>) forms. The namespace portion may contain
# letters, digits, ., /, _, and -.
REF_REGEX='CONTRACT:([a-zA-Z0-9][a-zA-Z0-9_./-]+:)?[A-Z][A-Za-z0-9_-]*\.[0-9]+\.[0-9]+'

# Find all CONTRACT: references in source files
while IFS= read -r line; do
  file=$(echo "$line" | cut -d: -f1)
  lineno=$(echo "$line" | cut -d: -f2)

  # Extract the full reference, then strip "CONTRACT:" prefix.
  full_ref=$(echo "$line" | grep -oE "$REF_REGEX" | head -1)
  [ -z "$full_ref" ] && continue
  full_ref="${full_ref#CONTRACT:}"
  total=$((total + 1))

  # Detect whether this ref is namespaced. The ID is `^[A-Z][A-Za-z0-9_-]*\.<v>$`
  # which never contains ':'. So a ':' present in full_ref means it's namespaced.
  if [[ "$full_ref" == *:* ]]; then
    # Namespaced. Validate namespace matches the configured one when in strict mode.
    ref_ns="${full_ref%:*}"
    bare_id="${full_ref##*:}"
    if [ -n "$REBAR_NS" ] && [ "$ref_ns" != "$REBAR_NS" ]; then
      # Foreign-namespace reference (e.g. consuming an upstream contract).
      # Don't try to resolve it against this repo's architecture/ — that's
      # a CONSUMES.md concern. Just count it.
      continue
    fi
  else
    bare_id="$full_ref"
    # Strict mode: in a migrated repo (REBAR_NS set), legacy refs are an error.
    if [ -n "$REBAR_NS" ]; then
      echo "MISSING-NAMESPACE: $file:$lineno references legacy CONTRACT:$bare_id"
      echo "                   Expected: CONTRACT:$REBAR_NS:$bare_id"
      unnamespaced=$((unnamespaced + 1))
      continue
    fi
  fi

  # Check if the contract file exists.
  expected="${ARCH_DIR}/CONTRACT-${bare_id}.md"
  if [ ! -f "$expected" ]; then
    echo "BROKEN: $file:$lineno references CONTRACT:$full_ref"
    echo "        Expected: $expected"
    broken=$((broken + 1))
  fi
done < <(grep -rEn "$REF_REGEX" \
  --include="*.go" --include="*.ts" --include="*.tsx" --include="*.js" \
  --include="*.py" --include="*.rs" --include="*.jsx" \
  . 2>/dev/null | grep -v "node_modules\|vendor\|dist\|\.git\|\.claude/worktrees")

echo ""
echo "Checked $total contract references, $broken broken, $unnamespaced missing namespace."

if [ "$broken" -gt 0 ] || [ "$unnamespaced" -gt 0 ]; then
  echo ""
  if [ "$unnamespaced" -gt 0 ]; then
    echo "Run \`rebar contract migrate-namespace --write\` to prefix legacy references with the configured namespace."
  fi
  if [ "$broken" -gt 0 ]; then
    echo "For broken refs:"
    echo "  1. Create the missing contract in $ARCH_DIR/"
    echo "  2. Update the code reference to the correct contract version"
  fi
  exit 1
fi

exit 0
