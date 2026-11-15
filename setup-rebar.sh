#!/usr/bin/env bash
# setup-rebar.sh — versioned one-line installer for rebar.
#
# Installs a rebar version side by side with others:
#
#   ~/.rebar/versions/<version>/   one complete rebar tree per version
#                                  (scripts/, templates/, agents/, bin/rebar, bin/ask*, …)
#   ~/.rebar/current               symlink to the active version
#
# and puts ~/.rebar/current/bin on PATH (via that version's bin/install), so
# switching versions never touches your shell RC again.
#
#   Release tag   download rebar_<ver>_<os>_<arch>.tar.gz from the GitHub
#                 Release, verify it against the release's SHA-256 checksums,
#                 unpack. No git or Go needed. If the release has no assets
#                 (true of every release before the release workflow), build
#                 from source instead.
#   Branch / SHA  build from source: git + Go.
#
# A checksum mismatch is fatal — never silently replaced by a source build.
#
# Usage (curl pipe):
#   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/main/setup-rebar.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/main/setup-rebar.sh | bash -s -- v3.0.0-beta.4
#
# Usage (local):
#   ./setup-rebar.sh [<version>|latest|<branch>|<sha>] [--source] [--no-switch]
#                    [--server HOST:PORT] [--dir PATH]
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
  $0 [<version>] [--source] [--no-switch] [--server HOST:PORT] [--dir PATH]
  $0 --help

  <version>     a release tag (v3.0.0-beta.4), "latest" (default: the highest
                release tag, pre-releases included), a branch, or a commit SHA
  --ref REF     same as <version> (kept for older docs)
  --source      build from source even if a prebuilt release exists
  --no-switch   install side by side but leave ~/.rebar/current where it is
  --server H:P  remote ASK server, written to your shell RC by bin/install
  --dir PATH    install base (default: ~/.rebar)

Layout:
  \$REBAR_DIR/versions/<version>/   one complete rebar tree per version
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

fetch() {  # fetch <url> <out>
  if have curl; then
    curl -fsSL --retry 2 -o "$2" "$1" 2>/dev/null
  elif have wget; then
    wget -q -O "$2" "$1"
  else
    return 1
  fi
}

sha256_of() {
  if have sha256sum; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# Highest semver tag on stdin (one per line). Pre-releases count and rank
# below their release (v3.0.0-beta.10 > v3.0.0-beta.2, v3.0.0 > v3.0.0-rc.1).
# Lines that are not vMAJOR.MINOR.PATCH[-PRE] are ignored.
semver_max() {
  awk '
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
      if (best == "" || cmp($0, best) > 0) best = $0
    }
    END { if (best != "") print best }
  '
}

list_tags() {
  if have git; then
    git ls-remote --tags --refs "$REBAR_REPO" 'v*' | sed 's#.*refs/tags/##'
  elif [[ -n "$SLUG" ]] && have curl; then
    # GitHub's /releases/latest skips pre-releases (all of rebar's, so far)
    # and /releases misses tags that never got a Release; the tags list has
    # every tag. 100 is the page cap — plenty for rebar's history.
    curl -fsSL "https://api.github.com/repos/${SLUG}/tags?per_page=100" \
      | sed -n 's/.*"name": *"\([^"]*\)".*/\1/p'
  else
    die "need git (or curl and a github.com REBAR_REPO) to resolve \"latest\"; pass a version"
  fi
}

