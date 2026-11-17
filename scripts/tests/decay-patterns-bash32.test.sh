#!/usr/bin/env bash
# Regression test: check-decay-patterns.sh survives a scan set of a few
# hundred files under bash 3.2 (macOS /bin/bash). scan() fed each file's grep
# into its read loop with `done < <(grep …)`; bash 3.2 keeps every
# process-substitution fd open until the scan() call returns, and its
# add_fifo_list() writes one byte past its 256-byte table at fd 256 — heap
# corruption ("pointer being freed was not allocated" SIGABRT, bus error, or a
# forked child spinning forever) from ~253 scanned files on.
#
#   T1  300 _test.go files + seeded findings, --all: every available shell (the
#       current bash, plus /bin/bash when it is 3.2) finishes inside a timeout,
#       exits 1 and reports exactly the seeded findings. Whether the overflow
#       actually crashes depends on heap layout (environment size, …), so:
#   T2  150 of the files under `ulimit -n 128` (--paths): a leak of one fd per
#       scanned file runs out of descriptors deterministically, on any heap
#       layout
#   T3  all reports are byte-identical across shells and fd limits
#
# Usage: bash scripts/tests/decay-patterns-bash32.test.sh
# Override the scripts under test with REBAR_SCRIPTS_DIR.
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
rebar_tmpdir decay-bash32
P="$TMP/project"
N=300
TIMEOUT=120

# run_timeout SECS CMD [ARGS...] — a hang fails (exit 124/142) instead of
# blocking. macOS has no timeout(1); perl's alarm survives the exec.
run_timeout() {
  local secs="$1"
  shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$secs" "$@"
  else
    perl -e 'alarm shift; exec @ARGV or die "exec: $!"' "$secs" "$@"
  fi
}

mkdir -p "$P/e2e" "$P/.github/workflows"
i=1
while [ $i -le $N ]; do
  printf 'package x\n\nfunc TestF%d() {}\n' "$i" > "$P/f${i}_test.go"
  i=$((i + 1))
done
# Seeded findings, built from parts so rebar's own decay scan doesn't flag them.
fail_call="test.fail"
printf '%s()\n// keep this in sync\n' "$fail_call" > "$P/e2e/a.spec.ts"
printf 'env:\n  DISABLE_%s: 1\n' AUTH > "$P/.github/workflows/ci.yml"
( cd "$P" && g init -q && g add -A && g commit -qm fixture ) || fail "fixture commit"

shells="bash"
if [ -x /bin/bash ] && [ "$(/bin/bash -c 'echo "${BASH_VERSINFO[0]}"')" = 3 ]; then
  shells="$shells /bin/bash"
else
  echo "note: /bin/bash is not bash 3.2 — testing the current bash only"
fi

# scan_with SHELL ULIMIT_N ARGS... — run the check under SHELL, with the fd
# limit lowered to ULIMIT_N unless it is empty; sets $out and $rc.
scan_with() {
  local sh="$1" fds="$2"
  shift 2
  out="$(cd "$P" && { [ -z "$fds" ] || ulimit -n "$fds"; } &&
    REBAR_PROJECT_ROOT="$P" run_timeout "$TIMEOUT" "$sh" \
      "$REBAR_SCRIPTS/check-decay-patterns.sh" "$@" 2>&1)"
  rc=$?
}

some_files=".github/workflows/ci.yml e2e/a.spec.ts"
i=1
while [ $i -le 150 ]; do some_files="$some_files f${i}_test.go"; i=$((i + 1)); done

# check_report LABEL — the run exited 1 with exactly the three seeded findings
# and nothing else (no "cannot make pipe" noise); report matches the first run.
first=""
check_report() {
  [ $rc -eq 1 ] || { echo "$out" | tail -20; fail "$1: exit $rc, want 1 (134 = abort, 124/142 = timeout)"; }
  contains "$out" -E 'process substitution|open files|ambiguous redirect' && { head -20 <<<"$out"; fail "$1: ran out of file descriptors"; }
  contains "$out" '^check-decay-patterns: 3 finding(s) in scan set$' || { head -20 <<<"$out"; fail "$1: finding count"; }
  contains "$out" '^e2e/a.spec.ts:1:P1-silenced-failure:' || { echo "$out"; fail "$1: P1"; }
  contains "$out" '^e2e/a.spec.ts:2:P4-keep-in-sync:' || { echo "$out"; fail "$1: P4"; }
  contains "$out" '^.github/workflows/ci.yml:2:P7-plausible-env-name:' || { echo "$out"; fail "$1: P7"; }
  if [ -z "$first" ]; then
    first="$out"
  elif [ "$out" != "$first" ]; then
    diff <(printf '%s\n' "$first") <(printf '%s\n' "$out") | head -20
    fail "T3: $1 report differs from the first run"
  fi
}

for sh in $shells; do
  ver="$("$sh" -c 'echo $BASH_VERSION')"
  scan_with "$sh" "" --all
  check_report "T1 ($sh $ver)"
  ok "T1: $sh ($ver) scanned $N+ files"
  # shellcheck disable=SC2086 # word-split the file list
  scan_with "$sh" 128 --paths $some_files
  check_report "T2 ($sh $ver, ulimit -n 128)"
  ok "T2: $sh ($ver) under ulimit -n 128"
done
ok "T3: reports identical across: $shells"

echo "PASS: check-decay-patterns under bash 3.2"
