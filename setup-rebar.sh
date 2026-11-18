#!/usr/bin/env bash
# setup-rebar.sh — versioned one-line installer for rebar.
#
# Installs a rebar version side by side with others:
#
#   ~/.rebar/versions/<version>/   one install per version: bin/rebar plus
#                                  the companions beside it (bin/ask*,
#                                  bin/rebar-install, bin/completions/)
#   ~/.rebar/current               symlink to the active version
#
# and puts ~/.rebar/current/bin on PATH (via that version's bin/rebar-install;
# older versions' bin/install is renamed to that on install), so switching
# versions never touches your shell RC again.
#
#   Release tag   download the self-contained binary rebar_<ver>_<os>_<arch>
#                 from the GitHub Release, verify it against the release's
#                 SHA-256 checksums, and let it write its companions
#                 (`rebar install-shims`). No git or Go needed. Releases
#                 v3.1.0-rc.1 to v3.1.x published only
#                 rebar_<ver>_<os>_<arch>.tar.gz (the whole tree), which is
#                 unpacked as before.
#   Branch / SHA  build from source: git + Go (the whole tree at that ref,
#                 plus bin/rebar).
#
# A release tag is built from source only with --source, or when it predates
# prebuilt releases (before v3.1.0-rc.1). A missing asset, an unreachable
# release server or a checksum mismatch is fatal: never silently replaced
# by a source build (which would skip the release's vulnerability gate).
#
# Usage (curl pipe):
#   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/main/setup-rebar.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/main/setup-rebar.sh | bash -s -- v3.0.0
#
# Usage (local):
#   ./setup-rebar.sh [<version>|latest|<branch>|<full-sha>] [--pre] [--source]
#                    [--no-switch] [--server HOST:PORT] [--dir PATH]
#
# "latest" (the default) is the highest published stable GitHub Release
# (drafts and bare tags never count); --pre includes pre-releases.
#
# An existing pre-versioned ~/.rebar (a plain git clone, the layout before
# this installer) is left untouched; versions/ and current are added next to
# it. See the "existing install" note in --help.
#
# Bash 3.2 compatible (macOS /bin/bash) under set -euo pipefail.

set -euo pipefail

log()  { printf '%s[rebar]%s %s\n' "$BLUE"   "$NC" "$*"; }
warn() { printf '%s[rebar]%s %s\n' "$YELLOW" "$NC" "$*" >&2; }
ok()   { printf '%s[rebar]%s %s\n' "$GREEN"  "$NC" "$*"; }
die()  { printf '%s[rebar]%s %s\n' "$RED"    "$NC" "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
  cat <<EOF
rebar installer (versioned)

Usage:
  $0 [<version>] [--pre] [--source] [--no-switch] [--server HOST:PORT] [--dir PATH]
  $0 --help

  <version>     a release tag (v3.0.0), "latest" (the default: the highest
                published stable GitHub Release), a branch, or a full
                40-character commit SHA
  --pre         with "latest": include pre-releases (v3.1.0-rc.1 over v3.0.0)
  --ref REF     same as <version> (kept for older docs)
  --source      build a release tag from source with your Go instead of
                downloading its prebuilt, govulncheck-gated assets. Without
                it a tag with no prebuilt assets is an error, except tags
                older than v3.1.0-rc.1 (published before there were assets),
                which are built from source
  --no-switch   install side by side but leave ~/.rebar/current where it is
  --server H:P  remote ASK server, written to your shell RC by bin/rebar-install
  --dir PATH    install base (default: ~/.rebar)

Layout:
  \$REBAR_DIR/versions/<version>/   one install per version (bin/rebar and
                                   its companions; a source build keeps
                                   the whole tree)
  \$REBAR_DIR/current               symlink to the active version; PATH
                                   points at \$REBAR_DIR/current/bin

Existing install: if \$REBAR_DIR is a plain git clone (the layout before this
installer), it is not modified, moved, or updated. versions/ and current are
added inside it (and git-excluded), the new PATH entry takes precedence, and
anything still pointing at the clone (.mcp.json files, REBAR_ROOT) keeps
working. Remove the clone's files by hand once nothing uses them.

Env vars:
  REBAR_DIR          install base               (default: ~/.rebar)
  REBAR_REPO         git remote for tags/builds (default: github.com/willackerly/rebar)
  REBAR_REF          version when no <version> argument is given
  REBAR_RELEASE_URL  base URL for release assets, <url>/<tag>/<asset>
                     (default: https://github.com/<owner>/<repo>/releases/download)
  REBAR_RELEASES_API URL of the GitHub Releases list "latest" is picked from
                     (default: https://api.github.com/repos/<owner>/<repo>/releases?per_page=100;
                     unauthenticated, so 60 requests/hour per IP). For mirrors
                     and the test suite, which points it at a file:// fixture
  ASK_SERVER         same as --server

Next steps after install:
  rebar new my-project -d "what it does"   # create a new rebar project
  rebar adopt                              # adopt rebar in an existing repo
EOF
}

# ── Helpers ──────────────────────────────────────────────────────────────────

# owner/repo when REBAR_REPO is on github.com, else empty.
github_slug() {
  local s="$REBAR_REPO"
  case "$s" in
    https://github.com/*)     s="${s#https://github.com/}" ;;
    git@github.com:*)         s="${s#git@github.com:}" ;;
    ssh://git@github.com/*)   s="${s#ssh://git@github.com/}" ;;
    *)                        return 0 ;;
  esac
  s="${s%/}"
  printf '%s\n' "${s%.git}"
}

