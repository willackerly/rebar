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

broken=0
total=0

# Find all CONTRACT: references in source files
while IFS= read -r line; do
  file=$(echo "$line" | cut -d: -f1)
  lineno=$(echo "$line" | cut -d: -f2)

  # Extract the contract ID (e.g., C1-BLOBSTORE.2.1)
  ref=$(echo "$line" | grep -o 'CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*' | head -1 | sed 's/CONTRACT://')

  [ -z "$ref" ] && continue
  total=$((total + 1))

  # Check if the contract file exists
  expected="${ARCH_DIR}/CONTRACT-${ref}.md"
  if [ ! -f "$expected" ]; then
    echo "BROKEN: $file:$lineno references CONTRACT:$ref"
    echo "        Expected: $expected"
    broken=$((broken + 1))
  fi
done < <(grep -rn "CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*" \
  --include="*.go" --include="*.ts" --include="*.tsx" --include="*.js" \
  --include="*.py" --include="*.rs" --include="*.jsx" \
  . 2>/dev/null | grep -v "node_modules\|vendor\|dist\|\.git")

echo ""
echo "Checked $total contract references, $broken broken."

if [ "$broken" -gt 0 ]; then
  echo ""
  echo "Fix by either:"
  echo "  1. Creating the missing contract in $ARCH_DIR/"
  echo "  2. Updating the code reference to the correct contract version"
  exit 1
fi

exit 0
