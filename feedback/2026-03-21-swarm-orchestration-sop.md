# Agent Orchestration SOP — Rebar Feedback

**Author:** OpenDocKit team
**Date:** 2026-03-21
**Context:** Lessons from 100+ worktree agent launches across 5 multi-day sessions, including 3 login expiration incidents affecting ~15 agents.

---

## Rebar as a Swarm Collective Learning & Coordination Framework

Rebar isn't just scaffolding for individual agents — it's the **coordination layer for a swarm**. Every dimension of the system benefits from collective intelligence:

- **Cross-agent:** 10 agents working the same codebase need shared progress tracking, conflict avoidance, and incremental checkpointing. When one agent discovers a pattern (e.g., "the PDF DOM is the oracle"), that learning should propagate to all agents in the swarm, not just the one that found it.
- **Cross-repo:** OpenDocKit agents learn font metrics patterns that apply to FontKit. Editor agents discover rendering bugs that feed back to core. The swarm's knowledge graph spans repositories.
- **Cross-role:** A fidelity agent, a test hardening agent, and a documentation agent all see different facets of the same code. Rebar should surface relevant findings across roles: "T-1 found that 30 skipped tests now pass after P-2's text fix" → that's a signal P-2 should know about.
- **Cross-session:** Today's agent learns that `git fsck --unreachable` recovers orphaned commits. That learning must survive session boundaries so tomorrow's agent doesn't rediscover it. Memory + SOPs + prompt templates are the persistence layer.
- **Cross-failure:** When an agent fails (login, merge conflict, regression), the failure mode and recovery become swarm knowledge. The next fan-out launches with those failure modes pre-mitigated.

The patterns below are the **first-generation coordination protocols** for this swarm. They solve the immediate problems (work survival, merge conflicts, observability). The recommended enhancements (P0-P5) evolve Rebar toward a true collective learning framework where agents don't just avoid stepping on each other — they actively amplify each other's work.

---

## Problem Statement

Claude Code's worktree agent model is powerful for parallelizing work across a monorepo, but has failure modes that silently destroy work:

1. **Login expiration mid-flight** — agents write files but can't commit. They report `status: completed` with result "Not logged in", giving the false impression of success.
2. **Worktree pruning** — when agents fail, worktree directories may be auto-cleaned, destroying uncommitted work AND orphaning committed-but-unreferenced branches.
3. **No progress observability** — the parent has no way to know what an agent accomplished until it returns. A 30-minute agent that dies at minute 29 looks identical to one that died at minute 1.
4. **Shared file conflicts** — parallel agents modifying the same files create merge conflicts that are tedious to resolve and sometimes result in dropped work.

## SOP: Resilient Agent Fan-Out

### 1. Infrastructure Layer (build once, reuse across sessions)

#### 1a. Agent Health Monitor Script

Source-able shell script that agents import. Provides:

```bash
# scripts/agent-health.sh
agent_checkpoint "commit message"  # stage + commit current work
agent_heartbeat                     # write timestamp to /tmp/agent-<id>.heartbeat
agent_metric "key" "value"          # append metric to /tmp/agent-<id>.metrics.jsonl
agent_rmse "doc" before after       # log improvement to shared progress tracker
```

**Key design decision:** `agent_checkpoint` commits tracked files only (`git add -u`), not untracked files. This prevents agents from accidentally committing generated artifacts.

#### 1b. Shared Progress Tracker

A single JSONL file that all agents append to:

```jsonl
{"agent":"P1-gradient","ts":"2026-03-21T15:22:31Z","doc":"gradient-heavy","rmse_before":0.301,"rmse_after":0.180,"commit":"abc123"}
{"agent":"D1-acp240","ts":"2026-03-21T15:25:00Z","doc":"acp240-main","rmse_before":0.241,"rmse_after":0.195,"commit":"def456"}
```

Parent can `cat agent-progress.jsonl | jq` to see exactly what each agent accomplished without reading transcripts.

