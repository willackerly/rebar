#!/usr/bin/env bash
# compute-registry.sh — Generate CONTRACT-REGISTRY.md from contract files on disk
# rebar-scripts: 2026.03.20
#
# Usage:
#   ./scripts/compute-registry.sh           Generate architecture/CONTRACT-REGISTRY.md
#   ./scripts/compute-registry.sh --check   Exit 0 if current, 1 if stale
#   ./scripts/compute-registry.sh --stdout   Print to stdout instead of writing file
#   ./scripts/compute-registry.sh --health   Print implementing-file counts and
#                                            zombie/shadow contracts to stdout
#
# The contract filesystem IS the registry. This script just renders it.
#
# The output is deterministic (no timestamp, C collation), so two branches
# that regenerate it from the same contracts produce identical bytes and merge
# cleanly. Implementing-file counts and the Contract Health section are derived
# from source code, so any PR that adds or removes a CONTRACT: header changes
# them. Busy repos should keep them out of the committed file:
#
#   .rebarrc:  registry_impl_counts = false    (env: REBAR_REGISTRY_IMPL_COUNTS)
#
# and read them with --health (or scripts/check-registry.sh) instead.
#
# Exit code: 0 = success/current, 1 = stale (--check mode)

set -euo pipefail

# Glob and sort order must not depend on the caller's locale, or two machines
# render the same contracts in different orders.
export LC_ALL=C

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="${REBAR_PROJECT_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
ARCH_DIR="$PROJECT_ROOT/architecture"
REGISTRY="$ARCH_DIR/CONTRACT-REGISTRY.md"
MODE="${1:-generate}"

# Settings from .rebarrc (see the helpers below for what each one does).
IMPL_COUNTS=true
SCOPE_PREFIXES=""
PURPOSE_HEADING=""
PURPOSE_STYLE=bytes
IMPL_GLOBS="*.go *.ts *.tsx *.js *.py *.rs *.java *.rb *.jsx *.sh bin/"
EXCLUDE_DIRS="node_modules vendor dist"
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  # shellcheck source=scripts/_rebar-config.sh
  source "$SCRIPT_DIR/_rebar-config.sh"
  IMPL_COUNTS=$(_rebar_config registry_impl_counts REBAR_REGISTRY_IMPL_COUNTS true)
  SCOPE_PREFIXES=$(_rebar_config registry_scope_prefixes _REBAR_NO_ENV "")
  PURPOSE_HEADING=$(_rebar_config registry_purpose_heading _REBAR_NO_ENV "")
  PURPOSE_STYLE=$(_rebar_config registry_purpose_style _REBAR_NO_ENV bytes)
  IMPL_GLOBS=$(_rebar_config registry_impl_globs _REBAR_NO_ENV "$IMPL_GLOBS")
  EXCLUDE_DIRS=$(_rebar_config registry_exclude_dirs _REBAR_NO_ENV "$EXCLUDE_DIRS")
fi
case "$IMPL_COUNTS" in
  false|0|no|off) IMPL_COUNTS=false ;;
  *) IMPL_COUNTS=true ;;
esac

# ─── Helpers ────────────────────────────────────────────────────────────────

# Parse contract filename → ID + version (reused logic from steward.sh)
parse_contract_file() {
  local filename
  filename="$(basename "$1")"

  local stem="${filename#CONTRACT-}"
  stem="${stem%.md}"

  if [[ "$stem" =~ ^(.+)\.([0-9]+)\.([0-9]+)$ ]]; then
    CONTRACT_ID="${BASH_REMATCH[1]}"
    CONTRACT_VERSION="${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
  elif [[ "$stem" =~ ^(.+)\.([0-9]+)$ ]]; then
    CONTRACT_ID="${BASH_REMATCH[1]}"
    CONTRACT_VERSION="${BASH_REMATCH[2]}.0"
  else
    CONTRACT_ID="$stem"
    CONTRACT_VERSION="1.0"
  fi
}

# Extract status from contract file (looks for **Status:** or Status: line)
extract_status() {
  local file="$1"
  local status
  status=$(grep -i '^\*\*Status:\*\*\|^Status:' "$file" 2>/dev/null | head -1 | sed 's/.*:\s*//' | tr -d '*' | xargs)
  echo "${status:-draft}"
}

