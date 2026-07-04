#!/usr/bin/env bash
# cold-start-checks.sh — SessionStart hook: cold-start health in one block
# rebar-scripts: 2026.07.04
#
# Usage: ./scripts/cold-start-checks.sh
#
# Runs the enforcement quad (check-contract-refs, check-todos,
# check-freshness, check-ground-truth) plus a maturity count of declared
# Status: fields across architecture/CONTRACT-*.md, printing one
# pass/fail line per check. All output is wrapped in
# <rebar-cold-start>...</rebar-cold-start> so agents can treat it as
# harness ground truth, not prose to interpret (REBAR-D in
# feedback/processed/2026-04-26-sessionstart-hook-cold-start-enforcement.md).
#
# Built for hook context: output stays small (~10 lines — failures are
# summarized, not dumped; rerun the named script for full detail) and
# the script ALWAYS exits 0. Drift should be visible at session start,
# never blocking (REBAR-A). Wire-up lives in .claude/settings.json.
#
# Bash 3.2 compatible (macOS default).
#
# Exit code: 0 always.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ARCH_DIR="architecture"
ESC=$(printf '\033')
START_EPOCH=$(date +%s)

echo "<rebar-cold-start>"

if ! cd "$PROJECT_ROOT" 2>/dev/null; then
  echo "cold-start-checks: cannot cd to $PROJECT_ROOT — no checks run."
  echo "</rebar-cold-start>"
  exit 0
fi

echo "rebar cold-start health — advisory (this hook always exits 0)"

fail_count=0
run_count=0

# Strip ANSI colors (check-ground-truth colors its output), then pick
# the one line that best summarizes a check's full output.
summarize() {
  local clean line
  clean=$(printf '%s\n' "$1" | sed "s/${ESC}\[[0-9;]*m//g")
  line=$(printf '%s\n' "$clean" | grep 'Checked [0-9]' | tail -1)
  [ -z "$line" ] && line=$(printf '%s\n' "$clean" | grep -E '^(FAIL|OK|SKIP|DRIFT):' | head -1)
  [ -z "$line" ] && line=$(printf '%s\n' "$clean" | grep -iE 'drift|match|No metrics' | tail -1)
  [ -z "$line" ] && line=$(printf '%s\n' "$clean" | grep -v '^[[:space:]]*$' | head -1)
  printf '%s\n' "$line"
}

run_check() {
  local name="$1" script="scripts/${1}.sh"
  local output status summary label
  if [ ! -f "$script" ]; then
    echo "[MISS] $name — $script not installed"
    return
  fi
  run_count=$((run_count + 1))
  output=$(bash "$script" 2>&1)
  status=$?
  summary=$(summarize "$output")
  if [ "$status" -eq 0 ]; then
    case "$summary" in
      SKIP:*) label="SKIP" ;;
      *)      label="PASS" ;;
    esac
  else
    label="FAIL"
    fail_count=$((fail_count + 1))
  fi
  echo "[$label] $name — ${summary:-(no output, exit $status)}"
}

run_check "check-contract-refs"
run_check "check-todos"
run_check "check-freshness"
run_check "check-ground-truth"

# Maturity count: declared Status: fields in contract header blocks
# (Cluster 1 vocabulary: stub/draft/in-progress/active/verified).
# Tolerates both '**Status:** active' and 'Status: active'. Templates
# and the registry are skeletons, not declarations — skipped. The glob
# does not recurse, so the conventions.md exclusion set
# (.claude/worktrees, node_modules, vendor, .git) cannot leak in.
n_stub=0; n_draft=0; n_inprog=0; n_active=0; n_verified=0; n_superseded=0; n_other=0
total_contracts=0
with_status=0
for f in "$ARCH_DIR"/CONTRACT-*.md; do
  [ -e "$f" ] || continue
  case "$f" in
    *TEMPLATE*|*REGISTRY*|*GAPS*|*.impl.md) continue ;;
  esac
  total_contracts=$((total_contracts + 1))
  status_val=$(grep -m1 -E '^\*{0,2}Status:' "$f" 2>/dev/null \
    | sed -e 's/^\*\*Status:\*\*//' -e 's/^Status://' \
    | awk '{print $1}' | tr '[:upper:]' '[:lower:]')
  [ -z "$status_val" ] && continue
  with_status=$((with_status + 1))
  case "$status_val" in
    stub)        n_stub=$((n_stub + 1)) ;;
    draft)       n_draft=$((n_draft + 1)) ;;
    in-progress) n_inprog=$((n_inprog + 1)) ;;
    active)      n_active=$((n_active + 1)) ;;
    verified)    n_verified=$((n_verified + 1)) ;;
    superseded)  n_superseded=$((n_superseded + 1)) ;;
    *)           n_other=$((n_other + 1)) ;;
  esac
done

if [ "$total_contracts" -eq 0 ]; then
  # A brand-new project isn't "pre-v3" — it just has no contracts yet.
  echo "maturity: no contracts yet"
elif [ "$with_status" -eq 0 ]; then
  # Pre-v3 repos have contracts without Status: fields.
  echo "maturity: pre-v3 (no Status fields)"
else
  parts=""
  add_part() {
    [ "$1" -eq 0 ] && return 0
    if [ -n "$parts" ]; then parts="$parts, $1 $2"; else parts="$1 $2"; fi
  }
  add_part "$n_stub" "stub"
  add_part "$n_draft" "draft"
  add_part "$n_inprog" "in-progress"
  add_part "$n_active" "active"
  add_part "$n_verified" "verified"
  add_part "$n_superseded" "superseded"
  add_part "$n_other" "unrecognized"
  echo "maturity: $parts ($with_status of $total_contracts contracts declare Status)"
fi

# Reflexive-push durability (rebar:feedback/2026-07-02-reflexive-push-
# durability-rule): an origin behind the laptop is a status surface lying
# to every consumer that isn't this machine. Silent when no upstream is
# configured (fresh projects) — nagging there would be noise, not signal.
if git -C "$PROJECT_ROOT" rev-parse --abbrev-ref '@{u}' >/dev/null 2>&1; then
  unpushed=$(git -C "$PROJECT_ROOT" rev-list --count '@{u}..HEAD' 2>/dev/null || echo 0)
  cur_branch=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "?")
  if [ "$unpushed" -gt 5 ]; then
    echo "[WARN] $unpushed unpushed commit(s) on $cur_branch — push now; durability lives at origin, not on this disk"
  elif [ "$unpushed" -gt 0 ]; then
    echo "unpushed: $unpushed commit(s) on $cur_branch — push when settled (reflexive-push rule)"
  fi
fi

elapsed=$(( $(date +%s) - START_EPOCH ))
if [ "$fail_count" -gt 0 ]; then
  echo "$fail_count of $run_count checks failing (${elapsed}s) — rerun the named scripts/*.sh for full output."
else
  echo "all $run_count checks passing (${elapsed}s)."
fi

echo "</rebar-cold-start>"
exit 0
