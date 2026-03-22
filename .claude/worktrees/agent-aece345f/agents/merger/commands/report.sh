#!/usr/bin/env bash
# Show latest merge report
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REPORT="$REPO_ROOT/agents/results/merge-report.md"
if [ -f "$REPORT" ]; then
  cat "$REPORT"
else
  echo "No merge report found. Run a merge first:"
  echo "  ask merger \"merge <branch1> <branch2>\""
fi