# Extract purpose: the first non-empty line after the purpose heading.
#
#   registry_purpose_heading  heading text (ERE) to take the purpose from,
#                             e.g. `Purpose|Why this exists`. When set, a
#                             numbered heading (`## 2. Purpose`) also matches.
#                             Unset: `## Purpose` only.
#   registry_purpose_style    bytes (default): first 80 bytes, as before.
#                             ellipsis: escape `|` for the table and cut to
#                             77 characters plus `...`.
extract_purpose() {
  local file="$1" heading_re purpose
  heading_re='^## Purpose'
  if [ -n "$PURPOSE_HEADING" ]; then
    heading_re="^## ([0-9]+[.][[:space:]]*)?(${PURPOSE_HEADING})"
  fi
  purpose=$(awk -v re="$heading_re" '$0 ~ re {found=1; next} found && /^[^#]/ && NF{print; exit}' "$file" 2>/dev/null)
  if [ "$PURPOSE_STYLE" = ellipsis ]; then
    # Character-aware slicing needs a UTF-8 locale; the rest of the script
    # stays in C for stable sort order.
    (
      export LC_ALL=C.UTF-8
      purpose=${purpose//|/\\|}
      if [ "${#purpose}" -gt 80 ]; then
        purpose="${purpose:0:77}..."
      fi
      printf '%s' "$purpose"
    )
  else
    printf '%s' "$purpose" | head -c 80
  fi
}

# Classify by prefix. registry_scope_prefixes (e.g. `SDK- MCP-`) are
# stripped, in order, before classifying, so SDK-S1-FOO files as a Service.
classify_prefix() {
  local id="$1" prefix
  for prefix in $SCOPE_PREFIXES; do
    id="${id#"$prefix"}"
  done
  case "$id" in
    S*) echo "Services" ;;
    C*) echo "Components" ;;
    I*) echo "Interfaces" ;;
    P*) echo "Protocols" ;;
    *)  echo "Other" ;;
  esac
}

# Search source files for a pattern: search_source <grep flags...> <pattern>.
#
# In a git work tree this searches tracked files only (git grep), so counts
# are the same on every checkout: an untracked scratch file or a gitignored
# build output with a CONTRACT: header doesn't change the registry. Outside
# git it falls back to grep -r.
#
# Searched files:
#   1. Source files matching the standard extension whitelist
#   2. Files anywhere under `bin/` (CLI entry points often have no extension —
#      `bin/ask`, `bin/ask-mcp-server`, `bin/rebar`)
# minus the top-level architecture/ (the contracts themselves) and any
# node_modules/, vendor/ or dist/ directory.
#
# `grep` returns 1 when there are no matches; callers run with pipefail
# temporarily disabled so an orphan contract doesn't abort the script.
IN_GIT=false
if git -C "$PROJECT_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  IN_GIT=true
fi

# The searched files are configurable in .rebarrc:
#   registry_impl_globs    git pathspecs; a trailing / means a whole directory
#                          (default: the list above plus bin/)
#   registry_exclude_dirs  directory names skipped anywhere
#                          (default: node_modules vendor dist)
set -f
GIT_PATHSPECS=()
GREP_INCLUDES=()
GREP_DIRS=()
for glob in $IMPL_GLOBS; do
  GIT_PATHSPECS+=("$glob")
  case "$glob" in
    */) GREP_DIRS+=("${glob%/}") ;;
    *)  GREP_INCLUDES+=("--include=$glob") ;;
  esac
done
GIT_PATHSPECS+=(':(exclude)architecture/')
GREP_EXCLUDE='\.git\|architecture/'
for dir in $EXCLUDE_DIRS; do
  GIT_PATHSPECS+=(":(exclude,glob)**/$dir/**")
  GREP_EXCLUDE="$dir\\|$GREP_EXCLUDE"
done
set +f

