#!/usr/bin/env bash
# Scan a single contract by ID
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
REBAR="$REPO_ROOT/bin/rebar"; [ -x "$REBAR" ] || REBAR=rebar
if [ -z "${1:-}" ]; then
  echo "Usage: ask steward check <contract-id>" >&2
  exit 4
fi
exec "$REBAR" --repo-root "$REPO_ROOT" run steward --check "$1"
