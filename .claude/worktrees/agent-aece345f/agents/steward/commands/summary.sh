#!/usr/bin/env bash
# One-line health summary
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
exec "$REPO_ROOT/scripts/steward.sh" --summary