search_source() {
  if [ "$IN_GIT" = true ]; then
    git -C "$PROJECT_ROOT" grep "$@" -- "${GIT_PATHSPECS[@]}" 2>/dev/null
  else
    {
      if [ ${#GREP_INCLUDES[@]} -gt 0 ]; then
        grep -r "$@" "$PROJECT_ROOT" "${GREP_INCLUDES[@]}" 2>/dev/null
      fi
      local dir
      for dir in ${GREP_DIRS[@]+"${GREP_DIRS[@]}"}; do
        if [ -d "$PROJECT_ROOT/$dir" ]; then
          grep -r "$@" "$PROJECT_ROOT/$dir" 2>/dev/null
        fi
      done
    } | grep -v "$GREP_EXCLUDE"
  fi
}

# Count implementing files for a contract, deduplicated by path.
count_implementations() {
  local id="$1"
  set +o pipefail
  search_source -l "CONTRACT:${id}" | sort -u | wc -l | tr -d ' '
  set -o pipefail
}

# ─── Collect Contracts ──────────────────────────────────────────────────────

declare -a services=() components=() interfaces=() protocols=() other=()

for contract in "$ARCH_DIR"/CONTRACT-*.md; do
  [ ! -f "$contract" ] && continue

  basename=$(basename "$contract")

  # Skip non-contract files
  case "$basename" in
    CONTRACT-TEMPLATE.md|CONTRACT-SEAM-TEMPLATE.md|CONTRACT-REGISTRY.md|CONTRACT-REGISTRY.template.md|CONTRACT-GAPS.md)
      continue ;;
    *.impl.md)
      continue ;;
  esac

  parse_contract_file "$contract"
  local_status=$(extract_status "$contract")
  local_purpose=$(extract_purpose "$contract")

  if [ "$IMPL_COUNTS" = true ]; then
    impl_count=$(count_implementations "$CONTRACT_ID")
    entry="| $CONTRACT_ID | $CONTRACT_VERSION | $local_status | $impl_count | ${local_purpose:-—} |"
  else
    entry="| $CONTRACT_ID | $CONTRACT_VERSION | $local_status | ${local_purpose:-—} |"
  fi

  category=$(classify_prefix "$CONTRACT_ID")
  case "$category" in
    Services)   services+=("$entry") ;;
    Components) components+=("$entry") ;;
    Interfaces) interfaces+=("$entry") ;;
    Protocols)  protocols+=("$entry") ;;
    *)          other+=("$entry") ;;
  esac
done

# ─── Generate Output ────────────────────────────────────────────────────────

generate_registry() {
  cat <<HEADER
# Contract Registry

<!-- AUTO-GENERATED by scripts/compute-registry.sh — do not edit manually -->
<!-- Regenerate: ./scripts/compute-registry.sh -->

HEADER

  # bash 3.2-compatible: pass array entries through a function instead of
  # using `local -n` namerefs (bash 4.3+). macOS ships bash 3.2 by default.
  emit_category() {
    local category="$1"; shift
    [ $# -eq 0 ] && return
    echo "## $category"
    echo ""
    if [ "$IMPL_COUNTS" = true ]; then
      echo "| ID | Version | Status | Impl Files | Purpose |"
      echo "|----|---------|--------|------------|---------|"
    else
      echo "| ID | Version | Status | Purpose |"
      echo "|----|---------|--------|---------|"
    fi
    local entry
    for entry in "$@"; do
      echo "$entry"
    done
    echo ""
  }

  emit_category "Services"   ${services[@]+"${services[@]}"}
  emit_category "Components" ${components[@]+"${components[@]}"}
  emit_category "Interfaces" ${interfaces[@]+"${interfaces[@]}"}
  emit_category "Protocols"  ${protocols[@]+"${protocols[@]}"}
  emit_category "Other"      ${other[@]+"${other[@]}"}

  # Contract files section (for tooling compatibility)
  echo "## Contract Files"
  echo ""
  echo "<!-- Plain list of filenames for grep-based tooling -->"
  for contract in "$ARCH_DIR"/CONTRACT-*.md; do
    [ ! -f "$contract" ] && continue
    basename=$(basename "$contract")
    case "$basename" in
      CONTRACT-TEMPLATE.md|CONTRACT-SEAM-TEMPLATE.md|CONTRACT-REGISTRY.md|CONTRACT-REGISTRY.template.md|CONTRACT-GAPS.md)
        continue ;;
      *.impl.md)
        continue ;;
    esac
    echo "- $basename"
  done
  echo ""

  if [ "$IMPL_COUNTS" = true ]; then
    generate_health
  fi
}

