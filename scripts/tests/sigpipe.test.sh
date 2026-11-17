#!/usr/bin/env bash
# Regression test: output checks must not die of SIGPIPE under pipefail.
#
# `echo "$out" | grep -q PATTERN` fails at random under `set -o pipefail`:
# grep -q exits at the first match, and echo, still writing, is killed by
# SIGPIPE (exit 141), so the pipeline fails although the pattern matched
# (checks-enable T4 on ubuntu CI). The same goes for `| head` and for
# `git archive | tar -x` with bsdtar, which stops reading at the
# end-of-archive marker (migrate T5 on macOS CI).
#
#   T1  with large output and an early match, the pipe form fails (the
#       hazard is real here, so T2 proves something)
#   T2  lib.sh's contains / count_matches give the right answer every time
#   T3  no test or shipped script uses the pipe forms again
#
# Usage: bash scripts/tests/sigpipe.test.sh
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"

# ~1 MB, far more than a pipe buffer; the match is on the first line.
big="first-line-marker"$'\n'"$(awk 'BEGIN { for (i = 0; i < 100000; i++) print "filler line " i }')"

# T1 — the old form.
piped=0
for _ in 1 2 3; do
  echo "$big" 2>/dev/null | grep -q 'first-line-marker' || piped=$((piped + 1))
done
[ "$piped" -gt 0 ] || fail "T1: echo | grep -q never failed on 1 MB with an early match; the hazard T2 guards against is not reproduced"
ok "T1: echo | grep -q under pipefail failed $piped/3 times on large output with an early match"

# T2 — the helpers.
i=0
while [ $i -lt 20 ]; do
  contains "$big" 'first-line-marker' || fail "T2: contains missed an early match (run $i)"
  i=$((i + 1))
done
contains "$big" -x 'filler line 99999' || fail "T2: contains missed the last line"
contains "$big" -E 'no|such' -x && fail "T2: contains matched a missing pattern"
contains "-n" -F -- '-n' || fail "T2: contains treated TEXT as echo options"
[ "$(count_matches "$big" '^filler line')" = 100000 ] || fail "T2: count_matches: $(count_matches "$big" '^filler line')"
[ "$(count_matches "$big" 'no such line')" = 0 ] || fail "T2: count_matches should print 0 for no match"
ok "T2: contains / count_matches are exact on large output, 20/20"

# T3 — no regressions. A check pipes text into an early-exiting reader:
# grep -q, or tar -x reading git archive.
cd "$REBAR_SCRIPTS" || fail "T3: cd $REBAR_SCRIPTS"
hits=$(grep -nE '(echo|printf)[^|]*\|[[:space:]]*grep[[:space:]]+-[A-Za-z]*q|archive[^|]*\|[[:space:]]*tar[[:space:]]' \
  ./*.sh tests/*.sh 2>/dev/null | grep -vE '^[^:]+:[0-9]+:[[:space:]]*#|^tests/sigpipe\.test\.sh:' || true)
[ -z "$hits" ] || fail "T3: SIGPIPE-prone pipes (use contains / a here-string / a temp file):
$hits"
ok "T3: no echo | grep -q or git archive | tar in scripts/ or scripts/tests/"

echo "PASS: output checks are SIGPIPE-proof"
