#!/usr/bin/env bash
# check-freshness.sh — Flag documentation with stale freshness dates
#
# Usage: ./scripts/check-freshness.sh [max-age-days]
# Default: 14 days
#
# Scans all .md files for <!-- freshness: YYYY-MM-DD --> markers
# and flags any that are older than the threshold.
#
# Exit code: 0 = all fresh, 1 = stale docs found

set -euo pipefail

MAX_AGE_DAYS="${1:-14}"
TODAY=$(date +%s)
stale=0
total=0

while IFS= read -r file; do
  # Extract freshness date
  date_str=$(grep -o 'freshness: [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}' "$file" 2>/dev/null | head -1 | sed 's/freshness: //')

  [ -z "$date_str" ] && continue

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
    echo "STALE ($age_days days): $file — freshness: $date_str"
    stale=$((stale + 1))
  fi
done < <(find . -name "*.md" -not -path "./.git/*" -not -path "*/node_modules/*" -type f)

echo ""
echo "Checked $total docs with freshness markers, $stale stale (>${MAX_AGE_DAYS} days)."

if [ "$stale" -gt 0 ]; then
  echo ""
  echo "Stale docs may contain outdated status claims."
  echo "Review and update the freshness date after verifying content."
  exit 1
fi

exit 0
