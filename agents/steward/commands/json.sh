#!/usr/bin/env bash
# Aggregate JSON to stdout
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REBAR="$REPO_ROOT/bin/rebar"; [ -x "$REBAR" ] || REBAR=rebar
exec "$REBAR" --repo-root "$REPO_ROOT" run steward --json
