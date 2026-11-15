#!/usr/bin/env bash
# _rebar-config.sh — Shared configuration for rebar enforcement scripts
# rebar-scripts: 2026.03.20
#
# Source this from any enforcement script:
#   source "$(dirname "$0")/_rebar-config.sh"
#
# Provides:
#   _rebar_tier    — returns the configured enforcement tier (1, 2, or 3)
#   _rebar_skip    — returns 0 (skip) or 1 (run) for a minimum tier and check
#   _rebar_check_tier — the tier a check runs at (3 when checks_enable names it)
#   _rebar_checks_enable — the checks .rebarrc checks_enable force-enables
#   _rebar_config  — reads a .rebarrc key (env override, then .rebarrc, then default)
#   _rebar_project_root — the repo the scripts check (REBAR_PROJECT_ROOT or ../)
#   _rebar_script  — path of a helper script, preferring the project's own copy
#   _rebar_contracts_dir — where the contracts live (.rebarrc contracts_dir)
#   _rebar_contract_namespace — this repo's contract namespace (.rebarrc contract_namespace), "" if unset
#   _rebar_contract_ref_ere — ERE for the start of a reference to this repo's contracts
#   _rebar_contract_namespace_warn — warn once per run about an invalid contract_namespace
#
# Tier definitions:
#   1 = Partial  — contract-refs + TODOs only (minimum viable)
#   2 = Adopted  — + contract-headers, freshness, jtbd/prefix (advisory),
#                    compliance's AGENTS.md sections
#   3 = Enforced — + ground-truth; jtbd/prefix block (full enforcement)
# The registry check (compute-registry.sh --check) runs at every tier.
#
# Configuration priority: REBAR_TIER env var > .rebarrc file > default (3)
#
# .rebarrc `checks_enable` (env REBAR_CHECKS_ENABLE) force-enables named checks
# below their tier: a space-separated list of check script basenames without
# .sh (`checks_enable = check-ground-truth check-jtbd-presence`). A named check
# runs as it would at tier 3. Unknown names are warned about and ignored.

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

# Checks `checks_enable` may name: the script basenames (without .sh) of every
# check the suites run. Mirrored by config.KnownChecks in the Go CLI.
_REBAR_KNOWN_CHECKS="check-bypass-flags check-compliance check-contract-headers check-contract-refs check-decay-patterns check-doc-refs check-fix-commit check-freshness check-ground-truth check-jtbd-presence check-prefix-uniqueness check-todos compute-registry steward sync-bootstrap"

# The checks .rebarrc `checks_enable` (env REBAR_CHECKS_ENABLE) force-enables,
# one per line, with any .sh suffix dropped. Unknown names are left in; see
# _rebar_checks_enable_warn.
_rebar_checks_enable() {
  local raw name
  local -a names=()
  raw=$(_rebar_config checks_enable REBAR_CHECKS_ENABLE "")
  read -r -a names <<<"$raw"
  for name in ${names[@]+"${names[@]}"}; do
    echo "${name%.sh}"
  done
}

# Warn on stderr about checks_enable names that are not checks, once per run:
# the first caller exports _REBAR_CHECKS_VALIDATED so the scripts it starts
# stay quiet. Unknown names never fail a run (they are ignored).
_rebar_checks_enable_warn() {
  [ -n "${_REBAR_CHECKS_VALIDATED:-}" ] && return 0
  export _REBAR_CHECKS_VALIDATED=1
  local name
  while IFS= read -r name; do
    [ -z "$name" ] && continue
    case " $_REBAR_KNOWN_CHECKS " in
      *" $name "*) ;;
      *) echo "rebar: WARN: checks_enable names unknown check '$name' (ignored; use a check script name without .sh, e.g. check-ground-truth)" >&2 ;;
    esac
  done <<<"$(_rebar_checks_enable)"
}

# The tier check <name> runs at: 3 when checks_enable names it, otherwise the
# configured tier.
_rebar_check_tier() {
  local check="${1%.sh}" name
  while IFS= read -r name; do
    if [ -n "$name" ] && [ "$name" = "$check" ]; then
      echo 3
      return
    fi
  done <<<"$(_rebar_checks_enable)"
  _rebar_tier
}

