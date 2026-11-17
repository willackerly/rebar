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
#   T1  no version → highest stable tag (v9.1.0 beats v9.0.10, and is chosen
#       over the higher pre-release v9.2.0-beta.10 and a non-semver tag);
#       prebuilt binary, checksum OK, companions written by the binary
#       (install-shims), no tree; current → it; ~/.bashrc gets
#       $REBAR_DIR/current/bin
#   T20 a release with only a whole-tree tarball (up to v3.1.0) still
#       installs as a tree
#   T1b --pre → highest tag including pre-releases (v9.2.0-beta.10 beats beta.2)
#   T1c an upstream with only pre-releases → latest falls back to the highest
#   T2  explicit version with no release assets → source fallback, stamped
#       version; current switches; PATH line not duplicated
#   T3  "9.0.10" resolves to v9.0.10; --no-switch leaves current alone
#   T4  branch feat/x → versions/feat-x, version "feat/x@<sha7>"
#   T5  re-install with --source replaces the version in place, no leftovers
#   T6  checksum mismatch (binary, and tarball) → refused, nothing installed,
#       nothing left behind
#   T7  pre-versioned ~/.rebar git clone → untouched (clean, same HEAD),
#       versions/ + current added and git-excluded, new PATH line wins
#   T8  a non-rebar, non-empty ~/.rebar → refused
#   T9  unknown ref → refused
#   T11 a truncated download (`curl | bash` cut short at any line, or inside
#       the last line) runs nothing and leaves ~/.rebar as it was
#   T13 a short commit SHA is refused with a clear error; a full one builds
#   T14 a git repo without rebar's markers is refused like any foreign dir
#   T15 two refs that map to one directory (feat/x, feat-x): the second is refused
#   T16 an install of a version another install holds the lock for is refused
#   T12 a re-install whose final move fails, or that is SIGTERMed between
#       moving the old copy aside and moving the new one in, keeps the old copy
# Once:
#   T17 HOME unset → clear error
#   T18 credentials in REBAR_REPO never reach output or .rebar-install
#   T19 an unreachable release server is reported as such, then source build
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
# Like the real binary, it carries bin/install and writes it (and an ask
# shim) with `install-shims --dir DIR`.
cp "$ROOT/bin/install" "$O/cli/install.txt"
cat > "$O/cli/main.go" <<'EOF'
package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/willackerly/rebar/cli/cmd"
)

//go:embed install.txt
var install []byte

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		os.Stdout.WriteString("rebar " + cmd.Version + "\n")
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "install-shims" && os.Args[2] == "--dir" {
		os.MkdirAll(os.Args[3], 0o755)
		os.WriteFile(filepath.Join(os.Args[3], "ask"), []byte("#!/bin/sh\nexec rebar ask \"$@\"\n"), 0o755)
		os.WriteFile(filepath.Join(os.Args[3], "install"), install, 0o755)
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
  g tag -a -m "annotated" v9.1.0 && g tag v9.2.0-beta.2 && g tag v9.2.0-beta.10 && g tag vnext
  g checkout -qb feat/x && echo x > x.txt && g add x.txt && g commit -qm x
  g checkout -qb feat-x main && echo y > y.txt && g add y.txt && g commit -qm y
  g checkout -q main
) || fail "could not build the fake upstream"
FEAT_SHA="$(g -C "$O" rev-parse --short=7 feat/x)"
FEAT_FULL="$(g -C "$O" rev-parse feat/x)"
# An upstream that has published only pre-releases (T1c).
PREONLY="$TMP/preonly.git"
g init -q --bare "$PREONLY" && g -C "$O" push -q "$PREONLY" main:main \
  && g -C "$PREONLY" tag v9.5.0-rc.1 main || fail "could not build the pre-release-only upstream"

# ── Fake GitHub Releases: v9.1.0 has binaries, v9.2.0-beta.2 a tarball ──
GOOS_HOST="$(go env GOOS)"; GOARCH_HOST="$(go env GOARCH)"
export GOCACHE="$TMP/gocache" GOPATH="$TMP/gopath" GOTOOLCHAIN=local GOPROXY=off GOFLAGS=""
"$PACKAGE" v9.1.0 --repo "$O" --ref v9.1.0 \
  --out "$TMP/releases/v9.1.0" --targets "$GOOS_HOST/$GOARCH_HOST" >/dev/null \
  || fail "package-release.sh failed against the fake upstream"
