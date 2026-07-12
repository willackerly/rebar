#!/usr/bin/env bash
# Regression test for scripts/inbox-watch.sh — the Principle-5 watcher.
#
# Guards two field-reported bugs, both 2026-07-11, both from the
# go-tak-server stress rig:
#
#   BUG 1 — full-backlog re-emit on a git op that transiently removes and
#           restores tracked inbox files. Fixed a92382b (append-only
#           seen-ledger: a filename, once reported, stays reported).
#   BUG 2 — phantom re-emit EVERY poll when the ledger's union re-collates
#           under the ambient locale while list_dir stays C-collated, so
#           comm(1) merge-walks two differently-ordered files. Fixed by
#           pinning LC_ALL=C on the union sort AND the comm walk.
#
# BUG 2 only manifests under a locale whose collation diverges from C, with
# MIXED-CASE filenames (C byte-order puts A-Z before a-z; most UTF-8 locales
# interleave them). Lowercase-only fixtures collate identically in both and
# silently pass a broken watcher — that gap is exactly why the first
# regression attempt missed BUG 2. So this test (a) uses case- and
# digit-heavy names shaped like real memo files, and (b) runs the watcher
# under a forced divergent UTF-8 locale. If no such locale is installed we
# SKIP loudly rather than pass falsely.
#
# Override the binary under test with INBOX_WATCH_BIN (used to confirm the
# test discriminates against a pre-fix copy).
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
WATCH="${INBOX_WATCH_BIN:-$HERE/../inbox-watch.sh}"

fail() { echo "FAIL: $*" >&2; exit 1; }
[ -f "$WATCH" ] || fail "watcher not found at $WATCH"

# --- pick a UTF-8 locale whose collation diverges from C -------------------
# Snapshot `locale -a` into a variable and match with a herestring rather than
# `locale -a | grep -q`: under `set -o pipefail`, grep -q exits on first match
# and closes the pipe, `locale -a` then dies with SIGPIPE (141), and pipefail
# reports the pipeline as failed even though the match succeeded.
AVAIL="$(locale -a 2>/dev/null || true)"
DIVERGENT=""
for L in en_US.UTF-8 en_GB.UTF-8 en_US.utf8 C.UTF-8 C.utf8; do
  if grep -Fixq "$L" <<<"$AVAIL"; then DIVERGENT="$L"; break; fi
done
if [ -z "$DIVERGENT" ]; then
  echo "SKIP: no divergent UTF-8 locale installed (need one of en_US.UTF-8 / C.UTF-8 / ...);" >&2
  echo "SKIP: BUG 2 cannot be reproduced under C collation, so this test would pass falsely." >&2
  exit 0
fi

TMP="$(mktemp -d)"
W=""
cleanup() { [ -n "$W" ] && kill "$W" 2>/dev/null; rm -rf "$TMP"; }
trap cleanup EXIT
mkdir -p "$TMP/inbox"

# Mixed-case, digit-heavy fixtures — the collation hazard zone.
PRE=(
  "2026-07-11-ACK-serve-surface.md"
  "2026-07-11-RATIFIED-a11.md"
  "2026-07-11-A1-triage.md"
  "2026-07-11-Zebra-upper.md"
  "2026-07-11-alpha-lower.md"
)
for f in "${PRE[@]}"; do : > "$TMP/inbox/$f"; done

OUT="$TMP/out.log"
# Arm the watcher UNDER THE DIVERGENT LOCALE. A correct watcher pins LC_ALL=C
# internally for every sort/merge and is immune; a broken one inherits this
# locale in its union and phantom-emits.
LC_ALL="$DIVERGENT" bash "$WATCH" -i 1 "$TMP/inbox" > "$OUT" 2> "$TMP/err.log" &
W=$!

poll() { sleep 2; }   # >= one 1s poll, with margin

poll; poll                                   # baseline polls: BUG 2 emits here
: > "$TMP/inbox/2026-07-11-NEW-first.md"      # genuine deposit #1
poll
mv "$TMP"/inbox/*.md "$TMP"/                  # BUG 1 trigger: transient remove...
poll
mv "$TMP"/*.md "$TMP"/inbox/ 2>/dev/null      # ...and restore
poll
: > "$TMP/inbox/2026-07-11-NEW-second.md"     # genuine deposit #2
poll

kill "$W" 2>/dev/null; wait "$W" 2>/dev/null; W=""

EMITTED="$(grep -o 'NEW INBOX DEPOSIT: [^ ]*' "$OUT" 2>/dev/null | sed 's/.*: //' | LC_ALL=C sort)"
EXPECTED="$(printf '%s\n' "2026-07-11-NEW-first.md" "2026-07-11-NEW-second.md" | LC_ALL=C sort)"

if [ "$EMITTED" = "$EXPECTED" ]; then
  echo "PASS (locale=$DIVERGENT): exactly the 2 genuine deposits emitted; no phantom or backlog re-emit."
  exit 0
fi

echo "FAIL (locale=$DIVERGENT): emitted set != the 2 genuine deposits." >&2
echo "--- expected ---" >&2; printf '%s\n' "$EXPECTED" >&2
echo "--- got ($(printf '%s' "$EMITTED" | grep -c . || true)) ---" >&2; printf '%s\n' "$EMITTED" >&2
exit 1
