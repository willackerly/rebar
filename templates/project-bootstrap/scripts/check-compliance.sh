#!/usr/bin/env bash
# check-compliance.sh — Verify rebar compliance: version, tier, README badge, AGENTS.md
# rebar-scripts: 2026.03.20
#
# Checks:
#   1. .rebar-version file exists and contains a valid semver tag
#   2. .rebarrc file exists and declares a tier (1, 2, or 3)
#   3. README.md has a well-formed rebar badge on the first content line after the title
#   4. Badge version matches .rebar-version
#   5. Badge tier matches .rebarrc tier
#   6. AGENTS.md has required load-bearing sections (Tier 2+)
#   7. AGENTS.md mentions ASK CLI (Tier 2+)
#
# Badge format (must be a blockquote, first line after # Title):
#   > **rebar vX.Y.Z** | **Tier N: LEVEL**
#
# Where LEVEL is: PARTIAL (1), ADOPTED (2), or ENFORCED (3)
#
# Usage: ./scripts/check-compliance.sh
# Exit code: 0 = compliant, 1 = non-compliant

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
README="$PROJECT_ROOT/README.md"
AGENTS="$PROJECT_ROOT/AGENTS.md"
VERSION_FILE="$PROJECT_ROOT/.rebar-version"
RC_FILE="$PROJECT_ROOT/.rebarrc"

errors=0

tier_label() {
  case "$1" in
    1) echo "PARTIAL" ;;
    2) echo "ADOPTED" ;;
    3) echo "ENFORCED" ;;
    *) echo "UNKNOWN" ;;
  esac
}

# ─── Check 1: .rebar-version file ────────────────────────────────────────

echo "=== Rebar compliance check ==="
echo ""

if [ ! -f "$VERSION_FILE" ]; then
  echo "FAIL: .rebar-version file not found"
  echo "  Create it with: echo 'v2.0.0' > .rebar-version"
  errors=$((errors + 1))
  declared_version=""