# ─── Contract Health: zombie + shadow detection ────────────────────────────
generate_health() {
  # zombie  — contract file exists but no CONTRACT:<ID> headers in source
  #           (deletion candidate unless tracked in TODO.md)
  # shadow  — CONTRACT:<ID> reference in source but no architecture/<ID>.md
  #           file (typo, deleted contract, or not-yet-written spec)

  local zombies=()
  local known_ids=" "  # space-padded list for easy substring matching
  for contract in "$ARCH_DIR"/CONTRACT-*.md; do
    [ ! -f "$contract" ] && continue
    basename=$(basename "$contract")
    case "$basename" in
      CONTRACT-TEMPLATE.md|CONTRACT-SEAM-TEMPLATE.md|CONTRACT-REGISTRY.md|CONTRACT-REGISTRY.template.md|CONTRACT-GAPS.md)
        continue ;;
      *.impl.md)
        continue ;;
    esac

    parse_contract_file "$contract"
    known_ids="$known_ids$CONTRACT_ID "

    local count
    count=$(count_implementations "$CONTRACT_ID")
    if [ "$count" -eq 0 ]; then
      if [ -f "$PROJECT_ROOT/TODO.md" ] && grep -q "$CONTRACT_ID" "$PROJECT_ROOT/TODO.md" 2>/dev/null; then
        zombies+=("- **$CONTRACT_ID** — tracked in TODO.md")
      else
        zombies+=("- **$CONTRACT_ID** — ⚠ untracked, 0 implementing files")
      fi
    fi
  done

  # Shadow detection: every CONTRACT:<ID>.<v.v> reference in source,
  # filtered to IDs not in known_ids. Same extension whitelist + bin/
  # second-pass as count_implementations() so we don't miss CLI scripts.
  local shadows_tmp
  shadows_tmp="$(mktemp)"
  set +o pipefail
  search_source -Eho 'CONTRACT:[A-Za-z0-9_-]+\.[0-9]+\.[0-9]+' \
    | sed -E 's/^CONTRACT://; s/\.[0-9]+\.[0-9]+$//' \
    | sort -u > "$shadows_tmp"
  set -o pipefail

  local shadows=()
  while IFS= read -r ref_id; do
    [ -z "$ref_id" ] && continue
    case "$known_ids" in
      *" $ref_id "*) ;;  # known — not a shadow
      *) shadows+=("- **$ref_id** — referenced in source, no architecture/CONTRACT-${ref_id}.*.md found") ;;
    esac
  done < "$shadows_tmp"
  rm -f "$shadows_tmp"

  if [ ${#zombies[@]} -gt 0 ] || [ ${#shadows[@]} -gt 0 ]; then
    echo "## Contract Health"
    echo ""

    if [ ${#zombies[@]} -gt 0 ]; then
      echo "### Zombies — contract files with no implementing code"
      echo ""
      echo "(Deletion candidates unless intentionally pre-impl. Add a TODO.md entry"
      echo "to suppress this warning while implementation is in flight.)"
      echo ""
      for z in "${zombies[@]}"; do
        echo "$z"
      done
      echo ""
    fi

    if [ ${#shadows[@]} -gt 0 ]; then
      echo "### Shadow refs — code references to nonexistent contracts"
      echo ""
      echo "(Either write the missing contract file or fix the typo in source.)"
      echo ""
      for s in "${shadows[@]}"; do
        echo "$s"
      done
      echo ""
    fi
  fi
}

# Implementing-file counts plus zombie/shadow detection, for --health.
generate_health_report() {
  echo "## Implementing Files"
  echo ""
  echo "| ID | Impl Files |"
  echo "|----|------------|"
  local contract basename
  for contract in "$ARCH_DIR"/CONTRACT-*.md; do
    [ ! -f "$contract" ] && continue
    basename=$(basename "$contract")
    case "$basename" in
      CONTRACT-TEMPLATE.md|CONTRACT-SEAM-TEMPLATE.md|CONTRACT-REGISTRY.md|CONTRACT-REGISTRY.template.md|CONTRACT-GAPS.md)
        continue ;;
      *.impl.md)
        continue ;;
    esac
    parse_contract_file "$contract"
    echo "| $CONTRACT_ID | $(count_implementations "$CONTRACT_ID") |"
  done | sort -u
  echo ""
  generate_health
}

# ─── Execute Mode ───────────────────────────────────────────────────────────

case "$MODE" in
  --check)
    if [ ! -f "$REGISTRY" ]; then
      echo "FAIL: $REGISTRY does not exist. Run scripts/compute-registry.sh to generate."
      exit 1
    fi

    # Generate to temp and compare
    tmpfile=$(mktemp)
    generate_registry > "$tmpfile"

    # Ignore the timestamp line older versions wrote
    if diff <(grep -v "^<!-- Generated:" "$REGISTRY") <(grep -v "^<!-- Generated:" "$tmpfile") >/dev/null 2>&1; then
      echo "OK: Registry is current."
      rm "$tmpfile"
      exit 0
    else
      echo "STALE: Registry needs regeneration. Run scripts/compute-registry.sh"
      rm "$tmpfile"
      exit 1
    fi
    ;;

  --stdout)
    generate_registry
    ;;

  --health)
    generate_health_report
    ;;

  *)
    generate_registry > "$REGISTRY"
    echo "Generated: $REGISTRY"
    ;;
esac