ASSET="rebar_9.1.0_${GOOS_HOST}_${GOARCH_HOST}"
[ -x "$TMP/releases/v9.1.0/$ASSET" ] || fail "package-release.sh wrote no executable $ASSET"
[ "$(ls "$TMP/releases/v9.1.0" | tr '\n' ' ')" = "rebar_9.1.0_checksums.txt $ASSET " ] \
  || fail "package-release.sh wrote $(ls "$TMP/releases/v9.1.0" | tr '\n' ' ')"
grep -q "  $ASSET\$" "$TMP/releases/v9.1.0/rebar_9.1.0_checksums.txt" \
  || fail "checksums file does not list $ASSET"
[ "$("$TMP/releases/v9.1.0/$ASSET" version)" = "rebar v9.1.0" ] || fail "$ASSET is not stamped v9.1.0"

# A release as v3.1.0 and earlier published it: the whole tree plus bin/rebar.
TARNAME="rebar_9.2.0-beta.2_${GOOS_HOST}_${GOARCH_HOST}"
TARASSET="$TARNAME.tar.gz"
mkdir -p "$TMP/tarwork/$TARNAME" "$TMP/releases/v9.2.0-beta.2"
# Through a file: bsdtar stops reading at the end-of-archive marker (SIGPIPE).
g -C "$O" archive -o "$TMP/tarwork/src.tar" v9.2.0-beta.2
tar -x -C "$TMP/tarwork/$TARNAME" -f "$TMP/tarwork/src.tar"
( cd "$TMP/tarwork/$TARNAME/cli" && CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X github.com/willackerly/rebar/cli/cmd.Version=v9.2.0-beta.2" -o ../bin/rebar . ) \
  || fail "could not build the tarball release"
tar -C "$TMP/tarwork" -czf "$TMP/releases/v9.2.0-beta.2/$TARASSET" "$TARNAME"
( cd "$TMP/releases/v9.2.0-beta.2" && if command -v sha256sum >/dev/null 2>&1; then sha256sum "$TARASSET"; else shasum -a 256 "$TARASSET"; fi ) \
  > "$TMP/releases/v9.2.0-beta.2/rebar_9.2.0-beta.2_checksums.txt"

mkdir -p "$TMP/badrel"
cp -R "$TMP/releases/v9.1.0" "$TMP/releases/v9.2.0-beta.2" "$TMP/badrel/"
printf 'tampered' >> "$TMP/badrel/v9.1.0/$ASSET"
printf 'tampered' >> "$TMP/badrel/v9.2.0-beta.2/$TARASSET"
ok "package-release.sh: $ASSET + checksums from the fake upstream (and a tarball release by hand)"

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
    REBAR_REPO="${IREPO:-$O}" REBAR_RELEASE_URL="$RELURL" \
    "$sh" "$INSTALLER" "$@" 2>&1
}
has() { contains "$1" -F -- "$2" || { printf '%s\n' "$1" | tail -20 >&2; fail "$3: missing '$2'"; }; }
current_of() { readlink "$1/.rebar/current"; }
leftovers() { ls -A "$1/.rebar/versions" | grep '^\.' || true; }

# mv shims for T12: when the installer moves its staged tree onto
# versions/v9.1.0, fail (shim-fail) or SIGTERM the installer (shim-term).
REALMV="$(command -v mv)"
for mode in fail term; do
  mkdir -p "$TMP/shim-$mode"
  if [ "$mode" = fail ]; then act='echo "mv: simulated failure" >&2; exit 1'; else act='kill -TERM $PPID; exit 1'; fi
  cat > "$TMP/shim-$mode/mv" <<EOF
#!/bin/sh
case "\$1" in */.incoming.*) case "\$2" in */versions/v9.1.0) $act ;; esac ;; esac
exec "$REALMV" "\$@"
EOF
  chmod +x "$TMP/shim-$mode/mv"
