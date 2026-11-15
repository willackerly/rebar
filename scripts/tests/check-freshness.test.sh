#!/usr/bin/env bash
# Regression test: check-freshness.sh reports a doc's age in whole days,
# independent of the time of day and of when its `date` calls happen.
#
# BSD `date -j -f "%Y-%m-%d"` fills the missing time fields from the current
# clock, so the parsed date was "that day at the current time". When a second
# ticked between reading TODAY and parsing the date, the age came out a day
# short (seen as 133 vs 134 days in CI, 2026-11-15). The test makes that tick
# certain with a `date` shim that sleeps before parsing, and runs in UTC: in a
# zone with DST, an hour of DST between the two dates hid the lost second.
#
# GNU date parses a bare date as midnight; on Linux the shim test is skipped.
# Bash 3.2 compatible.
set -uo pipefail
. "$(dirname "$0")/lib.sh"

if date --version >/dev/null 2>&1; then
  echo "SKIP: GNU date parses a bare date as midnight; the BSD bug cannot occur here"
  exit 0
fi

rebar_tmpdir check-freshness
P="$TMP/project"
mkdir -p "$P" "$TMP/shim"
export TZ=UTC
days_ago=100
stamp=$(date -j -v-"${days_ago}"d +%Y-%m-%d)
printf '# Notes\n<!-- freshness: %s -->\n' "$stamp" > "$P/NOTES.md"

# A `date` that, when parsing (-j -f), first waits past the next second.
real_date=$(command -v date)
cat > "$TMP/shim/date" <<SHIM
#!/bin/sh
case " \$* " in *" -f "*) sleep 1.1 ;; esac
exec "$real_date" "\$@"
SHIM
chmod +x "$TMP/shim/date"

out=$(cd "$P" && PATH="$TMP/shim:$PATH" REBAR_PROJECT_ROOT="$P" "$REBAR_SCRIPTS/check-freshness.sh" 2>&1)
echo "$out" | grep -q "STALE ($days_ago days): ./NOTES.md" \
  || fail "age is not exactly $days_ago days when the clock ticks during parsing: $out"
ok "age is $days_ago days even when a second ticks between reading now and parsing the date"
