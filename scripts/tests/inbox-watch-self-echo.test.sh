#!/usr/bin/env bash
# Regression test for scripts/inbox-watch.sh — own-sender suppression.
#
# BUG (rebar seat, 2026-08-02, first live session using the inbox protocol
# end to end): the protocol says to bank the OUTBOUND copy of every memo in
# your own inbox/ — both halves of a conversation in one committed directory,
# because half a conversation cannot carry provenance. Principle 5 says to
# watch your own inbox/. Following both, every memo you SEND arrives as a
# notification that you RECEIVED something.
#
# Observed: four outbound ANSWER/FINDING memos deposited, four
# NEW INBOX DEPOSIT lines, zero inbound mail. Signal-to-noise is worst in the
# common case — a seat answering a round of peer mail generates one false
# event per reply, and replies usually outnumber the memos prompting them.
# That trains seats to skim NEW-deposit events, the same failure mode that
# made the July re-emit bug urgent (a noisy watcher is worse than none).
#
# FIX: suppress deposits whose <sender> segment matches --self (defaulting to
# the repo directory name). The naming grammar makes this mechanical —
# YYYY-MM-DD-<sender>-<TYPE>-<description>.md, sender normatively second.
#
# INVARIANTS GUARDED
#   1. Own-sender deposits emit NO notification.
#   2. Peer deposits still emit — suppression must not swallow real mail.
#   3. Suppressed files STILL enter the seen-ledger. Otherwise changing
#      --self later would replay every previously-suppressed memo as a
#      backlog burst — reintroducing BUG 1 through a side door.
#   4. --self '' disables suppression entirely (escape hatch).
#
# Override the binary under test with INBOX_WATCH_BIN.

set -uo pipefail

. "$(dirname "$0")/lib.sh"
BIN="${INBOX_WATCH_BIN:-$REBAR_SCRIPTS/inbox-watch.sh}"

rebar_tmpdir inbox-watch-self-echo
rebar_on_exit '[ -n "${WATCH_PID:-}" ] && kill "$WATCH_PID"'

INBOX="$TMP/inbox"
OUT="$TMP/out.log"
mkdir -p "$INBOX"

# Pre-existing file: never reported, establishes the arming baseline.
touch "$INBOX/2026-08-01-peer-FYI-pre-existing-baseline.md"

# Arm with an explicit --self so the test does not depend on the repo's
# directory name (a checkout can be renamed).
( native_env && exec "$BIN" --interval 1 --self myrepo "$INBOX" ) > "$OUT" 2>"$TMP/err.log" &
WATCH_PID=$!
sleep 2

if ! kill -0 "$WATCH_PID" 2>/dev/null; then
  echo "FAIL: watcher exited during arming"; sed -n '1,20p' "$TMP/err.log"; exit 1
fi

# Own outbound mail (suppressed) interleaved with peer mail (reported).
touch "$INBOX/2026-08-02-myrepo-ANSWER-outbound-one.md"
touch "$INBOX/2026-08-02-peer-PROPOSAL-genuine-inbound.md"
touch "$INBOX/2026-08-02-myrepo-FINDING-outbound-two.md"
sleep 3

kill "$WATCH_PID" 2>/dev/null; wait "$WATCH_PID" 2>/dev/null
WATCH_PID=""

emitted="$(grep -c 'NEW INBOX DEPOSIT' "$OUT" 2>/dev/null | tr -d '[:space:]')"
[ -n "$emitted" ] || emitted=0

fail=0

# INVARIANT 1 — own-sender deposits suppressed.
if grep -q 'myrepo-ANSWER-outbound-one\|myrepo-FINDING-outbound-two' "$OUT" 2>/dev/null; then
  echo "FAIL (invariant 1): own-sender deposit was reported as inbound"
  grep 'NEW INBOX DEPOSIT' "$OUT"
  fail=1
fi

# INVARIANT 2 — peer deposits still reported.
if ! grep -q 'peer-PROPOSAL-genuine-inbound' "$OUT" 2>/dev/null; then
  echo "FAIL (invariant 2): genuine peer deposit was NOT reported — suppression is swallowing real mail"
  fail=1
fi

# Exactly one event: the peer memo, nothing else.
if [ "$emitted" -ne 1 ]; then
  echo "FAIL: expected exactly 1 deposit line, got $emitted"
  grep 'NEW INBOX DEPOSIT' "$OUT"
  fail=1
fi

# INVARIANT 4 — escape hatch. Re-arm over the SAME inbox with suppression
# off. Every file now pre-exists, so a correct watcher reports nothing; if
# --self '' were ignored we would still be suppressing, which this cannot
# distinguish — so also assert the arm line states suppression is off.
OUT2="$TMP/out2.log"
( native_env && exec "$BIN" --interval 1 --self '' "$INBOX" ) > "$OUT2" 2>"$TMP/err2.log" &
WATCH_PID=$!
sleep 2
touch "$INBOX/2026-08-02-myrepo-ACK-should-appear-when-disabled.md"
sleep 3
kill "$WATCH_PID" 2>/dev/null; wait "$WATCH_PID" 2>/dev/null
WATCH_PID=""

if ! grep -q 'own-sender suppression OFF' "$TMP/err2.log" 2>/dev/null; then
  echo "FAIL (invariant 4): --self '' did not report suppression as OFF at arm time"
  sed -n '1,5p' "$TMP/err2.log"
  fail=1
fi
if ! grep -q 'myrepo-ACK-should-appear-when-disabled' "$OUT2" 2>/dev/null; then
  echo "FAIL (invariant 4): with suppression off, an own-sender deposit was still swallowed"
  fail=1
fi

# INVARIANT 3 — suppressed files entered the ledger. Re-arm with suppression
# off over an UNCHANGED inbox: the two previously-suppressed memos must NOT
# replay. This is the side-door back into the full-backlog re-emit bug.
OUT3="$TMP/out3.log"
( native_env && exec "$BIN" --interval 1 --self '' "$INBOX" ) > "$OUT3" 2>/dev/null &
WATCH_PID=$!
sleep 3
kill "$WATCH_PID" 2>/dev/null; wait "$WATCH_PID" 2>/dev/null
WATCH_PID=""

replayed="$(grep -c 'NEW INBOX DEPOSIT' "$OUT3" 2>/dev/null | tr -d '[:space:]')"
[ -n "$replayed" ] || replayed=0
if [ "$replayed" -ne 0 ]; then
  echo "FAIL (invariant 3): $replayed previously-suppressed memo(s) replayed as new deposits"
  grep 'NEW INBOX DEPOSIT' "$OUT3"
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "PASS: own-sender deposits suppressed, peer mail still delivered, ledger intact, --self '' disables."
exit 0
