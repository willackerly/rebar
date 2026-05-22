#!/usr/bin/env bash
# setup-rebar.sh — One-line installer for rebar (versioned).
#
# Installs rebar to ~/.rebar/versions/<version>/, creates a 'current'
# symlink for PATH wiring, runs bin/install to add ASK + rebar CLIs.
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

# Versioned install directory
REBAR_DIR="$REBAR_BASE/versions/$REBAR_REF"
mkdir -p "$REBAR_BASE/versions"

if [[ -d "$REBAR_DIR/.git" ]]; then
  log "Updating $REBAR_REF at $REBAR_DIR"
  if ! git -C "$REBAR_DIR" diff --quiet || ! git -C "$REBAR_DIR" diff --cached --quiet; then
    warn "Uncommitted changes in $REBAR_DIR — skipping update"
  else
    git -C "$REBAR_DIR" fetch --tags origin "$REBAR_REF"
    git -C "$REBAR_DIR" checkout "$REBAR_REF"
    git -C "$REBAR_DIR" pull --ff-only origin "$REBAR_REF" || true
  fi
elif [[ -e "$REBAR_DIR" ]]; then
  err "$REBAR_DIR exists but is not a git checkout."
  err "Remove it or choose a different version."
  exit 1
else
  log "Installing $REBAR_REPO ($REBAR_REF) → $REBAR_DIR"
  git clone --branch "$REBAR_REF" --depth=1 "$REBAR_REPO" "$REBAR_DIR"
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
  • ${GREEN}rebar new my-project -d "what it does"${NC}
  • ${GREEN}rebar adopt${NC}

${BLUE}Multiple versions:${NC}
  Install another: curl ... | bash -s -- v3.2.0
  List installed: ls ~/.rebar/versions/
  Upgrade project: rebar upgrade <version>
EOF
