#!/usr/bin/env bash
# Run enforcement checks (ci-check.sh)
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REBAR="$REPO_ROOT/bin/rebar"; [ -x "$REBAR" ] || REBAR=rebar
exec "$REBAR" --repo-root "$REPO_ROOT" check "$@"
