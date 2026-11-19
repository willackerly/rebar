#!/usr/bin/env bash
# run.sh — run rebar's shell regression tests (scripts/tests/*.test.sh) in parallel.
#
# Usage: scripts/tests/run.sh [PATTERN]   # e.g. `inbox` runs inbox-watch*.test.sh
#
# The tests run the rebar binary: REBAR_TEST_BIN when set, else one built
# here from cli/ (go on PATH), else bin/rebar.
#
# Only meaningful in the rebar source repo: against an adopting project
# (`rebar check` sets REBAR_PROJECT_ROOT) it skips, like sync-bootstrap.sh.
# Exit: 0 = all passed, 1 = any failed. Bash 3.2 compatible.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="${REBAR_PROJECT_ROOT:-$(cd "$HERE/../.." && pwd)}"
if [ ! -d "$ROOT/templates/project-bootstrap" ] || [ ! -d "$ROOT/scripts/tests" ]; then
  echo "SKIP: script tests run only in the rebar source repo"
  exit 0
fi

LOGS="$(mktemp -d "${TMPDIR:-/tmp}/rebar-tests.XXXXXX")"
trap 'rm -rf "$LOGS"' EXIT

if [ -z "${REBAR_TEST_BIN:-}" ]; then
  if command -v go >/dev/null 2>&1; then
    (cd "$ROOT/cli" && go build -o "$LOGS/rebar" .) || { echo "FAIL  building rebar from cli/"; exit 1; }
    REBAR_TEST_BIN="$LOGS/rebar"
  else
    REBAR_TEST_BIN="$ROOT/bin/rebar"
  fi
fi
export REBAR_TEST_BIN

names=() pids=()
for t in "$HERE"/*"${1:-}"*.test.sh; do
  [ -f "$t" ] || continue
  name="$(basename "$t" .test.sh)"
  bash "$t" > "$LOGS/$name.log" 2>&1 &
  names+=("$name"); pids+=("$!")
done
[ ${#names[@]} -gt 0 ] || { echo "No tests match '${1:-}'"; exit 1; }

failed=0
i=0
while [ $i -lt ${#names[@]} ]; do
  if wait "${pids[$i]}"; then
    echo "PASS  ${names[$i]}"
  else
    echo "FAIL  ${names[$i]}"
    if [ -n "${GITHUB_ACTIONS:-}" ]; then
      # CI logs are the only record: show all of it, folded.
      echo "::group::${names[$i]} log"
      cat "$LOGS/${names[$i]}.log"
      echo "::endgroup::"
    else
      sed 's/^/      /' "$LOGS/${names[$i]}.log" | tail -15
    fi
    failed=$((failed + 1))
  fi
  i=$((i + 1))
done

echo ""
echo "${#names[@]} test file(s), $failed failed"
[ $failed -eq 0 ]
