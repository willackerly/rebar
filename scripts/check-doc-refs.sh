#!/usr/bin/env bash
# check-doc-refs.sh — Verify that every file referenced from a tracked *.md
# is itself tracked in git.
#
# Catches a class of drift Tier 2 ci-check.sh misses: a load-bearing doc
# (e.g., FEDERATION-STORIES-DRAFT.md) is added to the working tree, cited
# from another doc, but never `git add`-ed. The current repo passes; a
# fresh clone fails to resolve the link.
#
# Source: rebar/feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md §1
#
# Usage:
#   ./scripts/check-doc-refs.sh           # report broken refs, exit 1 if any
#   ./scripts/check-doc-refs.sh --quiet   # only print failures
#   ./scripts/check-doc-refs.sh --json    # JSON summary on stdout
#
# Heuristics:
#   - Walks every tracked *.md file (`git ls-files '*.md'`).
#   - Extracts markdown-link targets: `[text](TARGET)`.
#   - Skips: external URLs (http*, mailto, tel, ftp), anchor-only refs (#x),
#     home-relative refs (~/...), template placeholders ({...}), allowlisted
#     paths from .rebar/doc-refs-allow.txt (one path per line, # for comments).
#   - Resolves remaining targets relative to the source file's directory.
#   - Fails if resolved target is not in `git ls-files`.
#
# Bash 3.2 compatible (macOS default).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALLOWLIST="$PROJECT_ROOT/.rebar/doc-refs-allow.txt"

MODE="text"
case "${1:-}" in
  --quiet) MODE="quiet" ;;
  --json)  MODE="json" ;;
  -h|--help)
    sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
esac

if ! command -v git >/dev/null 2>&1; then
  echo "check-doc-refs: git not found" >&2
  exit 2
fi

cd "$PROJECT_ROOT"

# Load allowlist into a sorted file for grep -Fx checks. Empty if missing.
allowlist_tmp="$(mktemp)"
trap 'rm -f "$allowlist_tmp" "$tracked_tmp" "$findings_tmp"' EXIT
if [ -f "$ALLOWLIST" ]; then
  grep -v '^#' "$ALLOWLIST" 2>/dev/null | grep -v '^$' | sort -u > "$allowlist_tmp" || true
fi

# Snapshot all tracked files once; lookups become a sorted-file grep.
tracked_tmp="$(mktemp)"
git ls-files | sort -u > "$tracked_tmp"

# Findings accumulator: one line per broken ref.
# Format:  <source.md>:<lineno>:<target>
findings_tmp="$(mktemp)"
: > "$findings_tmp"

# Iterate every tracked *.md.
total_refs=0
broken_refs=0

