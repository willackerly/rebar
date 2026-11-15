#!/usr/bin/env bash
# Regression test: the source scanners skip node_modules/, vendor/, dist/
# (and .git/) directories, and nothing else.
#
# check-contract-refs.sh, check-registry.sh and check-todos.sh filtered their
# match lines with grep -v "node_modules\|vendor\|dist...", which also dropped
# matches under .github/, in files like distance.go, and on any line whose
# text mentioned those words (virtru-analyze, 2026-11-14: refs under .github/
# were never checked). A TODO in such a file passed the pre-commit gate.
# Bash 3.2 compatible.
set -uo pipefail
. "$(dirname "$0")/lib.sh"

rebar_tmpdir contract-refs
P="$TMP/project"
mkdir -p "$P/architecture" "$P/src" "$P/.github/scripts" \
  "$P/vendor/lib" "$P/node_modules/pkg" "$P/dist" "$P/src/dist"
printf '# S1-FOO\n' > "$P/architecture/CONTRACT-S1-FOO.1.0.md"
# Built from parts so rebar's own scans don't read these as references.
ref() { printf '// %s:%s %s\n' CONTRACT "$1" "${2:-}"; }
ref S1-FOO.1.0                > "$P/src/ok.go"
ref S2-GITHUB.1.0             > "$P/.github/scripts/check.js"
ref S3-DISTANCE.1.0           > "$P/src/distance.go"
ref S4-WORDS.1.0 "see vendor docs" > "$P/src/words.go"
ref S9-VENDOR.1.0             > "$P/vendor/lib/x.go"
ref S9-NODE.1.0               > "$P/node_modules/pkg/x.js"
ref S9-DIST.1.0               > "$P/dist/x.js"
ref S9-NESTED.1.0             > "$P/src/dist/x.go"

out=$(run_in "$P" check-contract-refs.sh) && fail "broken references not reported: $out"
for id in S2-GITHUB S3-DISTANCE S4-WORDS; do
  echo "$out" | grep -q "references CONTRACT:$id" || fail "$id not checked: $out"
done
ok "references under .github/, in distance.go and on lines mentioning vendor are checked"

for id in S9-VENDOR S9-NODE S9-DIST S9-NESTED; do
  echo "$out" | grep -q "CONTRACT:$id" && fail "$id in a skipped directory was checked: $out"
done
echo "$out" | grep -q 'Checked 4 contract references, 3 broken' || fail "unexpected totals: $out"
ok "node_modules/, vendor/, dist/ (also nested) are skipped"

# check-registry.sh: a contract referenced only from .github/ or distance.go
# is implemented, not an orphan; one referenced only from vendor/ is.
printf '# S2\n' > "$P/architecture/CONTRACT-S2-GITHUB.1.0.md"
printf '# S3\n' > "$P/architecture/CONTRACT-S3-DISTANCE.1.0.md"
printf '# S9\n' > "$P/architecture/CONTRACT-S9-VENDOR.1.0.md"
out=$(run_in "$P" check-registry.sh)
echo "$out" | grep -q 'ORPHAN.*S2-GITHUB' && fail "check-registry: S2-GITHUB (referenced from .github/) reported as orphan: $out"
echo "$out" | grep -q 'ORPHAN.*S3-DISTANCE' && fail "check-registry: S3-DISTANCE (referenced from distance.go) reported as orphan: $out"
echo "$out" | grep -q 'ORPHAN.*S9-VENDOR' || fail "check-registry: S9-VENDOR (referenced only from vendor/) not reported: $out"
ok "check-registry counts references by directory, not by substring"

# check-todos.sh: untracked TODOs in distance.go and on a line mentioning
# vendor are caught; one under vendor/ is not.
T="$TMP/todos"
mkdir -p "$T/src/vendor"
printf '# TODO: one\n' > "$T/src/distance.sh"
printf '# TODO: two — see vendor docs\n' > "$T/src/words.sh"
printf '# TODO: skipped\n' > "$T/src/vendor/lib.sh"
printf 'source_dirs = src\nsource_extensions = sh\n' > "$T/.rebarrc"
out=$(run_in "$T" check-todos.sh) && fail "check-todos: untracked TODOs not reported: $out"
echo "$out" | grep -q 'distance.sh' || fail "check-todos: TODO in distance.sh missed: $out"
echo "$out" | grep -q 'words.sh' || fail "check-todos: TODO on a line mentioning vendor missed: $out"
echo "$out" | grep -q 'vendor/lib.sh' && fail "check-todos: TODO under vendor/ reported: $out"
ok "check-todos catches TODOs by directory, not by substring"
