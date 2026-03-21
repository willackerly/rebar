#!/usr/bin/env bash
# check-contract-headers.sh — Verify every source file has a CONTRACT: or Architecture: header
# rebar-scripts: 2026.03.20
#
# Usage: ./scripts/check-contract-headers.sh [directories...]
# Default: scans src/ internal/ cmd/ client/ packages/ lib/ app/
#
# Exit code: 0 = all files have headers, 1 = missing headers found

set -euo pipefail

# Tier gate: contract headers are Tier 2+ (skip for Tier 1 / partial adoption)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$SCRIPT_DIR/_rebar-config.sh" ] && source "$SCRIPT_DIR/_rebar-config.sh" && _rebar_skip 2 && exit 0

# Configurable: file extensions to check
EXTENSIONS="${CONTRACT_EXTENSIONS:-.go .ts .tsx .js .jsx .py .rs}"

# Configurable: directories to scan (override with args)
if [ $# -gt 0 ]; then
  DIRS=("$@")
else
  DIRS=()
  for d in src internal cmd client packages lib app; do
    [ -d "$d" ] && DIRS+=("$d")
  done
fi

if [ ${#DIRS[@]} -eq 0 ]; then
  echo "No source directories found. Pass directories as arguments."
  exit 0
fi

# Build find expression for extensions
FIND_ARGS=()
first=true
for ext in $EXTENSIONS; do
  if $first; then
    FIND_ARGS+=(-name "*${ext}")
    first=false
  else
    FIND_ARGS+=(-o -name "*${ext}")
  fi
done

missing=0
total=0

while IFS= read -r file; do
  # Skip test files, generated files, vendor, node_modules
  case "$file" in
    *_test.go|*.test.ts|*.test.tsx|*.test.js|*.spec.ts|*.spec.tsx|*.spec.js) continue ;;
    */vendor/*|*/node_modules/*|*/dist/*|*/build/*|*/.git/*) continue ;;
    *_generated*|*.gen.*|*.pb.go|*.pb.ts) continue ;;
  esac

  total=$((total + 1))

  # Check first 15 lines for CONTRACT: or Architecture:
  if ! head -15 "$file" | grep -q "CONTRACT:\|Architecture:"; then
    echo "MISSING: $file"
    missing=$((missing + 1))
  fi
done < <(find "${DIRS[@]}" \( "${FIND_ARGS[@]}" \) -type f 2>/dev/null)

echo ""
echo "Scanned $total source files, $missing missing contract headers."

if [ "$missing" -gt 0 ]; then
  echo ""
  echo "Every source file must declare which contract it implements:"
  echo '  // CONTRACT:C1-BLOBSTORE.2.1'
  echo '  // Architecture: CONTRACT:S2-API-GATEWAY.1.0'
  echo ""
  echo "See architecture/README.md for the full convention."
  exit 1
fi

exit 0