while IFS= read -r src; do
  # Skip files that no longer exist on disk (stale worktrees, deletions
  # not yet committed). git ls-files reports tracked entries even if the
  # working-tree copy is missing.
  [ -f "$src" ] || continue

  # Skip generated files.
  if head -3 "$src" 2>/dev/null | grep -qE 'AUTO-GENERATED|<!-- generated'; then
    continue
  fi

  # Skip stale worktree dirs and feedback/processed/ archives. Also skip
  # templates/ since those files contain adopter-context paths (e.g.,
  # `../DESIGN.md`) that resolve in the adopter's project layout, not in
  # rebar's source layout — checking them here would be a false positive.
  case "$src" in
    .claude/worktrees/*) continue ;;
    feedback/processed/*) continue ;;
    templates/*) continue ;;
  esac

  src_dir="$(dirname "$src")"

  # Extract markdown-link targets per line, preserving line numbers.
  # Pattern: [text](target)  — captures the target only.
  # Awk handles this without trying to be a markdown parser.
  awk '
    {
      line = $0
      while (match(line, /\[[^]]*\]\(([^)]+)\)/)) {
        # Pull the target out of the matched substring.
        match_str = substr(line, RSTART, RLENGTH)
        gsub(/^\[[^]]*\]\(/, "", match_str)
        sub(/\)$/, "", match_str)
        # Strip a trailing " title" if present.
        sub(/[[:space:]]+"[^"]*"$/, "", match_str)
        print NR "\t" match_str
        line = substr(line, RSTART + RLENGTH)
      }
    }
  ' "$src" | while IFS=$'\t' read -r lineno target; do
    [ -z "$target" ] && continue
    total_refs=$((total_refs + 1))

    # Skip schemes and non-path refs.
    case "$target" in
      http://*|https://*|mailto:*|tel:*|ftp://*|sftp://*) continue ;;
      "#"*) continue ;;
      "~"*) continue ;;
      "{"*"}"*|*"{"*"}"*) continue ;;
    esac

    # Skip placeholder words used in docs to illustrate link syntax —
    # bare words with no `/` and no `.` are almost always doc placeholders
    # like `[text](path)` or `[text](target)`, not real file references.
    case "$target" in
      */*|*.*) ;;  # has slash or dot — keep checking
      *) continue ;;
    esac

    # Strip in-page anchor; keep only the path part.
    path_only="${target%%#*}"
    [ -z "$path_only" ] && continue
    # Strip query string.
    path_only="${path_only%%\?*}"

    # Resolve relative to the source file's directory.
    if [ "${path_only:0:1}" = "/" ]; then
      # Absolute path within repo: drop the leading slash.
      resolved="${path_only#/}"
    else
      # Relative — resolve via a quick path walk.
      if [ "$src_dir" = "." ]; then
        resolved="$path_only"
      else
        resolved="$src_dir/$path_only"
      fi
    fi

    # Normalize ./ and ../ segments without invoking realpath (which fails
    # for not-yet-existing files in pre-merge contexts).
    #
    # NOTE: an earlier version used `${resolved/\/.\//\/}` parameter
    # expansion. On macOS bash 3.2.57 that produces a literal backslash
    # in the result (e.g., `docs/./X.md` → `docs\/X.md`), causing every
    # relative link with a `./` segment to be reported as broken. Use
    # sed instead — portable across bash 3.2, bash 4+, and zsh.
    # Source: feedback/2026-04-25-bootstrap-template-script-drift-and-bash3.2.md §Bug 1.
    resolved="$(printf '%s' "$resolved" | sed -e 's#/\./#/#g' -e 's#^\./##')"
    # Collapse a/b/../c → a/c (one level at a time).
    while [[ "$resolved" == *"/../"* ]]; do
      # shellcheck disable=SC2001
      resolved="$(printf '%s' "$resolved" | sed -E 's#[^/]+/\.\./##')"
    done

    # Allowlist check.
    if [ -s "$allowlist_tmp" ] && grep -Fxq -- "$resolved" "$allowlist_tmp"; then
      continue
    fi

    # Tracked check. Strip trailing slash for directory targets — git
    # ls-files lists files, so a directory ref needs a wildcard match.
    resolved_noslash="${resolved%/}"
    if [ "$resolved_noslash" != "$resolved" ]; then
      # Directory ref: at least one tracked file must live under it.
      if grep -q "^${resolved_noslash}/" "$tracked_tmp"; then
        continue
      fi
    else
      if grep -Fxq -- "$resolved_noslash" "$tracked_tmp"; then
        continue
      fi
    fi

    echo "${src}:${lineno}:${target}" >> "$findings_tmp"
    broken_refs=$((broken_refs + 1))
  done
done < <(git ls-files '*.md')

# Recount findings (the inner subshell's broken_refs doesn't propagate).
broken_refs="$(wc -l < "$findings_tmp" | tr -d ' ')"

if [ "$MODE" = "json" ]; then
  printf '{"checked":"markdown-link-targets","broken":%s,"findings":[' "$broken_refs"
  first=1
  while IFS=: read -r src lineno target; do
    [ "$first" = 1 ] || printf ','
    first=0
    # Escape minimal JSON: backslash and quote.
    src_e="${src//\\/\\\\}"; src_e="${src_e//\"/\\\"}"
    target_e="${target//\\/\\\\}"; target_e="${target_e//\"/\\\"}"
    printf '{"file":"%s","line":%s,"target":"%s"}' "$src_e" "$lineno" "$target_e"
  done < "$findings_tmp"
  printf ']}\n'
elif [ "$MODE" = "quiet" ]; then
  cat "$findings_tmp"
else
  if [ "$broken_refs" -eq 0 ]; then
    echo "check-doc-refs: OK — no broken markdown links to untracked files"
  else
    echo "check-doc-refs: $broken_refs broken ref(s) — referenced file is not tracked in git"
    echo ""
    cat "$findings_tmp"
    echo ""
    echo "Each line: <source.md>:<lineno>:<target>"
    echo "Either \`git add\` the target, fix the link, or allowlist it via .rebar/doc-refs-allow.txt"
  fi
fi

[ "$broken_refs" -eq 0 ]
