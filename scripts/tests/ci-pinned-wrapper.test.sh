#!/usr/bin/env bash
# Regression test: the pinned-wrapper snippet in SETUP.md "## Run rebar in CI",
# extracted from the doc and run offline. A local repo with a tag stands in for
# github.com/willackerly/rebar and a stub `go` writes bin/rebar, so no network
# or Go toolchain is needed.
#   T1  first run fetches, builds and execs the pinned rebar, and leaves no
#       temp clone behind (the snippet once set an EXIT trap and then exec'd,
#       so the trap never ran and every cold run leaked one)
#   T2  a cached run execs straight away
#   T3  a tag that does not match the pinned commit is refused, and the temp
#       clone is still cleaned up
# Bash 3.2 compatible.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
rebar_tmpdir ci-wrapper

ROOT_DIR="$(cd "$REBAR_TESTS_DIR/../.." && pwd)"
doc_snippet "$ROOT_DIR/SETUP.md" '**The trap: pinned binary, no tree.**' > "$TMP/snippet.sh"
grep -q 'exec "$ROOT/bin/rebar"' "$TMP/snippet.sh" || fail "snippet not found in SETUP.md"

# Upstream: a cli/ dir and a tag.
up="$TMP/upstream"
mkdir -p "$up/cli"
echo "package main" > "$up/cli/main.go"
g -C "$up" init -q
g -C "$up" add -A
g -C "$up" commit -qm init
g -C "$up" tag v9.9.9
sha="$(g -C "$up" rev-parse HEAD)"

# Stub go: `go build ... -o OUT .` writes a rebar that reports its args.
mkdir -p "$TMP/stub"
cat > "$TMP/stub/go" <<'STUB'
#!/usr/bin/env bash
out=""
while [ $# -gt 0 ]; do [ "$1" = -o ] && out="$2"; shift; done
mkdir -p "$(dirname "$out")"
printf '#!/usr/bin/env bash\necho "pinned rebar $*"\n' > "$out"
chmod +x "$out"
STUB
chmod +x "$TMP/stub/go"

wrapper() {
  sed -e "s|^REBAR_PIN=.*|REBAR_PIN=\"v9.9.9\"|" \
      -e "s|^REBAR_PIN_COMMIT=.*|REBAR_PIN_COMMIT=\"$1\"|" "$TMP/snippet.sh" > "$TMP/rebar.sh"
  chmod +x "$TMP/rebar.sh"
}
mkdir -p "$TMP/tmpdir"
run_wrapper() {
  PATH="$TMP/stub:$PATH" TMPDIR="$TMP/tmpdir" REBAR_CACHE_DIR="$TMP/cache" \
    REBAR_REPOSITORY="file://$up" "$TMP/rebar.sh" "$@" 2>&1
}
leftovers() { ls -A "$TMP/tmpdir"; }

# T1
wrapper "$sha"
out="$(run_wrapper check -v)" || fail "T1: wrapper failed: $out"
[ "$out" = "pinned rebar check -v" ] || fail "T1: unexpected output: $out"
[ -z "$(leftovers)" ] || fail "T1: temp clone left behind: $(leftovers)"
[ "$(cat "$TMP/cache/v9.9.9/.pinned-commit")" = "$sha" ] || fail "T1: cache not pinned"
ok "T1 cold run execs the pinned rebar and leaves no temp clone"

# T2
rm "$TMP/stub/go"
out="$(run_wrapper run check-todos)" || fail "T2: cached run failed: $out"
[ "$out" = "pinned rebar run check-todos" ] || fail "T2: unexpected output: $out"
ok "T2 cached run execs without rebuilding"

# T3
rm -rf "$TMP/cache"
wrapper 0000000000000000000000000000000000000000
if out="$(run_wrapper check)"; then fail "T3: mismatched pin accepted: $out"; fi
case "$out" in *"does not point at"*) ;; *) fail "T3: unexpected error: $out" ;; esac
[ -z "$(leftovers)" ] || fail "T3: temp clone left behind: $(leftovers)"
ok "T3 mismatched pin refused, temp clone cleaned up"