#### 1c. GC Protection

Set at session start, before any agents launch:

```bash
git config gc.auto 0                          # prevent auto-GC during session
git config gc.reflogExpire 90.days            # keep reflogs longer
git config gc.reflogExpireUnreachable 90.days # keep orphan commits recoverable
```

**Why:** `git gc` runs automatically and can destroy dangling commits from pruned worktrees. With these settings, orphaned agent commits survive for 90 days in the object store, recoverable via `git fsck --unreachable`.

### 2. Agent Prompt Protocol (include in every agent prompt)

Every fidelity/code agent prompt MUST include these 6 rules verbatim:

```markdown
## Mandatory Agent Protocol

1. **Commit after each logical chunk** — don't accumulate. If you get logged out,
   only uncommitted work is lost. One fix = one commit.

2. **Measure before AND after** — run the relevant gate/test before your first
   change and after each fix. Record the metric. If it regressed, revert.

3. **Write progress** — after each commit, append to the shared progress file.

4. **Run package tests** after each change — not the full suite, just the
   affected package.

5. **Don't touch shared files** without coordination — list specific files
   that are high-conflict zones (IR types, color resolver, auto-generated
   bundles, etc.)

6. **Use the oracle** — if a reference implementation exists (e.g., PDF DOM
   for OOXML rendering), check it before guessing at fixes.
```

**Why each rule matters:**
- Rule 1 saved us from total loss in 3 incidents. Agents that committed per-chunk lost 0 work. Agents that accumulated lost everything.
- Rule 2 catches regressions before they compound. Without it, agents make 5 changes, one regresses, and you can't tell which.
- Rule 3 is the only way the parent knows what happened without reading 700-line transcripts.
- Rule 4 is fast (<10s) and catches compile errors immediately.
- Rule 5 eliminated the #1 source of merge conflicts (3 agents touching color-resolver.ts simultaneously).
- Rule 6 made agents 26x more effective per change (measured: PDF oracle vs RMSE guessing).

### 3. Fan-Out Execution Pattern

#### Pre-launch checklist

```bash
# 1. Clean state
git status --short  # must be clean
git worktree list   # must be just main + any permanent worktrees

# 2. GC protection
git config gc.auto 0
git config gc.reflogExpire 90.days
git config gc.reflogExpireUnreachable 90.days

# 3. Initialize progress tracker
echo '{"session_start":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'","agents":[]}' > test-data/ground-truth/agent-progress.jsonl

# 4. Verify login
# If using Claude Code, ensure session is fresh
```

#### Launch pattern

- **Use worktree isolation for ALL agents** — prevents cross-agent file conflicts entirely
- **Launch all agents in a single message** — maximizes parallelism
- **Use `run_in_background: true`** — don't block the parent
- **Scope agents to non-overlapping file sets** — even with worktree isolation, this prevents merge conflicts later
- **List shared danger files** explicitly in each prompt with "don't touch" instructions

#### File conflict zones (project-specific, but the pattern is universal)

Identify files that multiple agents might want to modify and explicitly assign ownership:

| File | Risk | Mitigation |
|------|------|------------|
| IR type definitions | High — every renderer imports | Freeze during fanout. If agent needs new types, document in a comment. |
| Color/theme resolver | High — rendering + parsing both use | Assign to at most 1 agent. |
| Auto-generated bundles | High — binary files that don't merge | Never edit manually. Regenerate after fanout. |
| Shared test fixtures | Medium — multiple agents may add tests | Append-only patterns (JSONL) merge cleanly. |

### 4. Recovery Protocol (when login expires)

Execute immediately after re-authenticating:

