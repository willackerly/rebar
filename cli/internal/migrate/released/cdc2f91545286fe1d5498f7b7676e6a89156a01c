#!/usr/bin/env bash
# check-freshness.sh — Flag documentation with stale freshness dates
# rebar-scripts: 2026.03.20
#
# Usage: ./scripts/check-freshness.sh [max-age-days]
# Default: 14 days
#
# Scans all .md files for <!-- freshness: YYYY-MM-DD --> markers
# and flags any that are older than the threshold.
#
# Exit code: 0 = all fresh, 1 = stale docs found

set -euo pipefail

# Tier gate: freshness checks are Tier 2+ (skip for Tier 1 / partial adoption)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$SCRIPT_DIR/_rebar-config.sh" ] && source "$SCRIPT_DIR/_rebar-config.sh" && _rebar_skip 2 && exit 0

MAX_AGE_DAYS="${1:-14}"
TODAY=$(date +%s)
stale=0
total=0
unmarked=0

while IFS= read -r file; do
  # Skip template files — their freshness markers are placeholders that
  # adopters update on copy. Holding them to the freshness threshold
  # produces noise, not signal.
  case "$file" in
    ./templates/*) continue ;;
  esac

  # Honor an explicit "freshness: archived" opt-out for historical
  # retrospectives, decided design docs, and other intentionally-static
  # content. The literal token is searched before the date pattern.
  if grep -q 'freshness: archived' "$file" 2>/dev/null; then
    continue
  fi

  # Extract the currency date. TWO vocabularies are in the wild and both
  # are honored: `freshness:` (the original) and `last-synced:` (used by
  # QUICKCONTEXT and CONSUMES). Parsing only the first silently exempted
  # the most-read doc in every rebar repo — rebar's own QUICKCONTEXT
  # drifted 29 days while cold-start reported freshness all-green.
  # See feedback/2026-08-02-freshness-check-parses-one-vocabulary-*.md.
  # `freshness:` wins when a file carries both.
  #
  # `grep -oE` exits 1 when no marker is present (most files have none).
  # Under `set -o pipefail` that aborts the whole pipeline, so disable
  # pipefail just for this lookup.
  set +o pipefail
  marker="freshness"
  date_str=$(grep -o 'freshness: [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}' "$file" 2>/dev/null | head -1 | sed 's/freshness: //')
  if [ -z "$date_str" ]; then
    marker="last-synced"
    date_str=$(grep -o 'last-synced: [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}' "$file" 2>/dev/null | head -1 | sed 's/last-synced: //')
  fi
  set -o pipefail

  if [ -z "$date_str" ]; then
    unmarked=$((unmarked + 1))
    continue
  fi

  # Skip placeholder dates
  case "$date_str" in
    YYYY-MM-DD|0000-00-00) continue ;;
  esac

  total=$((total + 1))

  # Calculate age (portable: works on macOS and Linux)
  if date --version >/dev/null 2>&1; then
    # GNU date (Linux)
    doc_epoch=$(date -d "$date_str" +%s 2>/dev/null || echo 0)
  else
    # BSD date (macOS)
    doc_epoch=$(date -j -f "%Y-%m-%d" "$date_str" +%s 2>/dev/null || echo 0)
  fi

  [ "$doc_epoch" -eq 0 ] && continue

  age_days=$(( (TODAY - doc_epoch) / 86400 ))

  if [ "$age_days" -gt "$MAX_AGE_DAYS" ]; then
    echo "STALE ($age_days days): $file — $marker: $date_str"
    stale=$((stale + 1))
  fi
done < <(find . -name "*.md" -not -path "./.git/*" -not -path "*/node_modules/*" -type f)

echo ""
echo "Checked $total docs with currency markers (freshness:/last-synced:), $stale stale (>${MAX_AGE_DAYS} days)."
echo "$unmarked .md file(s) carry no currency marker at all — unchecked, not verified."

if [ "$stale" -gt 0 ]; then
  echo ""
  echo "Stale docs may contain outdated status claims."
  echo "Review and update the freshness date after verifying content."
  exit 1
fi

exit 0
