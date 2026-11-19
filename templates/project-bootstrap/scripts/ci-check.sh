#!/usr/bin/env bash
# rebar-thin-wrapper
# ci-check.sh — CI entry point. The checks are built into the rebar binary;
# this project only configures them (.rebarrc). Usage: ./scripts/ci-check.sh
# Skip a check with its SKIP_* variable, e.g. SKIP_FRESHNESS=1.
set -euo pipefail
command -v rebar >/dev/null 2>&1 || {
  echo "ci-check: rebar not found in PATH — install: https://github.com/willackerly/rebar#installation" >&2
  exit 1
}
exec rebar check "$@"
