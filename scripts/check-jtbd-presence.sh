#!/usr/bin/env bash
# check-jtbd-presence.sh — Enforce JTBD framing sections on contracts
# rebar-scripts: 2026.07.04
#
# Usage: ./scripts/check-jtbd-presence.sh
#
# Every latest-version contract in architecture/ MUST have non-empty
# "## Why this exists", "## Who needs this", and "## Scenarios" sections —
# the JTBD framing required by CONTRACT-TEMPLATE.md. Catches the
# "interface description without motivation" anti-pattern: a contract that
# reads like a header file with no callers.
#
# Skipped:
#   - Template / registry / companion files (CONTRACT-TEMPLATE.md,
#     CONTRACT-SEAM-TEMPLATE.md, CONTRACT-REGISTRY*, CONTRACT-GAPS.md,
#     *.impl.md)
#   - Older versions of a contract ID — only the latest version on disk is
#     held to the requirement
#   - Contracts marked "SUPERSEDED BY:" — covers the window where the newer
#     version lives on another branch or lands in a concurrent commit
#   - SKIP_JTBD=1 skips the whole check
#
# Tier behavior: below Tier 2 = skip; Tier 2 = warn only (exit 0);
# Tier 3 = blocking (exit 1 on missing sections).
#
# Source: feedback/2026-04-24-contract-discipline-and-jtbd-framing.md §E
# See: practices/spike-first-contracts.md for the JTBD framing rationale
#
# Bash 3.2 compatible (macOS default).
#
# Exit code: 0 = all present (or advisory tier), 1 = missing sections at Tier 3

set -uo pipefail

# Honor the explicit skip flag (house style: ci-check.sh SKIP_* family)
if [ "${SKIP_JTBD:-0}" = "1" ]; then
  echo "SKIP: check-jtbd-presence (SKIP_JTBD=1)"
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ARCH_DIR="$PROJECT_ROOT/architecture"

# Tier gate: JTBD presence is Tier 2+ (warning) and blocks at Tier 3
TIER=3
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  source "$SCRIPT_DIR/_rebar-config.sh"
  _rebar_skip 2 && exit 0
  TIER="$(_rebar_tier)"
fi

if [ ! -d "$ARCH_DIR" ]; then
  echo "SKIP: no architecture/ directory found."
  exit 0
fi

index_tmp="$(mktemp)"
latest_tmp="$(mktemp)"
trap 'rm -f "$index_tmp" "$latest_tmp"' EXIT

# ─── Index contracts: ID, major, minor, filename ────────────────────────────

for contract in "$ARCH_DIR"/CONTRACT-*.md; do
  [ -f "$contract" ] || continue
  base="$(basename "$contract")"

  # Skip templates, registry files, gap tracker, and companions
  case "$base" in
    CONTRACT-TEMPLATE.md|CONTRACT-SEAM-TEMPLATE.md|CONTRACT-REGISTRY*|CONTRACT-GAPS.md)
      continue ;;
    *.impl.md)
      continue ;;
  esac

  # Parse ID + version from filename (same logic as compute-registry.sh)
  stem="${base#CONTRACT-}"
  stem="${stem%.md}"
  if [[ "$stem" =~ ^(.+)\.([0-9]+)\.([0-9]+)$ ]]; then
    id="${BASH_REMATCH[1]}"; major="${BASH_REMATCH[2]}"; minor="${BASH_REMATCH[3]}"
  elif [[ "$stem" =~ ^(.+)\.([0-9]+)$ ]]; then
    id="${BASH_REMATCH[1]}"; major="${BASH_REMATCH[2]}"; minor=0
  else
    id="$stem"; major=1; minor=0
  fi

  printf '%s\t%s\t%s\t%s\n' "$id" "$major" "$minor" "$base" >> "$index_tmp"
done

if [ ! -s "$index_tmp" ]; then
  echo "OK: no contracts found in architecture/ — nothing to check."
  exit 0
fi

# Keep only the latest version per contract ID (sort by ID, then version
# numerically; last row per ID wins). awk arrays keep this bash 3.2 safe —
# no associative arrays in the shell itself.
sort -t"$(printf '\t')" -k1,1 -k2,2n -k3,3n "$index_tmp" \
  | awk -F'\t' '{ latest[$1] = $4 } END { for (id in latest) print latest[id] }' \
  | sort > "$latest_tmp"

# ─── Section presence ────────────────────────────────────────────────────────

# section_present <file> <heading-regex>
# Returns 0 when the section exists AND has non-comment, non-whitespace
# content before the next same-or-higher-level heading. Subsections
# (### Scenario 1 — ...) belong to the section and count as content.
# HTML comments (the template's inline guidance) don't count as content.
section_present() {
  awk -v pat="$2" '
    BEGIN { insect = 0; incom = 0; content = 0 }
    insect == 0 && $0 ~ pat { insect = 1; next }
    insect == 1 && /^##?[[:space:]]/ { exit }
    insect == 1 {
      line = $0
      if (incom) {
        if (sub(/^.*-->/, "", line)) incom = 0
        else next
      }
      gsub(/<!--.*-->/, "", line)
      if (index(line, "<!--") > 0) { sub(/<!--.*$/, "", line); incom = 1 }
      gsub(/[[:space:]]/, "", line)
      if (length(line) > 0) content = 1
    }
    END { exit (content ? 0 : 1) }
  ' "$1"
}

checked=0
skipped_superseded=0
failed=0

while IFS= read -r base; do
  file="$ARCH_DIR/$base"
  [ -f "$file" ] || continue

  # Skip superseded contracts — only the live version carries the burden.
  # Matches the marker format from CONTRACT-TEMPLATE.md:
  #   SUPERSEDED BY: CONTRACT-{ID}-{NAME}.{NEW}
  if grep -q 'SUPERSEDED BY:' "$file" 2>/dev/null; then
    skipped_superseded=$((skipped_superseded + 1))
    continue
  fi

  checked=$((checked + 1))
  missing=""
  section_present "$file" '^## Why this exists' || missing="$missing 'Why this exists'"
  section_present "$file" '^## Who needs this'  || missing="$missing 'Who needs this'"
  section_present "$file" '^## Scenarios'       || missing="$missing 'Scenarios'"

  if [ -n "$missing" ]; then
    echo "MISSING: architecture/$base —$missing"
    failed=$((failed + 1))
  fi
done < "$latest_tmp"

echo ""
echo "Checked $checked latest-version contract(s), $skipped_superseded superseded skipped, $failed with missing JTBD sections."

if [ "$failed" -gt 0 ]; then
  echo ""
  echo "Every contract needs non-empty 'Why this exists', 'Who needs this', and"
  echo "'Scenarios' sections. An interface description is not a Job To Be Done —"
  echo "see architecture/CONTRACT-TEMPLATE.md for the required framing."
  if [ "$TIER" -ge 3 ]; then
    exit 1
  fi
  echo ""
  echo "WARN: advisory at Tier $TIER; blocking at Tier 3."
  exit 0
fi

echo "OK: all checked contracts carry the JTBD sections."
exit 0