# Prints tag | branch | commit for a ref, or dies.
classify_ref() {
  local ref="$1" out re='^[0-9a-f]{7,40}$' semver='^v[0-9]+\.[0-9]+\.[0-9]+'
  if ! have git; then
    [[ "$ref" =~ $semver ]] || die "git is required to install a branch or commit ($ref)"
    echo tag; return 0
  fi
  out="$(git ls-remote "$REBAR_REPO" "refs/tags/$ref" "refs/heads/$ref")" \
    || die "cannot reach $REBAR_REPO"
  if printf '%s\n' "$out" | awk -v r="refs/tags/$ref" '$2 == r {f=1} END {exit !f}'; then
    echo tag
  elif printf '%s\n' "$out" | awk -v r="refs/heads/$ref" '$2 == r {f=1} END {exit !f}'; then
    echo branch
  elif [[ "$ref" =~ $re ]]; then
    echo commit
  else
    die "no tag or branch named '$ref' in $REBAR_REPO"
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

# Returns 1 when there is nothing to download (no assets, unsupported
# platform, no hash tool) so the caller can build from source. Dies on a
# verification failure.
install_prebuilt() {
  local tag="$1" plat os arch vn asset sums expected actual
  [[ -n "$REBAR_RELEASE_URL" ]] || return 1
  plat="$(detect_platform)"
  if [[ -z "$plat" ]]; then
    warn "no prebuilt rebar for $(uname -s)/$(uname -m)"
    return 1
  fi
  os="${plat% *}"; arch="${plat#* }"
  if ! have sha256sum && ! have shasum; then
    warn "neither sha256sum nor shasum found; not installing an unverified download"
    return 1
  fi
  vn="${tag#v}"
  asset="rebar_${vn}_${os}_${arch}.tar.gz"
  sums="rebar_${vn}_checksums.txt"

  log "Looking for a prebuilt $tag ($os/$arch)…"
  if ! fetch "$REBAR_RELEASE_URL/$tag/$sums" "$TMP/$sums"; then
    log "Release $tag has no prebuilt assets."
    return 1
  fi
  expected="$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$TMP/$sums")"
  if [[ -z "$expected" ]]; then
    log "Release $tag has no $os/$arch build."
    return 1
  fi
  fetch "$REBAR_RELEASE_URL/$tag/$asset" "$TMP/$asset" \
    || die "$asset is listed in $sums but could not be downloaded. Retry, or use --source."
  actual="$(sha256_of "$TMP/$asset")"
  if [[ "$actual" != "$expected" ]]; then
    die "checksum mismatch for $asset: expected $expected, got $actual. Refusing to install."
  fi
  log "Checksum OK ($expected)"

  # Called from an `if`, where set -e is off: check each step.
  mkdir -p "$TMP/unpack" || die "cannot create $TMP/unpack"
  tar -xzf "$TMP/$asset" -C "$TMP/unpack" || die "cannot unpack $asset"
  [[ -d "$TMP/unpack/${asset%.tar.gz}" ]] || die "$asset does not contain ${asset%.tar.gz}/"
  mv "$TMP/unpack/${asset%.tar.gz}" "$STAGE" || die "cannot move the unpacked tree to $STAGE"
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

  log "Fetching ${ref} from ${REBAR_REPO}…"
  git init -q "$TMP/src"
  git -C "$TMP/src" fetch -q --depth 1 "$REBAR_REPO" "$want" \
    || die "could not fetch $ref from $REBAR_REPO"
  COMMIT="$(git -C "$TMP/src" rev-parse 'FETCH_HEAD^{commit}')"

  # Same tree a release tarball carries: the committed files, no .git.
  mkdir -p "$STAGE"
  git -C "$TMP/src" archive --format=tar "$COMMIT" | tar -x -C "$STAGE"
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
  METHOD="source ($REBAR_REPO@${COMMIT:0:12})"
  return 0
}

# EXIT trap. A version moved aside for replacement ($REPLACED) is deleted
# only once its successor is in place; if the swap never completed (mv
# failed, or a signal landed between the two moves) it goes back.
cleanup() {
  if [[ -n "${REPLACED:-}" && -e "$REPLACED" && -n "${TARGET:-}" && ! -e "$TARGET" ]]; then
    if mv "$REPLACED" "$TARGET"; then
      warn "install did not complete; restored the previous $(basename "$TARGET")"
    else
      warn "install did not complete; the previous copy is at $REPLACED"
      REPLACED=""
    fi
  fi
  rm -rf "${TMP:-}" "${STAGE:-}" "${REPLACED:-}"
}