# curl/wget flags: HTTPS only (redirects too), TLS 1.2+, bounded connect.
# file:// is allowed only when the URL itself is file:// (the
# REBAR_RELEASE_URL override, used by the test suite).
CURL_HTTPS=(--proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 20 --retry 2)

# fetch <url> <out> → 0 downloaded, 2 not found (HTTP 404 or a missing
# file:// path), 1 any other failure, described in $FETCH_ERR.
fetch() {
  local url="$1" out="$2" code rc
  FETCH_ERR=""
  if have curl; then
    case "$url" in
      file://*) code="$(curl -sS --proto '=file' -o "$out" -w '%{http_code}' "$url" 2>"$TMP/fetch.err")" && rc=0 || rc=$? ;;
      *)        code="$(curl -sSL "${CURL_HTTPS[@]}" -o "$out" -w '%{http_code}' "$url" 2>"$TMP/fetch.err")" && rc=0 || rc=$? ;;
    esac
    if [[ "$rc" -eq 0 ]]; then
      case "$code" in
        2??|000) return 0 ;;
        404)     return 2 ;;
        *)       FETCH_ERR="HTTP $code"; return 1 ;;
      esac
    fi
    [[ "$rc" -eq 37 ]] && return 2   # FILE_COULDNT_READ_FILE
    FETCH_ERR="$(head -1 "$TMP/fetch.err" 2>/dev/null)"
    FETCH_ERR="${FETCH_ERR:-curl exit $rc}"
    return 1
  elif have wget; then
    wget -q --https-only -O "$out" "$url" 2>"$TMP/fetch.err" && return 0 || rc=$?
    # 8: the server answered with an error status; wget -q does not say
    # which, and a missing asset is by far the likeliest.
    [[ "$rc" -eq 8 ]] && return 2
    FETCH_ERR="wget exit $rc"
    return 1
  fi
  FETCH_ERR="neither curl nor wget is installed"
  return 1
}

# REBAR_REPO with any user:password@ (or token@) removed, for messages and
# the install record.
redact() { printf '%s' "$1" | sed -E 's#(://)[^/@]+@#\1***@#'; }

