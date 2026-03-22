# Merge Coordinator

**LOE: Max**

Coordinate the integration of parallel worktree branches into the main branch.
Cherry-pick commits, resolve conflicts, and produce a summary of all decisions.

## Parameters

- `BRANCHES` — space-separated list of worktree branch names to merge
- `TARGET` — target branch (default: main)
- `STRATEGY` — merge strategy: `cherry-pick` (default) or `merge`
- `OUTPUT` — path for the summary report (default: agents/results/merge-report.md)

## Task

1. **Inventory**: For each branch in BRANCHES, list the commits since divergence
   from TARGET. Show files changed per commit.

2. **Conflict detection**: Identify files modified by multiple branches.
   For each conflict zone, determine which branch's version is the superset
   (contains all intended changes from both).

3. **Integration**: For each branch (in the order listed):
   - Cherry-pick or merge commits onto TARGET
   - If conflicts arise:
     a. Read both versions and understand the intent of each change
     b. Produce the merged version that preserves both intents
     c. Document the judgment call in the summary
   - Run a syntax check / typecheck after each branch integration
   - If a cherry-pick fails and cannot be resolved, skip it and document why

4. **Verification**: After all branches are integrated:
   - Run available test commands (T2 minimum)
   - Check for import/reference errors
   - Verify no files were accidentally deleted

5. **Summary**: Write the OUTPUT report with:
   - Branches merged (in order)
   - Commits cherry-picked (count per branch)
   - Conflicts resolved (file, nature of conflict, resolution chosen)
   - Judgment calls made (anything non-obvious)
   - Tests run and results
   - Anything that needs human review

## Rules

- **Never discard changes silently.** If you can't merge a change, document it.
- **Prefer the superset.** When two branches modify the same code, the version
  that contains all intended changes from both is correct.
- **Run tests after each branch.** Don't stack three broken merges.
- **Document judgment calls.** If you chose version A over version B, explain why.
- **Flag uncertainty.** If a conflict requires domain knowledge you don't have,
  flag it for human review rather than guessing.

## Output Format

```markdown
# Merge Report

## Summary
- Branches: N merged
- Commits: N cherry-picked
- Conflicts: N resolved, N flagged for review
- Tests: pass/fail

## Per-Branch Detail

### branch-name-1
- Commits: abc1234, def5678
- Conflicts: none
- Notes: clean merge

### branch-name-2
- Commits: 111aaaa, 222bbbb
- Conflicts:
  - `src/auth.ts` — both branches modified the token validation logic.
    Chose branch-2's version (superset: includes branch-1's expiry check
    plus branch-2's refresh logic).
- Notes: ran T2 tests after merge, all passing

## Flagged for Review
- (items needing human judgment)
```

## Anti-Patterns

- **Don't blindly take "theirs" or "ours."** Understand both intents.
- **Don't skip conflicts.** Resolve or flag — never ignore.
- **Don't merge without testing.** A merge that breaks tests is worse than
  no merge.
- **Don't combine unrelated branches.** If branches have completely disjoint
  changes, merge them in separate passes rather than one big batch.
