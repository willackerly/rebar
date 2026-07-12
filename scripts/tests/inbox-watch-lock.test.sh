#!/usr/bin/env bash
# Regression test for the per-inbox lock in scripts/inbox-watch.sh.
#
# The lock replaced a process-global `pgrep inbox-watch` stale-watcher check
# that false-positived on every legitimate sibling seat in a single-machine,
# multi-seat setup (go-tak-server / _atlas, 2026-07-11). The lock scopes the
# warning to the ACTUAL hazard: two live watchers on the SAME inbox.
#
# Asserts four behaviors:
#   T1  arming writes a PID lock, hidden from `ls -1` (can't leak into watch)
#   T2  a second watcher on the SAME inbox WARNS and does not steal the lock
#   T3  a clean exit (SIGTERM) releases the lock
#   T4  a stale lock (dead PID) is reclaimed SILENTLY (no false warning)
#
# Override the binary under test with INBOX_WATCH_BIN.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
WATCH="${INBOX_WATCH_BIN:-$HERE/../inbox-watch.sh}"
LOCK=".inbox-watch.lock"

fail() { echo "FAIL: $*" >&2; exit 1; }
[ -f "$WATCH" ] || fail "watcher not found at $WATCH"

TMP="$(mktemp -d)"
PIDS=""
cleanup() { for p in $PIDS; do kill "$p" 2>/dev/null; done; rm -rf "$TMP"; }
trap cleanup EXIT
mkdir -p "$TMP/inbox"
INBOX="$TMP/inbox"

arm() { # arm a watcher on $INBOX, redirecting out/err to given files; echo pid
  # Absolute path, NO subshell wrapper: `$!` must be the watcher's own PID
  # (the process that writes the lock). A `( cd && bash )` wrapper would make
  # `$!` the subshell PID and the watcher's real PID would be its child.
  # (The watcher warns that $INBOX is outside $PWD — harmless here; the
  # assertions grep for "already holds", a different line.)
  local out="$1" err="$2"
  bash "$WATCH" -i 1 "$INBOX" > "$out" 2> "$err" &
  local p=$!; PIDS="$PIDS $p"; echo "$p"
}
settle() { sleep 2; }

# --- T1: lock written, holds our PID, hidden from ls -1 --------------------
A="$(arm "$TMP/a.out" "$TMP/a.err")"
settle
[ -f "$INBOX/$LOCK" ] || fail "T1: no lock file written"
held="$(head -n1 "$INBOX/$LOCK" | tr -dc '0-9')"
[ "$held" = "$A" ] || fail "T1: lock holds '$held', expected watcher PID $A"
listed="$(ls -1 "$INBOX")"
[ -z "$listed" ] || fail "T1: lock leaked into ls -1 output: [$listed]"
echo "T1 PASS: lock written (PID $A), hidden from ls -1"

# --- T2: second watcher on the same inbox warns, does not steal ------------
B="$(arm "$TMP/b.out" "$TMP/b.err")"
settle
grep -q "already holds" "$TMP/b.err" || fail "T2: second watcher did not warn (b.err: $(cat "$TMP/b.err"))"
grep -q "PID $A" "$TMP/b.err" || fail "T2: warning did not name the live holder PID $A"
held="$(head -n1 "$INBOX/$LOCK" | tr -dc '0-9')"
[ "$held" = "$A" ] || fail "T2: lock was stolen; holds '$held', expected $A"
echo "T2 PASS: second watcher warned (named PID $A) and did not steal the lock"

# --- T3: clean exit releases the lock --------------------------------------
kill "$A" 2>/dev/null; wait "$A" 2>/dev/null
sleep 1
[ ! -f "$INBOX/$LOCK" ] || fail "T3: lock not released after holder exit (holder=$(cat "$INBOX/$LOCK" 2>/dev/null))"
echo "T3 PASS: clean exit released the lock"
# B is still running and lock-less now; stop it so it doesn't muddy T4
kill "$B" 2>/dev/null; wait "$B" 2>/dev/null

# --- T4: stale lock (dead PID) reclaimed silently --------------------------
sh -c 'exit 0' & DEAD=$!; wait "$DEAD" 2>/dev/null   # DEAD is now a dead PID
printf '%s\n' "$DEAD" > "$INBOX/$LOCK"
C="$(arm "$TMP/c.out" "$TMP/c.err")"
settle
grep -q "already holds" "$TMP/c.err" && fail "T4: false warning on a stale (dead-PID $DEAD) lock (c.err: $(cat "$TMP/c.err"))"
held="$(head -n1 "$INBOX/$LOCK" | tr -dc '0-9')"
[ "$held" = "$C" ] || fail "T4: stale lock not reclaimed; holds '$held', expected $C"
echo "T4 PASS: stale (dead-PID) lock reclaimed silently"

echo "ALL PASS: per-inbox lock behaves correctly (write/hide/warn/release/reclaim)"