else
  declared_version=$(cat "$VERSION_FILE" | tr -d '[:space:]')
  if [[ "$declared_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "OK: .rebar-version = $declared_version"
  else
    echo "FAIL: .rebar-version contains '$declared_version' — expected format: vX.Y.Z"
    errors=$((errors + 1))
  fi
fi

# ─── Check 2: .rebarrc tier ──────────────────────────────────────────────

if [ ! -f "$RC_FILE" ]; then
  echo "FAIL: .rebarrc file not found"
  echo "  Create it from .rebarrc.template"
  errors=$((errors + 1))
  declared_tier=""
else
  declared_tier=$(grep '^tier' "$RC_FILE" 2>/dev/null | head -1 | sed 's/.*=[[:space:]]*//' | tr -d ' ')
  if [[ "$declared_tier" =~ ^[123]$ ]]; then
    echo "OK: .rebarrc tier = $declared_tier ($(tier_label "$declared_tier"))"
  else
    echo "FAIL: .rebarrc tier is '$declared_tier' — expected 1, 2, or 3"
    errors=$((errors + 1))
  fi
fi

# ─── Check 3: README.md rebar badge ──────────────────────────────────────

if [ ! -f "$README" ]; then
  echo "FAIL: README.md not found"
  errors=$((errors + 1))
else
  # Find the badge line: first blockquote line containing "rebar v" after the title
  badge_line=$(grep -n '^\s*>\s*\*\*rebar v' "$README" | head -1 || true)

  if [ -z "$badge_line" ]; then
    echo "FAIL: README.md has no rebar badge"
    echo "  Add this as the first line after your # Title:"
    echo '  > **rebar v2.0.0** | **Tier 2: ADOPTED**'
    errors=$((errors + 1))
  else
    line_num=$(echo "$badge_line" | cut -d: -f1)
    line_content=$(echo "$badge_line" | cut -d: -f2-)

    # Validate format: > **rebar vX.Y.Z** | **Tier N: LEVEL**
    if [[ "$line_content" =~ \*\*rebar\ (v[0-9]+\.[0-9]+\.[0-9]+)\*\*.*\*\*Tier\ ([0-9]+):\ ([A-Z]+)\*\* ]]; then
      badge_version="${BASH_REMATCH[1]}"
      badge_tier="${BASH_REMATCH[2]}"
      badge_level="${BASH_REMATCH[3]}"

      echo "OK: README.md badge found on line $line_num"
      echo "    Version: $badge_version | Tier: $badge_tier ($badge_level)"

      # Check badge is near the top (within first 5 lines)
      if [ "$line_num" -gt 5 ]; then
        echo "WARN: Badge is on line $line_num — should be within the first 3 lines (right after # Title)"
      fi

      # ─── Check 4: Badge version matches .rebar-version ──────────
      if [ -n "$declared_version" ] && [ "$badge_version" != "$declared_version" ]; then
        echo "FAIL: Badge says $badge_version but .rebar-version says $declared_version"
        errors=$((errors + 1))
      fi

      # ─── Check 5: Badge tier matches .rebarrc tier ──────────────
      if [ -n "$declared_tier" ] && [ "$badge_tier" != "$declared_tier" ]; then
        echo "FAIL: Badge says Tier $badge_tier but .rebarrc says tier = $declared_tier"
        errors=$((errors + 1))
      fi

      # Check level matches tier number
      expected_level=$(tier_label "$badge_tier")
      if [ "$badge_level" != "$expected_level" ]; then
        echo "FAIL: Tier $badge_tier should be '$expected_level', not '$badge_level'"
        errors=$((errors + 1))
      fi
    else
      echo "FAIL: README.md badge is malformed on line $line_num"
      echo "  Found: $line_content"
      echo '  Expected: > **rebar vX.Y.Z** | **Tier N: LEVEL**'
      echo '  Where LEVEL is PARTIAL (1), ADOPTED (2), or ENFORCED (3)'
      errors=$((errors + 1))
    fi
  fi
fi

# ─── Check 6: AGENTS.md load-bearing sections (Tier 2+) ──────────────────

# Source tier config if available
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  source "$SCRIPT_DIR/_rebar-config.sh"
  current_tier=$(_rebar_tier)
else
  current_tier="${declared_tier:-3}"
fi

if [ "$current_tier" -ge 2 ] && [ -f "$AGENTS" ]; then
  echo ""
  echo "=== AGENTS.md required sections ==="

  # These are the load-bearing walls. Without them, agents don't know
  # about contracts, testing discipline, or the TODO system.
  required_sections=(
    "Cold Start\|Read Before Coding:Cold Start / Read Before Coding — agents must know the reading order"
    "Contract-Driven\|Contract.Driven:Contract-Driven Development — the 4 rules that make contracts operational"
    "Testing Cascade\|Testing Expectations\|Scout Rule:Testing expectations — cascade tiers or scout rule"
    "TODO Tracking:TODO Tracking — two-tag system prevents invisible debt"
  )

  for entry in "${required_sections[@]}"; do
    pattern="${entry%%:*}"
    description="${entry##*:}"

    if grep -qi "$pattern" "$AGENTS" 2>/dev/null; then
      echo "OK: $description"
    else
      echo "FAIL: AGENTS.md missing required section: $description"
      errors=$((errors + 1))
    fi
  done

  # Check 7: ASK CLI awareness — agents need to know they can query role-based agents
  if grep -qi '\bask\b\|ASK CLI\|ask architect\|ask product\|ask steward\|ask englead' "$AGENTS" 2>/dev/null; then
    echo "OK: AGENTS.md mentions ASK CLI"
  else
    echo "FAIL: AGENTS.md does not mention ASK CLI"
    echo "  Agents need to know they can use 'ask <role> \"question\"' for focused queries."
    echo "  Add a reference in the Cold Start or Reference section."
    errors=$((errors + 1))
  fi

elif [ "$current_tier" -ge 2 ] && [ ! -f "$AGENTS" ]; then
  echo ""
  echo "FAIL: AGENTS.md not found (required at Tier 2+)"
  errors=$((errors + 1))
fi

# ─── Summary ──────────────────────────────────────────────────────────────

echo ""
if [ "$errors" -gt 0 ]; then
  echo "FAIL: $errors compliance issue(s) found."
  exit 1
else
  echo "OK: Rebar compliance verified."
  exit 0
fi
