#!/usr/bin/env bash
# Regression test: `contracts_dir` in .rebarrc moves the contracts out of
# architecture/, and every contract-aware script follows it. The project
# also keeps unrelated docs (and a stray CONTRACT-*.md) in architecture/,
# which must be ignored. The Go side (integrity scanner, config) is covered
# by cli/internal/integrity/scanner_test.go and cli/internal/config.
#
#   T1  compute-registry writes and checks specs/CONTRACT-REGISTRY.md and
#       does not count specs/ as implementing code
#   T2  check-contract-refs resolves refs against specs/ (and fails on a
#       ref that only exists in architecture/)
#   T3  check-registry, check-jtbd-presence, check-prefix-uniqueness read specs/
#   T4  steward writes specs/.state/; cold-start counts specs/ contracts
#   T5  with no setting, architecture/ is still the default
#
# Usage: bash scripts/tests/contracts-dir.test.sh
# Runs the rebar binary under test (lib.sh: REBAR_TEST_BIN).
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
SRC="$REBAR_SCRIPTS"
rebar_tmpdir contracts-dir
run() { run_in "$P" "$@"; }

P="$TMP/project"
mkdir -p "$P/specs" "$P/architecture" "$P/src"
printf 'tier = 3\ncontracts_dir = specs   # contracts live here\nregistry_impl_counts = true\n' > "$P/.rebarrc"
contract() {  # contract <dir> <ID.ver>
  printf '# %s\n\n**Status:** active\n\n## Purpose\n\n%s does things.\n\n## Why this exists\n\nReason.\n\n## Who needs this\n\nUs.\n\n## Scenarios\n\nOne.\n' \
    "$2" "$2" > "$P/$1/CONTRACT-$2.md"
}
contract specs S1-FOO.1.0
contract specs C2-BAR.1.0
contract architecture S9-OLD.1.0             # stray: must not be seen
printf '# Design notes\n' > "$P/architecture/overview.md"
# References built from parts so rebar's own scans don't see them here.
printf '// %s:%s\npackage src\n' CONTRACT S1-FOO.1.0 > "$P/src/foo.go"
printf '// %s:%s\npackage src\n' CONTRACT C2-BAR.1.0 > "$P/src/bar.go"
( cd "$P" && g init -q -b main && g add . && g commit -q -m init ) || fail "setup"

# T1 — registry lives in specs/ and lists only specs/ contracts.
run compute-registry.sh >/dev/null || fail "T1: compute-registry failed"
[ -f "$P/specs/CONTRACT-REGISTRY.md" ] || fail "T1: registry not written to specs/"
[ -f "$P/architecture/CONTRACT-REGISTRY.md" ] && fail "T1: registry written to architecture/"
reg="$(cat "$P/specs/CONTRACT-REGISTRY.md")"
printf '%s\n' "$reg" | grep -q 'S1-FOO' || fail "T1: S1-FOO missing"
printf '%s\n' "$reg" | grep -q 'S9-OLD' && fail "T1: stray architecture/ contract listed"
# One implementing file each: specs/ itself is excluded like architecture/ was.
printf '%s\n' "$reg" | grep -q '| S1-FOO | 1.0 | active | 1 |' || { echo "$reg"; fail "T1: impl count"; }
( cd "$P" && g add . && g commit -q -m reg )
out="$(run compute-registry.sh --check)" || { echo "$out"; fail "T1: --check"; }
ok "T1: registry generated and checked in specs/"

# T2 — refs resolve against specs/.
out="$(run check-contract-refs.sh)" || { echo "$out"; fail "T2: valid refs reported broken"; }
printf '// %s:%s\n' CONTRACT S9-OLD.1.0 > "$P/src/old.go"
run check-contract-refs.sh >/dev/null && fail "T2: ref to architecture/-only contract passed"
rm "$P/src/old.go"
ok "T2: check-contract-refs resolves against specs/"

# T3 — the other contract checks read specs/.
out="$(run check-registry.sh)" || { echo "$out"; fail "T3: check-registry"; }
out="$(run check-jtbd-presence.sh)" || { echo "$out"; fail "T3: check-jtbd-presence"; }
printf '%s\n' "$out" | grep -q 'S9-OLD' && fail "T3: jtbd read architecture/"
out="$(run check-prefix-uniqueness.sh)" || { echo "$out"; fail "T3: check-prefix-uniqueness"; }
printf '%s\n' "$out" | grep -q 'SKIP' && { echo "$out"; fail "T3: prefix check did not find specs/ registry"; }
ok "T3: check-registry / jtbd / prefix-uniqueness read specs/"

# T4 — steward state and cold-start maturity follow specs/.
run steward.sh >/dev/null || fail "T4: steward failed"
[ -f "$P/specs/.state/steward-report.json" ] || fail "T4: steward state not in specs/.state"
[ -d "$P/architecture/.state" ] && fail "T4: steward wrote architecture/.state"
out="$(run cold-start-checks.sh)"
printf '%s\n' "$out" | grep -q 'of 2 contracts declare Status' || { echo "$out"; fail "T4: cold-start count"; }
ok "T4: steward and cold-start use specs/"

# T5 — default unchanged.
printf 'tier = 3\n' > "$P/.rebarrc"
out="$(run check-contract-refs.sh)" && { echo "$out"; fail "T5: default should look in architecture/ (refs there are missing)"; }
printf '%s\n' "$out" | grep -q 'Expected: architecture/CONTRACT-S1-FOO.1.0.md' || { echo "$out"; fail "T5: default dir"; }
ok "T5: architecture/ stays the default"

echo "PASS: contracts_dir"
