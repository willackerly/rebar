#!/usr/bin/env bash
# Regression test: .rebarrc `checks_enable` runs named checks regardless of the
# tier, each as it would run at tier 3; unset, tiers behave as before.
#
#   T1  tier 1, unset: ground truth SKIPs even with a broken METRICS floor;
#       the registry check is not tier-gated and still catches a stale registry
#   T2  tier 1 + checks_enable: ground truth runs, passes, and fails on a
#       broken floor; the registry check passes and fails on a stale registry
#   T3  through the suites: pre-commit.sh and cold-start-checks.sh run the
#       enabled check at tier 1
#   T4  an unknown name: one WARN line per suite run, ignored, exit unchanged;
#       `.sh` suffix and the REBAR_CHECKS_ENABLE env override are accepted
#   T5  a named jtbd check blocks at tier 2 (advisory without the setting);
#       contract headers run at tier 1; tier 3 unset is unchanged
#
# Usage: bash scripts/tests/checks-enable.test.sh
# Override the scripts under test with REBAR_SCRIPTS_DIR.
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
rebar_tmpdir checks-enable

P="$TMP/project"
mkdir -p "$P/architecture" "$P/src"
touch "$P/src/a_test.go" "$P/src/b_test.go"
printf '# C\n\n**Status:** active\n\n## Purpose\n\nThe foo.\n' > "$P/architecture/CONTRACT-S1-FOO.1.0.md"
printf 'files >= 2\n' > "$P/METRICS"
rc() {
  { printf 'tier = %s\n' "$1"; shift
    printf 'metric.files.paths = src\nmetric.files.names = *_test.go\n'
    printf '%s\n' "$@"; } > "$P/.rebarrc"
}
rc 1
run_in "$P" compute-registry.sh >/dev/null || fail "setup: registry generation"

# T1 — unset at tier 1: ground truth skips, registry still runs.
printf 'files >= 3\n' > "$P/METRICS"
out="$(run_in "$P" check-ground-truth.sh)" || { echo "$out"; fail "T1: ground truth should skip at tier 1"; }
contains "$out" '^SKIP: tier 1 < required tier 3' || { echo "$out"; fail "T1: no SKIP line"; }
printf '\n<!-- edited -->\n' >> "$P/architecture/CONTRACT-REGISTRY.md"
out="$(run_in "$P" compute-registry.sh --check)" && { echo "$out"; fail "T1: stale registry passed at tier 1"; }
run_in "$P" compute-registry.sh >/dev/null
ok "T1: tier 1 without checks_enable skips ground truth; the registry check runs at every tier"

# T2 — enabled at tier 1: both run and can fail.
rc 1 'checks_enable = check-ground-truth compute-registry'
printf 'files >= 2\n' > "$P/METRICS"
out="$(run_in "$P" check-ground-truth.sh)" || { echo "$out"; fail "T2: enabled ground truth should pass"; }
contains "$out" 'running check-ground-truth because .rebarrc checks_enable names it' \
  || { echo "$out"; fail "T2: no NOTE that the check was force-enabled"; }
contains "$out" 'files = 2  (floor 2)' || { echo "$out"; fail "T2: metric not computed"; }
printf 'files >= 3\n' > "$P/METRICS"
out="$(run_in "$P" check-ground-truth.sh)" && { echo "$out"; fail "T2: broken floor passed"; }
contains "$out" 'DRIFT' || { echo "$out"; fail "T2: no DRIFT for a broken floor"; }
out="$(run_in "$P" compute-registry.sh --check)" || { echo "$out"; fail "T2: current registry failed"; }
printf '\n<!-- edited -->\n' >> "$P/architecture/CONTRACT-REGISTRY.md"
out="$(run_in "$P" compute-registry.sh --check)" && { echo "$out"; fail "T2: stale registry passed"; }
run_in "$P" compute-registry.sh >/dev/null
ok "T2: checks_enable runs ground truth at tier 1 (pass and fail); registry passes and fails"