sha256_of() {
  if have sha256sum; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# Highest semver tag on stdin (one per line); with "stable", pre-releases
# are skipped. Pre-releases rank below their release (v3.0.0-beta.10 >
# v3.0.0-beta.2, v3.0.0 > v3.0.0-rc.1). Lines that are not
# vMAJOR.MINOR.PATCH[-PRE] are ignored.
semver_max() {
  awk -v stable="${1:-}" '
    function isnum(s) { return s ~ /^[0-9]+$/ }
    function cmp_pre(a, b,   na, nb, A, B, i) {
      if (a == b) return 0
      if (a == "") return 1
      if (b == "") return -1
      na = split(a, A, "."); nb = split(b, B, ".")
      for (i = 1; i <= na && i <= nb; i++) {
        if (A[i] == B[i]) continue
        if (isnum(A[i]) && isnum(B[i])) return (A[i] + 0 < B[i] + 0) ? -1 : 1
        if (isnum(A[i])) return -1
        if (isnum(B[i])) return 1
        return (A[i] "" < B[i] "") ? -1 : 1
      }
      return (na < nb) ? -1 : (na > nb ? 1 : 0)
    }
    function cmp(a, b,   pa, pb, CA, CB, i) {
      sub(/^v/, "", a); sub(/^v/, "", b)
      pa = ""; pb = ""
      if (i = index(a, "-")) { pa = substr(a, i + 1); a = substr(a, 1, i - 1) }
      if (i = index(b, "-")) { pb = substr(b, i + 1); b = substr(b, 1, i - 1) }
      split(a, CA, "."); split(b, CB, ".")
      for (i = 1; i <= 3; i++)
        if (CA[i] + 0 != CB[i] + 0) return (CA[i] + 0 < CB[i] + 0) ? -1 : 1
      return cmp_pre(pa, pb)
    }
    /^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/ {
      if (stable != "" && index($0, "-")) next
      if (best == "" || cmp($0, best) > 0) best = $0
    }
    END { if (best != "") print best }
  '
}

# "<tag> <draft> <prerelease>" for each release in a GitHub Releases list
# (JSON on stdin), without jq. A key always follows "," or "{", so splitting
# there puts every key at the start of a line. Inside a JSON string a quote
# is always escaped, so text in a release body (\"draft\": false) never
# starts a line with a bare quote and never matches. GitHub lists tag_name
# before draft and prerelease in each release.
release_rows() {
  tr ',{' '\n\n' \
    | sed -n -E 's/^[[:space:]]*"(tag_name|draft|prerelease)"[[:space:]]*:[[:space:]]*"?([^"[:space:]]*)"?[[:space:]]*$/\1 \2/p' \
    | awk '
        $1 == "tag_name" { if (tag != "") print tag, draft, pre; tag = $2; draft = "?"; pre = "?"; next }
        $1 == "draft"      { draft = $2 }
        $1 == "prerelease" { pre = $2 }
        END { if (tag != "") print tag, draft, pre }
      '
}

RATE_LIMIT_HELP="GitHub's API rate limit for unauthenticated requests (60/hour per IP) is used up. Retry later, or name the version (see https://github.com/willackerly/rebar/releases): setup-rebar.sh vX.Y.Z"

# Sets VERSION to the highest published release: non-draft, and not a
# pre-release (by GitHub's flag or a -suffix) unless --pre. Bare tags never
# count: a tag is pushed before the release workflow builds, scans and
# publishes its assets, and a tag whose workflow failed has none.
resolve_latest() {
  local api="$REBAR_RELEASES_API" json="$TMP/releases.json" st=0 rows stable newest
  [[ -n "$api" ]] \
    || die "cannot look up the latest release: $REPO_SHOWN is not on github.com and REBAR_RELEASES_API is not set; pass a version"
  fetch "$api" "$json" || st=$?
  if grep -qi 'rate limit' "$json" 2>/dev/null && ! grep -q '"tag_name"' "$json"; then
    die "could not look up the latest release: $RATE_LIMIT_HELP"
  fi
  case "$st" in
    0) ;;
    2) die "could not look up the latest release: $api was not found (or refused the request; with wget that includes the rate limit). Pass a version." ;;
    *) case "$FETCH_ERR" in
         "HTTP 403"|"HTTP 429") die "could not look up the latest release ($FETCH_ERR): $RATE_LIMIT_HELP" ;;
       esac
       die "could not look up the latest release at $api ($FETCH_ERR). Pass a version, or retry." ;;
  esac
  rows="$(release_rows < "$json")"
  [[ -n "$rows" ]] || die "found no published releases at $api; pass a version"
  stable="$(printf '%s\n' "$rows" | awk '$2 == "false" && $3 == "false" { print $1 }' | semver_max stable)"
  newest="$(printf '%s\n' "$rows" | awk '$2 == "false" { print $1 }' | semver_max)"
  if [[ "$PRE" -eq 1 ]]; then
    [[ -n "$newest" ]] || die "found no published releases at $api; pass a version"
    VERSION="$newest"
    log "Latest release (pre-releases included): $VERSION"
  elif [[ -n "$stable" ]]; then
    VERSION="$stable"
    log "Latest release: $VERSION"
  elif [[ -n "$newest" ]]; then
    die "no stable release is published yet (newest pre-release: $newest); pass --pre or a version"
  else
    die "found no published releases at $api; pass a version"
  fi
}

# The first release whose workflow published prebuilt assets. Older release
# tags have none and can only be built from source.
FIRST_ASSET_RELEASE="v3.1.0-rc.1"
predates_assets() {
  local semver='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'
  [[ "$1" =~ $semver && "$1" != "$FIRST_ASSET_RELEASE" ]] || return 1
  [[ "$(printf '%s\n%s\n' "$1" "$FIRST_ASSET_RELEASE" | semver_max)" == "$FIRST_ASSET_RELEASE" ]]
}

