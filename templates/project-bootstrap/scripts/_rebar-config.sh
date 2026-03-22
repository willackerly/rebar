#!/usr/bin/env bash
# _rebar-config.sh — Shared configuration for rebar enforcement scripts
# rebar-scripts: 2026.03.20
#
# Source this from any enforcement script:
#   source "$(dirname "$0")/_rebar-config.sh"
#
# Provides:
#   _rebar_tier    — returns the configured enforcement tier (1, 2, or 3)
#   _rebar_skip    — returns 0 (should run) or 1 (should skip) for a minimum tier
#
# Tier definitions:
#   1 = Partial  — contract-refs + TODOs only (minimum viable)
#   2 = Adopted  — + contract-headers, freshness, registry
#   3 = Enforced — + ground-truth, strict steward (full enforcement)
#
# Configuration priority: REBAR_TIER env var > .rebarrc file > default (3)

_rebar_tier() {
  # 1. Environment variable (highest priority)
  if [ -n "${REBAR_TIER:-}" ]; then
    echo "$REBAR_TIER"
    return
  fi

  # 2. .rebarrc file (project-level config)
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  local project_root
  project_root="$(cd "$script_dir/.." && pwd)"

  local rc_file="$project_root/.rebarrc"
  if [ -f "$rc_file" ]; then
    local tier
    tier=$(grep '^tier' "$rc_file" 2>/dev/null | head -1 | sed 's/.*=[[:space:]]*//' | tr -d ' ')
    if [ -n "$tier" ] && [[ "$tier" =~ ^[123]$ ]]; then
      echo "$tier"
      return
    fi
  fi

  # 3. Default: full enforcement
  echo "3"
}

# Check if a script should run based on its minimum required tier
# Usage: _rebar_skip 2 && exit 0   # skip if tier < 2
_rebar_skip() {
  local min_tier="$1"
  local current_tier
  current_tier=$(_rebar_tier)

  if [ "$current_tier" -lt "$min_tier" ]; then
    echo "SKIP: tier $current_tier < required tier $min_tier (set REBAR_TIER or .rebarrc to change)"
    return 0  # true = should skip
  fi
  return 1  # false = should run
}

# Read the rebar-scripts version from a script file
_rebar_script_version() {
  local file="$1"
  grep '^# rebar-scripts:' "$file" 2>/dev/null | head -1 | sed 's/^# rebar-scripts:[[:space:]]*//'
}
