#!/usr/bin/env bash
# Run enforcement checks (ci-check.sh)
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
exec "$REPO_ROOT/scripts/ci-check.sh" "$@"
