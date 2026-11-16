#!/usr/bin/env bash
# check-ground-truth.sh — Verify METRICS file matches codebase reality.
# rebar-scripts: 2026.03.20
#
# Computes project metrics from code and compares against claims in the
# METRICS file. Catches "silent success" drift where everything works but
# documented numbers describe a different reality.
#
# Metrics are declared in .rebarrc, so a project needs no copy of this
# script (see "Metrics declared in .rebarrc" below).
#
# METRICS lines are `key = value` (must match exactly) or `key >= value`
# (a floor: passes while the computed count is at least value). Use floors
# for counts that most PRs change, such as test files or CI workflows: with
# exact counts, two PRs that each add a test both edit the same line, so they
# conflict or, if both write the same number, leave main's count wrong.
#
# Usage: ./scripts/check-ground-truth.sh [--print]
#   --print  print each declared metric's computed value as key=value and
#            exit 0 (no METRICS comparison, no tier gate; used by
#            `rebar migrate`)
# Exit: 0 = all claims match, 1 = drift detected

set -euo pipefail

PRINT_ONLY=false
[ "${1:-}" = "--print" ] && PRINT_ONLY=true

# Tier gate: ground truth is Tier 3 only (skip for Tier 1-2)
SCRIPT_DIR_GT="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR_GT/_rebar-config.sh" ]; then
  source "$SCRIPT_DIR_GT/_rebar-config.sh"
  # Under `REBAR_CHECK_IMPL=go rebar check` (the default) the CLI's native port runs instead.
  _rebar_native check-ground-truth "$@"
  if ! $PRINT_ONLY && _rebar_skip 3 check-ground-truth; then
    exit 0
  fi
fi

REPO_ROOT="${REBAR_PROJECT_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
# METRICS or METRICS.md — the template ships .md and `rebar audit`
# accepts both, so this checker must too.
METRICS_FILE="$REPO_ROOT/METRICS"
[ ! -f "$METRICS_FILE" ] && [ -f "$REPO_ROOT/METRICS.md" ] && METRICS_FILE="$REPO_ROOT/METRICS.md"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
exit_code=0

# ── Metrics declared in .rebarrc ──
#
# Each metric is a group of `metric.<name>.<field> = value` lines. <name> is
# the METRICS key. All fields are optional; values are space-separated lists
# where a list makes sense, and `#` starts a comment.
#
#   type           files (default) or dirs
#   paths          where to look, relative to the repo root (default: .)
#   names          name globs to count (default: everything)
#   exclude_names  name globs to leave out
#   exclude_dirs   directory names to skip anywhere (.git is always skipped)
#   min_depth      as find -mindepth, relative to each path
#   max_depth      as find -maxdepth
#   contains       ERE; count only files whose content matches
#   distinct       ERE with one group, matched against each name; count the
#                  distinct values of the group (a name that does not match
#                  counts as itself)
#
# Example — test files under two roots, and contract IDs across versions:
#   metric.test_files.paths    = api sdk
#   metric.test_files.names    = *_test.go *.test.ts
#   metric.test_files.exclude_dirs = node_modules vendor dist
#   metric.contract_ids.paths  = architecture
#   metric.contract_ids.max_depth = 1
#   metric.contract_ids.names  = CONTRACT-*.md
#   metric.contract_ids.exclude_names = CONTRACT-TEMPLATE.md CONTRACT-REGISTRY.md
#   metric.contract_ids.distinct = ^CONTRACT-(.*)\.[0-9]+\.[0-9]+\.md$
RC_FILE="${REBAR_RCFILE:-$REPO_ROOT/.rebarrc}"

metric_names() {
  [ -f "$RC_FILE" ] || return 0
  sed -n -E 's/^metric\.([A-Za-z0-9_-]+)\.[a-z_]+[[:space:]]*=.*/\1/p' "$RC_FILE" \
    | awk '!seen[$0]++'
}

metric_field() {
  _rebar_config "metric.$1.$2" _REBAR_NO_ENV "${3:-}"
}