done

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
  has "$out" "Latest: v9.1.0" "T1 [$bv]"
  has "$out" "Checksum OK" "T1 [$bv]"
  [ "$(current_of "$H")" = "versions/v9.1.0" ] || fail "T1 [$bv] current is $(current_of "$H")"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.1.0" ] || fail "T1 [$bv] wrong version"
  [ -x "$H/.rebar/current/bin/ask" ] && [ -x "$H/.rebar/current/bin/install" ] \
    || fail "T1 [$bv] install-shims did not lay out the companions: $(ls "$H/.rebar/current/bin" | tr '\n' ' ')"
  [ "$(ls -A "$H/.rebar/current" | tr '\n' ' ')" = ".rebar-install bin " ] \
    || fail "T1 [$bv] a binary install holds $(ls -A "$H/.rebar/current" | tr '\n' ' ')"
  grep -q '^method=prebuilt (rebar_9.1.0_' "$H/.rebar/current/.rebar-install" || fail "T1 [$bv] not a prebuilt install"
  grep -q '^layout=binary$' "$H/.rebar/current/.rebar-install" || fail "T1 [$bv] layout not recorded"
  grep -qF "$H/.rebar/current/bin" "$H/.bashrc" || fail "T1 [$bv] PATH not wired to current/bin"
  ok "T1 [$bv] latest = v9.1.0 (stable over v9.2.0-beta.10), prebuilt binary + checksum + companions, current, PATH"

  # T20
  H20="$TMP/h$n-tarball"
  out="$(inst "$H20" "$sh" v9.2.0-beta.2)" || { echo "$out" | tail -20 >&2; fail "T20 [$bv] tarball install failed"; }
  has "$out" "Checksum OK" "T20 [$bv]"
  [ -f "$H20/.rebar/current/scripts/steward.sh" ] || fail "T20 [$bv] tarball tree is incomplete"
  [ "$("$H20/.rebar/current/bin/rebar" version)" = "rebar v9.2.0-beta.2" ] || fail "T20 [$bv] wrong version"
  grep -q '^layout=tree$' "$H20/.rebar/current/.rebar-install" || fail "T20 [$bv] layout not recorded"
  case "$out" in *"may not find its scripts"*) fail "T20 [$bv] warned about a complete tree" ;; esac
  ok "T20 [$bv] a whole-tree tarball release installs as before"

  # T1b
  out="$(inst "$TMP/h$n-pre" "$sh" --pre)" || { echo "$out" | tail -20 >&2; fail "T1b [$bv] --pre failed"; }
  has "$out" "Latest (pre-releases included): v9.2.0-beta.10" "T1b [$bv]"
  [ "$("$TMP/h$n-pre/.rebar/current/bin/rebar" version)" = "rebar v9.2.0-beta.10" ] || fail "T1b [$bv] wrong version"
  ok "T1b [$bv] --pre = v9.2.0-beta.10"

  # T1c
  out="$(IREPO="$PREONLY" inst "$TMP/h$n-preonly" "$sh")" || { echo "$out" | tail -20 >&2; fail "T1c [$bv] failed"; }
  has "$out" "no stable release yet" "T1c [$bv]"
  [ "$(current_of "$TMP/h$n-preonly")" = "versions/v9.5.0-rc.1" ] || fail "T1c [$bv] current is $(current_of "$TMP/h$n-preonly")"
  ok "T1c [$bv] only pre-releases upstream → latest = v9.5.0-rc.1"

  # T2
  out="$(inst "$H" "$sh" v9.0.0)" || { echo "$out" | tail -20 >&2; fail "T2 [$bv] failed"; }
  has "$out" "has no prebuilt assets" "T2 [$bv]"
  has "$out" "Falling back to a source build" "T2 [$bv]"
  has "$out" "current: v9.1.0 → v9.0.0" "T2 [$bv]"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.0.0" ] || fail "T2 [$bv] source build not stamped"
  [ "$(grep -cF "$H/.rebar/current/bin" "$H/.bashrc")" = "1" ] || fail "T2 [$bv] PATH line duplicated"
  ok "T2 [$bv] no assets → source fallback, stamped v9.0.0, current switched"

  # T3
  out="$(inst "$H" "$sh" 9.0.10 --no-switch --server 127.0.0.1:7232)" || { echo "$out" | tail -20 >&2; fail "T3 [$bv] failed"; }
  has "$out" "--server was not applied" "T3 [$bv]"
  [ "$(current_of "$H")" = "versions/v9.0.0" ] || fail "T3 [$bv] --no-switch moved current"
  [ "$("$H/.rebar/versions/v9.0.10/bin/rebar" version)" = "rebar v9.0.10" ] || fail "T3 [$bv] v9.0.10 missing"
  ok "T3 [$bv] 9.0.10 → v9.0.10 side by side, --no-switch keeps current"

  # T4
  out="$(inst "$H" "$sh" feat/x)" || { echo "$out" | tail -20 >&2; fail "T4 [$bv] failed"; }
  [ "$(current_of "$H")" = "versions/feat-x" ] || fail "T4 [$bv] current is $(current_of "$H")"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar feat/x@$FEAT_SHA" ] \
    || fail "T4 [$bv] branch version is '$("$H/.rebar/current/bin/rebar" version)'"
  ok "T4 [$bv] branch feat/x → versions/feat-x, rebar feat/x@$FEAT_SHA"

  # T15 — branch feat-x maps to the same directory as feat/x: refused.
  if out="$(inst "$H" "$sh" feat-x)"; then fail "T15 [$bv] feat-x replaced feat/x"; fi
  has "$out" "already holds 'feat/x', not 'feat-x'" "T15 [$bv]"
  [ "$("$H/.rebar/versions/feat-x/bin/rebar" version)" = "rebar feat/x@$FEAT_SHA" ] || fail "T15 [$bv] feat/x install changed"
  ok "T15 [$bv] a second ref mapping to versions/feat-x is refused"

  # T16 — a concurrent install of the same version is refused.
  mkdir "$H/.rebar/versions/.lock-v9.0.0" && echo 4242 > "$H/.rebar/versions/.lock-v9.0.0/pid"
  if out="$(inst "$H" "$sh" v9.0.0)"; then fail "T16 [$bv] ran despite the lock"; fi
  has "$out" "another install of v9.0.0 is running (pid 4242)" "T16 [$bv]"
  [ -f "$H/.rebar/versions/.lock-v9.0.0/pid" ] || fail "T16 [$bv] removed someone else's lock"
  rm -rf "$H/.rebar/versions/.lock-v9.0.0"
  ok "T16 [$bv] an install of a locked version is refused, the lock kept"

  # T5
  out="$(inst "$H" "$sh" v9.1.0 --source)" || { echo "$out" | tail -20 >&2; fail "T5 [$bv] failed"; }
  has "$out" "Replacing the existing v9.1.0" "T5 [$bv]"
  grep -q '^method=source' "$H/.rebar/versions/v9.1.0/.rebar-install" || fail "T5 [$bv] not rebuilt"
  [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.1.0" ] || fail "T5 [$bv] wrong version"
  [ -z "$(leftovers "$H")" ] || fail "T5 [$bv] left $(leftovers "$H") in versions/"
  [ "$(ls "$H/.rebar/versions" | tr '\n' ' ')" = "feat-x v9.0.0 v9.0.10 v9.1.0 " ] \
    || fail "T5 [$bv] versions: $(ls "$H/.rebar/versions" | tr '\n' ' ')"
  ok "T5 [$bv] --source re-install replaces in place; four versions side by side"

  # T6
  H6="$TMP/h$n-mismatch"
  if out="$(RELURL="file://$TMP/badrel" inst "$H6" "$sh" v9.1.0)"; then
    fail "T6 [$bv] a tampered tarball was installed"
  fi
  has "$out" "checksum mismatch for $ASSET" "T6 [$bv]"
  case "$out" in *"source build"*) fail "T6 [$bv] fell back to source after a mismatch" ;; esac
  if out="$(RELURL="file://$TMP/badrel" inst "$H6" "$sh" v9.2.0-beta.2)"; then
    fail "T6 [$bv] a tampered tarball was installed"
  fi
  has "$out" "checksum mismatch for $TARASSET" "T6 [$bv]"
  [ ! -e "$H6/.rebar/versions/v9.1.0" ] && [ ! -e "$H6/.rebar/versions/v9.2.0-beta.2" ] && [ ! -e "$H6/.rebar/current" ] \
    || fail "T6 [$bv] something was installed"
  [ -z "$(leftovers "$H6")" ] || fail "T6 [$bv] left $(leftovers "$H6")"
  ok "T6 [$bv] checksum mismatch refused, nothing installed or left behind"

  # T7
  H7="$TMP/h$n-legacy"
  mkdir -p "$H7"
  g clone -q "$O" "$H7/.rebar" || fail "T7 clone"
  printf 'export PATH="%s/.rebar/bin:$PATH"\n' "$H7" > "$H7/.bashrc"
  head_before="$(g -C "$H7/.rebar" rev-parse HEAD)"
  out="$(inst "$H7" "$sh" v9.1.0)" || { echo "$out" | tail -20 >&2; fail "T7 [$bv] failed"; }
  has "$out" "is a pre-versioned rebar clone" "T7 [$bv]"
  [ -z "$(g -C "$H7/.rebar" status --porcelain)" ] || fail "T7 [$bv] clone is dirty: $(g -C "$H7/.rebar" status --porcelain)"
  [ "$(g -C "$H7/.rebar" rev-parse HEAD)" = "$head_before" ] || fail "T7 [$bv] clone HEAD moved"
  [ -x "$H7/.rebar/bin/install" ] || fail "T7 [$bv] clone files touched"
  [ "$(current_of "$H7")" = "versions/v9.1.0" ] || fail "T7 [$bv] current not set"
  legacy_line="$(grep -nF "$H7/.rebar/bin:" "$H7/.bashrc" | cut -d: -f1)"
  new_line="$(grep -nF "$H7/.rebar/current/bin" "$H7/.bashrc" | cut -d: -f1)"
  [ -n "$new_line" ] && [ "$new_line" -gt "$legacy_line" ] \
    || fail "T7 [$bv] current/bin must come after (and so take precedence over) the legacy PATH line"
  ok "T7 [$bv] legacy clone untouched and clean; versions/ added; new PATH line wins"

  # T8
  H8="$TMP/h$n-other"
  mkdir -p "$H8/.rebar" && echo mine > "$H8/.rebar/notes.txt"
  if out="$(inst "$H8" "$sh" v9.1.0)"; then fail "T8 [$bv] installed into a foreign dir"; fi
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
      "$sh" -s -- v9.1.0 < "$TMP/trunc.sh" 2>&1)"
    case "$out" in *"Installing rebar"*|*"Replacing"*) echo "$out" | tail -5 >&2; fail "T11 [$bv] truncated at $cut ran an install" ;; esac
    [ "$(state)" = "$before" ] || fail "T11 [$bv] truncated at $cut changed ~/.rebar: $(state)"
  done
  ok "T11 [$bv] $(echo $cuts | wc -w | tr -d ' ') truncated copies of the script ran nothing"

  # T12 — re-installing a version that fails between moving the old copy
  # aside and moving the new one in (mv fails / SIGTERM) keeps the old copy.
  echo keep > "$H/.rebar/versions/v9.1.0/MARKER"
  for mode in fail term; do
    out="$(IPATH="$TMP/shim-$mode:$TPATH" inst "$H" "$sh" v9.1.0)" \
      && fail "T12 [$bv] $mode: the install should have failed"
    [ -f "$H/.rebar/versions/v9.1.0/MARKER" ] \
      || { echo "$out" | tail -5 >&2; fail "T12 [$bv] $mode: the previous v9.1.0 was lost"; }
    [ "$("$H/.rebar/current/bin/rebar" version)" = "rebar v9.1.0" ] || fail "T12 [$bv] $mode: current broken"
    [ -z "$(leftovers "$H")" ] || fail "T12 [$bv] $mode: left $(leftovers "$H")"
  done
  ok "T12 [$bv] a failed or interrupted swap restores the previous copy"

  # T13 — commits by full SHA only; a short SHA gets a clear error.
  H13="$TMP/h$n-sha"
  if out="$(inst "$H13" "$sh" "$FEAT_SHA")"; then fail "T13 [$bv] a short SHA installed"; fi
  has "$out" "full 40-character commit SHA" "T13 [$bv]"
  [ ! -e "$H13/.rebar/current" ] || fail "T13 [$bv] short SHA left a current link"
  out="$(inst "$H13" "$sh" "$FEAT_FULL")" || { echo "$out" | tail -20 >&2; fail "T13 [$bv] full SHA failed"; }
  [ "$("$H13/.rebar/current/bin/rebar" version)" = "rebar $FEAT_FULL@$FEAT_SHA" ] \
    || fail "T13 [$bv] full SHA version is '$("$H13/.rebar/current/bin/rebar" version)'"
  ok "T13 [$bv] short SHA refused with a clear error; full SHA builds"

  # T14 — a git repo that is not a rebar clone is foreign, like T8.
  H14="$TMP/h$n-gitdir"
  mkdir -p "$H14/proj" && g -C "$H14/proj" init -q && echo hi > "$H14/proj/a.txt"
  exclude_before="$(cksum < "$H14/proj/.git/info/exclude")"
  if out="$(inst "$H14" "$sh" v9.1.0 --dir "$H14/proj")"; then fail "T14 [$bv] installed into a non-rebar git repo"; fi
  has "$out" "neither a rebar install nor empty" "T14 [$bv]"
  [ ! -e "$H14/proj/versions" ] && [ ! -e "$H14/proj/current" ] || fail "T14 [$bv] wrote into the repo"
  [ "$(cksum < "$H14/proj/.git/info/exclude")" = "$exclude_before" ] || fail "T14 [$bv] edited info/exclude"
  [ ! -e "$H14/.bashrc" ] || fail "T14 [$bv] wired PATH"
  ok "T14 [$bv] a non-rebar git repo is refused and untouched"
