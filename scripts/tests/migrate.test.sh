#!/usr/bin/env bash
# Regression test: `rebar migrate` moves legacy script copies into .rebarrc
# settings, deletes only copies it verified, and is idempotent.
#
#   T1  dry run changes nothing (git status stays clean)
#   T2  a customized check-todos.sh becomes source_dirs / source_extensions;
#       after --apply the install's check still finds a TODO in a custom root
#   T3  a metric the schema cannot express keeps check-ground-truth.sh, and
#       is reported with its value; the expressible one moves to .rebarrc
#   T4  a second run proposes nothing
#   T5  a project bootstrapped from the pre-#14 template (from git history):
#       every copy is unmodified, so all go, and checks still pass
#   T6  a verified copy that a CI script and a pre-commit config call becomes
#       a thin wrapper; after --apply both callers still work; --apply
#       refuses a dirty work tree
#   T7  scripts/ symlinked to a directory outside the project: refused,
#       nothing there touched
#   T8  a clone of the rebar repo: refused
#
# Builds the CLI from this checkout, so it needs Go (SKIP otherwise).
# Usage: bash scripts/tests/migrate.test.sh
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
ROOT="$(cd "$REBAR_SCRIPTS/.." && pwd)"
rebar_tmpdir migrate

if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go not installed (rebar migrate is a CLI command)"
  exit 0
fi
( cd "$ROOT/cli" && go build -o "$TMP/rebar" . ) || fail "building the CLI"
rebar() { REBAR_ROOT="$ROOT" "$TMP/rebar" "$@"; }

P="$TMP/project"
mkdir -p "$P/scripts" "$P/lib" "$P/tools" "$P/src"
cat > "$P/.rebarrc" <<'EOF'
# our settings
tier = 3
EOF
printf 'lib_files = 1\nsh_lines = 2\n' > "$P/METRICS"
printf 'package lib\n' > "$P/lib/a.go"
printf '#!/bin/sh\necho hi\n' > "$P/tools/x.sh"

# Customized like virtru-analyze's: hard-coded roots and file types.
cat > "$P/scripts/check-todos.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
DIRS=()
for d in lib tools; do
  [ -d "$d" ] && DIRS+=("$d")
done
hits=$(grep -rn "TODO:" --include="*.go" --include="*.sh" "${DIRS[@]}" 2>/dev/null | grep -v "TRACKED-TASK:" | grep -v "node_modules\\|vendor\\|dist" || true)
if [ -n "$hits" ]; then
  echo "FAIL: $(printf '%s\n' "$hits" | grep -c .) untracked TODO(s) found:"
  echo ""
  echo "$hits"
  echo ""
  echo "Fix them. Also found 0 tracked task(s)."
  exit 1
fi
echo "OK: 0 untracked TODOs. 0 tracked task(s)."
EOF
cat > "$P/scripts/check-ground-truth.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
compute_metrics() {
  echo "lib_files=$(find "$REPO_ROOT/lib" -type f -name '*.go' | wc -l | tr -d ' ')"
  echo "sh_lines=$(cat "$REPO_ROOT"/tools/*.sh | wc -l | tr -d ' ')"
}
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  while IFS='=' read -r k v; do echo "OK: $k = $v"; done < <(compute_metrics)
fi
EOF
chmod +x "$P/scripts/"*.sh
g -C "$P" init -q && g -C "$P" add -A && g -C "$P" commit -qm init

# ── T1 dry run ──
out=$(cd "$P" && rebar migrate 2>&1) || fail "T1: dry run exited non-zero: $out"
[ -z "$(g -C "$P" status --porcelain)" ] || fail "T1: dry run changed the project: $(g -C "$P" status --porcelain)"
echo "$out" | grep -q 'Dry run: nothing changed' || fail "T1: no dry-run notice: $out"
ok "T1 dry run changes nothing"

# ── T2 / T3 proposals ──
echo "$out" | grep -q '+ source_dirs *= lib tools' || fail "T2: source_dirs not proposed: $out"
echo "$out" | grep -q '+ source_extensions *= go sh' || fail "T2: source_extensions not proposed: $out"
echo "$out" | grep -q 'delete   scripts/check-todos.sh' || fail "T2: check-todos.sh not verified: $out"
echo "$out" | grep -q 'kept      sh_lines = 2 — untranslatable' || fail "T3: untranslatable metric not reported with its value: $out"
echo "$out" | grep -q 'keep     scripts/check-ground-truth.sh' || fail "T3: ground truth copy not kept: $out"
echo "$out" | grep -q '+ metric.lib_files.paths *= lib' || fail "T3: expressible metric not proposed: $out"

