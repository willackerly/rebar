#!/usr/bin/env bash
# Tests for scripts/release-notes.sh (the release workflow's notes body).
#
#   T1 prints the tag's section without its heading, blank-trimmed, up to the
#      next heading or --- rule; a tag that prefixes another (v3.1.0 vs
#      v3.1.0-rc.3) picks the exact one
#   T2 a missing or empty section exits 1; no tag exits 2
#   T3 --link-base makes relative links absolute; URLs, #anchors, /paths and
#      mailto: stay
#   T4 the latest release headings in rebar's own CHANGELOG all yield notes
#
# Usage: bash scripts/tests/release-notes.test.sh
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"
SRC="$REBAR_SCRIPTS"
rebar_tmpdir release-notes

C="$TMP/CHANGELOG.md"
cat > "$C" <<'CHANGES'
# Changelog

## Unreleased

_Nothing yet._

---

## v3.1.0 — 2026-11-16

**Headline.** Summary line.

- one
- two

---

## v3.1.0-rc.3 — 2026-11-16

- rc three

## v3.0.0

## v2.0.0
- old
CHANGES

# T1
out="$(bash "$SRC/release-notes.sh" v3.1.0 "$C")" || fail "T1: v3.1.0 exited $?"
want="$(printf '%s\n' '**Headline.** Summary line.' '' '- one' '- two')"
[ "$out" = "$want" ] || fail "T1: v3.1.0 notes:
$out"
out="$(bash "$SRC/release-notes.sh" v3.1.0-rc.3 "$C")" || fail "T1: rc.3 exited $?"
[ "$out" = "- rc three" ] || fail "T1: rc.3 notes (stops at the next heading): $out"
out="$(bash "$SRC/release-notes.sh" v2.0.0 "$C")" || fail "T1: v2.0.0 exited $?"
[ "$out" = "- old" ] || fail "T1: heading alone on its line: $out"
ok "T1: the exact tag's section, trimmed, up to the next heading or rule"

# T2
bash "$SRC/release-notes.sh" v9.9.9 "$C" >/dev/null 2>&1; rc=$?
[ "$rc" = 1 ] || fail "T2: missing section exited $rc, want 1"
bash "$SRC/release-notes.sh" v3.0.0 "$C" >/dev/null 2>&1; rc=$?
[ "$rc" = 1 ] || fail "T2: empty section exited $rc, want 1"
bash "$SRC/release-notes.sh" >/dev/null 2>&1; rc=$?
[ "$rc" = 2 ] || fail "T2: no tag exited $rc, want 2"
ok "T2: missing or empty section exits 1, no tag exits 2"

# T3
cat > "$TMP/links.md" <<'CHANGES'
## v1.0.0

See [config](docs/CONFIGURATION.md) and [CI](SETUP.md#run-rebar-in-ci), [web](https://x.example/a), [top](#top), [mail](mailto:a@b.c), [root](/abs) and [two](a.md)[links](b.md).
CHANGES
B=https://github.com/o/r/blob/v1.0.0
out="$(bash "$SRC/release-notes.sh" --link-base "$B/" v1.0.0 "$TMP/links.md")" || fail "T3: exited $?"
want="See [config]($B/docs/CONFIGURATION.md) and [CI]($B/SETUP.md#run-rebar-in-ci), [web](https://x.example/a), [top](#top), [mail](mailto:a@b.c), [root](/abs) and [two]($B/a.md)[links]($B/b.md)."
[ "$out" = "$want" ] || fail "T3: relinked notes:
$out"
bash "$SRC/release-notes.sh" --link-base >/dev/null 2>&1; rc=$?
[ "$rc" = 2 ] || fail "T3: --link-base without a value exited $rc, want 2"
ok "T3: --link-base rewrites relative links only"

# T4
n=0
# The latest 8 headings; awk stops itself (a `| head -8` can SIGPIPE the writer).
for tag in $(awk '/^## v[0-9]/ { print $2; if (++n == 8) exit }' "$SRC/../CHANGELOG.md"); do
  bash "$SRC/release-notes.sh" "$tag" >/dev/null || fail "T4: no notes for $tag in rebar's CHANGELOG"
  n=$((n + 1))
done
[ "$n" -gt 0 ] || fail "T4: found no release headings in rebar's CHANGELOG"
ok "T4: the latest $n release headings in rebar's CHANGELOG all yield notes"

echo "PASS: release notes from CHANGELOG"
