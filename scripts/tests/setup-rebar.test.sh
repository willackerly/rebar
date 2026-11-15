#!/usr/bin/env bash
# Regression test: setup-rebar.sh, the versioned installer, end to end and
# offline. A local git repo stands in for github.com/willackerly/rebar (tags,
# a branch, a tiny dependency-free cli/ module that prints its stamped
# Version) and a file:// directory stands in for GitHub Releases, populated
# by scripts/package-release.sh exactly as the release workflow does. Every
# install runs under `env -i` with a throwaway HOME inside TMP.
#
# Run once per distinct bash found (/bin/bash — 3.2 on macOS — and the bash
# on PATH):
#   T1  no version → latest tag, pre-releases included (v9.1.0-beta.10 beats
#       beta.2, v9.0.10 and a non-semver tag); prebuilt tarball, checksum OK;
#       current → it; ~/.bashrc gets $REBAR_DIR/current/bin
#   T2  explicit version with no release assets → source fallback, stamped
#       version; current switches; PATH line not duplicated
#   T3  "9.0.10" resolves to v9.0.10; --no-switch leaves current alone
#   T4  branch feat/x → versions/feat-x, version "feat/x@<sha7>"
#   T5  re-install with --source replaces the version in place, no leftovers
#   T6  checksum mismatch → refused, nothing installed, nothing left behind
#   T7  pre-versioned ~/.rebar git clone → untouched (clean, same HEAD),
#       versions/ + current added and git-excluded, new PATH line wins
#   T8  a non-rebar, non-empty ~/.rebar → refused
#   T9  unknown ref → refused
#   T11 a truncated download (`curl | bash` cut short at any line, or inside
#       the last line) runs nothing and leaves ~/.rebar as it was
# Once:
#   T10 without Go: a prebuilt tag installs; a source-only tag explains Go is needed
#
# Usage: bash scripts/tests/setup-rebar.test.sh
# Needs git, go, curl, tar and shasum or sha256sum; skips without go/curl.
# Bash 3.2 compatible.

set -uo pipefail
. "$(dirname "$0")/lib.sh"

ROOT="$(cd "$REBAR_TESTS_DIR/../.." && pwd)"
INSTALLER="$ROOT/setup-rebar.sh"
PACKAGE="$ROOT/scripts/package-release.sh"

for tool in git go curl tar; do
  command -v "$tool" >/dev/null 2>&1 || { echo "SKIP: $tool not installed"; exit 0; }
done

rebar_tmpdir setup-rebar
TMP="$(cd "$TMP" && pwd -P)"

# ── Fake upstream ────────────────────────────────────────────────────────────
O="$TMP/origin"
mkdir -p "$O/bin" "$O/scripts" "$O/templates/project-bootstrap" "$O/cli/cmd"
cp "$ROOT/bin/install" "$O/bin/install"
printf '#!/usr/bin/env bash\necho steward\n' > "$O/scripts/steward.sh"
chmod +x "$O/scripts/steward.sh"
echo "template" > "$O/templates/project-bootstrap/README.md"
printf 'module github.com/willackerly/rebar/cli\n\ngo 1.21\n' > "$O/cli/go.mod"
printf 'package cmd\n\nvar Version = "v0.0.0-unstamped"\n' > "$O/cli/cmd/version.go"
cat > "$O/cli/main.go" <<'EOF'
package main

import (
	"os"

	"github.com/willackerly/rebar/cli/cmd"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		os.Stdout.WriteString("rebar " + cmd.Version + "\n")
		return
	}
	os.Exit(2)
}
EOF
(
  cd "$O" || exit 1
  g init -q -b main
  g add -A && g commit -qm one
  g tag v9.0.0 && g tag v9.0.10
  echo two > two.txt && g add two.txt && g commit -qm two
  g tag v9.1.0-beta.2 && g tag -a -m "annotated" v9.1.0-beta.10 && g tag vnext
  g checkout -qb feat/x && echo x > x.txt && g add x.txt && g commit -qm x
  g checkout -q main
) || fail "could not build the fake upstream"
FEAT_SHA="$(g -C "$O" rev-parse --short=7 feat/x)"

# ── Fake GitHub Releases: only v9.1.0-beta.10 has assets ─────────────────────
GOOS_HOST="$(go env GOOS)"; GOARCH_HOST="$(go env GOARCH)"
export GOCACHE="$TMP/gocache" GOPATH="$TMP/gopath" GOTOOLCHAIN=local GOPROXY=off GOFLAGS=""
"$PACKAGE" v9.1.0-beta.10 --repo "$O" --ref v9.1.0-beta.10 \
  --out "$TMP/releases/v9.1.0-beta.10" --targets "$GOOS_HOST/$GOARCH_HOST" >/dev/null \
  || fail "package-release.sh failed against the fake upstream"
