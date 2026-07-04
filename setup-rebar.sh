#!/usr/bin/env bash
# setup-rebar.sh — One-line installer for rebar.
#
# Clones rebar to $REBAR_DIR (default ~/.rebar), runs the canonical
# `bin/install` to add ASK + rebar to your PATH, and exits pointing
# you at `rebar new` / `rebar adopt` for actual project bootstrap.
#
# This is intentionally a thin shim over rebar's existing tooling
# (`bin/install`, `rebar new`, `rebar adopt`) — not a parallel
# bootstrap. Anything beyond clone + PATH wiring belongs in the
# Go CLI so it stays in sync with the rest of the project.
#
# Usage (curl pipe):
#   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.0.0-beta/setup-rebar.sh | bash
#
# Usage (local):
#   ./setup-rebar.sh [--server HOST:PORT] [--dir PATH]

set -euo pipefail

REBAR_REPO="${REBAR_REPO:-https://github.com/willackerly/rebar.git}"
REBAR_REF="${REBAR_REF:-v3.0.0-beta}"
REBAR_DIR="${REBAR_DIR:-$HOME/.rebar}"
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
rebar installer

Clones rebar to \$REBAR_DIR (default ~/.rebar), then runs bin/install
to add the ASK and rebar CLIs to your PATH. Project bootstrap (creating
or adopting a project) is left to the rebar CLI itself — run
\`rebar new\` or \`rebar adopt\` after this completes.

Usage:
  $0 [--server HOST:PORT] [--dir PATH] [--ref REF]
  $0 --help

Env vars (override flags):
  REBAR_DIR    target install dir            (default: ~/.rebar)
  REBAR_REPO   git remote to clone from      (default: github.com/willackerly/rebar)
  REBAR_REF    branch/tag to check out       (default: v3.0.0-beta)
  ASK_SERVER   remote ASK server, written to your shell RC by bin/install

Next steps after install:
  rebar new my-project -d "what it does"   # create a new rebar project
  rebar adopt                              # adopt rebar in an existing repo
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)   ASK_SERVER="$2"; shift 2 ;;
    --server=*) ASK_SERVER="${1#--server=}"; shift ;;
    --dir)      REBAR_DIR="$2"; shift 2 ;;
    --dir=*)    REBAR_DIR="${1#--dir=}"; shift ;;
    --ref)      REBAR_REF="$2"; shift 2 ;;
    --ref=*)    REBAR_REF="${1#--ref=}"; shift ;;
    -h|--help)  usage; exit 0 ;;
    *)          err "unknown option: $1"; usage >&2; exit 2 ;;
  esac
done

if ! command -v git >/dev/null 2>&1; then
  err "git is required. Install it and re-run."
  exit 1
fi

if [[ -d "$REBAR_DIR/.git" ]]; then
  log "Updating existing rebar checkout at $REBAR_DIR"
  if ! git -C "$REBAR_DIR" diff --quiet || ! git -C "$REBAR_DIR" diff --cached --quiet; then
    warn "$REBAR_DIR has uncommitted changes — skipping fetch/checkout"
  else
    git -C "$REBAR_DIR" fetch --tags origin "$REBAR_REF"
    git -C "$REBAR_DIR" checkout "$REBAR_REF"
    git -C "$REBAR_DIR" pull --ff-only origin "$REBAR_REF" || true
  fi
elif [[ -e "$REBAR_DIR" ]]; then
  err "$REBAR_DIR exists but is not a git checkout."
  err "Move it aside or set REBAR_DIR=<other path> and re-run."
  exit 1
else
  log "Cloning $REBAR_REPO ($REBAR_REF) → $REBAR_DIR"
  git clone --branch "$REBAR_REF" --depth=1 "$REBAR_REPO" "$REBAR_DIR"
fi

if [[ ! -x "$REBAR_DIR/bin/install" ]]; then
  err "$REBAR_DIR/bin/install not found or not executable."
  err "The clone may have failed or the ref does not contain bin/install."
  exit 1
fi

install_args=()
[[ -n "$ASK_SERVER" ]] && install_args+=(--server "$ASK_SERVER")
log "Running $REBAR_DIR/bin/install ${install_args[*]:-}"
"$REBAR_DIR/bin/install" "${install_args[@]}"

ok "rebar installed at $REBAR_DIR (ref: $REBAR_REF)"
cat <<EOF

${BLUE}Next steps:${NC}
  • Open a new terminal (or source your shell RC) so PATH picks up rebar
  • ${GREEN}rebar new my-project -d "what it does"${NC}   # new project
  • ${GREEN}rebar adopt${NC}                              # adopt rebar in an existing repo

${BLUE}Docs:${NC}
  • https://github.com/willackerly/rebar/blob/$REBAR_REF/QUICKSTART.md
  • https://github.com/willackerly/rebar/blob/$REBAR_REF/SETUP.md
EOF
