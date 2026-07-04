#!/usr/bin/env bash
# rebar-doc.sh — Resolve a rebar:<kind>/<name> cross-repo ref to a local file
# rebar-scripts: 2026.07.04
#
# Usage: ./scripts/rebar-doc.sh <ref> [--cat]
#        ./scripts/rebar-doc.sh -h|--help
#
# <ref> accepts either form: rebar:practice/inbox-watch or practice/inbox-watch
#
# Kinds (conventions.md §Cross-Repo References is the grammar of record):
#   practice/<name>        -> practices/<name>.md
#   script/<name>          -> scripts/<name>.sh
#   agents/<name>          -> agents/<name>.md
#   convention[/<section>] -> conventions.md (section is informational)
#   charter                -> CHARTER.md
#   doc/<name>             -> <name>.md at repo root, else docs/<name>.md
#   feedback/<name>        -> feedback/<name>.md, else feedback/processed/<name>.md
#
# Resolution order (identical to the Go resolver of record, `rebar doc`):
#   1. the current repo — nearest dir containing .git walking up from $PWD
#      ($PWD itself if none); covers a vendored/synced copy
#   2. $REBAR_ROOT
#   3. a discovered checkout: ~/.rebar, ~/dev/rebar, ~/src/rebar, ~/code/rebar
#      — verified as the rebar SOURCE repo via the presence of
#      templates/project-bootstrap/scripts/steward.sh (the same marker
#      cli/cmd/init.go findRebarRoot uses)
#   4. otherwise: print the canonical upstream URL and an ask-hint, exit 4
#
# Output (default): '<source>\t<path>', source ∈ local|REBAR_ROOT|checkout|upstream
# Output (--cat):   the resolved file's contents, nothing else on stdout
#
# Exit codes: 0 = resolved to an existing file
#             2 = usage error (unknown kind / malformed ref)
#             4 = unresolvable locally (upstream URL printed)

set -uo pipefail

UPSTREAM_BASE="https://github.com/willackerly/rebar/blob/main"

usage() {
  cat <<'EOF'
Usage: rebar-doc.sh <ref> [--cat]
       rebar-doc.sh -h|--help

Resolve a rebar:<kind>/<name> cross-repo ref to a local file.
The 'rebar:' prefix is optional.

Kinds:
  practice/<name>          script/<name>   agents/<name>   doc/<name>
  convention[/<section>]   charter         feedback/<name>

Default output is '<source>\t<path>' (source: local|REBAR_ROOT|checkout|upstream).
--cat prints the resolved file's contents instead.

Exit codes: 0 resolved, 2 usage/malformed ref, 4 unresolvable locally
(upstream URL printed).
EOF
}

die_usage() {
  echo "rebar-doc.sh: $1" >&2
  usage >&2
  exit 2
}

# --- argument parsing -------------------------------------------------------

REF=""
CAT=0
for arg in "$@"; do
  case "$arg" in
    -h|--help) usage; exit 0 ;;
    --cat) CAT=1 ;;
    -*) die_usage "unknown option: $arg" ;;
    *)
      [ -n "$REF" ] && die_usage "unexpected extra argument: $arg"
      REF="$arg"
      ;;
  esac
done
[ -n "$REF" ] || die_usage "missing <ref>"

# --- ref parsing: kind + name -> ordered candidate repo-relative paths ------

ref="${REF#rebar:}"
kind="${ref%%/*}"
name=""
[ "$kind" != "$ref" ] && name="${ref#*/}"

# Validate <name> for kinds that require one (convention sections are
# informational and exempt).
require_name() {
  case "$name" in
    '') die_usage "malformed ref '$REF': kind '$kind' requires a name" ;;
    */*) die_usage "malformed ref '$REF': name must not contain '/'" ;;
    *[[:space:]]*) die_usage "malformed ref '$REF': name must not contain whitespace" ;;
    *..*) die_usage "malformed ref '$REF': name must not contain '..'" ;;
  esac
}

# Ordered candidates; the first is the canonical upstream mapping.
CANDIDATES=()
case "$kind" in
  practice)   require_name; CANDIDATES=("practices/$name.md") ;;
  script)     require_name; CANDIDATES=("scripts/$name.sh") ;;
  agents)     require_name; CANDIDATES=("agents/$name.md") ;;
  convention) CANDIDATES=("conventions.md") ;;  # section is informational
  charter)
    [ -z "$name" ] || die_usage "malformed ref '$REF': 'charter' takes no name"
    CANDIDATES=("CHARTER.md")
    ;;
  doc)        require_name; CANDIDATES=("$name.md" "docs/$name.md") ;;
  feedback)   require_name; CANDIDATES=("feedback/$name.md" "feedback/processed/$name.md") ;;
  *) die_usage "unknown kind '$kind' (expected practice|script|agents|convention|charter|doc|feedback)" ;;
esac

# --- resolution helpers -----------------------------------------------------

# Nearest ancestor of $PWD (inclusive) containing .git; $PWD if none.
find_current_repo() {
  local dir="$PWD"
  while :; do
    if [ -e "$dir/.git" ]; then
      printf '%s\n' "$dir"
      return 0
    fi
    [ "$dir" = "/" ] && break
    dir="$(dirname "$dir")"
  done
  printf '%s\n' "$PWD"
}

# Same marker cli/cmd/init.go findRebarRoot uses to recognize the rebar
# source repo.
is_rebar_source() {
  [ -n "${1:-}" ] && [ -f "$1/templates/project-bootstrap/scripts/steward.sh" ]
}

# Print the first candidate that exists as a file under root $1; fail if none.
resolve_in() {
  local root="$1" c
  for c in "${CANDIDATES[@]}"; do
    if [ -f "$root/$c" ]; then
      printf '%s\n' "$root/$c"
      return 0
    fi
  done
  return 1
}

emit() {
  local source="$1" path="$2"
  if [ "$CAT" -eq 1 ]; then
    cat "$path"
  else
    printf '%s\t%s\n' "$source" "$path"
  fi
  exit 0
}

# --- resolution order (conventions.md §Cross-Repo References) ---------------

# 1. current repo (a vendored/synced copy)
repo="$(find_current_repo)"
if path="$(resolve_in "$repo")"; then
  emit local "$path"
fi

# 2. $REBAR_ROOT
if [ -n "${REBAR_ROOT:-}" ]; then
  if path="$(resolve_in "$REBAR_ROOT")"; then
    emit REBAR_ROOT "$path"
  fi
fi

# 3. discovered checkout (must be the rebar source repo)
home="${HOME:-}"
if [ -n "$home" ]; then
  for cand in "$home/.rebar" "$home/dev/rebar" "$home/src/rebar" "$home/code/rebar"; do
    if is_rebar_source "$cand"; then
      if path="$(resolve_in "$cand")"; then
        emit checkout "$path"
      fi
    fi
  done
fi

# 4. upstream — nothing local; point at the canonical source and a human.
printf '%s\t%s\n' upstream "$UPSTREAM_BASE/${CANDIDATES[0]}"
echo "unresolved locally — ask rebar architect (or the relevant role) for questions"
exit 4
