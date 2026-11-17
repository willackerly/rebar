#!/usr/bin/env bash
# Regression test: `contract_namespace` in .rebarrc lets source reference
# this repo's contracts as CONTRACT:<namespace>:<ID>.<v> as well as the short
# CONTRACT:<ID>.<v>, and every script that parses references agrees on it.
# References qualified with another namespace belong to another repo: they
# are never implementations of, or broken references to, our contracts.
# The Go side (config.ContractNamespace, config.LocalContractRef, `rebar
# context developer`) is covered by cli/internal/config and cli/cmd tests.
#
#   T1  check-contract-refs checks short and own-qualified refs, counts
#       foreign ones without failing, and catches a broken qualified ref
#   T2  compute-registry counts qualified implementations, ignores foreign
#       ones, and reports a qualified shadow by its bare ID
#   T3  check-registry sees a contract implemented only via a qualified ref
#   T4  steward lists qualified implementations, not foreign references
#   T5  the same with contracts_dir = specs
#   T6  unset: qualified refs stay invisible, exactly as before namespaces
#   T7  an invalid namespace (bad characters, or shaped like ID.version) is
#       ignored with one warning per run; env whitespace is trimmed
#   T8  look-alikes are foreign: an unescaped dot (githubXcom/...) and a
#       namespace prefix (<ns>-fork:); CONTRACT:<ID>.<v>:<word>.<digit> is a
#       short ref, so a missing one is still BROKEN
#
# Usage: bash scripts/tests/contract-namespace.test.sh
# Override the scripts under test with REBAR_SCRIPTS_DIR.
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
rebar_tmpdir contract-ns
run() { run_in "$P" "$@"; }
unset REBAR_CONTRACT_NAMESPACE REBAR_CONTRACTS_DIR REBAR_TIER _REBAR_NAMESPACE_VALIDATED

NS=github.com/acme/widgets
M=CONTRACT   # references are built from parts so rebar's own scans skip them

# make_project <dir> <contracts_dir> <rc-extra>
make_project() {
  P="$TMP/$1"
  local cdir="$2"
  mkdir -p "$P/$cdir" "$P/src"
  # Impl counts on: this test checks what the counts include.
  printf 'tier = 3\nregistry_impl_counts = true\n%b' "$3" > "$P/.rebarrc"
  local id
  for id in S1-FOO.1.0 C2-BAR.1.0; do
    printf '# %s\n\n**Status:** active\n\n## Purpose\n\n%s does things.\n\n## Why this exists\n\nReason.\n\n## Who needs this\n\nUs.\n\n## Scenarios\n\nOne.\n\n## Interfaces\n\n## Behavioral Contracts\n\n## Error Handling\n\n## Test Requirements\n\n## Implementing Files\n' \
      "$id" "$id" > "$P/$cdir/CONTRACT-$id.md"
  done
  printf '// %s:%s\npackage src\n' "$M" S1-FOO.1.0 > "$P/src/foo.go"                       # short
  printf '// %s:%s:%s\npackage src\n' "$M" "$NS" C2-BAR.1.0 > "$P/src/bar.go"               # own namespace
  printf '// %s:%s:%s\npackage src\n' "$M" github.com/acme/other S1-FOO.1.0 > "$P/src/consumer.go"  # foreign, same ID
  printf '// %s:%s:%s\npackage src\n' "$M" github.com/acme/other AUTH.1.0 > "$P/src/client.go"      # foreign, no local ID
  ( cd "$P" && g init -q -b main && g add . && g commit -q -m init ) || fail "setup $1"
}

# check_project <contracts_dir> <label>
check_project() {
  local cdir="$1" label="$2" out reg

  # T1 — refs.
  out="$(run check-contract-refs.sh)" || { echo "$out"; fail "$label T1: valid refs reported broken"; }
  contains "$out" 'Checked 2 contract references, 0 broken' || { echo "$out"; fail "$label T1: count"; }
  contains "$out" "Namespace $NS: 2 cross-repo reference(s)" || { echo "$out"; fail "$label T1: cross-repo count"; }
  printf '// %s:%s:%s\n' "$M" "$NS" NOPE.1.0 > "$P/src/bad.go"
  ( cd "$P" && g add src/bad.go )   # compute-registry searches tracked files
  out="$(run check-contract-refs.sh)" && { echo "$out"; fail "$label T1: broken qualified ref passed"; }
  contains "$out" "Expected: $cdir/CONTRACT-NOPE.1.0.md" || { echo "$out"; fail "$label T1: expected path"; }
  ok "$label T1: check-contract-refs"

  # T2 — registry counts and shadows (bad.go still present: NOPE is a shadow).
  out="$(run compute-registry.sh --health)" || { echo "$out"; fail "$label T2: --health"; }
  contains "$out" '| C2-BAR | 1 |' || { echo "$out"; fail "$label T2: qualified impl not counted"; }
  contains "$out" '| S1-FOO | 1 |' || { echo "$out"; fail "$label T2: foreign ref counted as impl"; }
  contains "$out" '\*\*NOPE\*\*' || { echo "$out"; fail "$label T2: qualified shadow not reported by bare ID"; }
  contains "$out" -E '\*\*(AUTH|github)' && { echo "$out"; fail "$label T2: foreign ref reported as shadow"; }
  ( cd "$P" && g rm -q -f src/bad.go )
  run compute-registry.sh >/dev/null || fail "$label T2: compute-registry"
  reg="$(cat "$P/$cdir/CONTRACT-REGISTRY.md")"
  contains "$reg" '| C2-BAR | 1.0 | active | 1 |' || { echo "$reg"; fail "$label T2: registry row"; }
  ok "$label T2: compute-registry"

  # T3 — C2-BAR is implemented only through its qualified reference.
  out="$(run check-registry.sh)" || { echo "$out"; fail "$label T3: check-registry"; }
  ok "$label T3: check-registry"

  # T4 — steward.
  run steward.sh >/dev/null || fail "$label T4: steward"
  out="$(jq -r '.impl_gate.implementing_files[]' "$P/$cdir/.state/C2-BAR.1.0.json")"
  contains "$out" 'src/bar.go$' || { echo "$out"; fail "$label T4: qualified impl missing"; }
  out="$(jq -r '.impl_gate.implementing_files[]' "$P/$cdir/.state/S1-FOO.1.0.json")"
  [ "$(count_matches "$out" .)" = 1 ] && contains "$out" 'src/foo.go$' \
    || { echo "$out"; fail "$label T4: S1-FOO should list only foo.go"; }
  ok "$label T4: steward"
}

