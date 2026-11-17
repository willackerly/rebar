#!/usr/bin/env bash
# release-notes.sh — print CHANGELOG.md's section for a release tag.
#
# The release workflow uses it as the GitHub Release body (GitHub's list of
# merged PRs is appended after it) and refuses to publish a tag whose section
# is missing or empty, so the CHANGELOG is written before the tag, not after.
#
# A section starts at the heading `## <tag>` (alone or followed by a space,
# e.g. `## v3.1.0 — 2026-11-16`; `## v3.1.0-rc.3` is a different tag) and
# ends at the next `## ` heading or `---` rule. Leading and trailing blank
# lines are dropped.
#
# --link-base URL rewrites relative Markdown links (`](docs/X.md)`) to
# URL/docs/X.md, since a GitHub Release page can't resolve them; absolute
# links, `#anchors`, `/paths` and `mailto:` stay as they are.
#
# Usage: scripts/release-notes.sh [--link-base URL] <tag> [CHANGELOG]
# Exit:  0 = section printed, 1 = no such section or it is empty, 2 = usage

set -euo pipefail

usage() { echo "usage: release-notes.sh [--link-base URL] <tag> [CHANGELOG]" >&2; exit 2; }
base=""
if [ "${1:-}" = "--link-base" ]; then
  [ $# -ge 2 ] || usage
  base="${2%/}"
  shift 2
fi
[ $# -ge 1 ] && [ -n "$1" ] || usage
tag="$1"
changelog="${2:-$(cd "$(dirname "$0")/.." && pwd)/CHANGELOG.md}"
[ -f "$changelog" ] || { echo "release-notes: $changelog not found" >&2; exit 1; }

notes="$(awk -v h="## $tag" -v base="$base" '
  # Prefix relative link targets with base: ](path) -> ](base/path).
  function relink(line,   out, target) {
    if (base == "") return line
    out = ""
    while (match(line, /\]\([^)]*\)/)) {
      target = substr(line, RSTART + 2, RLENGTH - 3)
      if (target !~ /^([a-zA-Z][a-zA-Z0-9+.-]*:|#|\/)/) target = base "/" target
      out = out substr(line, 1, RSTART - 1) "](" target ")"
      line = substr(line, RSTART + RLENGTH)
    }
    return out line
  }
  !found && ($0 == h || index($0, h " ") == 1) { found = 1; next }
  found && (/^## / || /^---[[:space:]]*$/) { exit }
  found {
    if ($0 ~ /^[[:space:]]*$/) { if (started) blanks++; next }
    while (blanks > 0) { print ""; blanks-- }
    started = 1
    print relink($0)
  }
' "$changelog")"

if [ -z "$notes" ]; then
  echo "release-notes: no \"## $tag\" section with content in $changelog" >&2
  exit 1
fi
printf '%s\n' "$notes"
