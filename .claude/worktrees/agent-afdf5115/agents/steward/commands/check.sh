#!/usr/bin/env bash
# Scan a single contract by ID
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
if [ -z "${1:-}" ]; then
  echo "Usage: ask steward check <contract-id>" >&2
  exit 4
fi
exec "$REPO_ROOT/scripts/steward.sh" --check "$1"
