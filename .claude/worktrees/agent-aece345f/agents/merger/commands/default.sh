#!/usr/bin/env bash
# List branches available for merging
echo "=== Merger: Available Branches ==="
echo ""
echo "Worktree branches:"
git branch --list 'worktree-*' 2>/dev/null | sed 's/^/  /' || echo "  (none)"
echo ""
echo "All branches:"
git branch | sed 's/^/  /'
echo ""
echo "Usage:"
echo "  ask merger \"merge worktree-agent-abc worktree-agent-def\""
echo "  ask merger \"cherry-pick abc1234 def5678 onto main\""
echo "  ask merger \"what conflicts exist between branch-a and branch-b?\""