done

# T10
# Needs a PATH that keeps git/curl/system tools but has no go. Some hosts
# (GitHub's ubuntu runners) also have go in a system directory, so check the
# candidate PATH itself rather than guessing from where go was found.
NOGO="$(dir_of git):$(dir_of curl):/usr/bin:/bin:/usr/sbin:/sbin"
if ! PATH="$NOGO" command -v go >/dev/null 2>&1; then
  H10="$TMP/h-nogo"
  out="$(IPATH="$NOGO" inst "$H10" bash v9.1.0)" || { echo "$out" | tail -20 >&2; fail "T10 prebuilt without Go failed"; }
  [ "$("$H10/.rebar/current/bin/rebar" version)" = "rebar v9.1.0" ] || fail "T10 wrong version"
  if out="$(IPATH="$NOGO" inst "$H10" bash v9.0.0)"; then fail "T10 built without Go?"; fi
  has "$out" "Go is required" "T10"
  [ "$(current_of "$H10")" = "versions/v9.1.0" ] || fail "T10 failed install moved current"
  ok "T10 without Go: prebuilt installs, source-only tag explains Go is needed, current kept"
else
  echo "skip T10: go is on the system PATH here ($(PATH="$NOGO" command -v go)); covered where it isn't (macOS CI)"
fi