ASSET="rebar_9.1.0-beta.10_${GOOS_HOST}_${GOARCH_HOST}.tar.gz"
[ -f "$TMP/releases/v9.1.0-beta.10/$ASSET" ] || fail "package-release.sh wrote no $ASSET"
grep -q "  $ASSET\$" "$TMP/releases/v9.1.0-beta.10/rebar_9.1.0-beta.10_checksums.txt" \
  || fail "checksums file does not list $ASSET"
mkdir -p "$TMP/badrel"
cp -R "$TMP/releases/v9.1.0-beta.10" "$TMP/badrel/"
printf 'tampered' >> "$TMP/badrel/v9.1.0-beta.10/$ASSET"
ok "package-release.sh: $ASSET + checksums from the fake upstream"

# ── Runner ───────────────────────────────────────────────────────────────────
dir_of() { dirname "$(command -v "$1")"; }
TPATH="$(dir_of go):$(dir_of git):$(dir_of curl):/usr/bin:/bin:/usr/sbin:/sbin"
RELURL="file://$TMP/releases"

# inst <home> <bash> [args...] — run the installer in a clean environment.
inst() {
  local home="$1" sh="$2"
  shift 2
  mkdir -p "$home"
  env -i HOME="$home" PATH="${IPATH:-$TPATH}" SHELL=/bin/bash TMPDIR="$TMP" \
    GOCACHE="$GOCACHE" GOPATH="$GOPATH" GOTOOLCHAIN=local GOPROXY=off \
    REBAR_REPO="$O" REBAR_RELEASE_URL="$RELURL" \
    "$sh" "$INSTALLER" "$@" 2>&1
}
has() { printf '%s\n' "$1" | grep -qF -- "$2" || { printf '%s\n' "$1" | tail -20 >&2; fail "$3: missing '$2'"; }; }
current_of() { readlink "$1/.rebar/current"; }
leftovers() { ls -A "$1/.rebar/versions" | grep '^\.' || true; }

BASHES="/bin/bash"
other="$(command -v bash)"
if [ ! -x /bin/bash ]; then
  BASHES="$other"
elif [ "$("$other" -c 'echo $BASH_VERSION')" != "$(/bin/bash -c 'echo $BASH_VERSION')" ]; then
  BASHES="$BASHES $other"
fi

