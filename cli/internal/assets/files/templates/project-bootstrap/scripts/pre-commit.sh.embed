#!/usr/bin/env bash
# rebar-thin-wrapper
# pre-commit.sh — git pre-commit hook entry point. The checks live in the
# rebar install. Install: ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
# Skip (sparingly): git commit --no-verify
set -euo pipefail
command -v rebar >/dev/null 2>&1 || {
  echo "pre-commit: rebar not found in PATH — skipping checks" >&2
  exit 0
}
exec rebar check --pre-commit
