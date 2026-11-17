#!/usr/bin/env bash
# scan-consumers.sh <contract-id> [--json]
#
# Owner-side script. Greps known sibling repos (from the ASK CLI registry)
# for CONSUMES.md declarations citing the given contract owned by THIS
# repo. Prints a list of consumers with their pin info.
#
# Cross-machine discovery is out of scope (CHARTER §2.10). For consumers
# on other machines, run with REBAR_REPOS=path1,path2,... explicit list.
#
# Fields read from each consumer's `## <owner>/<contract-id>.<version>` section:
#   version_pinned    required — a section without it is reported on stderr,
#                     left out, and the scan exits 1 (after the other consumers)
#   pin_date          optional, default "" (shown as "(unset)")
#   notify_on_change  optional, default "" (flush-notifications applies the
#                     owner default, REBAR_NOTIFY_DEFAULT: notify)
#   rationale         optional, default "" (not shown)
# A trailing `# comment` is dropped from version_pinned, pin_date and
# notify_on_change, as `rebar contract drift-check` does.
#
# Exit: 0 = scanned, 1 = a consumer section lacks a required field, 2 = usage.
#
# Source: CHARTER §1.6, feedback/2026-04-28-cross-repo-contract-federation.md

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/_rebar-config.sh" ]; then
  # shellcheck source=scripts/_rebar-config.sh
  source "$SCRIPT_DIR/_rebar-config.sh"
  # Under `rebar run` (REBAR_CHECK_IMPL=go, the default) the CLI's native port runs instead.
  _rebar_native scan-consumers "$@"
fi

CONTRACT_ID=""
JSON=0

while [ $# -gt 0 ]; do
  case "$1" in
    --json) JSON=1; shift ;;
    -h|--help)
      cat <<EOF
Usage: scan-consumers.sh <contract-id> [--json]

  scan-consumers.sh C1-AGENTS         # text output
  scan-consumers.sh C1-AGENTS --json  # machine-readable

Looks for CONSUMES.md across registered repos (per ~/.config/ask/projects)
and reports any that declare a dep on <owner>/<contract-id>.<*>.

Override search set: REBAR_REPOS=path1,path2,... scan-consumers.sh ID
EOF
      exit 0
      ;;
    *)
      [ -z "$CONTRACT_ID" ] && CONTRACT_ID="$1" || {
        echo "scan-consumers: unexpected arg '$1'" >&2; exit 2;
      }
      shift
      ;;
  esac
done

[ -n "$CONTRACT_ID" ] || { echo "Usage: scan-consumers.sh <contract-id>" >&2; exit 2; }

OWNER="$(basename "$(git rev-parse --show-toplevel 2>/dev/null || pwd)")"

# Determine the set of repos to scan
declare -a SCAN_PATHS=()
if [ -n "${REBAR_REPOS:-}" ]; then
  IFS=',' read -ra SCAN_PATHS <<< "$REBAR_REPOS"
else
  REGISTRY="${ASK_REGISTRY:-$HOME/.config/ask/projects}"
  if [ -f "$REGISTRY" ]; then
    while IFS='=' read -r rname rpath; do
      rname=$(echo "$rname" | sed 's/[[:space:]]*$//')
      rpath=$(echo "$rpath" | sed 's/^[[:space:]]*//')
      [ -z "$rname" ] && continue
      [ "$rname" = "$OWNER" ] && continue
      SCAN_PATHS+=("$rpath")
    done < "$REGISTRY"
  fi
fi

if [ "${#SCAN_PATHS[@]}" -eq 0 ]; then
  if [ "$JSON" -eq 1 ]; then
    echo '{"contract":"'"$OWNER/$CONTRACT_ID"'","consumers":[]}'
  else
    echo "scan-consumers: no repos to scan (registry empty + REBAR_REPOS unset)"
  fi
  exit 0
fi

# field NAME — the value of "- **NAME:** value" in $section ("" if absent).
field() {
  awk -v k="- **$1:**" 'index($0, k) == 1 {
    v = substr($0, length(k) + 1); sub(/^[ \t]+/, "", v); sub(/[ \t]+$/, "", v); print v; exit
  }' <<< "$section"
}
# value — drop a trailing "# comment" and the whitespace around the value.
value() { sed -e 's/#.*//' -e 's/[[:space:]]*$//'; }

# Build results in JSON-shaped lines for easy consumption
results=""
errors=0
for repo_path in "${SCAN_PATHS[@]}"; do
  consumes="$repo_path/CONSUMES.md"
  [ -f "$consumes" ] || continue
  if ! grep -q "^## $OWNER/$CONTRACT_ID\." "$consumes"; then
    continue
  fi

  consumer_name=$(basename "$repo_path")
  # Extract the section block for this contract
  section=$(awk -v hdr="^## $OWNER/$CONTRACT_ID\\\\." '
    $0 ~ hdr { in_section=1; print; next }
    in_section && /^## / { in_section=0 }
    in_section { print }
  ' "$consumes")

  pin=$(field version_pinned | value)
  pin_date=$(field pin_date | value)
  notify=$(field notify_on_change | value)
  rationale=$(field rationale)

  if [ -z "$pin" ]; then
    echo "scan-consumers: $consumes: section '${section%%$'\n'*}' has no version_pinned (required: the semver this consumer is pinned to, CHARTER §1.6) — skipped" >&2
    errors=$((errors + 1))
    continue
  fi

  if [ "$JSON" -eq 1 ]; then
    results+="{\"name\":\"$consumer_name\",\"path\":\"$repo_path\",\"version_pinned\":\"$pin\",\"pin_date\":\"$pin_date\",\"notify_on_change\":\"${notify:-}\",\"rationale\":\"$rationale\"},"
  else
    if [ -z "$results" ]; then
      echo "Consumers of $OWNER/$CONTRACT_ID:"
      echo ""
    fi
    echo "  $consumer_name → $repo_path"
    echo "    version_pinned:    $pin"
    echo "    pin_date:          ${pin_date:-(unset)}"
    echo "    notify_on_change:  ${notify:-(unset; owner default applies)}"
    [ -n "$rationale" ] && echo "    rationale:         $rationale"
    echo ""
    results="found"
  fi
done

if [ "$JSON" -eq 1 ]; then
  results="${results%,}"  # strip trailing comma
  echo "{\"contract\":\"$OWNER/$CONTRACT_ID\",\"consumers\":[$results]}"
elif [ -z "$results" ]; then
  echo "No consumers of $OWNER/$CONTRACT_ID found in scanned repos."
  echo "(Searched ${#SCAN_PATHS[@]} repo(s); cross-machine discovery is out of scope per CHARTER §2.10.)"
fi
[ "$errors" -eq 0 ] || exit 1