n=0
for sh in $BASHES; do
  n=$((n + 1))
  bv="$("$sh" -c 'echo $BASH_VERSION')"
  H="$TMP/h$n-main"

  # T1
  out="$(inst "$H" "$sh")" || { echo "$out" | tail -20 >&2; fail "T1 [$bv] latest install failed"; }
  has "$out" "Latest: v9.1.0-beta.10" "T1 [$bv]"
  has "$out" "Checksum OK" "T1 [$bv]"
  [ "$(current_of "$H")" = "versions/v9.1.0-beta.10" ] || fail "T1 [$bv] current is $(current_of "$H")"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.1.0-beta.10" ] || fail "T1 [$bv] wrong version"
  [ -f "$H/.rebar/current/scripts/steward.sh" ] || fail "T1 [$bv] tarball tree is incomplete"
  grep -q '^method=prebuilt' "$H/.rebar/current/.rebar-install" || fail "T1 [$bv] not a prebuilt install"
  grep -qF "$H/.rebar/current/bin" "$H/.bashrc" || fail "T1 [$bv] PATH not wired to current/bin"
  ok "T1 [$bv] latest = v9.1.0-beta.10 (pre-release), prebuilt + checksum, current, PATH"

  # T2
  out="$(inst "$H" "$sh" v9.0.0)" || { echo "$out" | tail -20 >&2; fail "T2 [$bv] failed"; }
  has "$out" "has no prebuilt assets" "T2 [$bv]"
  has "$out" "Falling back to a source build" "T2 [$bv]"
  has "$out" "current: v9.1.0-beta.10 → v9.0.0" "T2 [$bv]"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.0.0" ] || fail "T2 [$bv] source build not stamped"
  [ "$(grep -cF "$H/.rebar/current/bin" "$H/.bashrc")" = "1" ] || fail "T2 [$bv] PATH line duplicated"
  ok "T2 [$bv] no assets → source fallback, stamped v9.0.0, current switched"

  # T3
  out="$(inst "$H" "$sh" 9.0.10 --no-switch)" || { echo "$out" | tail -20 >&2; fail "T3 [$bv] failed"; }
  [ "$(current_of "$H")" = "versions/v9.0.0" ] || fail "T3 [$bv] --no-switch moved current"
  [ "$("$H/.rebar/versions/v9.0.10/bin/rebar" version)" = "rebar v9.0.10" ] || fail "T3 [$bv] v9.0.10 missing"
  ok "T3 [$bv] 9.0.10 → v9.0.10 side by side, --no-switch keeps current"

  # T4
  out="$(inst "$H" "$sh" feat/x)" || { echo "$out" | tail -20 >&2; fail "T4 [$bv] failed"; }
  [ "$(current_of "$H")" = "versions/feat-x" ] || fail "T4 [$bv] current is $(current_of "$H")"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar feat/x@$FEAT_SHA" ] \
    || fail "T4 [$bv] branch version is '$("$H/.rebar/current/bin/rebar" version)'"
  ok "T4 [$bv] branch feat/x → versions/feat-x, rebar feat/x@$FEAT_SHA"

  # T5
  out="$(inst "$H" "$sh" v9.1.0-beta.10 --source)" || { echo "$out" | tail -20 >&2; fail "T5 [$bv] failed"; }
  has "$out" "Replacing the existing v9.1.0-beta.10" "T5 [$bv]"
  grep -q '^method=source' "$H/.rebar/versions/v9.1.0-beta.10/.rebar-install" || fail "T5 [$bv] not rebuilt"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.1.0-beta.10" ] || fail "T5 [$bv] wrong version"
  [ -z "$(leftovers "$H")" ] || fail "T5 [$bv] left $(leftovers "$H") in versions/"
  [ "$(ls "$H/.rebar/versions" | tr '\n' ' ')" = "feat-x v9.0.0 v9.0.10 v9.1.0-beta.10 " ] \
    || fail "T5 [$bv] versions: $(ls "$H/.rebar/versions" | tr '\n' ' ')"
  ok "T5 [$bv] --source re-install replaces in place; four versions side by side"

  # T6
  H6="$TMP/h$n-mismatch"
  if out="$(RELURL="file://$TMP/badrel" inst "$H6" "$sh" v9.1.0-beta.10)"; then
    fail "T6 [$bv] a tampered tarball was installed"
  fi
  has "$out" "checksum mismatch for $ASSET" "T6 [$bv]"
  case "$out" in *"source build"*) fail "T6 [$bv] fell back to source after a mismatch" ;; esac
  [ ! -e "$H6/.rebar/versions/v9.1.0-beta.10" ] && [ ! -e "$H6/.rebar/current" ] \
    || fail "T6 [$bv] something was installed"
  [ -z "$(leftovers "$H6")" ] || fail "T6 [$bv] left $(leftovers "$H6")"
  ok "T6 [$bv] checksum mismatch refused, nothing installed or left behind"

  # T7
  H7="$TMP/h$n-legacy"
  mkdir -p "$H7"
  g clone -q "$O" "$H7/.rebar" || fail "T7 clone"
  printf 'export PATH="%s/.rebar/bin:$PATH"\n' "$H7" > "$H7/.bashrc"
  head_before="$(g -C "$H7/.rebar" rev-parse HEAD)"
  out="$(inst "$H7" "$sh" v9.1.0-beta.10)" || { echo "$out" | tail -20 >&2; fail "T7 [$bv] failed"; }
  has "$out" "is a pre-versioned rebar clone" "T7 [$bv]"
  [ -z "$(g -C "$H7/.rebar" status --porcelain)" ] || fail "T7 [$bv] clone is dirty: $(g -C "$H7/.rebar" status --porcelain)"
  [ "$(g -C "$H7/.rebar" rev-parse HEAD)" = "$head_before" ] || fail "T7 [$bv] clone HEAD moved"
  [ -x "$H7/.rebar/bin/install" ] || fail "T7 [$bv] clone files touched"
  [ "$(current_of "$H7")" = "versions/v9.1.0-beta.10" ] || fail "T7 [$bv] current not set"
  legacy_line="$(grep -nF "$H7/.rebar/bin:" "$H7/.bashrc" | cut -d: -f1)"
  new_line="$(grep -nF "$H7/.rebar/current/bin" "$H7/.bashrc" | cut -d: -f1)"
  [ -n "$new_line" ] && [ "$new_line" -gt "$legacy_line" ] \
    || fail "T7 [$bv] current/bin must come after (and so take precedence over) the legacy PATH line"
  ok "T7 [$bv] legacy clone untouched and clean; versions/ added; new PATH line wins"

  # T8
  H8="$TMP/h$n-other"
  mkdir -p "$H8/.rebar" && echo mine > "$H8/.rebar/notes.txt"
  if out="$(inst "$H8" "$sh" v9.1.0-beta.10)"; then fail "T8 [$bv] installed into a foreign dir"; fi
  has "$out" "neither a rebar install nor empty" "T8 [$bv]"
  [ "$(ls -A "$H8/.rebar")" = "notes.txt" ] || fail "T8 [$bv] foreign dir modified"
  ok "T8 [$bv] foreign ~/.rebar refused and untouched"

  # T9
  if out="$(inst "$H" "$sh" no-such-ref)"; then fail "T9 [$bv] unknown ref installed"; fi
  has "$out" "no tag or branch named 'no-such-ref'" "T9 [$bv]"
  ok "T9 [$bv] unknown ref refused"

  # T11 — `curl | bash` cut short must run nothing: every line-count prefix,
  # and every byte cut inside the last line (dropping only its newline is fine).
  state() { echo "$(current_of "$H") | $(ls -A "$H/.rebar/versions" | tr '\n' ' ') | $("$H/.rebar/current/bin/rebar" version) | $(cksum < "$H/.bashrc")"; }
  before="$(state)"
  lines="$(wc -l < "$INSTALLER" | tr -d ' ')"
  bytes="$(wc -c < "$INSTALLER" | tr -d ' ')"
  last="$(tail -n 1 "$INSTALLER" | wc -c | tr -d ' ')"
  cuts=""
  i=1; while [ "$i" -lt "$lines" ]; do cuts="$cuts n$i"; i=$((i + 17)); done
  cuts="$cuts n$((lines - 1))"
  k=2; while [ "$k" -lt "$last" ]; do cuts="$cuts c$((bytes - k))"; k=$((k + 1)); done
  for cut in $cuts; do
    case "$cut" in n*) head -n "${cut#n}" "$INSTALLER" ;; c*) head -c "${cut#c}" "$INSTALLER" ;; esac > "$TMP/trunc.sh"
    out="$(env -i HOME="$H" PATH="$TPATH" SHELL=/bin/bash TMPDIR="$TMP" GOCACHE="$GOCACHE" GOPATH="$GOPATH" \
      GOTOOLCHAIN=local GOPROXY=off REBAR_REPO="$O" REBAR_RELEASE_URL="$RELURL" \
      "$sh" -s -- v9.1.0-beta.10 < "$TMP/trunc.sh" 2>&1)"
    case "$out" in *"Installing rebar"*|*"Replacing"*) echo "$out" | tail -5 >&2; fail "T11 [$bv] truncated at $cut ran an install" ;; esac
    [ "$(state)" = "$before" ] || fail "T11 [$bv] truncated at $cut changed ~/.rebar: $(state)"
  done
  ok "T11 [$bv] $(echo $cuts | wc -w | tr -d ' ') truncated copies of the script ran nothing"