# T17 — HOME unset: a clear error instead of "unbound variable".
out="$(env -i PATH="$TPATH" REBAR_REPO="$O" REBAR_RELEASE_URL="$RELURL" bash "$INSTALLER" v9.1.0 2>&1)" \
  && fail "T17 installed without HOME"
has "$out" "HOME is not set; pass --dir PATH" "T17"
ok "T17 HOME unset → clear error"

# T18 — credentials in REBAR_REPO stay out of output and the install record.
# git rewrites the credentialed URL to the local upstream (insteadOf).
SECRET_REPO="https://user:s3cret-token@example.invalid/rebar.git"
H18="$TMP/h-cred"
mkdir -p "$H18"
out="$(env -i HOME="$H18" PATH="$TPATH" SHELL=/bin/bash TMPDIR="$TMP" GOCACHE="$GOCACHE" GOPATH="$GOPATH" \
  GOTOOLCHAIN=local GOPROXY=off GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="url.$O.insteadOf" \
  GIT_CONFIG_VALUE_0="$SECRET_REPO" REBAR_REPO="$SECRET_REPO" REBAR_RELEASE_URL="$RELURL" \
  bash "$INSTALLER" v9.0.0 2>&1)" || { echo "$out" | tail -20 >&2; fail "T18 install failed"; }
case "$out" in *s3cret*) fail "T18 the token appeared in the output" ;; esac
grep -q s3cret "$H18/.rebar/current/.rebar-install" && fail "T18 the token was recorded"
grep -qF 'repo=https://***@example.invalid/rebar.git' "$H18/.rebar/current/.rebar-install" || fail "T18 repo not recorded redacted"
ok "T18 credentials in REBAR_REPO are redacted"

# T19 — a release server that cannot be reached is not "no prebuilt assets".
out="$(RELURL="https://127.0.0.1:9" inst "$TMP/h-unreachable" bash v9.1.0)" || { echo "$out" | tail -20 >&2; fail "T19 failed"; }
has "$out" "could not check https://127.0.0.1:9/v9.1.0 for prebuilt assets" "T19"
case "$out" in *"has no prebuilt assets"*) fail "T19 reported an unreachable server as missing assets" ;; esac
has "$out" "Falling back to a source build" "T19"
ok "T19 unreachable release server → named error, then source build"

echo "PASS: setup-rebar.sh"
