# Multi-Agent Orchestration

**Referenced from AGENTS.md. Read when planning multi-agent fan-out.**

---

## Subagent Prompt Templates

If this project has an `agents/` directory, it contains reusable prompt
templates for subagent delegation. **Check the index before doing specialized
tasks** — there may be a template that encodes how we want it done.

```
agents/
  subagent-guidelines.md       # shared behavioral contract — every subagent reads this
  subagent-prompts-index.md    # catalog of available templates
  subagent-prompts/            # one .md per template (UX review, security scan, etc.)
  results/                     # subagent output files
  findings/                    # architectural/security findings from subagents
```

### Use Subagents Aggressively

**Default to delegation, not doing it yourself.** When there is a backlog of
work, consider fan-out strategies before deploying subagents — plan the
sharding, check for conflicts, then launch in parallel. A single orchestrator
doing 10 tasks sequentially is almost always slower than 10 subagents doing
them in parallel.

**Two hard rules:**
1. **Subagents writing code MUST use worktree isolation.** No exceptions.
2. **Subagents MUST commit before completing.** Uncommitted work in an
   ephemeral worktree is lost work.

### When to Use Templates

- **Single invocation:** Point one subagent at a template for a task you want
  done *your way*. A `ux-review.md` template encodes your definition of UX
  review — the agent doesn't guess.
- **Parallel fan-out:** Same template, N agents, different parameters (shard
  ranges, file subsets, package names).

### How to Invoke

```
Agent(prompt: "Read agents/subagent-guidelines.md for behavioral rules.
              Read agents/subagent-prompts/<template>.md for your task.
              Parameters: TARGET=<path> OUTPUT=agents/results/<name>.json")
```

For fan-out, add `isolation: "worktree"` and launch multiple in one message.

---

## Pre-Launch Audit (MANDATORY Before Fan-Out)

Before launching ANY parallel agent campaign, the orchestrating agent must
verify what the codebase actually contains — not what docs or memory say.
This prevents the 50% waste incident (see docs/learnings-from-opendockit.md §7).

1. **Grep for existing implementations** in target packages. If planning an
   agent to "add feature X," first check if X already exists.
2. **Check test counts.** If docs say 129 tests but `pnpm test` shows 684,
   substantial work has happened since your last context.
3. **Read actual source directories.** `ls` and `wc -l` tell you what exists.
4. **Cross-reference "What's Next"** in QUICKCONTEXT.md against code — verify
   planned items haven't already been implemented.
5. **Check for overlap between planned agents.** List the files each agent
   will likely modify. If two agents touch the same file, either combine
   them into one or explicitly assign non-overlapping sections. Overlap
   causes merge conflicts that consume significant post-merge context.
6. **Build a file-level conflict matrix.** For N planned agents, explicitly
   list which files each will touch and verify no overlaps:
   ```
   Agent A: editing-session.ts, shape-dom.ts
   Agent B: hit-testing.ts, cursor-positioning.ts
   Agent C: pptx-svg-adapter.ts, pptx-editor.ts
   → No overlaps. Safe to parallelize.
   
   Agent D: editing-session.ts, hit-testing.ts
   Agent A: editing-session.ts, shape-dom.ts
   → OVERLAP on editing-session.ts! Sequence D after A, or combine.
   ```
7. **Interface additions: assign to ONE agent.** If an agent needs to add
   a method to a shared interface (Store, Scanner, etc.), that agent goes
   first. All other agents add their mock stubs after that merge lands.
   See `practices/worktree-collaboration.md` for merge ordering strategy.

8. **Verify shared mock is current.** If the project has a shared mock
   store, ensure it implements ALL current interface methods before
   launching agents. Otherwise every merge requires adding stubs to N
   test files. (See "Shared Mock Consolidation" in worktree-collaboration.md.)
9. **Assign migration versions at merge time.** Agents use `version: 0`
   as a placeholder. The orchestrator assigns the correct sequential
   version during merge. Alternatively, use timestamp-based naming to
   eliminate version conflicts entirely.

This takes 5 minutes and prevents hours of wasted integration work.

---