out=$(cd "$P" && rebar migrate --apply 2>&1) || fail "--apply exited non-zero: $out"
[ ! -e "$P/scripts/check-todos.sh" ] || fail "T2: verified check-todos.sh not deleted"
[ -f "$P/scripts/check-ground-truth.sh" ] || fail "T3: unverified ground truth copy deleted"
head -2 "$P/.rebarrc" | grep -q '^# our settings$' || fail "existing .rebarrc lines not kept: $(cat "$P/.rebarrc")"
echo "$out" | grep -q 'PASS  check-todos via install' || fail "T2: no post-apply check result: $out"
printf '#!/bin/sh\n# TODO: probe\n' > "$P/tools/y.sh"
res=$(cd "$P" && rebar run check-todos 2>&1) && fail "T2: install's check-todos missed a TODO in tools/: $res"
echo "$res" | grep -q 'tools/y.sh' || fail "T2: TODO in tools/y.sh not reported: $res"
rm "$P/tools/y.sh"
ok "T2 check-todos moves to source_dirs/source_extensions and still scans tools/"
ok "T3 untranslatable metric keeps the copy and is reported"

# ── T4 idempotent ──
out=$(cd "$P" && rebar migrate 2>&1) || fail "T4: second run failed: $out"
echo "$out" | grep -q 'Nothing to migrate' || fail "T4: second run proposes changes: $out"
ok "T4 second run proposes nothing"

# ── T6 callers keep working ──
mkdir -p "$TMP/bin"
printf '#!/bin/sh\nREBAR_ROOT="%s" exec "%s" "$@"\n' "$ROOT" "$TMP/rebar" > "$TMP/bin/rebar"
chmod +x "$TMP/bin/rebar"
C="$TMP/callers"
mkdir -p "$C/scripts" "$C/src"
printf 'tier = 3\n' > "$C/.rebarrc"
printf 'package a\n' > "$C/src/a.go"
cp "$REBAR_SCRIPTS/check-todos.sh" "$C/scripts/check-todos.sh"
printf '# a comment migrate does not care about\n' >> "$C/scripts/check-todos.sh"
printf '#!/usr/bin/env bash\nset -e\n"$(dirname "$0")/check-todos.sh"\n' > "$C/scripts/ci.sh"
printf 'repos:\n  - repo: local\n    hooks:\n      - id: todos\n        entry: scripts/check-todos.sh\n        language: system\n' > "$C/.pre-commit-config.yaml"
chmod +x "$C/scripts/"*.sh
g -C "$C" init -q && g -C "$C" add -A && g -C "$C" commit -qm init
touch "$C/wip.txt"
out=$(cd "$C" && PATH="$TMP/bin:$PATH" rebar migrate --apply 2>&1) && fail "T6: --apply ran on a dirty tree: $out"
echo "$out" | grep -q 'uncommitted or untracked' || fail "T6: no dirty-tree refusal: $out"
cmp -s "$REBAR_SCRIPTS/check-todos.sh" "$C/scripts/check-todos.sh" && fail "T6: the refused apply changed the copy"
rm "$C/wip.txt"
out=$(cd "$C" && PATH="$TMP/bin:$PATH" rebar migrate --apply 2>&1) || fail "T6: --apply failed: $out"
grep -q '^# rebar-thin-wrapper' "$C/scripts/check-todos.sh" || fail "T6: called copy not replaced by a wrapper: $out"
echo "$out" | grep -q 'PASS  check-todos via wrapper scripts/check-todos.sh' || fail "T6: wrapper not exercised after apply: $out"
echo "$out" | grep -q 'CI: .pre-commit-config.yaml:5 runs scripts/check-todos.sh' || fail "T6: no CI note: $out"
res=$(cd "$C" && PATH="$TMP/bin:$PATH" bash scripts/ci.sh 2>&1) || fail "T6: scripts/ci.sh broke after apply: $res"
printf '// TODO: probe\n' >> "$C/src/a.go"
res=$(cd "$C" && PATH="$TMP/bin:$PATH" scripts/check-todos.sh 2>&1) && fail "T6: the wrapper missed a TODO: $res"
echo "$res" | grep -q 'src/a.go' || fail "T6: wrapper output lacks the finding: $res"
# A wrapper copied elsewhere (analyze's registry test copies the script into
# a temp dir) checks the directory it sits in, as the copy did.
mkdir -p "$TMP/standalone/scripts"
cp "$C/scripts/check-todos.sh" "$TMP/standalone/scripts/"
res=$(cd / && PATH="$TMP/bin:$PATH" bash "$TMP/standalone/scripts/check-todos.sh" 2>&1) || fail "T6: copied wrapper failed outside a repo: $res"
echo "$res" | grep -q 'No source directories found' || fail "T6: copied wrapper checked the wrong project: $res"
ok "T6 called copies become wrappers and callers still work (also when copied); dirty tree refused"