# Check if a script should run based on its minimum required tier.
# Usage: _rebar_skip 2 check-freshness && exit 0   # skip if tier < 2
# A check named in checks_enable runs at any tier. Without a check name the
# caller's script name is used.
_rebar_skip() {
  local min_tier="$1" check="${2:-$(basename "$0" .sh)}"
  local current_tier
  _rebar_checks_enable_warn
  current_tier=$(_rebar_tier)

  if [ "$current_tier" -lt "$min_tier" ]; then
    if [ "$(_rebar_check_tier "$check")" -ge "$min_tier" ]; then
      echo "NOTE: tier $current_tier < required tier $min_tier; running $check because .rebarrc checks_enable names it"
      return 1  # false = should run
    fi
    echo "SKIP: tier $current_tier < required tier $min_tier (set REBAR_TIER or .rebarrc to change, or name it in checks_enable)"
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

# Directory holding the contracts (CONTRACT-*.md, CONTRACT-REGISTRY.md and
# the steward's .state/), relative to the project root: .rebarrc
# `contracts_dir` (env REBAR_CONTRACTS_DIR), default `architecture`. It names
# the contracts location only; repos may keep other design docs elsewhere.
_rebar_contracts_dir() {
  local dir
  dir=$(_rebar_config contracts_dir REBAR_CONTRACTS_DIR architecture)
  dir=$(_rebar_norm_dir "$dir")
  echo "${dir:-architecture}"
}

# _rebar_norm_dir DIR — a relative directory without leading ./ or trailing /
# (./docs/specs/ → docs/specs), so scripts can match it as a path prefix.
_rebar_norm_dir() {
  local d="$1"
  while [ "${d#./}" != "$d" ]; do d="${d#./}"; done
  while [ "$d" != "/" ] && [ "${d%/}" != "$d" ]; do d="${d%/}"; done
  echo "$d"
}

# Directory names the source scanners (check-todos, check-contract-refs,
# check-registry) skip anywhere in the tree: .rebarrc `source_exclude_dirs`
# (default: node_modules vendor dist), plus .git and .claude (agent
# worktrees hold stale copies of the source; see conventions.md) always.
# Matched by name with grep --exclude-dir, never by substring.
_rebar_source_exclude_dirs() {
  echo "$(_rebar_config source_exclude_dirs _REBAR_NO_ENV "node_modules vendor dist") .git .claude"
}

# This repo's contract namespace: .rebarrc `contract_namespace` (env
# REBAR_CONTRACT_NAMESPACE), "" when unset. With a namespace, a reference to
# one of this repo's contracts may be written CONTRACT:<namespace>:<ID>.<v>
# as well as CONTRACT:<ID>.<v>; a reference qualified with any other
# namespace points at another repo and is not checked here. Allowed
# characters: letters, digits, . _ / - (e.g. github.com/org/repo), and the
# value must not start like a contract ID.version (S1-FOO.1.0), or
# a reference <ID>.<v>:see.2 would read as qualified. Anything else is
# ignored (warned once per run), so a typo cannot widen the patterns.
# Surrounding whitespace and a trailing / are dropped.
# Mirrors config.ContractNamespace in cli/internal/config/config.go.
_rebar_contract_namespace() {
  local ns
  ns=$(_rebar_contract_namespace_value)
  if [ -n "$ns" ] && ! _rebar_contract_namespace_valid "$ns"; then
    _rebar_contract_namespace_warn
    ns=""
  fi
  echo "$ns"
}

# The configured value, trimmed, before validation.
_rebar_contract_namespace_value() {
  local ns
  ns=$(_rebar_config contract_namespace REBAR_CONTRACT_NAMESPACE "")
  ns="${ns#"${ns%%[![:space:]]*}"}"
  ns="${ns%"${ns##*[![:space:]]}"}"
  echo "${ns%/}"
}

# Whether $1 may be a contract namespace (see _rebar_contract_namespace).
# Shared by the configured value and by references parsed from source.
_REBAR_NS_CHARS_RE='^[A-Za-z0-9._/-]+$'
_REBAR_ID_VERSION_RE='^[A-Za-z0-9_-]+\.[0-9]+\.[0-9]+'
_rebar_contract_namespace_valid() {
  [[ "$1" =~ $_REBAR_NS_CHARS_RE ]] && ! [[ "$1" =~ $_REBAR_ID_VERSION_RE ]]
}

# Warn on stderr about an invalid contract_namespace, once per run: the first
# caller exports _REBAR_NAMESPACE_VALIDATED so the scripts it starts stay
# quiet (ci-check, pre-commit and cold-start call it up front).
_rebar_contract_namespace_warn() {
  [ -n "${_REBAR_NAMESPACE_VALIDATED:-}" ] && return 0
  export _REBAR_NAMESPACE_VALIDATED=1
  local ns
  ns=$(_rebar_contract_namespace_value)
  if [ -n "$ns" ] && ! _rebar_contract_namespace_valid "$ns"; then
    echo "rebar: ignoring contract_namespace '$ns' (allowed: letters, digits, . _ / -; must not start like a contract ID.version)" >&2
  fi
  return 0
}

# ERE matching the start of a reference to one of this repo's contracts:
# `CONTRACT:` without a namespace, `CONTRACT:(<namespace>:)?` with one.
# Append the ID (and version) pattern; use with grep -E / sed -E.
_rebar_contract_ref_ere() {
  local ns
  ns=$(_rebar_contract_namespace)
  if [ -z "$ns" ]; then
    echo 'CONTRACT:'
  else
    # `.` is the only ERE metacharacter the allowed set can contain.
    echo "CONTRACT:(${ns//./\\.}:)?"
  fi
}

# Read the rebar-scripts version from a script file
_rebar_script_version() {
  local file="$1"
  grep '^# rebar-scripts:' "$file" 2>/dev/null | head -1 | sed 's/^# rebar-scripts:[[:space:]]*//'
}
