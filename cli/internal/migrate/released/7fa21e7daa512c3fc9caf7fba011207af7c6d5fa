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
unnamespaced=0
total=0

# Configured namespace (empty in legacy / unmigrated repos).
REBAR_NS="$(_rebar_namespace 2>/dev/null || true)"

while IFS= read -r file; do
  # Skip test files, generated files, vendor, node_modules
  case "$file" in
    *_test.go|*.test.ts|*.test.tsx|*.test.js|*.spec.ts|*.spec.tsx|*.spec.js) continue ;;
    */vendor/*|*/node_modules/*|*/dist/*|*/build/*|*/.git/*|*/.claude/*) continue ;;
    *_generated*|*.gen.*|*.pb.go|*.pb.ts) continue ;;
  esac

  total=$((total + 1))

  header=$(head -15 "$file")
  if ! echo "$header" | grep -q "CONTRACT:\|Architecture:"; then
    echo "MISSING: $file"
    missing=$((missing + 1))
    continue
  fi

  # Strict mode: when a namespace is configured, the header must use the
  # namespaced form (CONTRACT:<ns>:<id> ...). Legacy bare refs fail.
  if [ -n "$REBAR_NS" ]; then
    if ! echo "$header" | grep -qE 'CONTRACT:[a-zA-Z0-9][a-zA-Z0-9_./-]+:[A-Z][A-Za-z0-9_-]*\.[0-9]+\.[0-9]+'; then
      echo "MISSING-NAMESPACE: $file (header has legacy CONTRACT: ref; expected CONTRACT:$REBAR_NS:<id>)"
      unnamespaced=$((unnamespaced + 1))
    fi
  fi
done < <(find "${DIRS[@]}" \( "${FIND_ARGS[@]}" \) -type f 2>/dev/null)

echo ""
echo "Scanned $total source files, $missing missing headers, $unnamespaced missing namespace."

if [ "$missing" -gt 0 ] || [ "$unnamespaced" -gt 0 ]; then
  # NOTE: literal CONTRACT prefix split via variable so the shadow-ref
  # detector in compute-registry.sh doesn't false-positive on these example
  # strings. See feedback/processed/2026-04-25-bootstrap-template-script-drift-and-bash3.2.md
  # for the related cli/cmd/context.go fix.
  P="CONTRACT:"
  echo ""
  if [ "$missing" -gt 0 ]; then
    echo "Every source file must declare which contract it implements:"
    if [ -n "$REBAR_NS" ]; then
      echo "  // ${P}$REBAR_NS:C1-BLOBSTORE.2.1"
      echo "  // Architecture: ${P}$REBAR_NS:S2-API-GATEWAY.1.0"
    else
      echo "  // ${P}C1-BLOBSTORE.2.1"
      echo "  // Architecture: ${P}S2-API-GATEWAY.1.0"
    fi
  fi
  if [ "$unnamespaced" -gt 0 ]; then
    echo ""
    echo "Strict mode is active (contract_namespace=$REBAR_NS in .rebarrc)."
    echo "Run \`rebar contract migrate-namespace --write\` to prefix legacy references."
  fi
  echo ""
  echo "See architecture/README.md for the full convention."
  exit 1
fi

exit 0
