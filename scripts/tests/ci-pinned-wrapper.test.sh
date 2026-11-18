#!/usr/bin/env bash
# Regression test: the pinned-wrapper snippet in SETUP.md "## Run rebar in CI",
# extracted from the doc and run offline. A file:// directory stands in for
# GitHub Releases (a stub rebar binary for this platform plus the checksums
# file), so no network is needed; the wrapper gets the pin and the release's
# checksums pasted into REBAR_SHA256, as the doc says.
#   T1  first run downloads, verifies and execs the pinned rebar, caches it,
#       and leaves no temp directory behind
#   T2  a cached run execs straight away (no download)
#   T3  a binary that does not match the pinned SHA-256 is refused, even
#       when the release's own checksums file was replaced to match it;
#       nothing is cached, and the temp directory is cleaned up
#   T4  a nested `rebar` (PATH) is the pinned one
#   T5  the doc's placeholder (no hash pinned for this platform) is refused
#       before any download
# Bash 3.2 compatible.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
rebar_tmpdir ci-wrapper

ROOT_DIR="$(cd "$REBAR_TESTS_DIR/../.." && pwd)"
doc_snippet "$ROOT_DIR/SETUP.md" '**Pinned wrapper.**' > "$TMP/snippet.sh"
grep -q 'exec "$ROOT/rebar"' "$TMP/snippet.sh" || fail "snippet not found in SETUP.md"
command -v curl >/dev/null 2>&1 || { echo "SKIP: curl not installed"; exit 0; }

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "SKIP: no release name for $(uname -m)"; exit 0 ;;
esac
asset="rebar_9.9.9_${os}_${arch}"
sha256() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }

# release <dir>: a fake release v9.9.9 whose binary reports its args and
# which rebar is first on PATH.
release() {
  mkdir -p "$1/v9.9.9"
  printf '#!/usr/bin/env bash\necho "pinned rebar $* (path: $(command -v rebar))"\n' > "$1/v9.9.9/$asset"
  (cd "$1/v9.9.9" && sha256 "$asset" > rebar_9.9.9_checksums.txt)
}
release "$TMP/releases"
release "$TMP/bad"
printf 'tampered\n' >> "$TMP/bad/v9.9.9/$asset"
(cd "$TMP/bad/v9.9.9" && sha256 "$asset" > rebar_9.9.9_checksums.txt)  # a replaced asset with a matching sums file

# pin <out> <checksums file>: the snippet with REBAR_PIN v9.9.9 and the
# REBAR_SHA256 block replaced by the file's lines.
pin() {
  awk -v sums="$2" '
    /^REBAR_PIN=/ { print "REBAR_PIN=\"v9.9.9\""; next }
    /^REBAR_SHA256="$/ { print; while ((getline l < sums) > 0) print l; skip = 1; next }
    skip && /^"$/ { skip = 0 }
    !skip { print }
  ' "$TMP/snippet.sh" > "$1"
  chmod +x "$1"
}
pin "$TMP/rebar.sh" "$TMP/releases/v9.9.9/rebar_9.9.9_checksums.txt"
grep -q "  $asset\$" "$TMP/rebar.sh" || fail "could not pin the checksums into the snippet"
mkdir -p "$TMP/tmpdir"
run_wrapper() {  # run_wrapper <releases dir> args...
  local rel="$1"
  shift
  TMPDIR="$TMP/tmpdir" REBAR_CACHE_DIR="$TMP/cache" REBAR_RELEASES="file://$rel" "$TMP/rebar.sh" "$@" 2>&1
}
leftovers() { ls -A "$TMP/tmpdir"; }

# T1
out="$(run_wrapper "$TMP/releases" check -v)" || fail "T1: wrapper failed: $out"
case "$out" in "pinned rebar check -v "*) ;; *) fail "T1: unexpected output: $out" ;; esac
[ -x "$TMP/cache/v9.9.9/rebar" ] || fail "T1: binary not cached"
[ -z "$(leftovers)" ] || fail "T1: temp directory left behind: $(leftovers)"
ok "T1 cold run downloads, verifies and execs the pinned rebar, no temp left"

# T4
case "$out" in *"(path: $TMP/cache/v9.9.9/rebar)") ;; *) fail "T4: nested rebar is not the pinned one: $out" ;; esac
ok "T4 nested rebar calls get the pinned binary"

# T2
out="$(run_wrapper "$TMP/nowhere" run check-todos)" || fail "T2: cached run failed: $out"
case "$out" in "pinned rebar run check-todos "*) ;; *) fail "T2: unexpected output: $out" ;; esac
ok "T2 cached run execs without downloading"

# T3
rm -rf "$TMP/cache"
if out="$(run_wrapper "$TMP/bad" check)"; then fail "T3: tampered binary accepted: $out"; fi
case "$out" in *"checksum mismatch for $asset"*) ;; *) fail "T3: unexpected error: $out" ;; esac
[ ! -e "$TMP/cache/v9.9.9" ] || fail "T3: a tampered binary was cached"
[ -z "$(leftovers)" ] || fail "T3: temp directory left behind: $(leftovers)"
ok "T3 a binary not matching the pinned SHA-256 is refused (the release's sums file is not trusted), nothing cached"

# T5
sed -e 's|^REBAR_PIN=.*|REBAR_PIN="v9.9.9"|' "$TMP/snippet.sh" > "$TMP/unpinned.sh"
chmod +x "$TMP/unpinned.sh"
if out="$(TMPDIR="$TMP/tmpdir" REBAR_CACHE_DIR="$TMP/cache5" REBAR_RELEASES="file://$TMP/releases" "$TMP/unpinned.sh" check 2>&1)"; then
  fail "T5: ran without a pinned hash: $out"
fi
case "$out" in *"no SHA-256 pinned for $asset"*) ;; *) fail "T5: unexpected error: $out" ;; esac
[ ! -e "$TMP/cache5" ] && [ -z "$(leftovers)" ] || fail "T5: downloaded or cached something"
ok "T5 no pinned hash for this platform → refused before downloading"