make_project ns architecture "contract_namespace = $NS   # ours\n"
check_project architecture "namespace:"

make_project ns-specs specs "contracts_dir = specs\ncontract_namespace = $NS/\n"
check_project specs "namespace+specs:"
[ -e "$P/architecture" ] && fail "T5: architecture/ was created"
ok "T5: contract_namespace composes with contracts_dir"

# T6 — unset: qualified references are not understood, as before.
make_project plain architecture ""
out="$(run check-contract-refs.sh)" || { echo "$out"; fail "T6: refs"; }
contains "$out" 'Checked 1 contract references, 0 broken' || { echo "$out"; fail "T6: count"; }
contains "$out" 'Namespace' && { echo "$out"; fail "T6: namespace line printed"; }
out="$(run compute-registry.sh --health)"
contains "$out" '| C2-BAR | 0 |' || { echo "$out"; fail "T6: C2-BAR should have no implementations"; }
out="$(run check-registry.sh)" && { echo "$out"; fail "T6: C2-BAR should be an orphan"; }
contains "$out" 'ORPHAN (untracked): architecture/CONTRACT-C2-BAR.1.0.md' || { echo "$out"; fail "T6: orphan message"; }
ok "T6: unset keeps the pre-namespace behavior"

# T7 — invalid namespace.
printf 'tier = 3\ncontract_namespace = acme widgets\n' > "$P/.rebarrc"
out="$(run check-contract-refs.sh)" || { echo "$out"; fail "T7: refs"; }
contains "$out" "ignoring contract_namespace 'acme widgets'" || { echo "$out"; fail "T7: no warning"; }
contains "$out" 'Checked 1 contract references' || { echo "$out"; fail "T7: not treated as unset"; }
count_warnings() { count_matches "$1" "ignoring contract_namespace"; }
[ "$(count_warnings "$out")" = 1 ] || { echo "$out"; fail "T7: one script should warn once"; }
out="$(run ci-check.sh)"
[ "$(count_warnings "$out")" = 1 ] || { grep -n 'ignoring contract_namespace' <<<"$out"; fail "T7: ci-check should warn once per run"; }
printf 'tier = 3\ncontract_namespace = acme.1.2\n' > "$P/.rebarrc"
out="$(run check-contract-refs.sh)" || { echo "$out"; fail "T7: ID.version-like refs"; }
contains "$out" "ignoring contract_namespace 'acme.1.2'" || { echo "$out"; fail "T7: ID.version-like namespace accepted"; }
printf 'tier = 3\n' > "$P/.rebarrc"
out="$(REBAR_CONTRACT_NAMESPACE="  $NS  " run check-contract-refs.sh)" || { echo "$out"; fail "T7: env refs"; }
contains "$out" 'ignoring' && { echo "$out"; fail "T7: padded env value rejected"; }
contains "$out" "Namespace $NS: 2 cross-repo" || { echo "$out"; fail "T7: padded env value not trimmed"; }
ok "T7: invalid namespace ignored, warned once; env trimmed"

# T8 — look-alikes, in the namespaced project.
P="$TMP/ns"
printf '// %s:%s:%s\npackage src\n' "$M" githubXcom/acme/widgets C2-BAR.1.0 > "$P/src/dot.go"
printf '// %s:%s:%s\npackage src\n' "$M" "$NS-fork" C2-BAR.1.0 > "$P/src/fork.go"
( cd "$P" && g add src/dot.go src/fork.go )   # compute-registry searches tracked files
out="$(run check-contract-refs.sh)" || { echo "$out"; fail "T8: look-alikes reported broken"; }
contains "$out" 'Checked 2 contract references, 0 broken' || { echo "$out"; fail "T8: look-alike counted local"; }
contains "$out" "Namespace $NS: 4 cross-repo" || { echo "$out"; fail "T8: look-alikes not foreign"; }
out="$(run compute-registry.sh --health)"
contains "$out" '| C2-BAR | 1 |' || { echo "$out"; fail "T8: look-alike counted as implementation"; }
run steward.sh >/dev/null || fail "T8: steward"
out="$(jq -r '.impl_gate.implementing_files[]' "$P/architecture/.state/C2-BAR.1.0.json")"
[ "$(count_matches "$out" .)" = 1 ] || { echo "$out"; fail "T8: steward counted a look-alike"; }
( cd "$P" && g rm -q -f src/dot.go src/fork.go )
printf '// %s:%s:see.2 section\n' "$M" MISSING.1.0 > "$P/src/missing.go"
out="$(run check-contract-refs.sh)" && { echo "$out"; fail "T8: missing short ref before a colon passed"; }
contains "$out" 'Expected: architecture/CONTRACT-MISSING.1.0.md' || { echo "$out"; fail "T8: MISSING not reported"; }
rm "$P/src/missing.go"
ok "T8: look-alikes are foreign; ID.version before a colon stays local"

echo "PASS: contract_namespace"
