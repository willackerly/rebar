#!/usr/bin/env bash
# Regression test: repo-specific settings live in .rebarrc, and the scripts of
# a rebar install check a project that carries no copies of them
# (REBAR_PROJECT_ROOT).
#
#   T1  declared metrics: files, dirs, depth, exclude_names, exclude_dirs,
#       contains, distinct, missing path
#   T1b metric tracked = true: only git-tracked files / dirs (all outside git)
#   T2  METRICS exact and >= floor pass; drift and a broken floor fail
#   T2b metrics_strict (.rebarrc or REBAR_METRICS_STRICT): a missing METRICS,
#       a computed key missing from it, and no metric declarations fail (the
#       first two not reported as drift); by default all three pass
#   T3  registry_scope_prefixes / registry_purpose_heading / _style
#   T4  source_dirs / source_extensions reach check-todos
#   T5  a project copy of a check still runs, with a deprecation notice;
#       thin wrappers are never picked
#   T6  check-ground-truth.sh --print prints values at any tier, with no
#       METRICS; REBAR_RCFILE replaces the project's .rebarrc (rebar migrate)
#
#   T7  check-ground-truth colours only on a terminal, never with NO_COLOR
#
# Usage: bash scripts/tests/declarative-config.test.sh
# Override the scripts under test with REBAR_SCRIPTS_DIR.
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
SRC="$REBAR_SCRIPTS"
rebar_tmpdir declarative

# The project has no scripts/ at all: everything runs from $SRC.
P="$TMP/project"
mkdir -p "$P/architecture" "$P/api/a" "$P/api/node_modules/x" "$P/sdk/go-sdk" \
  "$P/sdk/web-sdk" "$P/sdk/other" "$P/agents/architect" "$P/agents/steward"
touch "$P/api/a/one_test.go" "$P/api/a/two_test.go" "$P/api/a/main.go" \
  "$P/api/node_modules/x/dep_test.go" \
  "$P/agents/architect/AGENT.md" "$P/agents/steward/AGENT.md"
# Built from parts so rebar's own registry scan doesn't see a reference here.
printf '// %s:%s\npackage a\n' CONTRACT S1-FOO.1.0 > "$P/api/a/main.go"
for f in CONTRACT-S1-FOO.1.0.md CONTRACT-S1-FOO.2.0.md CONTRACT-SDK-C2-BAR.1.0.md \
  CONTRACT-TEMPLATE.md; do
  printf '# %s\n\n**Status:** active\n\n## Why this exists\n\nShort | purpose.\n' "$f" \
    > "$P/architecture/$f"
done
cat > "$P/.rebarrc" <<'EOF'
tier = 3
registry_impl_counts = false

metric.test_files.paths        = api
metric.test_files.names        = *_test.go   # trailing comment is ignored
metric.test_files.exclude_dirs = node_modules

metric.sdks.type      = dirs
metric.sdks.paths     = sdk
metric.sdks.min_depth = 1
metric.sdks.max_depth = 1
metric.sdks.names     = *-sdk

metric.agents.paths     = agents
metric.agents.min_depth = 2
metric.agents.max_depth = 2
metric.agents.names     = AGENT.md

metric.contract_files.paths         = architecture
metric.contract_files.names         = CONTRACT-*.md
metric.contract_files.exclude_names = CONTRACT-TEMPLATE.md

metric.contract_ids.paths         = architecture
metric.contract_ids.names         = CONTRACT-*.md
metric.contract_ids.exclude_names = CONTRACT-TEMPLATE.md
metric.contract_ids.distinct      = ^CONTRACT-(.*)\.[0-9]+\.[0-9]+\.md$

metric.headed_sources.paths    = api
metric.headed_sources.names    = *.go
metric.headed_sources.contains = CONTRACT:[A-Za-z0-9_-]+\.[0-9]+\.[0-9]+

metric.nothing.paths = no-such-dir
EOF
( cd "$P" && g init -q -b main && g add . && g commit -q -m init ) || fail "setup"

gt() { ( cd / && REBAR_PROJECT_ROOT="$P" "$SRC/check-ground-truth.sh" ) 2>&1; }

# T1 — every metric computes the expected value.
printf 'test_files = 2\nsdks = 2\nagents = 2\ncontract_files = 3\ncontract_ids = 2\nheaded_sources = 1\nnothing = 0\n' \
  > "$P/METRICS"