```bash
# Step 1: Find surviving worktrees
git worktree list
for wt in .claude/worktrees/agent-*/; do
  echo "=== $(basename $wt) ==="
  (cd "$wt" && git status --short && git log --oneline -3)
done

# Step 2: Check main working tree for uncommitted agent work
git status --short

# Step 3: Find dangling commits (the gold mine)
git fsck --no-reflogs --unreachable --no-progress 2>/dev/null \
  | grep "^unreachable commit" \
  | while read _ _ hash; do
      echo "$(git log --format='%H %ai %s' -1 "$hash")"
    done \
  | sort -k2 -r \
  | grep -v "WIP on\|index on\|Merge branch"

# Step 4: Triage each dangling commit
for hash in <list>; do
  # Check if already on a branch
  on_main=$(git merge-base --is-ancestor $hash main 2>/dev/null && echo "YES" || echo "no")
  msg=$(git log --format='%s' -1 $hash)
  cherry=$(git log --oneline main | grep -F "$msg" | head -1)
  echo "$hash on_main=$on_main cherry=${cherry:-none} $msg"
done

# Step 5: Cherry-pick recovered commits
git cherry-pick <hash>  # one at a time, test between each

# Step 6: Prune and verify
git worktree prune
git worktree list  # should be clean
```

**Key insight:** `git fsck --unreachable` finds commits that agents made before their branches were pruned. These are fully recoverable via `git cherry-pick`. In our experience, 100% of committed agent work was recoverable through this method — the only true loss was uncommitted work (which Rule 1 above minimizes).

### 5. Post-Fanout Merge Strategy

After all agents complete (or are recovered):

```bash
# 1. List all worktree branches with commits
git worktree list

# 2. Cherry-pick each, test between merges
for branch in $(git worktree list | grep worktree-agent | awk '{print $NF}' | tr -d '[]'); do
  echo "=== Merging $branch ==="
  commits=$(git log --oneline main..$branch | tac)
  while read hash msg; do
    git cherry-pick $hash
    pnpm --filter @opendockit/core test  # fast package test
  done <<< "$commits"
done

# 3. Full test suite after all merges
pnpm test

# 4. Baseline comparison
node scripts/full-fidelity-baseline.mjs --compare test-data/ground-truth/full-baseline-report.json

# 5. Update baselines
pnpm test:fidelity:baseline

# 6. Clean up
git worktree prune
git gc --prune=now
```

### 6. Metrics That Matter

Track across sessions to measure agent orchestration effectiveness:

| Metric | How to measure | Target |
|--------|---------------|--------|
| **Work survival rate** | Commits recovered / commits attempted | >95% |
| **Agent productivity** | RMSE delta per agent-hour | Improving trend |
| **Merge conflict rate** | Conflicted cherry-picks / total cherry-picks | <10% |
| **Recovery time** | Minutes from login failure to full recovery | <5 min |
| **Fanout efficiency** | Useful commits / total agents launched | >80% |

---

## Anti-Patterns (learned the hard way)

### 1. "Big bang commit at the end"
**What happens:** Agent does 30 minutes of work, accumulates all changes, tries to commit at the end. Login expires at minute 29. Total loss.
**Fix:** Commit after every logical chunk. One fix = one commit.

### 2. "Two agents, same color resolver"
**What happens:** Two agents independently refactor color-resolver.ts with incompatible approaches. Both produce working code. Cherry-picking the second creates merge conflicts. Manual resolution takes longer than the original work.
**Fix:** Identify shared danger files before launch. Assign ownership or freeze them.

### 3. "Agent completed successfully" (but actually didn't)
**What happens:** Login expires. Agent reports `status: completed` with result `"Not logged in"`. Parent assumes success. Worktree gets pruned. Work disappears.
**Fix:** Always check agent results for "Not logged in" or empty results. Run `git fsck --unreachable` after any login incident. Never trust completion status alone.

### 4. "Optimistic merge — tests later"
**What happens:** Cherry-pick 8 agents' work without testing between merges. Test suite fails with 15 failures. Can't tell which agent caused which failure.
**Fix:** Cherry-pick one agent at a time. Run package tests after each. If tests fail, fix or revert before continuing.