# Everything that acts lives in main, called on the last line, so a
# truncated download (`curl | bash` cut short) defines functions at most and
# runs nothing. The end-of-script marker must arrive intact as the last
# argument, so a cut inside the final line cannot run main either.
main() {
  REBAR_REPO="${REBAR_REPO:-https://github.com/willackerly/rebar.git}"
  REBAR_DIR="${REBAR_DIR:-$HOME/.rebar}"
  ASK_SERVER="${ASK_SERVER:-}"
  VERSION="${REBAR_REF:-}"
  FROM_SOURCE=0
  SWITCH=1
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
      --rebar-end-of-script)
                   [[ $# -eq 1 ]] || die "--rebar-end-of-script must come last"; COMPLETE=1; shift ;;
      -h|--help)   usage; exit 0 ;;
      -*)          usage >&2; die "unknown option: $1" ;;
      *)           VERSION="$1"; shift ;;
    esac
  done
  # Run as `main "$@" --rebar-end-of-script` from the script's last line only.
  [[ "$COMPLETE" -eq 1 ]] || die "setup-rebar.sh is incomplete (download cut short?); not running it"

  SLUG="$(github_slug)"
  if [[ -z "${REBAR_RELEASE_URL:-}" && -n "$SLUG" ]]; then
    REBAR_RELEASE_URL="https://github.com/${SLUG}/releases/download"
  fi
  REBAR_RELEASE_URL="${REBAR_RELEASE_URL:-}"

  # ── Resolve what to install ──────────────────────────────────────────────────

  if [[ -z "$VERSION" || "$VERSION" == "latest" ]]; then
    log "Resolving the latest rebar release (pre-releases included)…"
    VERSION="$(list_tags | semver_max)" || die "could not list the tags of $REBAR_REPO"
    [[ -n "$VERSION" ]] || die "found no release tags in $REBAR_REPO; pass a version explicitly"
    log "Latest: $VERSION"
    KIND=tag
  else
    # "3.0.0" → "v3.0.0" when only the v-prefixed tag exists.
    if [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9] ]] && have git \
       && git ls-remote --tags --refs "$REBAR_REPO" "refs/tags/v$VERSION" | grep -q .; then
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
    if [[ -e "$REBAR_DIR/.git" ]]; then
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

  TMP="$(mktemp -d "${TMPDIR:-/tmp}/rebar-install.XXXXXX")"
  STAGE="$REBAR_DIR/versions/.incoming.$$"
  REPLACED="$REBAR_DIR/versions/.replaced.$$"
  TARGET="$REBAR_DIR/versions/$NAME"
  trap cleanup EXIT
  # Turn signals into a normal exit so the EXIT trap (and its restore) runs.
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM

  # ── Install ──────────────────────────────────────────────────────────────────

  METHOD=""
  COMMIT=""
  log "Installing rebar $VERSION → $REBAR_DIR/versions/$NAME"
  if [[ "$KIND" == "tag" && "$FROM_SOURCE" -eq 0 ]] && install_prebuilt "$VERSION"; then
    :
  else
    if [[ "$KIND" == "tag" && "$FROM_SOURCE" -eq 0 ]]; then
      log "Falling back to a source build."
    fi
    install_source "$VERSION" "$KIND"
  fi

  [[ -x "$STAGE/bin/rebar" ]] || die "install produced no bin/rebar"
  "$STAGE/bin/rebar" version >/dev/null 2>&1 \
    || die "$STAGE/bin/rebar does not run on this machine"
  if [[ ! -d "$STAGE/templates/project-bootstrap" || ! -f "$STAGE/scripts/steward.sh" ]]; then
    warn "$VERSION lacks templates/project-bootstrap/ or scripts/steward.sh; the CLI may not find its scripts"
  fi

  cat > "$STAGE/.rebar-install" <<EOF
version=$VERSION
kind=$KIND
commit=$COMMIT
method=$METHOD
repo=$REBAR_REPO
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
    ln -sfn "versions/$NAME" "$REBAR_DIR/current"
    SWITCHED=1
  else
    SWITCHED=0
  fi

  INSTALLED_VERSION="$("$TARGET/bin/rebar" version 2>/dev/null | head -1)"

  # ── PATH wiring ──────────────────────────────────────────────────────────────
  # Run through the symlink so bin/install records $REBAR_DIR/current/bin (it
  # uses the logical `pwd`), which stays valid across version switches.

  if [[ "$SWITCHED" -eq 1 && -x "$REBAR_DIR/current/bin/install" ]]; then
    install_args=()
    [[ -n "$ASK_SERVER" ]] && install_args+=(--server "$ASK_SERVER")
    log "Running $REBAR_DIR/current/bin/install ${install_args[*]:-}"
    # ${arr[@]+...}: bash 3.2 treats an empty "${arr[@]}" as unbound under set -u.
    # bin/install picks the RC file from $SHELL, which `env -i` and some CI
    # shells leave unset.
    SHELL="${SHELL:-/bin/sh}" "$REBAR_DIR/current/bin/install" ${install_args[@]+"${install_args[@]}"}
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
    warn "REBAR_ROOT=$REBAR_ROOT is set; the CLI runs that tree's scripts, not $REBAR_DIR/current's"
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
