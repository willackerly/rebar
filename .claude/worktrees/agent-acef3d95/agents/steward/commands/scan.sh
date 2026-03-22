#!/usr/bin/env bash
# Full quality scan
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
exec "$REPO_ROOT/scripts/steward.sh" "$@"
