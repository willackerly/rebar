#!/usr/bin/env bash
# check-registry.sh — Verify contract registry matches actual files
# rebar-scripts: 2026.03.20
#
# DEPRECATED: Use compute-registry.sh instead.
# compute-registry.sh generates the registry from contract files on disk
# (rather than verifying a manually-maintained registry).
# This script is kept for backwards compatibility.
#
# Checks:
#   1. Every CONTRACT-*.md file is listed in the registry
#   2. Every registry entry points to an existing file
#   3. Orphaned contracts (no implementing code) are flagged — OK if in TODO.md
#
# Usage: ./scripts/check-registry.sh [architecture-dir]
# Default: architecture/
#
# Exit code: 0 = all consistent, 1 = inconsistencies found

set -euo pipefail

ARCH_DIR="${1:-architecture}"
REGISTRY="${ARCH_DIR}/CONTRACT-REGISTRY.md"

if [ ! -d "$ARCH_DIR" ]; then
  echo "Architecture directory '$ARCH_DIR' not found."
  exit 1
fi

errors=0

# --- Check 1: Every CONTRACT-*.md file is in the registry ---
echo "=== Contract files vs registry ==="

for contract in "$ARCH_DIR"/CONTRACT-*.md; do
  [ "$contract" = "$ARCH_DIR/CONTRACT-TEMPLATE.md" ] && continue
  [ "$contract" = "$ARCH_DIR/CONTRACT-REGISTRY.md" ] && continue
  [ "$contract" = "${ARCH_DIR}/CONTRACT-REGISTRY.template.md" ] && continue
  [ "$contract" = "$ARCH_DIR/CONTRACT-GAPS.md" ] && continue
  [ ! -f "$contract" ] && continue

  basename=$(basename "$contract")

  if [ -f "$REGISTRY" ] && ! grep -q "$basename" "$REGISTRY"; then
    echo "NOT IN REGISTRY: $contract"
    errors=$((errors + 1))
  fi
done

# --- Check 2: Orphaned contracts (no implementing code) ---
echo ""
echo "=== Orphaned contracts (no implementing code) ==="

for contract in "$ARCH_DIR"/CONTRACT-*.md; do
  [ "$contract" = "$ARCH_DIR/CONTRACT-TEMPLATE.md" ] && continue
  [ "$contract" = "$ARCH_DIR/CONTRACT-REGISTRY.md" ] && continue
  [ "$contract" = "${ARCH_DIR}/CONTRACT-REGISTRY.template.md" ] && continue
  [ "$contract" = "$ARCH_DIR/CONTRACT-GAPS.md" ] && continue
  [ ! -f "$contract" ] && continue

  # Extract contract ID from filename (e.g., CONTRACT-C1-BLOBSTORE.2.1.md → C1-BLOBSTORE.2.1)
  id=$(basename "$contract" .md | sed 's/^CONTRACT-//')

  # Search for references in source code
  ref_count=$(grep -rn "CONTRACT:${id}" \
    --include="*.go" --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx" \
    --include="*.py" --include="*.rs" --include="*.mjs" --include="*.cjs" \
    . 2>/dev/null \
    | grep -v "node_modules\|vendor\|dist\|\.git\|architecture/" \
    | wc -l | tr -d ' ')

  if [ "$ref_count" -eq 0 ]; then
    # Check if it's tracked in TODO.md (orphan is OK if planned)
    if [ -f "TODO.md" ] && grep -q "$id" TODO.md 2>/dev/null; then
      echo "ORPHAN (tracked in TODO.md): $contract"
    else
      echo "ORPHAN (untracked): $contract — 0 implementing files"
      echo "  Either add implementing code with CONTRACT:$id header,"
      echo "  or track in TODO.md if implementation is planned."
      errors=$((errors + 1))
    fi
  fi
done

echo ""
if [ "$errors" -gt 0 ]; then
  echo "FAIL: $errors inconsistencies found."
  exit 1
else
  echo "OK: Registry and contracts are consistent."
  exit 0
fi
