#!/usr/bin/env bash
# setup-rebar.sh — One-line installer for rebar (versioned).
#
# Installs rebar to ~/.rebar/versions/<version>/, creates a 'current'
# symlink for PATH wiring, runs bin/install to add ASK + rebar CLIs.
#
# For release tags (vX.Y.Z): downloads prebuilt binaries from GitHub Releases
# For branches: clones and builds from source
#
# Multiple rebar versions can coexist. Projects pin their version via
# .rebar-version; the CLI auto-resolves the correct framework install.
#
# Usage (curl pipe):
#   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.0.1-alpha/setup-rebar.sh | bash -s -- v3.0.1-alpha
#
# Usage (local):
#   ./setup-rebar.sh [version] [--server HOST:PORT]

set -euo pipefail

REBAR_REPO="${REBAR_REPO:-https://github.com/willackerly/rebar.git}"
REBAR_GITHUB="${REBAR_GITHUB:-ttschampel/rebar}"
REBAR_REF="${1:-v3.0.1-alpha}"
REBAR_BASE="${REBAR_BASE:-$HOME/.rebar}"
ASK_SERVER="${ASK_SERVER:-}"

if [[ -t 1 ]]; then
  RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
  BLUE=$'\033[0;34m'; NC=$'\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; BLUE=''; NC=''
fi

log()  { printf '%s[rebar]%s %s\n' "$BLUE"   "$NC" "$*"; }
warn() { printf '%s[rebar]%s %s\n' "$YELLOW" "$NC" "$*" >&2; }
err()  { printf '%s[rebar]%s %s\n' "$RED"    "$NC" "$*" >&2; }
ok()   { printf '%s[rebar]%s %s\n' "$GREEN"  "$NC" "$*"; }

usage() {
  cat <<EOF
rebar installer (versioned)

Installs rebar to ~/.rebar/versions/<version>/, updates the 'current'
symlink for PATH, runs bin/install to wire ASK + rebar CLIs.

Multiple versions can coexist. Projects pin via .rebar-version.

Usage:
  $0 <version> [--server HOST:PORT]
  $0 --help

Examples:
  curl ... | bash -s -- v3.1.0
  ./setup-rebar.sh v3.0.1-alpha --server localhost:8080

Env vars:
  REBAR_BASE   install base dir         (default: ~/.rebar)
  REBAR_REPO   git remote               (default: github.com/willackerly/rebar)
  ASK_SERVER   remote ASK server

Next steps:
  rebar new my-project -d "what it does"
  rebar adopt
EOF
}

# First positional arg is version (already captured as $1 → REBAR_REF)
# Remaining args are flags
shift || true  # consume version arg
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)   ASK_SERVER="$2"; shift 2 ;;
    --server=*) ASK_SERVER="${1#--server=}"; shift ;;
    -h|--help)  usage; exit 0 ;;
    *)          err "unknown option: $1"; usage >&2; exit 2 ;;
  esac
done

if ! command -v git >/dev/null 2>&1; then
  err "git is required. Install it and re-run."
  exit 1
fi

# Detect platform for binary downloads
detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os="Darwin" ;;
    Linux)  os="Linux" ;;
    MINGW*|MSYS*|CYGWIN*) os="Windows" ;;
    *) err "Unsupported OS: $(uname -s)"; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch="x86_64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) err "Unsupported arch: $(uname -m)"; exit 1 ;;
  esac
  echo "${os}_${arch}"
}