# count_metric <name>: print the computed value of one declared metric.
count_metric() {
  local name="$1" type paths names exclude_names exclude_dirs min max contains distinct
  type=$(metric_field "$name" type files)
  paths=$(metric_field "$name" paths .)
  names=$(metric_field "$name" names)
  exclude_names=$(metric_field "$name" exclude_names)
  exclude_dirs=$(metric_field "$name" exclude_dirs)
  min=$(metric_field "$name" min_depth)
  max=$(metric_field "$name" max_depth)
  contains=$(metric_field "$name" contains)
  distinct=$(metric_field "$name" distinct)

  # Globs in the values are patterns for find, not for this shell.
  set -f
  local args=() p first
  for p in $paths; do
    [ -e "$REPO_ROOT/$p" ] && args+=("$REPO_ROOT/$p")
  done
  if [ ${#args[@]} -eq 0 ]; then
    set +f
    echo 0
    return
  fi
  [ -n "$min" ] && args+=(-mindepth "$min")
  [ -n "$max" ] && args+=(-maxdepth "$max")
  args+=( '(' -type d '(' -name .git )
  for p in $exclude_dirs; do args+=(-o -name "$p"); done
  args+=( ')' ')' -prune -o )
  case "$type" in
    dirs) args+=( '(' -type d ) ;;
    *)    args+=( '(' -type f ) ;;
  esac
  if [ -n "$names" ]; then
    args+=( '(' )
    first=true
    for p in $names; do
      $first || args+=(-o)
      args+=(-name "$p")
      first=false
    done
    args+=( ')' )
  fi
  for p in $exclude_names; do args+=( ! -name "$p" ); done
  args+=( ')' )
  if [ -n "$contains" ]; then
    args+=(-exec grep -lE -- "$contains" {} +)
  else
    args+=(-print)
  fi
  set +f

  if [ -n "$distinct" ]; then
    { find "${args[@]}" 2>/dev/null || true; } \
      | sed 's|.*/||' | sed -E "s#${distinct}#\\1#" | LC_ALL=C sort -u | wc -l | tr -d ' '
  else
    { find "${args[@]}" 2>/dev/null || true; } | wc -l | tr -d ' '
  fi
}

compute_metrics() {
  type _rebar_config >/dev/null 2>&1 || return 0
  local name
  for name in $(metric_names); do
    echo "$name=$(count_metric "$name")"
  done
}

if $PRINT_ONLY; then
  compute_metrics
  exit 0
fi

# ── Verification engine (do not modify below this line) ──

verify() {
  if [ ! -f "$METRICS_FILE" ]; then
    echo -e "${YELLOW}SKIP${NC}: No METRICS file found"
    echo "  Create a METRICS file with key = value pairs and declare how"
    echo "  to compute each one as metric.<key>.* lines in .rebarrc."
    return 0
  fi

  local computed
  computed=$(compute_metrics)

  if [ -z "$computed" ]; then
    echo "No metrics declared. Add metric.<key>.* lines to .rebarrc (see this script's header)."
    return 0
  fi

  while IFS='=' read -r key value; do
    # Skip empty lines and comments
    [ -z "$key" ] && continue
    echo "$key" | grep -q '^[[:space:]]*#' && continue

    key=$(echo "$key" | xargs)
    value=$(echo "$value" | xargs)

    # Look up key in METRICS file: `key = N` (exact) or `key >= N` (floor)
    local line documented
    line=$(grep -E "^[[:space:]]*${key}[[:space:]]*>?=" "$METRICS_FILE" 2>/dev/null | head -1) || true
    documented=$(echo "$line" | sed -E 's/^[^=]*=//' | xargs)

    if [ -z "$documented" ]; then
      echo -e "${YELLOW}NEW${NC}:   $key = $value  (not in METRICS file)"
    elif echo "$line" | grep -Eq "^[[:space:]]*${key}[[:space:]]*>="; then
      if [[ "$value" =~ ^[0-9]+$ && "$documented" =~ ^[0-9]+$ ]] && [ "$value" -ge "$documented" ]; then
        echo -e "${GREEN}OK${NC}:    $key = $value  (floor $documented)"
      else
        echo -e "${RED}DRIFT${NC}: $key — METRICS floor is $documented, code says $value"
        exit_code=1
      fi
    elif [ "$value" = "$documented" ]; then
      echo -e "${GREEN}OK${NC}:    $key = $value"
    else
      echo -e "${RED}DRIFT${NC}: $key — METRICS says $documented, code says $value"
      exit_code=1
    fi
  done <<< "$computed"
}

echo "=== Ground Truth Verification ==="
echo ""
verify
echo ""

if [ $exit_code -eq 0 ]; then
  echo -e "${GREEN}All documented metrics match codebase reality${NC}"
else
  echo -e "${RED}Metric drift detected — update METRICS to match reality${NC}"
fi

exit $exit_code