## Agent Rules Reference

Agents follow **The 10 Rules** defined in `agents/subagent-guidelines.md`.
That file is the single source of truth — every subagent reads it before
starting work. Do not duplicate the rules here; reference them.

When writing agent prompts, include:
```
Read agents/subagent-guidelines.md for The 10 Rules.
```

The orchestrator's responsibilities (pre-launch audit, merge strategy,
conflict zones) are below. The agent's responsibilities (isolation, commit
discipline, file ownership, no removals) are in subagent-guidelines.md.

### Orchestrator's Integration Checklist

After merging each agent, verify:

- [ ] Build passes (`go build ./...` / `pnpm build`)
- [ ] Tests pass (`go test ./...` / `pnpm test`)
- [ ] No duplicate declarations (two files defining same function)
- [ ] No removed types/functions (compare against pre-merge)
- [ ] Migration version correct (if applicable)
- [ ] Mock stubs complete for all interface methods
- [ ] Router/App wired for new endpoints/components
- [ ] Contract headers present on new files

**Target:** 5 minutes per merge. If it's taking 15+, a pre-launch step
was skipped.

### Crafting Agent Prompts

Every parallel agent prompt should include:

1. **File allowlist** — what they may create/modify (Rule 3)
2. **Recent changes** — `git log --oneline -10` since worktree branched (Rule 9)
3. **Template reference** — which subagent-prompt to follow
4. **Scope boundary** — explicit "do NOT modify" list for shared files (Rule 8)

---

## Feature Inventory Protocol

Before assigning a worktree agent to modify a file with **>300 lines of
logic**, generate a feature inventory first: an explicit list of every
behavior the file implements, linked to its exercising test.

Use the `agents/subagent-prompts/feature-inventory.md` template, then include
the output in the worktree agent's prompt with: "Preserve all listed features
unless explicitly told to remove them."

**Why:** Without an inventory, agents restructure files around their assigned
task and may silently delete existing features they don't recognize as
intentional (see docs/learnings-from-opendockit.md §3, the W6 incident).

---

## GC Protection (Set Before Any Fan-Out)

Git's automatic garbage collection can destroy dangling commits from pruned
worktrees. Set these at session start, before launching any agents:

```bash
git config gc.auto 0                          # prevent auto-GC during session
git config gc.reflogExpire 90.days            # keep reflogs longer
git config gc.reflogExpireUnreachable 90.days # keep orphan commits recoverable
```

**Why:** `git gc` runs automatically and can destroy commits from agents whose
worktrees were pruned. With these settings, orphaned agent commits survive
for 90 days in the object store, recoverable via `git fsck --unreachable`.

After a fan-out session is fully merged and verified, restore defaults:

```bash
git config gc.auto 1
git config gc.reflogExpire 90.days.ago
git config gc.reflogExpireUnreachable 30.days.ago
git gc --prune=now
```

---

## Agent Health Infrastructure

Source `scripts/agent-health.sh` in agent prompts for structured health signals:

```bash
source scripts/agent-health.sh

agent_checkpoint "fix: correct font ascender metric"  # stage + commit tracked files
agent_heartbeat                                        # write timestamp to /tmp/agent-<id>.heartbeat
agent_metric "tests_fixed" "3"                         # key-value to shared progress
agent_rmse "gradient-heavy" 0.301 0.180                # fidelity-specific metric
```

All agents append to a shared JSONL progress file. The parent can
`cat agent-progress.jsonl | jq` to see what each agent accomplished
without reading transcripts.

---

## Shared File Conflict Zones

Before launching a fan-out, identify files that multiple agents might modify
and explicitly assign ownership or freeze them:

| File Type | Risk | Mitigation |
|-----------|------|------------|
| Shared type definitions | High — every module imports | Freeze during fanout |
| Theme/config resolvers | High — rendering + parsing both use | Assign to at most 1 agent |
| Auto-generated files | High — binary/JSON that don't merge | Never edit manually; regenerate post-fanout |
| Shared test fixtures | Medium — multiple agents add tests | Use append-only patterns (JSONL) |
| Lock files (package-lock, etc.) | High — non-mergeable | Only one agent installs deps |