# Download and extract release binary
download_release() {
  local version="$1"
  local platform
  platform=$(detect_platform)
  local archive="rebar_${version}_${platform}.tar.gz"
  local url="https://github.com/${REBAR_GITHUB}/releases/download/${version}/${archive}"

  log "Downloading release ${version} for ${platform}"
  local tmpdir
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$tmpdir/$archive" || {
      err "Failed to download $url"
      err "Release may not exist. Use a branch name to build from source."
      exit 1
    }
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$tmpdir/$archive" || {
      err "Failed to download $url"
      err "Release may not exist. Use a branch name to build from source."
      exit 1
    }
  else
    err "curl or wget required for release downloads"
    exit 1
  fi

  log "Extracting to $REBAR_DIR"
  mkdir -p "$REBAR_DIR"
  tar -xzf "$tmpdir/$archive" -C "$REBAR_DIR"
  chmod +x "$REBAR_DIR/rebar"

  # Move binary to bin/ and create structure
  mkdir -p "$REBAR_DIR/bin"
  mv "$REBAR_DIR/rebar" "$REBAR_DIR/bin/rebar"
}

# Clone and build from source
install_from_source() {
  local ref="$1"
  if [[ -d "$REBAR_DIR/.git" ]]; then
    log "Updating $ref at $REBAR_DIR"
    if ! git -C "$REBAR_DIR" diff --quiet || ! git -C "$REBAR_DIR" diff --cached --quiet; then
      warn "Uncommitted changes in $REBAR_DIR — skipping update"
    else
      git -C "$REBAR_DIR" fetch --tags origin "$ref"
      git -C "$REBAR_DIR" checkout "$ref"
      git -C "$REBAR_DIR" pull --ff-only origin "$ref" || true
    fi
  elif [[ -e "$REBAR_DIR" ]]; then
    err "$REBAR_DIR exists but is not a git checkout."
    err "Remove it or choose a different version."
    exit 1
  else
    log "Installing $REBAR_REPO ($ref) → $REBAR_DIR"
    git clone --branch "$ref" --depth=1 "$REBAR_REPO" "$REBAR_DIR"
  fi

  # Build the binary
  if ! command -v go >/dev/null 2>&1; then
    err "go is required to build from source. Install it or use a release tag."
    exit 1
  fi

  log "Building rebar CLI from source"
  (cd "$REBAR_DIR/cli" && go build -ldflags "-X github.com/willackerly/rebar/cli/cmd.Version=$ref" -o ../bin/rebar)
}

# Versioned install directory
REBAR_DIR="$REBAR_BASE/versions/$REBAR_REF"
mkdir -p "$REBAR_BASE/versions"

# Detect if this is a release tag or branch
if [[ "$REBAR_REF" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
  # Release tag: try to download prebuilt binary
  log "Detected release tag: $REBAR_REF"
  if [[ -e "$REBAR_DIR/bin/rebar" ]]; then
    log "$REBAR_REF already installed at $REBAR_DIR"
  else
    download_release "$REBAR_REF"
  fi
else
  # Branch or non-standard ref: build from source
  log "Detected branch/ref: $REBAR_REF (will build from source)"
  install_from_source "$REBAR_REF"
fi

# Update 'current' symlink for PATH
ln -sf "versions/$REBAR_REF" "$REBAR_BASE/current"
log "Updated ~/.rebar/current → $REBAR_REF"

if [[ ! -x "$REBAR_DIR/bin/install" ]]; then
  err "$REBAR_DIR/bin/install not found."
  exit 1
fi

install_args=()
[[ -n "$ASK_SERVER" ]] && install_args+=(--server "$ASK_SERVER")
log "Running $REBAR_DIR/bin/install ${install_args[*]:-}"
"$REBAR_DIR/bin/install" "${install_args[@]}"

ok "rebar $REBAR_REF installed"
cat <<EOF

${BLUE}Installed to:${NC} $REBAR_DIR
${BLUE}PATH symlink:${NC} ~/.rebar/current → versions/$REBAR_REF

${BLUE}Next steps:${NC}
  • Open new terminal (or source shell RC)
  • ${GREEN}rebar new my-project -d "what it does"${NC}  (new project)
  • ${GREEN}rebar adopt${NC}  (add rebar to existing project)

${BLUE}Multiple versions:${NC}
  Install another: curl ... | bash -s -- v3.2.0
  List installed: ls ~/.rebar/versions/
  Upgrade project: rebar upgrade <version>
EOF