# Prints tag | branch | commit for a ref, or dies.
classify_ref() {
  local ref="$1" out full='^[0-9a-f]{40}$' short='^[0-9a-f]{4,39}$' semver='^v[0-9]+\.[0-9]+\.[0-9]+'
  if ! have git; then
    [[ "$ref" =~ $semver ]] || die "git is required to install a branch or commit ($ref)"
    echo tag; return 0
  fi
  out="$(git ls-remote -- "$REBAR_REPO" "refs/tags/$ref" "refs/heads/$ref")" \
    || die "cannot reach $REPO_SHOWN"
  if printf '%s\n' "$out" | awk -v r="refs/tags/$ref" '$2 == r {f=1} END {exit !f}'; then
    echo tag
  elif printf '%s\n' "$out" | awk -v r="refs/heads/$ref" '$2 == r {f=1} END {exit !f}'; then
    echo branch
  elif [[ "$ref" =~ $full ]]; then
    echo commit
  elif [[ "$ref" =~ $short ]]; then
    # A shallow fetch can name a commit only by its full id.
    die "'$ref' looks like a short commit SHA; pass the full 40-character commit SHA"
  else
    die "no tag or branch named '$ref' in $REPO_SHOWN"
  fi
}

detect_platform() {  # prints <os> <arch> (Go names), or nothing if unsupported
  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux)  os=linux ;;
    *)      return 0 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *)             return 0 ;;
  esac
  echo "$os $arch"
}

# ── Prebuilt release ─────────────────────────────────────────────────────────

# no_prebuilt <reason>: for a tag that predates prebuilt assets, note the
# reason and return 1 (the caller builds from source); otherwise die.
no_prebuilt() {
  if predates_assets "$VERSION"; then
    log "$1; $VERSION predates prebuilt release assets ($FIRST_ASSET_RELEASE), building from source."
    return 1
  fi
  die "$1. Not building from source on my own: pass --source to build $VERSION with your Go toolchain (it skips the release workflow's govulncheck gate)."
}

# Installs the release's prebuilt binary (v4.0.0 on) or whole-tree tarball
# (v3.1.x). Returns 1 only when there is nothing to download and the tag
# predates prebuilt assets (see no_prebuilt); every other failure, including
# a download error other than "not found" and a checksum mismatch, is fatal.
install_prebuilt() {
  local tag="$1" plat os arch vn bin tarball asset sums expected actual
  [[ -n "$REBAR_RELEASE_URL" ]] \
    || { no_prebuilt "no release download URL for $REPO_SHOWN (set REBAR_RELEASE_URL)"; return 1; }
  plat="$(detect_platform)"
  [[ -n "$plat" ]] \
    || { no_prebuilt "no prebuilt rebar for $(uname -s)/$(uname -m)"; return 1; }
  os="${plat% *}"; arch="${plat#* }"
  if ! have sha256sum && ! have shasum; then
    no_prebuilt "neither sha256sum nor shasum found; not installing an unverified download"
    return 1
  fi
  vn="${tag#v}"
  bin="rebar_${vn}_${os}_${arch}"
  tarball="$bin.tar.gz"
  sums="rebar_${vn}_checksums.txt"

  log "Looking for a prebuilt $tag ($os/$arch)…"
  local st=0
  fetch "$REBAR_RELEASE_URL/$tag/$sums" "$TMP/$sums" || st=$?
  if [[ "$st" -eq 2 ]]; then
    no_prebuilt "Release $tag has no prebuilt assets (not published yet, or its release workflow failed)"
    return 1
  elif [[ "$st" -ne 0 ]]; then
    die "could not check $REBAR_RELEASE_URL/$tag for prebuilt assets ($FETCH_ERR). Retry, or use --source."
  fi
  # The binary (v4.0.0 and later), else the whole-tree tarball (v3.1.x;
  # v4.0.x also publish a small tarball for older installers).
  for asset in "$bin" "$tarball"; do
    expected="$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$TMP/$sums")"
    [[ -n "$expected" ]] && break
  done
  if [[ -z "$expected" ]]; then
    no_prebuilt "Release $tag has no $os/$arch build"
    return 1
  fi
  fetch "$REBAR_RELEASE_URL/$tag/$asset" "$TMP/$asset" \
    || die "$asset is listed in $sums but could not be downloaded (${FETCH_ERR:-not found}). Retry, or use --source."
  actual="$(sha256_of "$TMP/$asset")"
  if [[ "$actual" != "$expected" ]]; then
    die "checksum mismatch for $asset: expected $expected, got $actual. Refusing to install."
  fi
  log "Checksum OK ($expected)"

  # Called from an `if`, where set -e is off: check each step.
  if [[ "$asset" == "$bin" ]]; then
    mkdir -p "$STAGE/bin" || die "cannot create $STAGE/bin"
    mv "$TMP/$asset" "$STAGE/bin/rebar" || die "cannot move $asset to $STAGE/bin/rebar"
    chmod 755 "$STAGE/bin/rebar" || die "cannot make $STAGE/bin/rebar executable"
    "$STAGE/bin/rebar" version >/dev/null 2>&1 || die "$asset does not run on this machine"
    "$STAGE/bin/rebar" install-shims --dir "$STAGE/bin" >/dev/null \
      || die "$asset could not write its companions (rebar install-shims)"
    LAYOUT=binary
  else
    mkdir -p "$TMP/unpack" || die "cannot create $TMP/unpack"
    tar -xzf "$TMP/$asset" --no-same-owner -C "$TMP/unpack" || die "cannot unpack $asset"
    [[ -d "$TMP/unpack/$bin" ]] || die "$asset does not contain $bin/"
    mv "$TMP/unpack/$bin" "$STAGE" || die "cannot move the unpacked tree to $STAGE"
    LAYOUT=tree
  fi
  METHOD="prebuilt ($asset)"
  COMMIT=""
  return 0
}

