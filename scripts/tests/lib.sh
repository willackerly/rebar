# shellcheck shell=bash
# lib.sh — shared helpers for rebar's shell regression tests (scripts/tests/*.test.sh).
#
# Source it first:  . "$(dirname "$0")/lib.sh"
#
#   REBAR_TESTS_DIR       this directory
#   REBAR_SCRIPTS         the scripts under test (override with REBAR_SCRIPTS_DIR)
#   fail MSG...           print "FAIL: MSG" to stderr and exit 1
#   ok MSG...             print "ok MSG"
#   g ARGS...             git with a throwaway identity, no signing, no hooks
#   rebar_tmpdir [NAME]   create a temp dir in TMP; it is removed on exit
#   rebar_on_exit CMD     eval CMD on exit, before TMP is removed (e.g. kill a watcher)
#   run_in DIR SCRIPT [ARGS...]
#                         run SCRIPT from REBAR_SCRIPTS against project DIR, the
#                         way `rebar check` does (REBAR_PROJECT_ROOT); stderr is merged
#
# Bash 3.2 compatible.

REBAR_TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Tests set tiers and checks in their fixtures' .rebarrc; don't let a caller's
# settings leak in (ci-check.sh exports _REBAR_CHECKS_VALIDATED, which would
# silence the checks_enable warnings under test).
unset REBAR_TIER REBAR_CHECKS_ENABLE _REBAR_CHECKS_VALIDATED
REBAR_SCRIPTS="${REBAR_SCRIPTS_DIR:-$REBAR_TESTS_DIR/..}"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok $*"; }
g() { git -c user.name=t -c user.email=t@example.com -c commit.gpgsign=false -c core.hooksPath=/dev/null "$@"; }

_REBAR_EXIT_CMDS=""
rebar_on_exit() { _REBAR_EXIT_CMDS="$_REBAR_EXIT_CMDS$1"$'\n'; }
_rebar_exit() {
  local cmd
  while IFS= read -r cmd; do
    [ -n "$cmd" ] && eval "$cmd" >/dev/null 2>&1
  done <<<"$_REBAR_EXIT_CMDS"
  if [ -n "${TMP:-}" ] && [ -d "$TMP" ]; then rm -rf "$TMP"; fi
}
trap _rebar_exit EXIT

rebar_tmpdir() { TMP="$(mktemp -d "${TMPDIR:-/tmp}/rebar-${1:-test}.XXXXXX")"; }

run_in() {
  local dir="$1" script="$2"
  shift 2
  ( cd "$dir" && REBAR_PROJECT_ROOT="$dir" "$REBAR_SCRIPTS/$script" "$@" ) 2>&1
}
