#!/usr/bin/env bash
# agent-health.sh — Source-able health primitives for worktree agents
# rebar-scripts: 2026.03.21
#
# Usage: source this file in agent prompts or test-stack scripts.
#
#   source scripts/agent-health.sh
#   agent_checkpoint "fix: correct font ascender metric"
#   agent_heartbeat
#   agent_metric "rmse_delta" "-0.045"
#
# All output goes to /tmp/agent-<id>.* files. The parent can poll these
# or the shared progress file to monitor the swarm.

set -euo pipefail

# Agent identity — derived from worktree name or PID
AGENT_ID="${AGENT_ID:-$(basename "$(git rev-parse --show-toplevel 2>/dev/null || echo "agent-$$")")}"
PROGRESS_FILE="${AGENT_PROGRESS_FILE:-agent-progress.jsonl}"

_agent_ts() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

# --- Checkpoint: stage tracked files and commit ---
# Commits only tracked files (git add -u), never untracked. This prevents
# agents from accidentally committing generated artifacts.
agent_checkpoint() {
  local msg="${1:?Usage: agent_checkpoint \"commit message\"}"
  git add -u
  if git diff --cached --quiet; then
    echo "[agent-health] nothing to commit"
    return 0
  fi
  git commit -m "$msg"
  local hash
  hash=$(git rev-parse --short HEAD)
  echo "[agent-health] checkpoint: $hash $msg"

  # Append to shared progress file if it exists
  if [ -n "$PROGRESS_FILE" ]; then
    local entry
    entry=$(printf '{"agent":"%s","ts":"%s","type":"checkpoint","commit":"%s","message":"%s"}\n' \
      "$AGENT_ID" "$(_agent_ts)" "$hash" "$msg")
    echo "$entry" >> "$PROGRESS_FILE" 2>/dev/null || true
  fi
}

# --- Heartbeat: signal that agent is alive ---
agent_heartbeat() {
  local hb_file="/tmp/agent-${AGENT_ID}.heartbeat"
  _agent_ts > "$hb_file"
}

# --- Metric: record a key-value measurement ---
agent_metric() {
  local key="${1:?Usage: agent_metric \"key\" \"value\"}"
  local value="${2:?Usage: agent_metric \"key\" \"value\"}"
  local metrics_file="/tmp/agent-${AGENT_ID}.metrics.jsonl"
  printf '{"agent":"%s","ts":"%s","key":"%s","value":%s}\n' \
    "$AGENT_ID" "$(_agent_ts)" "$key" "$value" >> "$metrics_file"

  # Also append to shared progress
  if [ -n "$PROGRESS_FILE" ]; then
    printf '{"agent":"%s","ts":"%s","type":"metric","key":"%s","value":%s}\n' \
      "$AGENT_ID" "$(_agent_ts)" "$key" "$value" >> "$PROGRESS_FILE" 2>/dev/null || true
  fi
}

# --- RMSE delta: specialized metric for fidelity work ---
agent_rmse() {
  local doc="${1:?Usage: agent_rmse \"doc\" before after}"
  local before="${2:?}"
  local after="${3:?}"
  agent_metric "rmse" "$(printf '{"doc":"%s","before":%s,"after":%s}' "$doc" "$before" "$after")"

  if [ -n "$PROGRESS_FILE" ]; then
    printf '{"agent":"%s","ts":"%s","type":"rmse","doc":"%s","rmse_before":%s,"rmse_after":%s}\n' \
      "$AGENT_ID" "$(_agent_ts)" "$doc" "$before" "$after" >> "$PROGRESS_FILE" 2>/dev/null || true
  fi
}

echo "[agent-health] loaded for agent=$AGENT_ID"
