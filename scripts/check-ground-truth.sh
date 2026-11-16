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
# Strictness: by default a missing METRICS file is a SKIP and a computed key
# missing from METRICS is reported as NEW; both pass, as does a .rebarrc that
# declares no metrics. With .rebarrc `metrics_strict = true` (env
# REBAR_METRICS_STRICT) all three fail.
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
  if ! $PRINT_ONLY && _rebar_skip 3 check-ground-truth; then
    exit 0
  fi
fi

REPO_ROOT="${REBAR_PROJECT_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
# METRICS or METRICS.md — the template ships .md and `rebar audit`
# accepts both, so this checker must too.
METRICS_FILE="$REPO_ROOT/METRICS"
[ ! -f "$METRICS_FILE" ] && [ -f "$REPO_ROOT/METRICS.md" ] && METRICS_FILE="$REPO_ROOT/METRICS.md"

STRICT=false
if type _rebar_config >/dev/null 2>&1; then
  case "$(_rebar_config metrics_strict REBAR_METRICS_STRICT false)" in
    true|1|yes|on) STRICT=true ;;
  esac
fi

# Colour only on a terminal, and never with NO_COLOR set (no-color.org).
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  NC='\033[0m'
else
  RED='' GREEN='' YELLOW='' NC=''
fi
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
#   tracked        true: count only what git tracks (a dir counts when it
#                  holds a tracked file), so gitignored or untracked local
#                  files don't inflate the count. Outside a git work tree it
#                  has no effect (a note goes to stderr).
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
  local name="$1" type paths names exclude_names exclude_dirs min max contains distinct tracked
  type=$(metric_field "$name" type files)
  paths=$(metric_field "$name" paths .)
  names=$(metric_field "$name" names)
  exclude_names=$(metric_field "$name" exclude_names)
  exclude_dirs=$(metric_field "$name" exclude_dirs)
  min=$(metric_field "$name" min_depth)
  max=$(metric_field "$name" max_depth)
  contains=$(metric_field "$name" contains)
  distinct=$(metric_field "$name" distinct)
  tracked=$(metric_field "$name" tracked false)
  case "$tracked" in
    true|1|yes|on)
      if git -C "$REPO_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        tracked=true
      else
        echo "rebar: metric.$name.tracked: $REPO_ROOT is not a git work tree; counting every file" >&2
        tracked=false
      fi
      ;;
    *) tracked=false ;;
  esac

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
    { find "${args[@]}" 2>/dev/null || true; } | only_tracked "$tracked" \
      | sed 's|.*/||' | sed -E "s#${distinct}#\\1#" | LC_ALL=C sort -u | wc -l | tr -d ' '
  else
    { find "${args[@]}" 2>/dev/null || true; } | only_tracked "$tracked" | wc -l | tr -d ' '
  fi
}

# only_tracked true|false: filter find's paths (under $REPO_ROOT) to those
# git tracks, and directories holding a tracked file; false passes all.
only_tracked() {
  if [ "$1" != true ]; then
    cat
    return
  fi
  local list
  list=$(mktemp "${TMPDIR:-/tmp}/rebar-tracked.XXXXXX")
  git -C "$REPO_ROOT" -c core.quotepath=off ls-files > "$list" 2>/dev/null || true
  awk -v root="$REPO_ROOT/" -v list="$list" '
    FILENAME == list {
      t[$0] = 1
      p = $0
      while (sub(/\/[^\/]*$/, "", p)) t[p] = 1
      t["."] = 1
      next
    }
    {
      rel = $0
      if (index(rel, root) == 1) rel = substr(rel, length(root) + 1)
      else if (rel == substr(root, 1, length(root) - 1)) rel = "."
      gsub(/\/\/+/, "/", rel)
      gsub(/\/\.\//, "/", rel)
      sub(/^\.\//, "", rel)
      sub(/\/$/, "", rel)
      if (rel in t) print
    }
  ' "$list" -
  rm -f "$list"
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
    if $STRICT; then
      echo -e "${RED}FAIL${NC}: No METRICS file found (metrics_strict = true)"
      exit_code=1
    else
      echo -e "${YELLOW}SKIP${NC}: No METRICS file found"
    fi
    echo "  Create a METRICS file with key = value pairs and declare how"
    echo "  to compute each one as metric.<key>.* lines in .rebarrc."
    return 0
  fi

  local computed
  computed=$(compute_metrics)

  if [ -z "$computed" ]; then
    if $STRICT; then
      echo -e "${RED}FAIL${NC}: No metrics declared (metrics_strict = true)"
      exit_code=1
    else
      echo "No metrics declared."
    fi
    echo "  Add metric.<key>.* lines to .rebarrc (see this script's header)."
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

    if [ -z "$documented" ] && $STRICT; then
      echo -e "${RED}MISSING${NC}: $key = $value  (not in METRICS file; metrics_strict = true)"
      exit_code=1
      drift=true
    elif [ -z "$documented" ]; then
      echo -e "${YELLOW}NEW${NC}:   $key = $value  (not in METRICS file)"
    elif echo "$line" | grep -Eq "^[[:space:]]*${key}[[:space:]]*>="; then
      if [[ "$value" =~ ^[0-9]+$ && "$documented" =~ ^[0-9]+$ ]] && [ "$value" -ge "$documented" ]; then
        echo -e "${GREEN}OK${NC}:    $key = $value  (floor $documented)"
      else
        echo -e "${RED}DRIFT${NC}: $key — METRICS floor is $documented, code says $value"
        exit_code=1
        drift=true
      fi
    elif [ "$value" = "$documented" ]; then
      echo -e "${GREEN}OK${NC}:    $key = $value"
    else
      echo -e "${RED}DRIFT${NC}: $key — METRICS says $documented, code says $value"
      exit_code=1
      drift=true
    fi
  done <<< "$computed"
}

echo "=== Ground Truth Verification ==="
echo ""
drift=false
verify
echo ""

if [ $exit_code -eq 0 ]; then
  echo -e "${GREEN}All documented metrics match codebase reality${NC}"
elif $drift; then
  echo -e "${RED}Metric drift detected — update METRICS to match reality${NC}"
else
  echo -e "${RED}Ground truth not verified — metrics_strict = true needs a METRICS file and metric declarations${NC}"
fi

exit $exit_code