### 5. "Agent guessing at rendering fixes"
**What happens:** Agent sees visual gap, hypothesizes root cause, writes fix, RMSE gets worse. Tries another hypothesis. Cycle repeats.
**Fix:** Use an oracle. In our case, the PDF DOM provides verified-correct positions, fonts, sizes, and colors. Agents using the oracle were 26x more effective per change than agents guessing.

---

## Recommended Rebar Enhancements — Toward Swarm Collective Intelligence

### Coordination Layer (immediate)

#### P0: Agent health signal
Agents should report structured health data (heartbeat, metrics, progress) through a first-class API rather than writing to temp files. The parent should receive notifications for:
- Agent committed (with commit hash + message)
- Agent metric (key-value pair)
- Agent stalled (no activity for N seconds)

#### P1: Incremental commit protocol
Support a `commit_on_checkpoint` agent configuration that automatically commits after each tool call sequence. This eliminates Rule 1 entirely — agents can't forget to checkpoint.

#### P2: Login expiration warning
Before the session expires, warn agents (via a system message) that they have N minutes remaining. Agents can then do a final checkpoint commit. Currently agents die silently mid-tool-call.

#### P3: Dangling commit recovery built-in
After login recovery, automatically run `git fsck --unreachable` and present recoverable commits to the user, rather than requiring manual recovery.

### Observability Layer (high value)

#### P4: Swarm progress dashboard
A built-in way to see all running agents, their last heartbeat, last commit, current file, and cumulative metrics. Currently this requires polling temp files. Should be a live view, not a log.

#### P5: Shared file conflict detection at launch
When launching multiple worktree agents, warn if their prompts reference overlapping files. "Agents P-1 and D-3 both mention table-renderer.ts — merge conflicts likely." This is static analysis of prompts, not runtime detection.

### Collective Learning Layer (the real vision)

#### P6: Cross-agent knowledge propagation
When an agent discovers something useful (e.g., "the PDF DOM shows Arial ascender is 8.74pt, not 10.86pt"), that finding should be broadcastable to sibling agents in the same swarm. Today this requires the parent to manually relay findings. Rebar should support:
- `agent_broadcast("finding", "Arial ascender is 8.74pt per PDF DOM")` — sends to all agents in the swarm
- `agent_subscribe("findings")` — agent opts in to receiving sibling broadcasts
- Findings persisted across sessions via the memory system

#### P7: Cross-repo swarm memory
Agent learnings from one repo should be accessible when working in a sibling repo. Example: FontKit agent discovers that Carlito and Calibri are truly metric-compatible (2110/2112 codepoints identical). OpenDocKit agents should be able to query this finding without re-deriving it. Rebar's memory system should support cross-project memory namespaces.

#### P8: Role-aware task routing
Instead of the parent manually constructing 10 agent prompts, Rebar should support role definitions:
```yaml
roles:
  fidelity-agent:
    protocol: [commit-per-chunk, measure-before-after, pdf-oracle]
    danger-files: [ir/index.ts, color-resolver.ts]
    gate: "RMSE must improve or stay stable"
  test-agent:
    protocol: [commit-per-batch, full-suite-after-each]
    gate: "skip count must decrease"
```
Then launching is: `rebar fanout --roles fidelity-agent:8 test-agent:1 infra-agent:1`

#### P9: Failure pattern library
Every agent failure (login, merge conflict, regression, stall) should be automatically classified and stored. Over time, Rebar builds a failure taxonomy:
- "Login expiration during long-running fidelity test" → mitigation: shorter test cycles
- "Merge conflict on color-resolver.ts" → mitigation: assign single owner
- "RMSE regression after font metrics change" → mitigation: always measure before/after

New fan-outs automatically incorporate mitigations from the failure library.

#### P10: Swarm retrospective
After a fan-out completes, Rebar should auto-generate a retrospective:
- Which agents produced the most value per hour?
- Which files had the most merge conflicts?
- Which agents' work was superseded by other agents?
- What findings were discovered that should become permanent knowledge?

This closes the loop: swarm runs → retrospective → updated protocols → next swarm runs better.