# T3 — the suites honour it too.
out="$(run_in "$P" pre-commit.sh)" && { echo "$out"; fail "T3: pre-commit passed a broken floor"; }
contains "$out" 'DRIFT' || { echo "$out"; fail "T3: pre-commit did not run ground truth"; }
out="$(run_in "$P" cold-start-checks.sh)"
contains "$out" '^\[FAIL\] check-ground-truth' || { echo "$out"; fail "T3: cold start did not run ground truth"; }
rc 1
out="$(run_in "$P" pre-commit.sh)" || { echo "$out"; fail "T3: pre-commit at tier 1 should skip ground truth"; }
printf 'files >= 2\n' > "$P/METRICS"
ok "T3: pre-commit.sh and cold-start-checks.sh run an enabled check at tier 1"

# T4 — unknown names warn once and are ignored; .sh suffix and env override work.
rc 1 'checks_enable = check-ground-truth.sh no-such-check'
out="$(run_in "$P" check-ground-truth.sh)" || { echo "$out"; fail "T4: unknown name changed the exit"; }
contains "$out" "WARN: checks_enable names unknown check 'no-such-check'" \
  || { echo "$out"; fail "T4: no warning for an unknown name"; }
contains "$out" 'running check-ground-truth' || { echo "$out"; fail "T4: .sh suffix not accepted"; }
out="$(run_in "$P" pre-commit.sh)" || { echo "$out"; fail "T4: pre-commit failed on an unknown name"; }
[ "$(count_matches "$out" 'unknown check')" = 1 ] || { echo "$out"; fail "T4: warning not printed exactly once per suite run"; }
rc 1
out="$(cd "$P" && REBAR_PROJECT_ROOT="$P" REBAR_CHECKS_ENABLE=check-ground-truth "$REBAR_SCRIPTS/check-ground-truth.sh" 2>&1)" \
  || { echo "$out"; fail "T4: env override run failed"; }
contains "$out" 'running check-ground-truth' || { echo "$out"; fail "T4: REBAR_CHECKS_ENABLE ignored"; }
ok "T4: unknown names warn once and are ignored; .sh suffix and REBAR_CHECKS_ENABLE work"

# T5 — a named check runs as at tier 3; other tiers unchanged.
printf '# C\n\n## Purpose\n\nNo JTBD sections.\n' > "$P/architecture/CONTRACT-S2-BAR.1.0.md"
rc 2
out="$(run_in "$P" check-jtbd-presence.sh)" || { echo "$out"; fail "T5: jtbd should be advisory at tier 2"; }
contains "$out" 'advisory at Tier 2' || { echo "$out"; fail "T5: no advisory line"; }
rc 2 'checks_enable = check-jtbd-presence'
out="$(run_in "$P" check-jtbd-presence.sh)" && { echo "$out"; fail "T5: a named jtbd check should block"; }
rc 1
out="$(run_in "$P" check-contract-headers.sh)"
contains "$out" '^SKIP: tier 1' || { echo "$out"; fail "T5: contract headers should skip at tier 1"; }
rc 1 'checks_enable = check-contract-headers'
out="$(run_in "$P" check-contract-headers.sh)"
contains "$out" '^SKIP:' && { echo "$out"; fail "T5: enabled contract headers skipped"; }
rm "$P/architecture/CONTRACT-S2-BAR.1.0.md"
rc 3
printf 'files >= 3\n' > "$P/METRICS"
out="$(run_in "$P" check-ground-truth.sh)" && { echo "$out"; fail "T5: tier 3 ground truth passed a broken floor"; }
contains "$out" 'checks_enable' && { echo "$out"; fail "T5: tier 3 unset mentions checks_enable"; }
ok "T5: a named check runs as at tier 3 (jtbd blocks, headers run); tier 3 unset unchanged"