# ── Source build ─────────────────────────────────────────────────────────────

install_source() {
  local ref="$1" kind="$2" want stamp
  have git || die "git is required to build rebar from source"
  have go  || die "Go is required to build rebar from source (https://go.dev/dl/), or install a release that has prebuilt assets"
  case "$kind" in
    tag)    want="refs/tags/$ref" ;;
    branch) want="refs/heads/$ref" ;;
    *)      want="$ref" ;;
  esac

  log "Fetching ${ref} from ${REPO_SHOWN}…"
  git init -q "$TMP/src"
  git -C "$TMP/src" fetch -q --depth 1 -- "$REBAR_REPO" "$want" \
    || die "could not fetch $ref from $REPO_SHOWN"
  COMMIT="$(git -C "$TMP/src" rev-parse 'FETCH_HEAD^{commit}')"

  # Same tree a release tarball carries: the committed files, no .git.
  mkdir -p "$STAGE"
  git -C "$TMP/src" archive --format=tar "$COMMIT" | tar -x --no-same-owner -C "$STAGE"
  [[ -d "$STAGE/cli" ]] || die "$ref has no cli/ to build"

  if [[ "$kind" == "tag" ]]; then
    stamp="$ref"
  else
    stamp="${ref}@${COMMIT:0:7}"
  fi
  log "Building rebar $stamp (go build)…"
  # Refs older than the ldflags-settable cli/cmd.Version ignore -X and report
  # the version hard-coded in their source.
  ( cd "$STAGE/cli" && CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X ${GO_PKG}.Version=${stamp}" -o ../bin/rebar . ) \
    || die "go build failed for $ref"
  METHOD="source ($REPO_SHOWN@${COMMIT:0:12})"
  LAYOUT=tree
  return 0
}

# Point $REBAR_DIR/current at versions/<name> by renaming a new symlink over
# the old one, so there is no moment without a current. GNU mv spells "treat
# the target as a file" -T, BSD/macOS mv -h; plain ln -sfn is the fallback.
switch_current() {
  local tmp="$REBAR_DIR/.current.$$"
  rm -f "$tmp"
  ln -s "versions/$1" "$tmp"
  if mv -T "$tmp" "$REBAR_DIR/current" 2>/dev/null || mv -h "$tmp" "$REBAR_DIR/current" 2>/dev/null; then
    return 0
  fi
  rm -f "$tmp"
  ln -sfn "versions/$1" "$REBAR_DIR/current"
}

# EXIT trap. A version moved aside for replacement ($REPLACED) is deleted
# only once its successor is in place; if the swap never completed (mv
# failed, or a signal landed between the two moves) it goes back.
cleanup() {
  [[ -n "${LOCK:-}" ]] && rm -rf "$LOCK"
  if [[ -n "${REPLACED:-}" && -e "$REPLACED" && -n "${TARGET:-}" && ! -e "$TARGET" ]]; then
    if mv "$REPLACED" "$TARGET"; then
      warn "install did not complete; restored the previous $(basename "$TARGET")"
    else
      warn "install did not complete; the previous copy is at $REPLACED"
      REPLACED=""
    fi
  fi
  local d
  for d in "${TMP:-}" "${STAGE:-}" "${REPLACED:-}"; do
    [[ -n "$d" ]] && rm -rf "$d"
  done
  [[ -n "${REBAR_DIR:-}" ]] && rm -f "$REBAR_DIR/.current.$$"
  return 0
}

