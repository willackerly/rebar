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
#   8. Federation drift-check wired when CONSUMES.md declares dependencies
#   9. Contract maturity — reads DECLARED `**Status:**` fields from
#      architecture/CONTRACT-*.md and weights the badge (v3, Cluster 1):
#        <33% stub-or-draft → tier stands as declared
#        33–66%             → tier annotated "— IN PROGRESS" (advisory)
#        >66%               → badge demoted one tier (compliance failure)
#      Zero Status: fields anywhere → pre-v3 repo: no penalty, one advisory.
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
PROJECT_ROOT="${REBAR_PROJECT_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
README="$PROJECT_ROOT/README.md"
AGENTS="$PROJECT_ROOT/AGENTS.md"
VERSION_FILE="$PROJECT_ROOT/.rebar-version"
RC_FILE="$PROJECT_ROOT/.rebarrc"
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  # shellcheck source=scripts/_rebar-config.sh
  source "$SCRIPT_DIR/_rebar-config.sh"
  # Under `REBAR_CHECK_IMPL=go rebar check` (the default) the CLI's native port runs instead.
  _rebar_native check-compliance "$@"
fi

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
  echo "  Create it with: echo 'v3.1.2' > .rebar-version"
  errors=$((errors + 1))
  declared_version=""
else
  declared_version=$(cat "$VERSION_FILE" | tr -d '[:space:]')
  # Semver with optional pre-release suffix (v3.1.0, v3.2.0-rc.1)
  if [[ "$declared_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
    echo "OK: .rebar-version = $declared_version"
  else
    echo "FAIL: .rebar-version contains '$declared_version' — expected format: vX.Y.Z or vX.Y.Z-prerelease"
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
  # '|| true' — under errexit a .rebarrc with no tier line must reach the
  # FAIL message below, not kill the script mid-pipeline with no diagnostic.
  declared_tier=$(grep '^tier' "$RC_FILE" 2>/dev/null | head -1 | sed 's/.*=[[:space:]]*//' | tr -d ' ' || true)
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
    echo '  > **rebar v3.1.2** | **Tier 2: ADOPTED**'
    errors=$((errors + 1))
  else
    line_num=$(echo "$badge_line" | cut -d: -f1)
    line_content=$(echo "$badge_line" | cut -d: -f2-)

    # Validate format: > **rebar vX.Y.Z[-prerelease]** | **Tier N: LEVEL**
    if [[ "$line_content" =~ \*\*rebar\ (v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?)\*\*.*\*\*Tier\ ([0-9]+):\ ([A-Z]+)\*\* ]]; then
      badge_version="${BASH_REMATCH[1]}"
      badge_tier="${BASH_REMATCH[3]}"
      badge_level="${BASH_REMATCH[4]}"

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

# The tier config, when it was sourced above.
if type _rebar_check_tier >/dev/null 2>&1; then
  current_tier=$(_rebar_check_tier check-compliance)
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

# ─── Check 8: Federation — drift-check wired when CONSUMES.md exists ─────
#
# CHARTER §1.6 makes federation opt-in but compliance-gated: once a repo
# adds a CONSUMES.md (declaring cross-repo dependencies), it MUST run
# `rebar contract drift-check` in CI so consumers don't silently age out
# while owner contracts evolve. Adopters self-select by adding the file.

CONSUMES_FILE="$PROJECT_ROOT/CONSUMES.md"
if [ -f "$CONSUMES_FILE" ]; then
  echo ""
  echo "=== Federation (CONSUMES.md present) ==="

  # Has at least one real entry (## owner/contract.version section)?
  # Count REAL declarations only — a fenced code block documenting the format
  # is not a dependency. The bootstrap's own CONSUMES.md carries five worked
  # examples inside a ```markdown fence; a line-anchored grep counted all five,
  # so every freshly-bootstrapped project failed this check out of the box
  # ("declares dependencies but drift-check is not wired") without ever having
  # declared anything. sch-repos named this class exactly, in another context,
  # the same day: an anchored regex cannot distinguish a real declaration from
  # a document quoting one. Toggling on ``` fences is the cheap correct fix.
  consumes_entries=$(awk '
    /^```/ { fence = !fence; next }
    !fence && /^## [A-Za-z0-9_-]+\/[A-Za-z0-9_.-]+$/ { n++ }
    END { print n + 0 }
  ' "$CONSUMES_FILE" 2>/dev/null | tr -d '[:space:]' || echo 0)
  if [ "${consumes_entries:-0}" = "0" ]; then
    echo "OK: CONSUMES.md present but no entries declared (federation opt-in not yet active)"
  else
    echo "OK: CONSUMES.md declares $consumes_entries cross-repo dependency(ies)"

    # drift-check must be wired into a CI-relevant script. Check the
    # standard rebar surfaces in priority order. An adopter can override
    # by adding their own framework (Makefile, GitHub Actions, etc.) —
    # we look across the obvious places.
    drift_wired=0
    for f in "$PROJECT_ROOT/scripts/ci-check.sh" "$PROJECT_ROOT/scripts/pre-commit.sh" "$PROJECT_ROOT/Makefile" "$PROJECT_ROOT/.github/workflows"/*; do
      [ -e "$f" ] || continue
      if [ -d "$f" ]; then
        if grep -rq "drift-check" "$f" 2>/dev/null; then
          drift_wired=1
          break
        fi
      elif grep -q "drift-check" "$f" 2>/dev/null; then
        drift_wired=1
        break
      fi
    done

    if [ "$drift_wired" -eq 1 ]; then
      echo "OK: drift-check is wired into CI"
    else
      echo "FAIL: CONSUMES.md declares dependencies but \`rebar contract drift-check\` is not wired into CI"
      echo "  Add to scripts/ci-check.sh (or your CI of choice):"
      echo "    rebar contract drift-check"
      echo "  Without this, consumed contracts can silently age out as upstream evolves (CHARTER §1.6)."
      errors=$((errors + 1))
    fi
  fi
fi

# ─── Check 9: Contract maturity (declared Status: fields) ────────────────
#
# v3 maturity honesty (docs/v3-beta-plan.md Cluster 1): contracts DECLARE
# maturity in a `**Status:** <value>` header line — stub / draft /
# in-progress / active / verified. This is the human/agent honesty marker,
# distinct from the Steward's COMPUTED lifecycle
# (draft/active/testing/impl-present), which this check never reads.
# The badge is weighted by how much of the live contract set is still
# stub-or-draft, so a tier can't sit on top of placeholder contracts.

CONTRACTS_DIR=architecture
type _rebar_contracts_dir >/dev/null 2>&1 && CONTRACTS_DIR=$(_rebar_contracts_dir)
ARCH_DIR="$PROJECT_ROOT/$CONTRACTS_DIR"

if [ -d "$ARCH_DIR" ]; then
  echo ""
  echo "=== Contract maturity (declared Status:) ==="

  status_total=0     # Status: fields found across all contract files
  live_total=0       # live (non-superseded) contracts considered
  live_declared=0    # live contracts with a recognized maturity value
  stub_draft=0       # live contracts declared stub or draft
  missing_status=""  # newline-joined names of live contracts without Status:

  for contract in "$ARCH_DIR"/CONTRACT-*.md; do
    [ -f "$contract" ] || continue
    cbase="$(basename "$contract")"

    # Skip templates, the generated registry, and companion files —
    # same exclusions as compute-registry.sh.
    case "$cbase" in
      CONTRACT-TEMPLATE.md|CONTRACT-SEAM-TEMPLATE.md|CONTRACT-REGISTRY.md|CONTRACT-REGISTRY.template.md|CONTRACT-GAPS.md)
        continue ;;
      *.impl.md)
        continue ;;
    esac

    # Tolerant parse, identical to cold-start-checks.sh: bolded or bare
    # 'Status:' line, first word, case-folded. The canonical form stays
    # '**Status:** value' (conventions.md) but parsers of record must agree —
    # and conventions.md now publishes THIS regex verbatim so prose cannot
    # drift from the parser again.
    #
    # The strip is a single asterisk-tolerant expression rather than two
    # fixed alternatives. The old chain matched '^\*{0,2}Status:' but only
    # stripped the two-asterisk and bare forms, so '*Status:* active' fell
    # through with its marker intact, awk yielded '*Status:*', and the value
    # landed on the '*)' arm — silently counted as stub-or-draft. A
    # formatting slip became a badge penalty explained only by a WARN.
    # See feedback/2026-08-02-sch-repos-canonical-status-grep-*.md
    cstatus_raw="$(grep -m1 -E '^\*{0,2}Status:' "$contract" 2>/dev/null || true)"
    cstatus="$(printf '%s' "$cstatus_raw" \
      | sed -E 's/^\*{0,2}Status:\*{0,2}//' \
      | awk '{print $1}' | tr -d '*' | tr '[:upper:]' '[:lower:]' || true)"

    if [ -n "$cstatus" ]; then
      status_total=$((status_total + 1))
    fi

    # Terminal states are out of the live maturity mix — a superseded
    # contract kept around for its migration window shouldn't drag (or
    # inflate) the badge.
    case "$cstatus" in
      superseded|deprecated|retired) continue ;;
    esac

    live_total=$((live_total + 1))

    case "$cstatus" in
      "")
        # Warned about later — only when the repo has *some* Status: fields
        # (partially migrated). A pre-v3 repo gets one advisory, not N warns.
        # Once any contract declares, undeclared live contracts COUNT AS
        # stub-or-draft: selective declaration must not bypass demotion.
        missing_status="${missing_status}${cbase}
"
        ;;
      stub|draft)
        live_declared=$((live_declared + 1))
        stub_draft=$((stub_draft + 1))
        ;;
      in-progress|active|verified)
        live_declared=$((live_declared + 1))
        ;;
      *)
        # Report the RAW line, not just the parsed token. For a near-miss
        # format the parsed value is mangled, and a WARN naming the mangled
        # value is not actionable — the author cannot see what to change.
        echo "WARN: $cbase declares Status: '$cstatus' — not in the maturity vocabulary (stub|draft|in-progress|active|verified); counted as stub-or-draft"
        echo "  source line: $(printf '%s' "$cstatus_raw" | head -c 120)"
        stub_draft=$((stub_draft + 1))
        ;;
    esac
  done

  if [ "$status_total" -eq 0 ]; then
    if [ "$live_total" -gt 0 ]; then
      echo "ADVISORY: no contract declares a Status: field — treating as a pre-v3 repo (no maturity penalty)."
      echo "  Add '**Status:** <stub|draft|in-progress|active|verified>' to each contract header"
      echo "  (see $CONTRACTS_DIR/CONTRACT-TEMPLATE.md) so the badge can reflect real maturity."
    else
      echo "OK: no contracts found — maturity weighting not applicable"
    fi
  elif [ -n "$missing_status" ]; then
    # Partially migrated: some contracts declare Status:, these don't.
    printf '%s' "$missing_status" | while IFS= read -r mname; do
      echo "WARN: $mname has no Status: line — add one (see $CONTRACTS_DIR/CONTRACT-TEMPLATE.md header)"
    done
  fi

  if [ "$status_total" -gt 0 ] && [ "$live_declared" -eq 0 ] && [ "$live_total" -eq 0 ]; then
    echo "OK: no live contracts declare maturity (all terminal) — no maturity weighting"
  elif [ "$status_total" -gt 0 ] && [ "$live_total" -gt 0 ]; then
    # Weight over ALL live contracts: undeclared ones already counted as
    # stub-or-draft above (selective declaration must not launder a badge).
    undeclared=$((live_total - live_declared))
    if [ "$undeclared" -gt 0 ]; then
      stub_draft=$((stub_draft + undeclared))
    fi
    pct_tenths=$((stub_draft * 1000 / live_total))
    pct="$((pct_tenths / 10)).$((pct_tenths % 10))"
    echo "OK: $live_declared of $live_total live contract(s) declare maturity — $stub_draft stub-or-draft-or-undeclared (${pct}%)"

    if [[ "$declared_tier" =~ ^[123]$ ]]; then
      # Product comparisons — integer division floor must not soften the
      # documented thresholds (<33% ok, 33-66% annotate, >66% demote).
      #
      # DEGENERATE POPULATION GUARD (sch-repos, 2026-08-02). At live_total=1
      # a percentage quantizes to exactly 0% or 100%: the middle NOTE band is
      # mathematically unreachable, so the only honest downgrade available to
      # a one-contract repo jumps straight from OK to FAIL + tier demotion.
      # That made dishonesty the green path — declaring your single contract
      # `draft` (the honest value while it drifts from its implementation)
      # failed CI, while leaving it `active` passed. Exactly the inversion v3
      # exists to remove, and it lands on new/small adopters — the on-ramp.
      #
      # Threshold is -lt 2 because n=1 is precisely the degenerate case; from
      # n=2 the NOTE band is reachable and the formula behaves as documented.
      # 100%-stub still fails at every n>=2, so anti-laundering is intact.
      # Gameable in principle by deleting contracts to drop under it — but
      # shrinking your governance surface to dodge a badge is self-defeating,
      # visible in the diff, and leaves the repo in the state the advisory
      # already prints.
      # See feedback/2026-08-02-sch-repos-check-compliance-maturity-*.md
      if [ "$live_total" -lt 2 ]; then
        echo "NOTE: only $live_total live contract(s) — maturity is advisory at this population"
        echo "  (a percentage over n=1 quantizes to 0% or 100%; badge not weighted)"
      elif [ $((stub_draft * 100)) -lt $((live_total * 33)) ]; then
        echo "OK: maturity supports the declared badge (Tier $declared_tier: $(tier_label "$declared_tier"))"
      elif [ $((stub_draft * 100)) -le $((live_total * 66)) ]; then
        echo "NOTE: ${pct}% of live contracts are stub-or-draft — badge reads as"
        echo "  'Tier $declared_tier: $(tier_label "$declared_tier") — IN PROGRESS' until the set matures"
      else
        demoted_tier=$((declared_tier - 1))
        echo "FAIL: ${pct}% of live contracts are stub-or-draft (>66%) — badge demoted one tier"
        if [ "$demoted_tier" -ge 1 ]; then
          echo "  Declared: Tier $declared_tier: $(tier_label "$declared_tier") — Effective: Tier $demoted_tier: $(tier_label "$demoted_tier")"
        else
          echo "  Declared: Tier $declared_tier: $(tier_label "$declared_tier") — Effective: below Tier 1 (adoption not yet real)"
        fi
        echo "  Reason: a tier claimed on top of a mostly stub/draft contract set overstates adoption."
        echo "  Fix: mature the contracts (or lower the badge) until <=66% are stub-or-draft."
        errors=$((errors + 1))
      fi
    else
      echo "NOTE: no valid tier declared — maturity computed but badge weighting skipped"
    fi
  fi
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