**Namespaced outputs:** Auto-generated files (baselines, reports, metrics)
should be namespaced by agent ID rather than written to a shared path.
Example: `fidelity-baselines-P3-charts.json` instead of `fidelity-baselines.json`.
Merge them post-fanout with a simple script.

---

## Recovery Protocol (Login Expiration or Agent Crash)

Execute immediately after re-authenticating or discovering a failed agent:

```bash
# Step 1: Survey surviving worktrees
git worktree list
for wt in .claude/worktrees/agent-*/; do
  echo "=== $(basename "$wt") ==="
  (cd "$wt" && git status --short && git log --oneline -3) 2>/dev/null || echo "  (inaccessible)"
done

# Step 2: Find dangling commits (the gold mine)
git fsck --no-reflogs --unreachable --no-progress 2>/dev/null \
  | grep "^unreachable commit" \
  | while read _ _ hash; do
      echo "$(git log --format='%H %ai %s' -1 "$hash")"
    done \
  | sort -k2 -r \
  | grep -v "WIP on\|index on\|Merge branch"

# Step 3: Triage each dangling commit
for hash in <list>; do
  on_main=$(git merge-base --is-ancestor "$hash" main 2>/dev/null && echo "YES" || echo "no")
  msg=$(git log --format='%s' -1 "$hash")
  cherry=$(git log --oneline main | grep -F "$msg" | head -1)
  echo "$hash on_main=$on_main cherry=${cherry:-none} $msg"
done

# Step 4: Cherry-pick recovered commits (one at a time, test between each)
git cherry-pick <hash>

# Step 5: Clean up
git worktree prune
git worktree list  # should be clean
```

**Key insight:** `git fsck --unreachable` finds commits that agents made before
their worktree branches were pruned. 100% of committed agent work has been
recoverable through this method — the only true loss is uncommitted work
(which the commit-per-chunk protocol minimizes).

---

## Post-Fanout Merge Strategy

After all agents complete (or are recovered), merge one at a time:

```bash
# 1. Cherry-pick each agent's commits, testing between merges
for branch in $(git worktree list | awk '/worktree-agent/ {print $NF}' | tr -d '[]'); do
  echo "=== Merging $branch ==="
  git log --oneline main.."$branch" | tac | while read hash msg; do
    git cherry-pick "$hash"
    # Fast package test after each pick
    pnpm --filter <affected-pkg> test
  done
done

# 2. Full test suite after all merges
pnpm test

# 3. Clean up
git worktree prune
```

**Never do an "optimistic merge"** — cherry-picking 8 agents' work without
testing between merges. When the suite fails with 15 failures, you can't
tell which agent caused which.

---

## Anti-Patterns (Learned the Hard Way)

### 1. "Big bang commit at the end"
Agent does 30 minutes of work, accumulates all changes, tries to commit at
the end. Login expires at minute 29. Total loss.
**Fix:** Commit after every logical chunk. One fix = one commit.

### 2. "Two agents, same shared file"
Two agents independently refactor the same file with incompatible approaches.
Both produce working code. Cherry-picking the second creates merge conflicts
that take longer to resolve than the original work.
**Fix:** Identify shared danger files before launch. Assign ownership or freeze.

### 3. "Agent completed successfully" (but actually didn't)
Login expires. Agent reports `status: completed` with result "Not logged in".
Parent assumes success. Worktree gets pruned. Work disappears.
**Fix:** Always check agent results for empty/error results. Run `git fsck
--unreachable` after any login incident.

### 4. "Optimistic merge — tests later"
Cherry-pick 8 agents' work without testing between merges. Test suite fails
with 15 failures. Can't tell which agent caused which failure.
**Fix:** Cherry-pick one agent at a time. Run package tests after each.

### 5. "Agent guessing without an oracle"
Agent sees a problem, hypothesizes root cause, writes fix, metric gets worse.
Tries another hypothesis. Cycle repeats.
**Fix:** Use an oracle. Reference implementations, specs, or ground truth
data make agents dramatically more effective (measured: 26x improvement).
