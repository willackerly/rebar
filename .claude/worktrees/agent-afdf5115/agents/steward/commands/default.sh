#!/usr/bin/env bash
# Full quality scan (default)
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
exec "$REPO_ROOT/scripts/steward.sh" "$@"
