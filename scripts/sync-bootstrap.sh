#!/usr/bin/env bash
# sync-bootstrap.sh — Mirror /scripts/ into templates/project-bootstrap/scripts/.
#
# `templates/project-bootstrap/scripts/` exists so adopters can do
# `cp -r templates/project-bootstrap/* ../my-project/` and end up with a
# working project in one command. It is a literal copy of /scripts/, kept
# in sync mechanically by this script + check-bootstrap-sync.sh.
#
# Usage:
#   ./scripts/sync-bootstrap.sh             # copy /scripts/ → templates/project-bootstrap/scripts/
#   ./scripts/sync-bootstrap.sh --check     # exit 0 if synced, 1 if drifted (used by ci-check)
#
# When you edit anything under /scripts/, run this to propagate. The
# pre-commit hook + ci-check enforces that the templates copy is current.
#
# Bash 3.2 compatible (macOS default).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SRC="$PROJECT_ROOT/scripts"
DST="$PROJECT_ROOT/templates/project-bootstrap/scripts"

MODE="sync"
case "${1:-}" in
  --check) MODE="check" ;;
  -h|--help)
    sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  "") ;;
  *) echo "sync-bootstrap: unknown arg '$1'" >&2; exit 2 ;;
esac

if [ ! -d "$SRC" ]; then
  echo "sync-bootstrap: $SRC missing" >&2
  exit 2
fi

# This script is only meaningful inside the rebar source repo, which has a
# `templates/project-bootstrap/` to mirror /scripts/ into. Adopting projects
# don't have that directory and shouldn't fail their ci-check on its absence.
# Skip silently when run outside the rebar source repo.
if [ ! -d "$PROJECT_ROOT/templates/project-bootstrap" ]; then
  if [ "$MODE" = "check" ]; then
    echo "check-bootstrap-sync: SKIP — not in rebar source repo (no templates/project-bootstrap/)"
  else
    echo "sync-bootstrap: SKIP — not in rebar source repo (no templates/project-bootstrap/)"
  fi
  exit 0
fi

mkdir -p "$DST"

# Files we don't sync into the bootstrap copy:
#   README.md                — adopter copy may diverge with project-specific notes
#   sync-bootstrap.sh        — only meaningful in the rebar source repo itself
#   check-bootstrap-sync.sh  — same
#   test-e2e-live.sh         — maintainer-facing; assumes the rebar dev layout
#                              (~/dev/<adopted-repos>) that's only true for the
#                              rebar maintainer
skip_file() {
  case "$1" in
    README.md|sync-bootstrap.sh|check-bootstrap-sync.sh|test-e2e-live.sh) return 0 ;;
  esac
  return 1
}

if [ "$MODE" = "check" ]; then
  drifted=0
  drift_files=""
  for src_file in "$SRC"/*; do
    [ -f "$src_file" ] || continue
    base="$(basename "$src_file")"
    if skip_file "$base"; then continue; fi
    dst_file="$DST/$base"
    if [ ! -f "$dst_file" ] || ! diff -q "$src_file" "$dst_file" >/dev/null 2>&1; then
      drifted=$((drifted + 1))
      drift_files="$drift_files\n  - $base"
    fi
  done
  # Also flag templates files that no longer exist in /scripts/ (orphans).
  for dst_file in "$DST"/*; do
    [ -f "$dst_file" ] || continue
    base="$(basename "$dst_file")"
    if skip_file "$base"; then continue; fi
    src_file="$SRC/$base"
    if [ ! -f "$src_file" ]; then
      drifted=$((drifted + 1))
      drift_files="$drift_files\n  - $base (orphan in templates/, not in /scripts/)"
    fi
  done
  if [ "$drifted" -eq 0 ]; then
    echo "check-bootstrap-sync: OK — templates/project-bootstrap/scripts/ matches /scripts/"
    exit 0
  fi
  printf 'check-bootstrap-sync: %s file(s) drifted between /scripts/ and templates/project-bootstrap/scripts/:%b\n' "$drifted" "$drift_files"
  echo ""
  echo "Run: ./scripts/sync-bootstrap.sh"
  exit 1
fi

# Sync mode.
copied=0
for src_file in "$SRC"/*; do
  [ -f "$src_file" ] || continue
  base="$(basename "$src_file")"
  if skip_file "$base"; then continue; fi
  dst_file="$DST/$base"
  if [ ! -f "$dst_file" ] || ! diff -q "$src_file" "$dst_file" >/dev/null 2>&1; then
    cp "$src_file" "$dst_file"
    chmod --reference="$src_file" "$dst_file" 2>/dev/null || chmod +x "$dst_file" 2>/dev/null || true
    echo "  synced $base"
    copied=$((copied + 1))
  fi
done

# Remove orphans (files that exist in templates but not in /scripts/).
for dst_file in "$DST"/*; do
  [ -f "$dst_file" ] || continue
  base="$(basename "$dst_file")"
  if skip_file "$base"; then continue; fi
  src_file="$SRC/$base"
  if [ ! -f "$src_file" ]; then
    rm "$dst_file"
    echo "  removed $base (orphan)"
    copied=$((copied + 1))
  fi
done

echo "sync-bootstrap: $copied file(s) updated"
