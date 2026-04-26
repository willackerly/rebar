#!/usr/bin/env bash
# check-fix-commit.sh — Gate G enforcement: fix:/regression: commits must
# document where the symptom was reproduced.
#
# Source: feedback/processed/2026-04-24-process-gates-G-through-L.md §Gate G.
#
# Rule: any commit prefixed `fix:`, `fix(...)`, or `regression:` /
# `regression(...)` MUST contain a `Reproduced on:` line in the body
# referencing a SHA, deploy URL, log excerpt, or screenshot link. Without
# this, the author is fixing a phantom — the user-reported symptom hasn't
# been observed independently before fixing began.
#
# Modes:
#   ./scripts/check-fix-commit.sh                       # check HEAD's commit msg
#   ./scripts/check-fix-commit.sh <commit-msg-file>     # check given file (commit-msg hook usage)
#   ./scripts/check-fix-commit.sh --range <sha>..<sha>  # audit a range of commits
#   ./scripts/check-fix-commit.sh --warn                # flag but don't fail
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
      echo "check-fix-commit: unknown flag '$1'" >&2; exit 2 ;;
    *)
      MODE="file"; ARG="$1"; shift ;;
  esac
done

# A commit qualifies if its subject starts with `fix:` / `fix(scope):` /
# `regression:` / `regression(scope):`. Bash 3.2's regex engine struggles
# with the nested group; using grep -E for portability.
qualifies() {
  local subject="$1"
  echo "$subject" | grep -qE '^(fix|regression)(\([^)]*\))?:[[:space:]]'
}

# A commit message satisfies Gate G if its body contains a non-empty
# `Reproduced on:` line. The text after the colon is the evidence reference.
satisfies() {
  local body="$1"
  echo "$body" | grep -qE '^Reproduced[[:space:]]on:[[:space:]]*\S'
}

check_one() {
  local subject="$1" body="$2" label="$3"
  if ! qualifies "$subject"; then
    return 0  # not a fix:/regression: commit
  fi
  if satisfies "$body"; then
    return 0  # gate satisfied
  fi
  echo ""
  echo "check-fix-commit: VIOLATION ($label)"
  echo "  subject:  $subject"
  echo "  Gate G requires a line of the form:"
  echo "    Reproduced on: <SHA / deploy URL / log excerpt / screenshot path>"
  echo ""
  echo "  In the body. Without it, the fix is for a phantom — the user's"
  echo "  symptom hasn't been observed independently before fixing began."
  echo "  Source: practices/regression-fix-protocol.md §Gate G"
  return 1
}

violations=0

case "$MODE" in
  head)
    subject="$(git log -1 --format='%s' HEAD 2>/dev/null || true)"
    body="$(git log -1 --format='%B' HEAD 2>/dev/null || true)"
    if [ -z "$subject" ]; then
      echo "check-fix-commit: no commit found at HEAD"
      exit 0
    fi
    check_one "$subject" "$body" "HEAD" || violations=$((violations + 1))
    ;;

  file)
    if [ ! -f "$ARG" ]; then
      echo "check-fix-commit: file not found: $ARG" >&2
      exit 2
    fi
    # First non-comment, non-empty line is the subject.
    subject="$(grep -vE '^#|^$' "$ARG" | head -1)"
    body="$(cat "$ARG")"
    check_one "$subject" "$body" "$ARG" || violations=$((violations + 1))
    ;;

  range)
    # Iterate every commit in the range.
    while IFS= read -r sha; do
      [ -z "$sha" ] && continue
      subject="$(git log -1 --format='%s' "$sha")"
      body="$(git log -1 --format='%B' "$sha")"
      check_one "$subject" "$body" "$sha" || violations=$((violations + 1))
    done < <(git rev-list "$ARG" 2>/dev/null)
    ;;
esac

if [ "$violations" -eq 0 ]; then
  echo "check-fix-commit: OK — Gate G satisfied (no fix:/regression: commits without Reproduced on:)"
  exit 0
fi

echo ""
echo "check-fix-commit: $violations violation(s)"
echo "Wire as commit-msg hook:  ln -sf ../../scripts/check-fix-commit.sh .git/hooks/commit-msg"

if [ "$WARN_ONLY" -eq 1 ]; then
  exit 0
fi
exit 1
