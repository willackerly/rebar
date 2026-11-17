#!/usr/bin/env bash
# Regression test: check-registry.sh reports every orphan contract (no
# implementing file) instead of stopping silently at the first one. Its
# `grep | grep -v` search ran under pipefail, so a contract with no
# reference made grep exit 1 and ended the script with no output.
#
#   T1  two untracked orphans: both ORPHAN lines, the FAIL summary, exit 1
#   T2  an orphan tracked in TODO.md passes; an implemented contract is no orphan
#
# Usage: bash scripts/tests/check-registry-orphans.test.sh
# Runs the rebar binary under test (lib.sh: REBAR_TEST_BIN).
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
rebar_tmpdir registry-orphans
P="$TMP/project"
mkdir -p "$P/architecture" "$P/src"
printf 'tier = 3\n' > "$P/.rebarrc"
for id in USED.1.0 LONELY.1.0 PLANNED.1.0; do
  printf '# %s\n\n**Status:** active\n' "$id" > "$P/architecture/CONTRACT-$id.md"
done
# Reference built from parts so rebar's own scans don't see it here.
printf '// %s:%s\npackage src\n' CONTRACT USED.1.0 > "$P/src/used.go"

# T1 — both orphans reported.
out="$(run_in "$P" check-registry.sh)" && { echo "$out"; fail "T1: orphans passed"; }
contains "$out" 'ORPHAN (untracked): architecture/CONTRACT-LONELY.1.0.md' || { echo "$out"; fail "T1: LONELY not reported"; }
contains "$out" 'ORPHAN (untracked): architecture/CONTRACT-PLANNED.1.0.md' || { echo "$out"; fail "T1: PLANNED not reported"; }
contains "$out" 'FAIL: 2 inconsistencies found' || { echo "$out"; fail "T1: summary"; }
contains "$out" 'CONTRACT-USED' && { echo "$out"; fail "T1: implemented contract reported"; }
ok "T1: every orphan reported"

# T2 — TODO.md exemption reached; only LONELY fails.
printf -- '- [ ] implement PLANNED.1.0\n' > "$P/TODO.md"
out="$(run_in "$P" check-registry.sh)" && { echo "$out"; fail "T2: LONELY should still fail"; }
contains "$out" 'ORPHAN (tracked in TODO.md): architecture/CONTRACT-PLANNED.1.0.md' || { echo "$out"; fail "T2: TODO exemption"; }
contains "$out" 'FAIL: 1 inconsistencies found' || { echo "$out"; fail "T2: summary"; }
rm "$P/architecture/CONTRACT-LONELY.1.0.md"
out="$(run_in "$P" check-registry.sh)" || { echo "$out"; fail "T2: should pass with only a tracked orphan"; }
ok "T2: TODO.md exemption and implemented contracts"

echo "PASS: check-registry orphans"
