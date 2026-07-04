#!/usr/bin/env bash
# check-prefix-uniqueness.sh — Fail on duplicate contract prefix numbers
# rebar-scripts: 2026.07.04
#
# Usage: ./scripts/check-prefix-uniqueness.sh
#
# Parses the "Contract Files" list in architecture/CONTRACT-REGISTRY.md and
# fails when one prefix number (S1, I3, ...) is claimed by two DIFFERENT
# contract IDs — e.g., I3-LLM-CLIENT and I3-SCANNER both squatting on I3.
# Multiple versions of the same ID (S1-STEWARD.1.0 + S1-STEWARD.2.0) are
# fine — that's supersession, not collision.
#
# The registry is the parse surface because it is the computed, canonical
# index ("the contract filesystem IS the registry"); staleness of the
# registry itself is caught separately by compute-registry.sh --check.
#
# Skipped:
#   - SKIP_PREFIX_UNIQUENESS=1 skips the whole check
#   - Missing registry file (advisory note, exit 0 — generation is a
#     separate concern)
#   - IDs whose first segment is not a letters+digits prefix token
#     (e.g., SEAM-* integration seam contracts have no prefix number)
#
# Tier behavior: below Tier 2 = skip; Tier 2 = warn only (exit 0);
# Tier 3 = blocking (exit 1 on collisions).
#
# Source: feedback/2026-04-24-contract-discipline-and-jtbd-framing.md
#         ("I3 number collision" drift class)
# See: practices/contract-supersession.md for versioning vs. renumbering
#
# Bash 3.2 compatible (macOS default).
#
# Exit code: 0 = unique (or advisory tier), 1 = collisions at Tier 3

set -uo pipefail

# Honor the explicit skip flag (house style: ci-check.sh SKIP_* family)
if [ "${SKIP_PREFIX_UNIQUENESS:-0}" = "1" ]; then
  echo "SKIP: check-prefix-uniqueness (SKIP_PREFIX_UNIQUENESS=1)"
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REGISTRY="$PROJECT_ROOT/architecture/CONTRACT-REGISTRY.md"

# Tier gate: prefix uniqueness is Tier 2+ (warning) and blocks at Tier 3
TIER=3
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  source "$SCRIPT_DIR/_rebar-config.sh"
  _rebar_skip 2 && exit 0
  TIER="$(_rebar_tier)"
fi

if [ ! -f "$REGISTRY" ]; then
  echo "SKIP: no architecture/CONTRACT-REGISTRY.md (run scripts/compute-registry.sh to generate)."
  exit 0
fi

pairs_tmp="$(mktemp)"
dupes_tmp="$(mktemp)"
trap 'rm -f "$pairs_tmp" "$dupes_tmp"' EXIT

# ─── Extract (prefix, ID) pairs from the registry's file list ───────────────
#
# The "## Contract Files" section lists one filename per line:
#   - CONTRACT-S1-STEWARD.1.0.md
# Strip the wrapper and the trailing version to get the ID (S1-STEWARD),
# then take the first hyphen-delimited segment as the prefix token (S1).

grep '^- CONTRACT-' "$REGISTRY" 2>/dev/null \
  | sed -e 's/^- CONTRACT-//' -e 's/\.md$//' \
  | sed -E 's/\.[0-9]+(\.[0-9]+)?$//' \
  | while IFS= read -r id; do
      [ -z "$id" ] && continue
      prefix="${id%%-*}"
      # Prefix token must be letters followed by digits (S1, I12, ...).
      # Anything else (SEAM-*, unnumbered names) has no number to collide.
      if [[ "$prefix" =~ ^[A-Za-z]+[0-9]+$ ]]; then
        printf '%s\t%s\n' "$prefix" "$id"
      fi
    done \
  | sort -u > "$pairs_tmp"

if [ ! -s "$pairs_tmp" ]; then
  echo "OK: no numbered contract IDs listed in the registry — nothing to check."
  exit 0
fi

# ─── Find prefixes claimed by more than one distinct ID ─────────────────────

awk -F'\t' '
  { count[$1]++; ids[$1] = ids[$1] ", " $2 }
  END {
    for (p in count) {
      if (count[p] > 1) {
        sub(/^, /, "", ids[p])
        printf "%s: %s\n", p, ids[p]
      }
    }
  }
' "$pairs_tmp" | sort > "$dupes_tmp"

total="$(wc -l < "$pairs_tmp" | tr -d ' ')"
dupe_count="$(wc -l < "$dupes_tmp" | tr -d ' ')"

if [ "$dupe_count" -eq 0 ]; then
  echo "OK: $total contract ID(s) in registry, no prefix-number collisions."
  exit 0
fi

echo "FAIL: $dupe_count prefix number(s) claimed by more than one contract ID:"
echo ""
sed 's/^/  /' "$dupes_tmp"
echo ""
echo "Each prefix number (S1, I3, ...) must map to exactly one contract ID."
echo "Versions of the same ID are fine; two different IDs sharing a number"
echo "overload the prefix. Renumber the newer contract to the next free number,"
echo "update its implementing-file headers (grep -rn \"CONTRACT:<old-id>\"), and"
echo "regenerate the registry with scripts/compute-registry.sh."

if [ "$TIER" -ge 3 ]; then
  exit 1
fi
echo ""
echo "WARN: advisory at Tier $TIER; blocking at Tier 3."
exit 0
