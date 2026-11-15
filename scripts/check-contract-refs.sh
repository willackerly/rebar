#!/usr/bin/env bash
# check-contract-refs.sh — Verify every CONTRACT: reference points to a real file
# rebar-scripts: 2026.03.20
#
# Usage: ./scripts/check-contract-refs.sh [contracts-dir]
# Default: .rebarrc contracts_dir, else architecture/
#
# With .rebarrc contract_namespace set, CONTRACT:<namespace>:<ID>.<v> is a
# reference to this repo's contract <ID>.<v> and is checked like the short
# form; a reference qualified with another namespace belongs to another repo
# and is counted but not checked.
#
# Exit code: 0 = all refs valid, 1 = broken refs found

set -euo pipefail

# File types to scan: .rebarrc `source_extensions`, else the default list.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOURCE_EXTENSIONS="go ts tsx js py rs jsx"
CONTRACTS_DIR=architecture
NAMESPACE=""
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  # shellcheck source=scripts/_rebar-config.sh
  source "$SCRIPT_DIR/_rebar-config.sh"
  SOURCE_EXTENSIONS=$(_rebar_config source_extensions _REBAR_NO_ENV "$SOURCE_EXTENSIONS")
  CONTRACTS_DIR=$(_rebar_contracts_dir)
  NAMESPACE=$(_rebar_contract_namespace)
fi
ARCH_DIR="${1:-$CONTRACTS_DIR}"
INCLUDES=()
for ext in $SOURCE_EXTENSIONS; do
  INCLUDES+=("--include=*.${ext#.}")
done

if [ ! -d "$ARCH_DIR" ]; then
  echo "Architecture directory '$ARCH_DIR' not found."
  exit 1
fi

broken=0
total=0
crossrepo=0

# Without a namespace, look for the short form only (unchanged behavior).
# With one, also find qualified references; each line is then rewritten so
# this repo's CONTRACT:<namespace>: becomes CONTRACT: and any other
# namespace becomes CROSSREF: (skipped below, counted in $crossrepo). A
# would-be namespace that starts like <ID>.<v> is not one: in
# <ID>.<v>:see.2 the reference is <ID>.<v>, so short refs are
# masked (CONTRACT~) while foreign ones are rewritten.
SEARCH_ERE='CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*'
if [ -n "$NAMESPACE" ]; then
  SEARCH_ERE='CONTRACT:([A-Za-z0-9._/-]+:)?[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*'
  NS_ERE="${NAMESPACE//./\\.}"
fi

# Find all CONTRACT: references in source files. node_modules/, vendor/,
# dist/ and .git/ directories are skipped by name; filtering the match lines
# with grep -v also dropped .github/, distance.go, and any line whose text
# mentioned those words.
while IFS= read -r line; do
  file=$(echo "$line" | cut -d: -f1)
  lineno=$(echo "$line" | cut -d: -f2)

  if [ -n "$NAMESPACE" ]; then
    line=$(printf '%s\n' "$line" | sed -E \
      -e "s#CONTRACT:${NS_ERE}:#CONTRACT:#g" \
      -e 's#CONTRACT:([A-Za-z0-9_-]+\.[0-9]+\.[0-9]+)#CONTRACT~\1#g' \
      -e 's#CONTRACT:([A-Za-z0-9._/-]+:[A-Za-z0-9][A-Za-z0-9_-]*\.[0-9])#CROSSREF:\1#g' \
      -e 's#CONTRACT~#CONTRACT:#g')
    n=$(printf '%s\n' "$line" | awk '{ n += gsub(/CROSSREF:/, "") } END { print n + 0 }')
    crossrepo=$((crossrepo + n))
  fi

  # Extract the contract ID (e.g., C1-BLOBSTORE.2.1)
  ref=$(echo "$line" | grep -o 'CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*' | head -1 | sed 's/CONTRACT://' || true)

  [ -z "$ref" ] && continue
  total=$((total + 1))

  # Check if the contract file exists
  expected="${ARCH_DIR}/CONTRACT-${ref}.md"
  if [ ! -f "$expected" ]; then
    echo "BROKEN: $file:$lineno references CONTRACT:$ref"
    echo "        Expected: $expected"
    broken=$((broken + 1))
  fi
done < <(grep -rnE "$SEARCH_ERE" \
  "${INCLUDES[@]}" \
  --exclude-dir=node_modules --exclude-dir=vendor --exclude-dir=dist --exclude-dir=.git \
  . 2>/dev/null)

echo ""
echo "Checked $total contract references, $broken broken."
if [ -n "$NAMESPACE" ]; then
  echo "Namespace $NAMESPACE: $crossrepo cross-repo reference(s) to other namespaces not checked."
fi

if [ "$broken" -gt 0 ]; then
  echo ""
  echo "Fix by either:"
  echo "  1. Creating the missing contract in $ARCH_DIR/"
  echo "  2. Updating the code reference to the correct contract version"
  exit 1
fi

exit 0