# ── T7 symlinked scripts/ ──
S="$TMP/shared-scripts"
mkdir -p "$S" "$TMP/linked"
cp "$REBAR_SCRIPTS/check-todos.sh" "$S/"
printf 'tier = 3\n' > "$TMP/linked/.rebarrc"
ln -s "$S" "$TMP/linked/scripts"
g -C "$TMP/linked" init -q && g -C "$TMP/linked" add -A && g -C "$TMP/linked" commit -qm init
out=$(cd "$TMP/linked" && rebar migrate --apply 2>&1) && fail "T7: symlinked scripts/ not refused: $out"
echo "$out" | grep -q 'outside the project' || fail "T7: wrong refusal: $out"
[ -f "$S/check-todos.sh" ] || fail "T7: a file outside the project was deleted"
ok "T7 symlinked scripts/ is refused"

# ── T8 a rebar clone ──
if g clone -q --shared "$ROOT" "$TMP/rebar-clone" 2>/dev/null; then
  out=$(cd "$TMP/rebar-clone" && rebar migrate 2>&1) && fail "T8: a rebar clone was not refused: $out"
  echo "$out" | grep -qE 'rebar source tree|shares history' || fail "T8: wrong refusal: $out"
  ok "T8 a rebar clone is refused"
fi

# ── T5 pre-#14 template ──
last=$(git -C "$ROOT" rev-list -1 HEAD -- templates/project-bootstrap/scripts/check-todos.sh 2>/dev/null || true)
if [ -z "$last" ] || ! git -C "$ROOT" cat-file -e "$last^:templates/project-bootstrap/scripts/check-todos.sh" 2>/dev/null; then
  echo "SKIP T5: no pre-#14 template in this checkout's history"
  exit 0
fi
F="$TMP/fresh"
mkdir -p "$F"
git -C "$ROOT" archive "$last^" templates/project-bootstrap | tar -x -C "$TMP" \
  && cp -R "$TMP/templates/project-bootstrap/." "$F/" || fail "T5: extracting the old template"
# The old template's check-freshness.sh can report an age one day short when
# the clock ticks mid-run (fixed in the install), so its output would differ
# from the install's at random and migrate would rightly keep it. Date the
# markers today: no ages are printed and the outputs are deterministic.
today=$(date +%Y-%m-%d)
for md in $(grep -rlE '(freshness|last-synced): [0-9]{4}-[0-9]{2}-[0-9]{2}' "$F" --include='*.md'); do
  sed -E "s/(freshness|last-synced): [0-9]{4}-[0-9]{2}-[0-9]{2}/\1: $today/" "$md" > "$md.tmp" && mv "$md.tmp" "$md"
done
g -C "$F" init -q && g -C "$F" add -A && g -C "$F" commit -qm init
before=$(cd "$F" && rebar check --pre-commit >/dev/null 2>&1; echo $?)
out=$(cd "$F" && rebar migrate --apply 2>&1) || fail "T5: --apply failed: $out"
left=$(cd "$F/scripts" && ls | LC_ALL=C sort | tr '\n' ' ')  # ls order follows the locale
# cold-start-checks.sh stays as a wrapper: the old template's SessionStart
# hook in .claude/settings.json calls it.
[ "$left" = "README.md ci-check.sh cold-start-checks.sh pre-commit.sh " ] || fail "T5: left in scripts/: $left
$out"
grep -q '^# rebar-thin-wrapper' "$F/scripts/ci-check.sh" || fail "T5: ci-check.sh is not the thin wrapper"
grep -q '^# rebar-thin-wrapper' "$F/scripts/cold-start-checks.sh" || fail "T5: cold-start-checks.sh is not a thin wrapper"
grep -q 'rebar migrate' "$F/.rebarrc" && fail "T5: unmodified copies needed settings: $(cat "$F/.rebarrc")"
after=$(cd "$F" && rebar check --pre-commit >/dev/null 2>&1; echo $?)
[ "$before" = "$after" ] || fail "T5: rebar check --pre-commit exit $before before, $after after"
out=$(cd "$F" && rebar migrate 2>&1) || fail "T5: second run failed: $out"
echo "$out" | grep -q 'Nothing to migrate' || fail "T5: second run proposes changes: $out"
ok "T5 unmodified template copies all go; pre-commit result unchanged ($after); second run proposes nothing"