out="$(gt)" || { echo "$out"; fail "T1: declared metrics did not match"; }
for m in 'test_files = 2' 'sdks = 2' 'agents = 2' 'contract_files = 3' 'contract_ids = 2' \
  'headed_sources = 1' 'nothing = 0'; do
  printf '%s\n' "$out" | grep -q "$m" || { echo "$out"; fail "T1: missing '$m'"; }
done
ok "T1: declared metrics compute files, dirs, depth, excludes, contains, distinct"

# T1b — tracked = true counts only what git tracks (dirs: holding a tracked
# file); outside a git work tree it counts everything, with a note.
mkdir -p "$P/ops/kept" "$P/ops/local"
printf 'MARK\n' > "$P/ops/kept/a.sh"
: > "$P/ops/b.sh"
printf 'local/\n' > "$P/ops/.gitignore"
( cd "$P" && g add ops && g commit -q -m ops ) || fail "T1b: setup"
: > "$P/ops/local/ignored.sh"
printf 'MARK\n' > "$P/ops/untracked.sh"
cat > "$TMP/tracked.rc" <<'RC'
metric.all_sh.paths         = ops
metric.all_sh.names         = *.sh
metric.tracked_sh.paths     = ./ops/
metric.tracked_sh.names     = *.sh
metric.tracked_sh.tracked   = true
metric.tracked_dirs.type      = dirs
metric.tracked_dirs.paths     = ops
metric.tracked_dirs.min_depth = 1
metric.tracked_dirs.tracked   = true
metric.tracked_marked.paths    = ops
metric.tracked_marked.contains = MARK
metric.tracked_marked.tracked  = true
metric.tracked_names.paths    = ops
metric.tracked_names.names    = *.sh
metric.tracked_names.distinct = ^(.*)$
metric.tracked_names.tracked  = true
RC
out="$( cd / && REBAR_RCFILE="$TMP/tracked.rc" REBAR_PROJECT_ROOT="$P" "$SRC/check-ground-truth.sh" --print 2>&1 )"
want='all_sh=4
tracked_sh=2
tracked_dirs=1
tracked_marked=1
tracked_names=2'
[ "$out" = "$want" ] || { echo "$out"; fail "T1b: tracked counts"; }
NG="$TMP/nogit"
mkdir -p "$NG" && cp -R "$P/ops" "$NG/ops"
out="$( cd / && REBAR_RCFILE="$TMP/tracked.rc" REBAR_PROJECT_ROOT="$NG" "$SRC/check-ground-truth.sh" --print 2>"$TMP/err" )"
printf '%s\n' "$out" | grep -qx 'tracked_sh=4' || { echo "$out"; fail "T1b: outside git, tracked should count every file"; }
grep -q 'not a git work tree' "$TMP/err" || { cat "$TMP/err"; fail "T1b: no note outside git"; }
( cd "$P" && g rm -rq ops && g commit -q -m "drop ops" ) || fail "T1b: cleanup"
rm -rf "$P/ops"
ok "T1b: tracked = true counts git-tracked files and dirs only; outside git, everything"

# T2 — floors pass at or above the value; drift fails either way.
printf 'test_files >= 1\nsdks = 2\n' > "$P/METRICS"
out="$(gt)" || { echo "$out"; fail "T2: floor below the count should pass"; }
printf '%s\n' "$out" | grep -q 'test_files = 2  (floor 1)' || { echo "$out"; fail "T2: floor not reported"; }
printf 'test_files >= 3\n' > "$P/METRICS"
gt >/dev/null && fail "T2: floor above the count should fail"
printf 'test_files = 5\n' > "$P/METRICS"
gt >/dev/null && fail "T2: exact mismatch should fail"
ok "T2: METRICS exact and floor semantics hold for declared metrics"

