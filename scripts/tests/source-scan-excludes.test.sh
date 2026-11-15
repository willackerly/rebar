#!/usr/bin/env bash
# Regression test: the source scanners skip node_modules/, vendor/, dist/
# (and .git/) directories, and nothing else.
#
# check-contract-refs.sh, check-registry.sh and check-todos.sh filtered their
# match lines with grep -v "node_modules\|vendor\|dist...", which also dropped
# matches under .github/, in files like distance.go, and on any line whose
# text mentioned those words (virtru-analyze, 2026-11-14: refs under .github/
# were never checked). A TODO in such a file passed the pre-commit gate.
# compute-registry.sh's non-git fallback had the same bug on absolute paths,
# so a project under a path containing "dist" lost every match.
# Bash 3.2 compatible.
set -uo pipefail
. "$(dirname "$0")/lib.sh"

rebar_tmpdir source-scan-excludes
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

# check-todos.sh: its TODO.md / check-todos filters also ran on whole lines.
printf '# TODO: move this into TODO.md\n' > "$T/src/a.sh"
printf '# TODO: see the check-todos docs\n' > "$T/src/c.sh"
out=$(run_in "$T" check-todos.sh)
echo "$out" | grep -q 'src/a.sh' || fail "check-todos: TODO mentioning TODO.md missed: $out"
echo "$out" | grep -q 'src/c.sh' || fail "check-todos: TODO mentioning check-todos missed: $out"
ok "check-todos excludes TODO.md and itself by file name, not by line text"

# Agent worktrees under .claude/ hold stale source; they are skipped.
mkdir -p "$P/.claude/worktrees/agent-1/src"
ref S9-WORKTREE.1.0 > "$P/.claude/worktrees/agent-1/src/old.go"
out=$(run_in "$P" check-contract-refs.sh) || true
echo "$out" | grep -q 'S9-WORKTREE' && fail "check-contract-refs scanned .claude/worktrees: $out"
ok ".claude/ agent worktrees are skipped"

# source_exclude_dirs replaces the default list (here: also skip generated/).
mkdir -p "$P/generated"
ref S9-GENERATED.1.0 > "$P/generated/x.go"
out=$(run_in "$P" check-contract-refs.sh) || true
echo "$out" | grep -q 'S9-GENERATED' || fail "generated/ skipped without being configured: $out"
printf 'source_exclude_dirs = node_modules vendor dist generated\n' > "$P/.rebarrc"
out=$(run_in "$P" check-contract-refs.sh) || true
echo "$out" | grep -q 'S9-GENERATED' && fail "source_exclude_dirs not honoured: $out"
rm "$P/.rebarrc"
ok "source_exclude_dirs configures the skipped directories"

# A contracts_dir written ./docs/specs/ is excluded from reference counting.
R="$TMP/registry"
mkdir -p "$R/docs/specs/examples"
printf '# S5\n' > "$R/docs/specs/CONTRACT-S5-SPEC.1.0.md"
ref S5-SPEC.1.0 > "$R/docs/specs/examples/demo.go"
printf 'contracts_dir = ./docs/specs/\n' > "$R/.rebarrc"
out=$(run_in "$R" check-registry.sh) || true
echo "$out" | grep -q 'ORPHAN.*S5-SPEC' || fail "check-registry counted a reference inside the contracts dir (./docs/specs/): $out"
rm "$R/.rebarrc"
out=$(cd "$R" && REBAR_PROJECT_ROOT="$R" "$REBAR_SCRIPTS/check-registry.sh" ./docs/specs/ 2>&1) || true
echo "$out" | grep -q 'ORPHAN.*S5-SPEC' || fail "check-registry ./docs/specs/ (argument) counted a reference inside the contracts dir: $out"
ok "contracts_dir is normalised (./docs/specs/ → docs/specs) before excluding it"

# compute-registry's non-git fallback: a project whose own path contains
# "dist" or ".git" still counts .github/ and distance.go references, and
# skips .claude/ worktrees and node_modules/.
for sub in plain dist-tools/proj foo.github.io/proj; do
  C="$TMP/cr/$sub"
  mkdir -p "$C/architecture" "$C/.github/scripts" "$C/src" "$C/node_modules/x" "$C/.claude/worktrees/a"
  for id in S2-GITHUB S3-DISTANCE S4-HIDDEN; do
    printf '# %s\n**Status:** active\n## Purpose\nx\n' "$id" > "$C/architecture/CONTRACT-$id.1.0.md"
  done
  ref S2-GITHUB.1.0 > "$C/.github/scripts/check.js"
  ref S3-DISTANCE.1.0 > "$C/src/distance.go"
  ref S4-HIDDEN.1.0 > "$C/.claude/worktrees/a/x.go"
  ref S4-HIDDEN.1.0 > "$C/node_modules/x/y.js"
  health=$(run_in "$C" compute-registry.sh --health)
  for want in '| S2-GITHUB | 1 |' '| S3-DISTANCE | 1 |' '| S4-HIDDEN | 0 |'; do
    echo "$health" | grep -qF "$want" || fail "compute-registry (non-git, $sub): want '$want': $health"
  done
done
ok "compute-registry's non-git fallback prunes by path, not by substring"
