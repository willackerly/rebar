#!/usr/bin/env bash
# check-bypass-flags.sh — Gate I enforcement: bypass flags require ticket refs.
#
# Source: feedback/processed/2026-04-24-process-gates-G-through-L.md §Gate I.
#
# Rule: any commit whose body mentions a test-bypass flag (--skip-stress,
# --skip-tests, --no-verify, --force, SKIP_TESTS=1, etc.) MUST contain a
# `Bypass tickets:` line listing the broken-test IDs being deferred. Without
# this, the bypass is a quiet escape hatch that ships regressions in
# regression-fix PRs.
#
# The rule covers the COMMIT MESSAGE — it's the audit trail for why a bypass
# was chosen. The actual flag invocation (e.g., wrapping promote-to-prod) is
# project-specific and lives in your project's deploy scripts.
#
# Modes:
#   ./scripts/check-bypass-flags.sh                       # check HEAD's commit msg
#   ./scripts/check-bypass-flags.sh <commit-msg-file>     # check given file (commit-msg hook usage)
#   ./scripts/check-bypass-flags.sh --range <sha>..<sha>  # audit a range of commits
#   ./scripts/check-bypass-flags.sh --warn                # flag but don't fail
#
# Exit code: 0 if all qualifying commits are clean, 1 if any violations.
#
# Bash 3.2 compatible.

set -uo pipefail

WARN_ONLY=0
MODE="head"
ARG=""

while [ $# -gt 0 ]; do
  case "$1" in
    --warn) WARN_ONLY=1; shift ;;
    --range) MODE="range"; ARG="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    -*)
      echo "check-bypass-flags: unknown flag '$1'" >&2; exit 2 ;;
    *)
      MODE="file"; ARG="$1"; shift ;;
  esac
done

# Pattern set covering the most common bypass flags. If your project has
# others (e.g., domain-specific --skip-X), extend BYPASS_REGEX in your local
# copy.
#
# Note: we look in the FULL commit body, not the diff. Authors who use a
# bypass flag should mention WHY in the commit body — that's what creates the
# audit trail. If they used the flag silently, that's a separate problem
# (project-specific shell-history tooling).
BYPASS_REGEX='(--skip-(stress|tests|all|e2e|smoke|ci|build|fix|preflight)|--no-verify|--no-gpg-sign|--force[[:space:]]|--force$|SKIP_TESTS=|SKIP_E2E=|SKIP_STRESS=|BYPASS_=)'

# A commit satisfies Gate I if every bypass mention is paired with a
# `Bypass tickets:` line.
satisfies() {
  local body="$1"
  echo "$body" | grep -qE '^Bypass[[:space:]]tickets:[[:space:]]*\S'
}

mentions_bypass() {
  local body="$1"
  echo "$body" | grep -qE "$BYPASS_REGEX"
}

check_one() {
  local subject="$1" body="$2" label="$3"
  if ! mentions_bypass "$body"; then
    return 0  # no bypass mentioned
  fi
  # Documentation/meta commits frequently mention bypass flags as content
  # (e.g., "the script catches --skip-stress / --no-verify"). Honor an
  # explicit `Bypass-flags-meta: <reason>` opt-out so authors can mark a
  # commit as referring to bypass flags rather than invoking them.
  if echo "$body" | grep -qE '^Bypass-flags-meta:[[:space:]]*\S'; then
    return 0
  fi
  if satisfies "$body"; then
    return 0  # gate satisfied
  fi
  echo ""
  echo "check-bypass-flags: VIOLATION ($label)"
  echo "  subject:  $subject"
  echo ""
  echo "  Body mentions a test-bypass flag (matched: $(echo "$body" | grep -oE "$BYPASS_REGEX" | head -3 | tr '\n' ' '))"
  echo "  Gate I requires a line of the form:"
  echo "    Bypass tickets: TICKET-1, TICKET-2 (one ticket per broken test)"
  echo ""
  echo "  And a justification per ticket. Without this, bypassing a test"
  echo "  ships an unowned regression. Source: practices/regression-fix-protocol.md §Gate I"
  return 1
}

violations=0

case "$MODE" in
  head)
    subject="$(git log -1 --format='%s' HEAD 2>/dev/null || true)"
    body="$(git log -1 --format='%B' HEAD 2>/dev/null || true)"
    if [ -z "$subject" ]; then
      echo "check-bypass-flags: no commit found at HEAD"
      exit 0
    fi
    check_one "$subject" "$body" "HEAD" || violations=$((violations + 1))
    ;;

  file)
    if [ ! -f "$ARG" ]; then
      echo "check-bypass-flags: file not found: $ARG" >&2
      exit 2
    fi
    subject="$(grep -vE '^#|^$' "$ARG" | head -1)"
    body="$(cat "$ARG")"
    check_one "$subject" "$body" "$ARG" || violations=$((violations + 1))
    ;;

  range)
    while IFS= read -r sha; do
      [ -z "$sha" ] && continue
      subject="$(git log -1 --format='%s' "$sha")"
      body="$(git log -1 --format='%B' "$sha")"
      check_one "$subject" "$body" "$sha" || violations=$((violations + 1))
    done < <(git rev-list "$ARG" 2>/dev/null)
    ;;
esac

if [ "$violations" -eq 0 ]; then
  echo "check-bypass-flags: OK — Gate I satisfied (no bypass mentions without Bypass tickets:)"
  exit 0
fi

echo ""
echo "check-bypass-flags: $violations violation(s)"
echo "Wire as commit-msg hook:  ln -sf ../../scripts/check-bypass-flags.sh .git/hooks/commit-msg"

if [ "$WARN_ONLY" -eq 1 ]; then
  exit 0
fi
exit 1
