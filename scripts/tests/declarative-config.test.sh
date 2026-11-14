#!/usr/bin/env bash
# Regression test: repo-specific settings live in .rebarrc, and the scripts of
# a rebar install check a project that carries no copies of them
# (REBAR_PROJECT_ROOT).
#
#   T1  declared metrics: files, dirs, depth, exclude_names, exclude_dirs,
#       contains, distinct, missing path
#   T2  METRICS exact and >= floor pass; drift and a broken floor fail
#   T3  registry_scope_prefixes / registry_purpose_heading / _style
#   T4  source_dirs / source_extensions reach check-todos
#   T5  a project copy of a check still runs, with a deprecation notice;
#       thin wrappers are never picked
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

# T2 — floors pass at or above the value; drift fails either way.
printf 'test_files >= 1\nsdks = 2\n' > "$P/METRICS"
out="$(gt)" || { echo "$out"; fail "T2: floor below the count should pass"; }
printf '%s\n' "$out" | grep -q 'test_files = 2  (floor 1)' || { echo "$out"; fail "T2: floor not reported"; }
printf 'test_files >= 3\n' > "$P/METRICS"
gt >/dev/null && fail "T2: floor above the count should fail"
printf 'test_files = 5\n' > "$P/METRICS"
gt >/dev/null && fail "T2: exact mismatch should fail"
ok "T2: METRICS exact and floor semantics hold for declared metrics"

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

echo "PASS: declarative .rebarrc config"
