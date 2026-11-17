#!/usr/bin/env bash
# sync-bootstrap.sh — Keep templates/project-bootstrap/scripts/ thin.
#
# Adopters bootstrap with `cp -r templates/project-bootstrap/. ../my-project/`.
# Every check runs from the rebar install (`rebar check`, `rebar run`), so
# the template's scripts/ holds only entry points that hand off to the CLI.
# This script keeps it that way:
#   - only the files in THIN_FILES exist there
#   - each .sh is marked `# rebar-thin-wrapper` and execs `rebar`
# It also keeps the template's architecture/CONTRACT-TEMPLATE.md identical
# to the root one (`cp -r` copies the template's), and the rebar binary's
# embedded copy (cli/internal/assets/files/: go:embed cannot reach outside
# cli/) identical to its sources, EMBED_SOURCES: the template, which `rebar
# new / init / adopt / migrate` write out, and the bin/ companions `rebar
# install-shims` writes beside the binary.
#
# (Until v3.0.0-beta.4 the template was a mirror of /scripts/ and this script
# copied files into it.)
#
# Usage:
#   ./scripts/sync-bootstrap.sh           # remove anything else from the template
#   ./scripts/sync-bootstrap.sh --check   # exit 0 if thin, 1 if not (used by ci-check)
#
# Bash 3.2 compatible (macOS default).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="${REBAR_PROJECT_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
DST="$PROJECT_ROOT/templates/project-bootstrap/scripts"

THIN_FILES="README.md ci-check.sh pre-commit.sh"

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

# Only meaningful inside the rebar source repo. Adopting projects don't have
# templates/project-bootstrap/ and shouldn't fail their ci-check on it.
if [ ! -d "$PROJECT_ROOT/templates/project-bootstrap" ]; then
  echo "check-bootstrap-sync: SKIP — not in rebar source repo (no templates/project-bootstrap/)"
  exit 0
fi

is_thin_file() {
  case " $THIN_FILES " in
    *" $1 "*) return 0 ;;
  esac
  return 1
}

problems=0
report=""
add_problem() {
  problems=$((problems + 1))
  report="$report\n  - $1"
}

# Anything not on the list is rebar logic that belongs in the install.
for path in "$DST"/* "$DST"/.[!.]*; do
  [ -e "$path" ] || continue
  base="$(basename "$path")"
  if is_thin_file "$base"; then continue; fi
  if [ "$MODE" = "sync" ]; then
    rm -rf "$path"
    echo "  removed $base"
  else
    add_problem "$base (not an entry point; checks run from the rebar install)"
  fi
done

for base in $THIN_FILES; do
  file="$DST/$base"
  if [ ! -f "$file" ]; then
    add_problem "$base missing"
    continue
  fi
  case "$base" in
    *.sh)
      grep -qx '# rebar-thin-wrapper' "$file" || add_problem "$base lacks the '# rebar-thin-wrapper' marker"
      grep -q '^exec rebar ' "$file" || add_problem "$base does not exec the rebar CLI"
      [ -x "$file" ] || add_problem "$base is not executable"
      ;;
  esac
done

# Two copies of the contract template: the root one is the source.
ROOT_TEMPLATE="$PROJECT_ROOT/architecture/CONTRACT-TEMPLATE.md"
BOOT_TEMPLATE="$PROJECT_ROOT/templates/project-bootstrap/architecture/CONTRACT-TEMPLATE.md"
if [ -f "$ROOT_TEMPLATE" ] && ! cmp -s "$ROOT_TEMPLATE" "$BOOT_TEMPLATE"; then
  if [ "$MODE" = "sync" ]; then
    cp "$ROOT_TEMPLATE" "$BOOT_TEMPLATE"
    echo "  copied architecture/CONTRACT-TEMPLATE.md into the template"
  else
    add_problem "architecture/CONTRACT-TEMPLATE.md differs from the root copy"
  fi
fi

# The binary's embedded copy: each source file mirrored byte for byte as
# <path>.embed (the suffix keeps rebar's own scanners — freshness, doc refs,
# contracts — and git from treating the copies as live files; modes are not
# compared, embed keeps none), nothing else. Same list as
# cli/internal/assets.Sources.
EMBED_SOURCES="templates/project-bootstrap bin/ask bin/ask-agent-loop bin/ask-mcp-server bin/ask-server bin/install bin/completions"
EMBED="$PROJECT_ROOT/cli/internal/assets/files"
embed_note=""
if [ -d "$PROJECT_ROOT/cli/internal/assets" ]; then
  embed_note="; the binary's embedded copy matches"
  for src in $EMBED_SOURCES; do
    for f in $(cd "$PROJECT_ROOT" && find "$src" -type f | LC_ALL=C sort); do
      if cmp -s "$PROJECT_ROOT/$f" "$EMBED/$f.embed"; then continue; fi
      if [ "$MODE" = "sync" ]; then
        mkdir -p "$(dirname "$EMBED/$f")"
        cp "$PROJECT_ROOT/$f" "$EMBED/$f.embed"
        echo "  embedded $f"
      else
        add_problem "cli/internal/assets/files/$f.embed differs from $f"
      fi
    done
  done
  if [ -d "$EMBED" ]; then
    for e in $(cd "$EMBED" && find . -type f | sed 's#^\./##' | LC_ALL=C sort); do
      f="${e%.embed}"
      listed=0
      for src in $EMBED_SOURCES; do
        case "$f" in "$src"|"$src"/*) listed=1 ;; esac
      done
      if [ "$e" != "$f" ] && [ "$listed" -eq 1 ] && [ -f "$PROJECT_ROOT/$f" ]; then continue; fi
      if [ "$MODE" = "sync" ]; then
        rm -f "$EMBED/$e"
        echo "  removed embedded $e"
      else
        add_problem "cli/internal/assets/files/$e has no source"
      fi
    done
  fi
fi

if [ "$problems" -eq 0 ]; then
  echo "check-bootstrap-sync: OK — templates/project-bootstrap/scripts/ holds only thin entry points; CONTRACT-TEMPLATE.md matches the root copy$embed_note"
  exit 0
fi
printf 'check-bootstrap-sync: %s problem(s) in templates/project-bootstrap/:%b\n' "$problems" "$report"
if [ "$MODE" = "check" ]; then
  echo ""
  echo "Run: ./scripts/sync-bootstrap.sh (removes extra files, copies the contract template, refreshes the embedded copy; fix the rest by hand)"
fi
exit 1
