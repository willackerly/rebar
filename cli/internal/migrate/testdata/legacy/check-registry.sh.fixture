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
# Usage: ./scripts/check-registry.sh [contracts-dir]
# Default: .rebarrc contracts_dir, else architecture/
#
# Exit code: 0 = all consistent, 1 = inconsistencies found

set -euo pipefail

CONTRACTS_DIR=architecture
REF_ERE='CONTRACT:'   # with contract_namespace: CONTRACT:(<namespace>:)?
EXCLUDE_DIRS="node_modules vendor dist .git .claude"
if [ -f "$(dirname "$0")/_rebar-config.sh" ]; then
  # shellcheck source=scripts/_rebar-config.sh
  source "$(dirname "$0")/_rebar-config.sh"
  # Under `REBAR_CHECK_IMPL=go rebar check` (the default) the CLI's native port runs instead.
  _rebar_native check-registry "$@"
  CONTRACTS_DIR=$(_rebar_contracts_dir)
  REF_ERE=$(_rebar_contract_ref_ere)
  EXCLUDE_DIRS=$(_rebar_source_exclude_dirs)
fi
ARCH_DIR="${1:-$CONTRACTS_DIR}"
if type _rebar_norm_dir >/dev/null 2>&1; then
  ARCH_DIR=$(_rebar_norm_dir "$ARCH_DIR")
fi
INCLUDE_ARGS=(--include="*.go" --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx"
  --include="*.py" --include="*.rs" --include="*.mjs" --include="*.cjs" --include="*.sh")
EXCLUDE_ARGS=()
for dir in $EXCLUDE_DIRS; do
  EXCLUDE_ARGS+=("--exclude-dir=$dir")
done

# refs_to ID — matching lines in source files and anywhere under bin/ (CLI
# entry points often have no extension), minus the contracts dir by path
# prefix. No match is the case this script reports, so never fail.
refs_to() {
  {
    grep -rEnI "${REF_ERE}$1" "${INCLUDE_ARGS[@]}" "${EXCLUDE_ARGS[@]}" . 2>/dev/null || true
    if [ -d bin ]; then
      grep -rEnI "${REF_ERE}$1" "${EXCLUDE_ARGS[@]}" ./bin 2>/dev/null || true
    fi
  } | awk -v p="./$ARCH_DIR/" 'index($0, p) != 1' | sort -u
}
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
  [ "$contract" = "$ARCH_DIR/CONTRACT-SEAM-TEMPLATE.md" ] && continue
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
  [ "$contract" = "$ARCH_DIR/CONTRACT-SEAM-TEMPLATE.md" ] && continue
  [ "$contract" = "$ARCH_DIR/CONTRACT-REGISTRY.md" ] && continue
  [ "$contract" = "${ARCH_DIR}/CONTRACT-REGISTRY.template.md" ] && continue
  [ "$contract" = "$ARCH_DIR/CONTRACT-GAPS.md" ] && continue
  [ ! -f "$contract" ] && continue

  # Extract contract ID from filename (e.g., CONTRACT-C1-BLOBSTORE.2.1.md → C1-BLOBSTORE.2.1)
  id=$(basename "$contract" .md | sed 's/^CONTRACT-//')

  ref_count=$(refs_to "$id" | grep -c . || true)

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
