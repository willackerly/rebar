#!/usr/bin/env bash
# check-todos.sh — Enforce the two-tag TODO tracking system
# rebar-scripts: 2026.03.20
#
# Usage: ./scripts/check-todos.sh [directories...]
# Default: .rebarrc `source_dirs`, else src/ internal/ cmd/ client/ packages/
# lib/ app/. File types: .rebarrc `source_extensions` (e.g. `go ts sh`), else
# go ts tsx js py rs jsx.
#
# Rules:
#   - TODO: in code = untracked = BLOCKS COMMIT
#   - TRACKED-TASK: in code = tracked in TODO.md = allowed
#
# Exit code: 0 = no untracked TODOs, 1 = untracked TODOs found

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOURCE_DIRS="src internal cmd client packages lib app"
SOURCE_EXTENSIONS="go ts tsx js py rs jsx"
EXCLUDE_DIRS="node_modules vendor dist .git .claude"
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  # shellcheck source=scripts/_rebar-config.sh
  source "$SCRIPT_DIR/_rebar-config.sh"
  # Under `REBAR_CHECK_IMPL=go rebar check` the CLI's native port runs instead.
  _rebar_native check-todos "$@"
  SOURCE_DIRS=$(_rebar_config source_dirs _REBAR_NO_ENV "$SOURCE_DIRS")
  SOURCE_EXTENSIONS=$(_rebar_config source_extensions _REBAR_NO_ENV "$SOURCE_EXTENSIONS")
  EXCLUDE_DIRS=$(_rebar_source_exclude_dirs)
fi
INCLUDES=()
for ext in $SOURCE_EXTENSIONS; do
  INCLUDES+=("--include=*.${ext#.}")
done

# Configurable: directories to scan
if [ $# -gt 0 ]; then
  DIRS=("$@")
else
  DIRS=()
  for d in $SOURCE_DIRS; do
    [ -d "$d" ] && DIRS+=("$d")
  done
fi

if [ ${#DIRS[@]} -eq 0 ]; then
  echo "No source directories found. Pass directories as arguments."
  exit 0
fi

# Count untracked TODOs (exclude TRACKED-TASK lines and comments about the system)
# Excluded directories are skipped by name, and TODO.md and this script by
# file name: filtering the match lines by substring also let a TODO in
# distance.go, or on a line mentioning those words, past the gate.
EXCLUDES=(--exclude=TODO.md --exclude=check-todos.sh)
for dir in $EXCLUDE_DIRS; do
  EXCLUDES+=("--exclude-dir=$dir")
done
untracked=$(grep -rnI "TODO:" \
  "${INCLUDES[@]}" "${EXCLUDES[@]}" \
  "${DIRS[@]}" 2>/dev/null \
  | grep -v "TRACKED-TASK:" \
  || true)

# Count tracked tasks (informational)
tracked=$(grep -rnI "TRACKED-TASK:" \
  "${INCLUDES[@]}" "${EXCLUDES[@]}" \
  "${DIRS[@]}" 2>/dev/null \
  || true)

tracked_count=$(echo "$tracked" | grep -c . 2>/dev/null || echo 0)
[ -z "$tracked" ] && tracked_count=0

if [ -n "$untracked" ]; then
  untracked_count=$(echo "$untracked" | grep -c .)
  echo "FAIL: $untracked_count untracked TODO(s) found:"
  echo ""
  echo "$untracked"
  echo ""
  echo "Before committing, you must either:"
  echo "  1. Fix the TODO immediately (remove it), OR"
  echo "  2. Add it to TODO.md and convert to: TRACKED-TASK: description"
  echo ""
  echo "Also found $tracked_count tracked tasks (OK)."
  exit 1
else
  echo "OK: 0 untracked TODOs. $tracked_count tracked tasks."
  exit 0
fi
