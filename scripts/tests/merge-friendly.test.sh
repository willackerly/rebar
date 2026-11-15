#!/usr/bin/env bash
# Regression test: generated rebar files must not make parallel PRs conflict.
#
# Busy adopters (virtru-analyze, 2026-11) hit a merge conflict on every PR:
# compute-registry.sh stamped a timestamp and implementing-file counts into
# CONTRACT-REGISTRY.md, and METRICS held exact counts that most PRs change.
#
# Each case builds a throwaway git repo, makes two branches that each add an
# implementing file and a test, regenerates on both, and merges them.
#
#   T1  registry_impl_counts = false + METRICS floors: both merges are clean,
#       and the merged registry and METRICS still pass their checks
#   T2  impl counts on (default): the merge is clean but leaves main's registry
#       stale. This is why busy repos turn them off.
#   T3  a branch that adds a contract merges cleanly with one that only
#       regenerates
#   T4  output does not depend on the caller's locale
#   T5  --health still reports implementing-file counts
#   T6  untracked files don't change the counts (git grep over tracked files),
#       and outside git the grep -r fallback still counts
#
# Override the scripts under test with REBAR_SCRIPTS_DIR.
set -uo pipefail

. "$(dirname "$0")/lib.sh"
SRC="$REBAR_SCRIPTS"
rebar_tmpdir merge-friendly

# Built at runtime so rebar's own registry scan doesn't read it as a reference.
HEADER="CONTRACT"":S1-FOO.1.0"


# new_repo <dir> <impl_counts>: base repo with two contracts, one impl, one test
new_repo() {
  local dir="$1" counts="$2"
  mkdir -p "$dir/scripts" "$dir/architecture" "$dir/src" "$dir/tests"
  cp "$SRC/compute-registry.sh" "$SRC/_rebar-config.sh" "$dir/scripts/"
  # Ground-truth checker with one real metric in place of the template no-op.
  sed 's|^  :  # no-op.*|  echo "test_files=$(ls tests \| wc -l \| tr -d '"' '"')"|' \
    "$SRC/check-ground-truth.sh" > "$dir/scripts/check-ground-truth.sh"
  chmod +x "$dir/scripts/"*.sh
  printf 'tier = 3\nregistry_impl_counts = %s\n' "$counts" > "$dir/.rebarrc"
  printf '# Ground truth\ntest_files >= 1\n' > "$dir/METRICS"
  for id in S1-FOO S10-BAR; do
    printf '# %s\n\n**Status:** active\n\n## Purpose\n\n%s does things.\n' "$id" "$id" \
      > "$dir/architecture/CONTRACT-$id.1.0.md"
  done
  printf '// %s\npackage src\n' "$HEADER" > "$dir/src/base.go"
  echo base > "$dir/tests/base_test.go"
  ( cd "$dir" && g init -q -b main && scripts/compute-registry.sh >/dev/null \
      && g add . && g commit -q -m base ) || fail "base repo setup in $dir"
}

# branch <dir> <name>: add an implementing file and a test, regenerate, commit
branch() {
  local dir="$1" name="$2"
  ( cd "$dir" && g checkout -q -b "$name" main \
      && printf '// %s\npackage src\n' "$HEADER" > "src/$name.go" \
      && echo "$name" > "tests/${name}_test.go" \
      && scripts/compute-registry.sh >/dev/null \
      && g add . && g commit -q -m "$name" ) || fail "branch $name in $dir"
}

merge_both() {
  local dir="$1"
  ( cd "$dir" && g checkout -q main && g merge -q --no-edit a && g merge -q --no-edit b )
}

# --- T1: busy-repo settings merge cleanly and stay valid ---------------------
R1="$TMP/t1"
new_repo "$R1" false
branch "$R1" a
branch "$R1" b
merge_both "$R1" || fail "T1: branches conflicted with registry_impl_counts = false"
( cd "$R1" && scripts/compute-registry.sh --check >/dev/null ) \
  || fail "T1: merged registry is stale"
( cd "$R1" && scripts/check-ground-truth.sh >/dev/null ) \
  || fail "T1: METRICS floor failed after merge"
