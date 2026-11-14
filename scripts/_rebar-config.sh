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
#   _rebar_config  — reads a .rebarrc key (env override, then .rebarrc, then default)
#   _rebar_project_root — the repo the scripts check (REBAR_PROJECT_ROOT or ../)
#   _rebar_script  — path of a helper script, preferring the project's own copy
#
# Tier definitions:
#   1 = Partial  — contract-refs + TODOs only (minimum viable)
#   2 = Adopted  — + contract-headers, freshness, registry
#   3 = Enforced — + ground-truth, strict steward (full enforcement)
#
# Configuration priority: REBAR_TIER env var > .rebarrc file > default (3)

# The repo these scripts check. REBAR_PROJECT_ROOT lets the scripts/ of a
# rebar install check another repo (`rebar check` sets it); otherwise it is
# the parent of the scripts/ directory.
_rebar_project_root() {
  if [ -n "${REBAR_PROJECT_ROOT:-}" ]; then
    echo "$REBAR_PROJECT_ROOT"
    return
  fi
  (cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
}

# Path of helper script <name> to run. A rebar install checking another repo
# still runs that repo's own scripts/<name> if it has one, so adopters with
# customized copies keep working while they move the customization into
# .rebarrc; each such use prints a deprecation notice on stderr. Files marked
# `# rebar-thin-wrapper` are entry points, not checks, and are never picked.
_rebar_script() {
  local name="$1" script_dir root local_copy
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
  root="$(_rebar_project_root)"
  local_copy="$root/scripts/$name"
  if [ -f "$local_copy" ] \
    && [ "$(cd "$root/scripts" && pwd -P)" != "$script_dir" ] \
    && ! grep -q '^# rebar-thin-wrapper' "$local_copy" 2>/dev/null; then
    echo "rebar: running project copy scripts/$name (deprecated: move its settings to .rebarrc and delete it)" >&2
    echo "$local_copy"
    return
  fi
  echo "$script_dir/$name"
}

_rebar_tier() {
  # 1. Environment variable (highest priority)
  if [ -n "${REBAR_TIER:-}" ]; then
    echo "$REBAR_TIER"
    return
  fi

  # 2. .rebarrc file (project-level config)
  local rc_file
  rc_file="$(_rebar_project_root)/.rebarrc"
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

# Read a key from .rebarrc, with an env override and a default.
# Usage: _rebar_config <key> <ENV_VAR> <default>
#   value=$(_rebar_config registry_impl_counts REBAR_REGISTRY_IMPL_COUNTS true)
_rebar_config() {
  local key="$1" env_var="$2" default="$3"
  if [ -n "${!env_var:-}" ]; then
    echo "${!env_var}"
    return
  fi

  local rc_file value=""
  rc_file="$(_rebar_project_root)/.rebarrc"
  if [ -f "$rc_file" ]; then
    value=$(grep "^${key}[[:space:]]*=" "$rc_file" 2>/dev/null | head -1 \
      | sed 's/^[^=]*=[[:space:]]*//; s/[[:space:]]*#.*$//; s/[[:space:]]*$//')
  fi
  echo "${value:-$default}"
}

# Read the rebar-scripts version from a script file
_rebar_script_version() {
  local file="$1"
  grep '^# rebar-scripts:' "$file" 2>/dev/null | head -1 | sed 's/^# rebar-scripts:[[:space:]]*//'
}