done

# T10
# Needs a PATH that keeps git/curl/system tools but has no go. Some hosts
# (GitHub's ubuntu runners) also have go in a system directory, so check the
# candidate PATH itself rather than guessing from where go was found.
NOGO="$(dir_of git):$(dir_of curl):/usr/bin:/bin:/usr/sbin:/sbin"
if ! PATH="$NOGO" command -v go >/dev/null 2>&1; then
  H10="$TMP/h-nogo"
  out="$(IPATH="$NOGO" inst "$H10" bash v9.1.0-beta.10)" || { echo "$out" | tail -20 >&2; fail "T10 prebuilt without Go failed"; }
  [ "$("$H10/.rebar/current/bin/rebar" version)" = "rebar v9.1.0-beta.10" ] || fail "T10 wrong version"
  if out="$(IPATH="$NOGO" inst "$H10" bash v9.0.0)"; then fail "T10 built without Go?"; fi
  has "$out" "Go is required" "T10"
  [ "$(current_of "$H10")" = "versions/v9.1.0-beta.10" ] || fail "T10 failed install moved current"
  ok "T10 without Go: prebuilt installs, source-only tag explains Go is needed, current kept"
else
  echo "skip T10: go is on the system PATH here ($(PATH="$NOGO" command -v go)); covered where it isn't (macOS CI)"
fi

echo "PASS: setup-rebar.sh"