grep -q 'Generated:' "$R1/architecture/CONTRACT-REGISTRY.md" \
  && fail "T1: registry still carries a timestamp"
grep -q 'Impl Files' "$R1/architecture/CONTRACT-REGISTRY.md" \
  && fail "T1: registry has impl counts despite registry_impl_counts = false"
echo "ok T1: impl counts off + METRICS floors merge cleanly and stay valid"

# --- T2: impl counts on leave main stale after a clean merge -----------------
R2="$TMP/t2"
new_repo "$R2" true
branch "$R2" a
branch "$R2" b
if merge_both "$R2"; then
  ( cd "$R2" && scripts/compute-registry.sh --check >/dev/null ) \
    && fail "T2: expected a stale registry after merging two impl-count changes"
fi
echo "ok T2: impl counts on are not merge-safe (why busy repos turn them off)"

# --- T3: a new contract on one branch, regeneration on the other ------------
R3="$TMP/t3"
new_repo "$R3" false
( cd "$R3" && g checkout -q -b a main \
    && printf '# S2-NEW\n\n**Status:** draft\n\n## Purpose\n\nNew.\n' > architecture/CONTRACT-S2-NEW.1.0.md \
    && scripts/compute-registry.sh >/dev/null && g add . && g commit -q -m a ) || fail "T3: branch a"
branch "$R3" b
merge_both "$R3" || fail "T3: new-contract branch conflicted with a regenerating branch"
( cd "$R3" && scripts/compute-registry.sh --check >/dev/null ) || fail "T3: merged registry is stale"
echo "ok T3: adding a contract merges cleanly"

# --- T4: locale-independent output --------------------------------------------
c_out="$(cd "$R1" && LC_ALL=C scripts/compute-registry.sh --stdout)"
for loc in en_US.UTF-8 C.UTF-8; do
  out="$(cd "$R1" && LC_ALL="$loc" scripts/compute-registry.sh --stdout 2>/dev/null)"
  [ "$out" = "$c_out" ] || fail "T4: output under LC_ALL=$loc differs from LC_ALL=C"
done
echo "ok T4: output is locale-independent"

# --- T5: --health still reports counts --------------------------------------
health="$(cd "$R1" && scripts/compute-registry.sh --health)"
echo "$health" | grep -q '^| S1-FOO | 3 |' \
  || fail "T5: --health does not report S1-FOO's 3 implementing files"
echo "ok T5: --health reports implementing-file counts"

# --- T6: tracked files only ---------------------------------------------------
printf '// %s\npackage src\n' "$HEADER" > "$R1/src/scratch.go"
health="$(cd "$R1" && scripts/compute-registry.sh --health)"
echo "$health" | grep -q '^| S1-FOO | 3 |' \
  || fail "T6: an untracked file changed S1-FOO's implementing-file count"
rm -rf "$R1/.git"
health="$(cd "$R1" && scripts/compute-registry.sh --health)"
echo "$health" | grep -q '^| S1-FOO | 4 |' \
  || fail "T6: grep -r fallback outside git does not count all 4 files"
echo "ok T6: counts cover tracked files only; non-git fallback still counts"

# --- T7: the regenerate hint names the CLI, and an old hint isn't STALE -------
R7="$TMP/t7"
new_repo "$R7" false
grep -q '^<!-- Regenerate: rebar run compute-registry -->' "$R7/architecture/CONTRACT-REGISTRY.md" \
  || fail "T7: registry hint does not name rebar run compute-registry"
sed 's|^<!-- Regenerate: .*-->|<!-- Regenerate: ./scripts/compute-registry.sh -->|' \
  "$R7/architecture/CONTRACT-REGISTRY.md" > "$TMP/t7.md" && mv "$TMP/t7.md" "$R7/architecture/CONTRACT-REGISTRY.md"
( cd "$R7" && scripts/compute-registry.sh --check >/dev/null ) \
  || fail "T7: a registry with the pre-3.1 hint is reported STALE"
echo "ok T7: new regenerate hint; a registry with the old hint is still current"

echo "PASS: merge-friendly generated files"