# Everything that acts lives in main, called on the last line, so a
# truncated download (`curl | bash` cut short) defines functions at most and
# runs nothing. The end-of-script marker must arrive intact as the last
# argument, so a cut inside the final line cannot run main either.
main() {
  REBAR_REPO="${REBAR_REPO:-https://github.com/willackerly/rebar.git}"
  REBAR_DIR="${REBAR_DIR:-}"
  ASK_SERVER="${ASK_SERVER:-}"
  VERSION="${REBAR_REF:-}"
  FROM_SOURCE=0
  SWITCH=1
  PRE=0
  GO_PKG="github.com/willackerly/rebar/cli/cmd"

  if [[ -t 1 ]]; then
    RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
    BLUE=$'\033[0;34m'; NC=$'\033[0m'
  else
    RED=''; GREEN=''; YELLOW=''; BLUE=''; NC=''
  fi

  COMPLETE=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --server)    [[ $# -ge 2 ]] || die "--server needs a value"; ASK_SERVER="$2"; shift 2 ;;
      --server=*)  ASK_SERVER="${1#--server=}"; shift ;;
      --dir)       [[ $# -ge 2 ]] || die "--dir needs a value"; REBAR_DIR="$2"; shift 2 ;;
      --dir=*)     REBAR_DIR="${1#--dir=}"; shift ;;
      --ref)       [[ $# -ge 2 ]] || die "--ref needs a value"; VERSION="$2"; shift 2 ;;
      --ref=*)     VERSION="${1#--ref=}"; shift ;;
      --source)    FROM_SOURCE=1; shift ;;
      --no-switch) SWITCH=0; shift ;;
      --pre)       PRE=1; shift ;;
      --rebar-end-of-script)
                   [[ $# -eq 1 ]] || die "--rebar-end-of-script must come last"; COMPLETE=1; shift ;;
      -h|--help)   usage; exit 0 ;;
      -*)          usage >&2; die "unknown option: $1" ;;
      *)           VERSION="$1"; shift ;;
    esac
  done
  # Run as `main "$@" --rebar-end-of-script` from the script's last line only.
  [[ "$COMPLETE" -eq 1 ]] || die "setup-rebar.sh is incomplete (download cut short?); not running it"
  if [[ -z "$REBAR_DIR" ]]; then
    [[ -n "${HOME:-}" ]] || die "HOME is not set; pass --dir PATH (or set REBAR_DIR)"
    REBAR_DIR="$HOME/.rebar"
  fi
  REPO_SHOWN="$(redact "$REBAR_REPO")"

  SLUG="$(github_slug)"
  if [[ -z "${REBAR_RELEASE_URL:-}" && -n "$SLUG" ]]; then
    REBAR_RELEASE_URL="https://github.com/${SLUG}/releases/download"
  fi
  REBAR_RELEASE_URL="${REBAR_RELEASE_URL:-}"
  if [[ -z "${REBAR_RELEASES_API:-}" && -n "$SLUG" ]]; then
    REBAR_RELEASES_API="https://api.github.com/repos/${SLUG}/releases?per_page=100"
  fi
  REBAR_RELEASES_API="${REBAR_RELEASES_API:-}"

  # Scratch space (downloads, the releases list); removed on exit.
  TMP="$(mktemp -d "${TMPDIR:-/tmp}/rebar-install.XXXXXX")"
  trap cleanup EXIT
  # Turn signals into a normal exit so the EXIT trap (and its restore) runs.
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM

  # ── Resolve what to install ──────────────────────────────────────────────────

  if [[ -z "$VERSION" || "$VERSION" == "latest" ]]; then
    resolve_latest
    KIND=tag
  else
    [[ "$PRE" -eq 1 ]] && warn "--pre only applies to \"latest\"; installing the version given ($VERSION)"
    # "3.0.0" → "v3.0.0" when only the v-prefixed tag exists.
    if [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9] ]] && have git \
       && git ls-remote --tags --refs -- "$REBAR_REPO" "refs/tags/v$VERSION" | grep -q .; then
      VERSION="v$VERSION"
    fi
    KIND="$(classify_ref "$VERSION")"
  fi

  # Directory name: tags as-is; branches and SHAs made path-safe
  # (feat/foo → feat-foo).
  NAME="$(printf '%s' "$VERSION" | sed 's/[^A-Za-z0-9._-]/-/g; s/^[.-]*//')"
  [[ -n "$NAME" && "$NAME" != "current" ]] || die "cannot use '$VERSION' as a version name"

  # ── Prepare $REBAR_DIR ───────────────────────────────────────────────────────

  LEGACY=0
  if [[ -e "$REBAR_DIR" && ! -d "$REBAR_DIR" ]]; then
    die "$REBAR_DIR exists and is not a directory. Move it aside or use --dir."
  fi
  if [[ -d "$REBAR_DIR" ]]; then
    # A pre-versioned install is a rebar clone: a git checkout with rebar's
    # markers. Any other git repo is foreign.
    if [[ -e "$REBAR_DIR/.git" && -d "$REBAR_DIR/templates/project-bootstrap" \
          && -f "$REBAR_DIR/scripts/steward.sh" ]]; then
      LEGACY=1
    elif [[ ! -d "$REBAR_DIR/versions" ]] && [[ -n "$(ls -A "$REBAR_DIR" 2>/dev/null)" ]]; then
      die "$REBAR_DIR exists but is neither a rebar install nor empty. Move it aside or use --dir."
    fi
    if [[ -e "$REBAR_DIR/current" && ! -L "$REBAR_DIR/current" ]]; then
      die "$REBAR_DIR/current exists and is not a symlink; refusing to replace it."
    fi
  fi
  mkdir -p "$REBAR_DIR/versions"
  REBAR_DIR="$(cd "$REBAR_DIR" && pwd)"

  if [[ "$LEGACY" -eq 1 ]]; then
    warn "$REBAR_DIR is a pre-versioned rebar clone. Leaving it as is:"
    warn "  versions/ and current/ go inside it; the clone itself is not updated."
    warn "  Projects whose .mcp.json or REBAR_ROOT point at $REBAR_DIR keep using the clone."
    if [[ -f "$REBAR_DIR/bin/install" ]]; then
      warn "  If your shell RC still puts $REBAR_DIR/bin on PATH, the clone's bin/install"
      warn "  shadows the system install(1); remove that line once nothing needs the clone."
    fi
    if have git; then
      exclude="$(git -C "$REBAR_DIR" rev-parse --git-path info/exclude 2>/dev/null || true)"
      if [[ -n "$exclude" ]]; then
        case "$exclude" in /*) ;; *) exclude="$REBAR_DIR/$exclude" ;; esac
        mkdir -p "$(dirname "$exclude")"
        for pat in /versions/ /current; do
          grep -qxF "$pat" "$exclude" 2>/dev/null || printf '%s\n' "$pat" >> "$exclude"
        done
      fi
    fi
  fi

  STAGE="$REBAR_DIR/versions/.incoming.$$"
  REPLACED="$REBAR_DIR/versions/.replaced.$$"
  TARGET="$REBAR_DIR/versions/$NAME"

  # One install per version directory at a time (mkdir is atomic). A lock
  # left by a killed install names its pid; remove it by hand.
  if ! mkdir "$REBAR_DIR/versions/.lock-$NAME" 2>/dev/null; then
    die "another install of $NAME is running (pid $(cat "$REBAR_DIR/versions/.lock-$NAME/pid" 2>/dev/null || echo '?')); if not, remove $REBAR_DIR/versions/.lock-$NAME"
  fi
  LOCK="$REBAR_DIR/versions/.lock-$NAME"
  echo $$ > "$LOCK/pid"

  # Branch names are made path-safe, so two refs can share a directory
  # (feat/x and feat-x both → versions/feat-x). Refuse to replace a
  # different ref's install.
  if [[ -f "$TARGET/.rebar-install" ]]; then
    held="$(sed -n 's/^version=//p' "$TARGET/.rebar-install")"
    if [[ -n "$held" && "$held" != "$VERSION" ]]; then
      die "versions/$NAME already holds '$held', not '$VERSION' (both map to that name). Remove $TARGET first, or use --dir for a separate install base."
    fi
  fi

  # ── Install ──────────────────────────────────────────────────────────────────

  METHOD=""
  COMMIT=""
  LAYOUT=""
  log "Installing rebar $VERSION → $REBAR_DIR/versions/$NAME"
  # A tag installs prebuilt; install_prebuilt returns 1 only for a tag that
  # predates prebuilt assets. Branches, commits and --source build here.
  if [[ "$KIND" == "tag" && "$FROM_SOURCE" -eq 0 ]] && install_prebuilt "$VERSION"; then
    :
  else
    install_source "$VERSION" "$KIND"
  fi

  [[ -x "$STAGE/bin/rebar" ]] || die "install produced no bin/rebar"
  "$STAGE/bin/rebar" version >/dev/null 2>&1 \
    || die "$STAGE/bin/rebar does not run on this machine"
  # A tree whose binary predates the embedded assets needs its scripts and
  # templates beside it.
  if [[ "$LAYOUT" == tree ]] && ! "$STAGE/bin/rebar" install-shims --dir "$TMP/shims-probe" >/dev/null 2>&1 \
     && [[ ! -d "$STAGE/templates/project-bootstrap" || ! -f "$STAGE/scripts/steward.sh" ]]; then
    warn "$VERSION lacks templates/project-bootstrap/ or scripts/steward.sh; the CLI may not find its scripts"
  fi

  # Versions before v3.1.2 ship the PATH helper as bin/install. current/bin
  # comes first on PATH, where that name shadows the system install(1)
  # (`install -d dir` → "Unknown option: -d", breaking `make install`), so
  # give it the current name.
  if [[ -f "$STAGE/bin/install" ]]; then
    if [[ -e "$STAGE/bin/rebar-install" ]]; then
      rm -f "$STAGE/bin/install"
    else
      mv "$STAGE/bin/install" "$STAGE/bin/rebar-install"
    fi
  fi

  cat > "$STAGE/.rebar-install" <<EOF
version=$VERSION
kind=$KIND
commit=$COMMIT
method=$METHOD
layout=$LAYOUT
repo=$REPO_SHOWN
installed=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

  if [[ -e "$TARGET" ]]; then
    log "Replacing the existing $NAME"
    mv "$TARGET" "$REPLACED"
  fi
  mv "$STAGE" "$TARGET"

  PREVIOUS=""
  if [[ -L "$REBAR_DIR/current" ]]; then
    PREVIOUS="$(readlink "$REBAR_DIR/current")"
  fi
  if [[ "$SWITCH" -eq 1 || -z "$PREVIOUS" ]]; then
    [[ "$SWITCH" -eq 1 ]] || warn "no current version yet; making $NAME current despite --no-switch"
    switch_current "$NAME"
    SWITCHED=1
  else
    SWITCHED=0
  fi

  INSTALLED_VERSION="$("$TARGET/bin/rebar" version 2>/dev/null | head -1)"

  # ── PATH wiring ──────────────────────────────────────────────────────────────
  # Run through the symlink so bin/rebar-install records $REBAR_DIR/current/bin
  # (it uses the logical `pwd`), which stays valid across version switches.

  if [[ "$SWITCHED" -eq 0 && -n "$ASK_SERVER" ]]; then
    warn "--server was not applied: --no-switch skips bin/rebar-install. Apply it with: $REBAR_DIR/current/bin/rebar-install --server $ASK_SERVER"
  elif [[ "$SWITCHED" -eq 1 && -z "${HOME:-}" ]]; then
    warn "HOME is not set, so PATH was not wired; add $REBAR_DIR/current/bin to PATH yourself"
  elif [[ "$SWITCHED" -eq 1 && -x "$REBAR_DIR/current/bin/rebar-install" ]]; then
    install_args=()
    [[ -n "$ASK_SERVER" ]] && install_args+=(--server "$ASK_SERVER")
    log "Running $REBAR_DIR/current/bin/rebar-install ${install_args[*]:-}"
    # ${arr[@]+...}: bash 3.2 treats an empty "${arr[@]}" as unbound under set -u.
    # bin/rebar-install picks the RC file from $SHELL, which `env -i` and some CI
    # shells leave unset.
    SHELL="${SHELL:-/bin/sh}" "$REBAR_DIR/current/bin/rebar-install" ${install_args[@]+"${install_args[@]}"}
  fi

  # ── Report ───────────────────────────────────────────────────────────────────

  ok "Installed $INSTALLED_VERSION at $TARGET ($METHOD)"
  if [[ "$SWITCHED" -eq 1 ]]; then
    if [[ -n "$PREVIOUS" && "$PREVIOUS" != "versions/$NAME" ]]; then
      ok "current: ${PREVIOUS#versions/} → $NAME"
    else
      ok "current: $NAME"
    fi
  else
    ok "current stays at ${PREVIOUS#versions/} (--no-switch). Switch with: ln -sfn versions/$NAME $REBAR_DIR/current"
  fi
  log "Installed versions: $(cd "$REBAR_DIR/versions" && ls -1 | tr '\n' ' ')"

  on_path="$(command -v rebar 2>/dev/null || true)"
  if [[ -n "$on_path" && "$on_path" != "$REBAR_DIR/current/bin/rebar" ]]; then
    warn "this shell runs rebar from $on_path; open a new terminal (or source your RC) to pick up $REBAR_DIR/current/bin"
  fi
  if [[ -n "${REBAR_ROOT:-}" ]]; then
    warn "REBAR_ROOT=$REBAR_ROOT is set; REBAR_CHECK_IMPL=shell and \`rebar run\` of scripts without a built-in use that tree"
  fi

  cat <<EOF

${BLUE}Next steps:${NC}
  • Open a new terminal (or source your shell RC) so PATH picks up rebar
  • ${GREEN}rebar new my-project -d "what it does"${NC}   # new project
  • ${GREEN}rebar adopt${NC}                              # adopt rebar in an existing repo

${BLUE}Versions:${NC}
  • Install another side by side: ${GREEN}setup-rebar.sh <version> [--no-switch]${NC}
  • Remove one: rm -rf $REBAR_DIR/versions/<version>

${BLUE}Docs:${NC} https://github.com/${SLUG:-willackerly/rebar}/blob/${COMMIT:-$VERSION}/QUICKSTART.md
EOF
}

main "$@" --rebar-end-of-script
