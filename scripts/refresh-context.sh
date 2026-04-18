#!/usr/bin/env bash
# refresh-context.sh — Check QUICKCONTEXT freshness and report discrepancies
#
# Run at session start, at checkpoints, or whenever you suspect drift.
# See practices/session-lifecycle.md for the full session lifecycle protocol.
#
# Usage:
#   scripts/refresh-context.sh          # Full freshness report
#   scripts/refresh-context.sh --quick  # Just the staleness check

set -euo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$REPO_ROOT"

QUICK_MODE=false
if [[ "${1:-}" == "--quick" ]]; then
  QUICK_MODE=true
fi

# Colors (if terminal supports them)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

warn() { echo -e "${YELLOW}WARNING:${NC} $*"; }
ok()   { echo -e "${GREEN}OK:${NC} $*"; }
err()  { echo -e "${RED}ERROR:${NC} $*"; }

echo "=== QUICKCONTEXT Freshness Check ==="
echo ""

# Check if QUICKCONTEXT exists
if [[ ! -f "QUICKCONTEXT.md" ]]; then
  err "QUICKCONTEXT.md not found in repo root"
  exit 1
fi

# Extract last-synced date
LAST_SYNCED=$(grep -oE 'last-synced: [0-9]{4}-[0-9]{2}-[0-9]{2}' QUICKCONTEXT.md 2>/dev/null | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' || echo "")

if [[ -z "$LAST_SYNCED" ]]; then
  warn "No last-synced date found in QUICKCONTEXT.md"
  warn "Cannot determine freshness — treat all claims as suspect"
else
  echo "Last synced: $LAST_SYNCED"

  # Calculate days since sync (portable across macOS and Linux)
  if date -v-1d >/dev/null 2>&1; then
    # macOS
    SYNC_EPOCH=$(date -j -f "%Y-%m-%d" "$LAST_SYNCED" "+%s" 2>/dev/null || echo 0)
    NOW_EPOCH=$(date "+%s")
  else
    # Linux
    SYNC_EPOCH=$(date -d "$LAST_SYNCED" "+%s" 2>/dev/null || echo 0)
    NOW_EPOCH=$(date "+%s")
  fi

  if [[ "$SYNC_EPOCH" -gt 0 ]]; then
    DAYS_STALE=$(( (NOW_EPOCH - SYNC_EPOCH) / 86400 ))
    if [[ "$DAYS_STALE" -gt 7 ]]; then
      warn "QUICKCONTEXT is $DAYS_STALE days stale — treat ALL claims as suspect"
    elif [[ "$DAYS_STALE" -gt 3 ]]; then
      warn "QUICKCONTEXT is $DAYS_STALE days old — verify critical claims"
    else
      ok "QUICKCONTEXT is $DAYS_STALE days old"
    fi
  fi

  # Show recent commits since last sync
  echo ""
  echo "Commits since last sync:"
  COMMIT_COUNT=$(git log --since="$LAST_SYNCED" --oneline 2>/dev/null | wc -l | tr -d ' ')
  git log --since="$LAST_SYNCED" --oneline 2>/dev/null | head -20

  if [[ "$COMMIT_COUNT" -gt 20 ]]; then
    echo "  ... and $((COMMIT_COUNT - 20)) more"
  fi

  if [[ "$COMMIT_COUNT" -gt 10 ]]; then
    warn "$COMMIT_COUNT commits since last sync — QUICKCONTEXT is likely stale"
  elif [[ "$COMMIT_COUNT" -gt 0 ]]; then
    echo "  ($COMMIT_COUNT commits — review against QUICKCONTEXT claims)"
  else
    ok "No commits since last sync"
  fi
fi

if $QUICK_MODE; then
  exit 0
fi

# Full mode: additional checks

echo ""
echo "=== TODO Freshness ==="

if [[ -f "TODO.md" ]]; then
  TODO_SYNCED=$(grep -oE 'last-synced: [0-9]{4}-[0-9]{2}-[0-9]{2}' TODO.md 2>/dev/null | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' || echo "unknown")
  echo "TODO.md last synced: $TODO_SYNCED"

  # Count open vs completed items
  OPEN=$(grep -c '^\- \[ \]' TODO.md 2>/dev/null || echo 0)
  DONE=$(grep -c '^\- \[x\]' TODO.md 2>/dev/null || echo 0)
  echo "Open items: $OPEN | Completed items: $DONE"

  if [[ "$DONE" -gt "$OPEN" ]]; then
    warn "More completed than open items — consider archiving completed items"
  fi
fi

echo ""
echo "=== Untracked TODOs in Code ==="

# Search for TODO comments not linked to TODO.md
UNTRACKED=$(grep -rn "TODO:" \
  --include="*.ts" --include="*.tsx" --include="*.js" --include="*.jsx" \
  --include="*.go" --include="*.py" --include="*.rs" \
  src/ lib/ internal/ packages/ cmd/ 2>/dev/null \
  | grep -v "TRACKED-TASK" \
  | grep -v "node_modules" \
  | head -10 || true)

if [[ -n "$UNTRACKED" ]]; then
  warn "Untracked TODO comments found:"
  echo "$UNTRACKED"
else
  ok "No untracked TODO comments"
fi

echo ""
echo "=== Worktree State ==="

WORKTREES=$(git worktree list 2>/dev/null)
WORKTREE_COUNT=$(echo "$WORKTREES" | wc -l | tr -d ' ')

echo "$WORKTREES"

if [[ "$WORKTREE_COUNT" -gt 1 ]]; then
  warn "$((WORKTREE_COUNT - 1)) worktree(s) exist — verify they're active or clean them up"
  echo "  To clean: git worktree prune"
else
  ok "No extra worktrees"
fi

# Test baseline (opt-in — can be slow)
if [[ "${1:-}" == "--test-baseline" || "${2:-}" == "--test-baseline" ]]; then
  echo ""
  echo "=== Test Health Baseline ==="

  # Detect test runner
  TEST_CMD=""
  if [[ -f "package.json" ]]; then
    if command -v pnpm &>/dev/null && [[ -f "pnpm-lock.yaml" ]]; then
      TEST_CMD="pnpm test"
    elif command -v npm &>/dev/null; then
      TEST_CMD="npm test"
    fi
  elif [[ -f "go.mod" ]]; then
    TEST_CMD="go test ./..."
  elif [[ -f "pyproject.toml" ]] || [[ -f "setup.py" ]]; then
    TEST_CMD="pytest"
  fi

  if [[ -n "$TEST_CMD" ]]; then
    echo "Running: $TEST_CMD"
    if $TEST_CMD 2>&1 | tail -20; then
      ok "Test suite passed"
    else
      err "Test suite has failures — fix before starting work"
      echo "  This saves hours of discovering pre-existing failures mid-session"
    fi
  else
    warn "No test runner detected — skipping test baseline"
    echo "  Supported: pnpm/npm (package.json), go (go.mod), pytest (pyproject.toml)"
  fi
fi

echo ""
echo "=== Summary ==="
echo "Run 'rebar context session-start' for full context loading"
echo "Run 'rebar context' for the Cold Start Quad"
echo "Run 'scripts/refresh-context.sh --test-baseline' to verify tests pass"