# T2b — metrics_strict: by default a computed key missing from METRICS (NEW)
# and a missing METRICS (SKIP) pass; strict, both fail.
FULL='test_files = 2\nsdks = 2\nagents = 2\ncontract_files = 3\ncontract_ids = 2\nheaded_sources = 1\nnothing = 0\n'
printf 'test_files = 2\n' > "$P/METRICS"
out="$(gt)" || { echo "$out"; fail "T2b: a key missing from METRICS should pass by default"; }
printf '%s\n' "$out" | grep -q 'NEW.*sdks = 2' || { echo "$out"; fail "T2b: NEW not reported"; }
out="$(REBAR_METRICS_STRICT=true gt)" && { echo "$out"; fail "T2b: strict, a key missing from METRICS should fail"; }
printf '%s\n' "$out" | grep -q 'MISSING.*sdks = 2' || { echo "$out"; fail "T2b: MISSING not reported"; }
cp "$P/.rebarrc" "$TMP/rc.bak"
printf 'metrics_strict = true\n' >> "$P/.rebarrc"
gt >/dev/null && fail "T2b: .rebarrc metrics_strict = true not honored"
printf "$FULL" > "$P/METRICS"
out="$(gt)" || { echo "$out"; fail "T2b: strict with every key in METRICS should pass"; }
rm "$P/METRICS"
out="$(gt)" && { echo "$out"; fail "T2b: strict, a missing METRICS should fail"; }
printf '%s\n' "$out" | grep -q 'FAIL.*No METRICS file' || { echo "$out"; fail "T2b: missing METRICS not reported"; }
printf '%s\n' "$out" | grep -q 'drift' && { echo "$out"; fail "T2b: a missing METRICS is not drift"; }
printf '%s\n' "$out" | grep -q 'Ground truth not verified' || { echo "$out"; fail "T2b: missing METRICS summary"; }
cp "$TMP/rc.bak" "$P/.rebarrc"
out="$(gt)" || { echo "$out"; fail "T2b: a missing METRICS should pass by default"; }
printf '%s\n' "$out" | grep -q 'SKIP.*No METRICS file' || { echo "$out"; fail "T2b: SKIP not reported"; }
out="$(REBAR_METRICS_STRICT=false gt)" || fail "T2b: REBAR_METRICS_STRICT=false should not be strict"
printf "$FULL" > "$P/METRICS"
# No metric.* lines at all: passes by default, fails strict (not as drift).
printf 'tier = 3\n' > "$TMP/nometrics.rc"
gtrc() { ( cd / && REBAR_RCFILE="$TMP/nometrics.rc" REBAR_PROJECT_ROOT="$P" "$SRC/check-ground-truth.sh" ) 2>&1; }
out="$(gtrc)" || { echo "$out"; fail "T2b: no metric declarations should pass by default"; }
printf '%s\n' "$out" | grep -q 'No metrics declared' || { echo "$out"; fail "T2b: no-declarations note missing"; }
printf 'metrics_strict = true\n' >> "$TMP/nometrics.rc"
out="$(gtrc)" && { echo "$out"; fail "T2b: strict, no metric declarations should fail"; }
printf '%s\n' "$out" | grep -q 'FAIL.*No metrics declared' || { echo "$out"; fail "T2b: strict no-declarations FAIL not reported"; }
printf '%s\n' "$out" | grep -q 'drift' && { echo "$out"; fail "T2b: no declarations is not drift"; }
ok "T2b: metrics_strict fails a missing METRICS, keys missing from it, and no declarations"

# T3 — registry classification and purpose rendering come from .rebarrc.
reg() { ( cd / && REBAR_PROJECT_ROOT="$P" "$SRC/compute-registry.sh" --stdout ); }
out="$(reg)"
# Unscoped, SDK-C2-BAR starts with S and lands in Services.
printf '%s\n' "$out" | grep -A6 '^## Services' | grep -q 'SDK-C2-BAR' \
  || { echo "$out"; fail "T3: SDK-C2-BAR should be a Service without prefixes"; }
printf '%s\n' "$out" | grep -q '| S1-FOO | 1.0 | active | — |' \
  || { echo "$out"; fail "T3: default heading should not read 'Why this exists'"; }
cat >> "$P/.rebarrc" <<'EOF'
registry_scope_prefixes  = SDK-
registry_purpose_heading = Purpose|Why this exists
EOF
out="$(reg)"
printf '%s\n' "$out" | grep -A4 '^## Components' | grep -q 'SDK-C2-BAR' \
  || { echo "$out"; fail "T3: SDK-C2-BAR should be a Component"; }
printf '%s\n' "$out" | grep -qF '| S1-FOO | 1.0 | active | Short \| purpose. |' \
  || { echo "$out"; fail "T3: purpose heading not applied, or ellipsis (the default) did not escape |"; }
printf 'registry_purpose_style   = bytes\n' >> "$P/.rebarrc"
out="$(reg)"
printf '%s\n' "$out" | grep -qF '| S1-FOO | 1.0 | active | Short | purpose. |' \
  || { echo "$out"; fail "T3: registry_purpose_style = bytes should keep the raw purpose"; }
ok "T3: registry scope prefixes and purpose settings apply (ellipsis default, bytes opt-out)"

# T4 — source_dirs and source_extensions drive check-todos.
mkdir -p "$P/tools"
printf '#!/bin/sh\n# TODO: untracked\n' > "$P/tools/x.sh"
( cd "$P" && REBAR_PROJECT_ROOT="$P" "$SRC/check-todos.sh" ) >/dev/null || fail "T4: tools/ is not a default source dir"
printf 'source_dirs = tools\nsource_extensions = sh\n' >> "$P/.rebarrc"
( cd "$P" && REBAR_PROJECT_ROOT="$P" "$SRC/check-todos.sh" ) >/dev/null && fail "T4: configured source dir not scanned"
rm -rf "$P/tools"
ok "T4: source_dirs / source_extensions reach check-todos"

# T5 — a project copy wins with a notice; a thin wrapper never does.
mkdir -p "$P/scripts"
printf '#!/usr/bin/env bash\necho LOCAL-COPY\n' > "$P/scripts/check-ground-truth.sh"
chmod +x "$P/scripts/check-ground-truth.sh"
out="$( cd / && REBAR_PROJECT_ROOT="$P" bash -c "source '$SRC/_rebar-config.sh'; \"\$(_rebar_script check-ground-truth.sh)\"" 2>&1 )"
printf '%s\n' "$out" | grep -q LOCAL-COPY || { echo "$out"; fail "T5: project copy not used"; }
printf '%s\n' "$out" | grep -q deprecated || { echo "$out"; fail "T5: no deprecation notice"; }
printf '#!/usr/bin/env bash\n# rebar-thin-wrapper\nexec rebar check\n' > "$P/scripts/ci-check.sh"
path="$( cd / && REBAR_PROJECT_ROOT="$P" bash -c "source '$SRC/_rebar-config.sh'; _rebar_script ci-check.sh" 2>/dev/null )"
[ "$path" = "$(cd "$SRC" && pwd -P)/ci-check.sh" ] || fail "T5: thin wrapper picked ($path)"
ok "T5: project copies run with a deprecation notice; thin wrappers are skipped"

# T6 — --print and REBAR_RCFILE, as rebar migrate uses them.
rm -f "$P/scripts/check-ground-truth.sh" "$P/scripts/ci-check.sh"
mv "$P/METRICS" "$P/METRICS.off"
out="$( cd / && REBAR_TIER=1 REBAR_PROJECT_ROOT="$P" "$SRC/check-ground-truth.sh" --print 2>&1 )" \
  || { echo "$out"; fail "T6: --print exited non-zero"; }
printf '%s\n' "$out" | grep -qx 'test_files=2' || { echo "$out"; fail "T6: --print lacks test_files=2"; }
printf '%s\n' "$out" | grep -qx 'sdks=2' || { echo "$out"; fail "T6: --print lacks sdks=2"; }
printf 'metric.only_this.paths = api\nmetric.only_this.names = main.go\n' > "$TMP/alt.rc"
out="$( cd / && REBAR_RCFILE="$TMP/alt.rc" REBAR_PROJECT_ROOT="$P" "$SRC/check-ground-truth.sh" --print 2>&1 )"
[ "$out" = "only_this=1" ] || { echo "$out"; fail "T6: REBAR_RCFILE not used instead of .rebarrc"; }
mv "$P/METRICS.off" "$P/METRICS"
ok "T6: --print ignores tier and METRICS; REBAR_RCFILE replaces .rebarrc"

# T7 — colour only on a terminal, and never with NO_COLOR.
ESC=$(printf '\033')
out="$(gt)"
case "$out" in *"$ESC"*) fail "T7: colour codes in piped output" ;; esac
tty_out="$(cd / && REBAR_PROJECT_ROOT="$P" run_tty "$SRC/check-ground-truth.sh" 2>&1)"
rc=$?
if [ "$rc" -eq 77 ]; then
  ok "T7: piped output has no colour (no script(1) for the terminal case)"
else
  case "$tty_out" in *"$ESC["*) ;; *) echo "$tty_out"; fail "T7: no colour on a terminal" ;; esac
  tty_out="$(cd / && NO_COLOR=1 REBAR_PROJECT_ROOT="$P" run_tty "$SRC/check-ground-truth.sh" 2>&1)"
  case "$tty_out" in *"$ESC["*) echo "$tty_out"; fail "T7: colour despite NO_COLOR" ;; esac
  ok "T7: colour on a terminal only, and not with NO_COLOR"
fi

echo "PASS: declarative .rebarrc config"
