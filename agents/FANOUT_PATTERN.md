# Fanout Pattern — Parallel Subagents Without Merge Conflicts

**Status:** active — load-bearing doctrine for multi-subagent fanout
**Source:** [`feedback/2026-04-28-multi-subagent-fanout-playbook.md`](../feedback/2026-04-28-multi-subagent-fanout-playbook.md)
**Provenance:** dapple-sdk, 2026-04-28 — two fanout cycles in one ~16hr
session, 9 fanned-out worktree branches, all merged with zero conflict
resolution; ~12–16 hours of sequential work landed in ~6–8 hours at the
same correctness bar.
**Audience:** the orchestrating (parent) agent. Subagents read
[`subagent-guidelines.md`](subagent-guidelines.md) instead.

This document codifies the fanout shape so no project re-derives it —
or wings it and merge-conflicts its way through. The deep mechanics
live elsewhere and are referenced, not duplicated:

- Pre-launch audit, GC protection, agent health, recovery protocol —
  [`practices/multi-agent-orchestration.md`](../practices/multi-agent-orchestration.md)
- Conflict zones, merge ordering, cherry-pick resolution —
  [`practices/worktree-collaboration.md`](../practices/worktree-collaboration.md)
- Subagent behavioral contract (The 10 Rules, verify-before-relying) —
  [`subagent-guidelines.md`](subagent-guidelines.md)

---

## The shape in one glance

```
Move 0  decide whether to fan out at all          ← parent
Move 1  dependency graph + file-ownership table   ← parent
Move 2  one worktree + one branch per chunk       ← parent creates
Move 3  dispatch: allowlist + do-not-touch +      ← parent prompts
        verify-before-relying clause
        (subagents work in parallel: verify brief,
         work, commit to own branch — no push, no
         branch switch, no main worktree)
Move 4  merge sequentially, --no-ff, test between ← parent
Move 5  post-merge sweep of shared surfaces,      ← parent, one commit
        then worktree prune
```

Every move that touches shared state belongs to the parent. Subagents
own exactly their allowlisted files on exactly their own branch —
nothing else. That division is why the merges are boring.

---

## Move 0 — Decide whether to fan out at all

Fanout pays when 4–6 chunks of work have genuinely disjoint file
footprints. It is not the default for all work.

### When NOT to fan out

| Signal | Why | Do instead |
|--------|-----|------------|
| **Security-critical code path** — crypto touchpoints, key derivation, authn/authz boundaries | These warrant single-thread care; a subagent inherits every error in your brief and executes it at full speed | Parent does it with full attention (in a worktree if isolating from other parallel work) |
| **Prompt longer than the expected output** | A 5-minute manual edit becomes a 30k-token briefing; overhead exceeds the work | Just do it yourself |
| **Shared mutable state that can't be isolated** — two specs hitting the same Postgres rows, browser cookies, or running rig | Parallel runs corrupt each other nondeterministically; failures won't reproduce | Sequence the chunks, or give each isolated test data on the shared rig |

[`practices/worktree-collaboration.md`](../practices/worktree-collaboration.md)
adds three more never-cases: single-shared-file changes, work needing
real-time coordination, and unclear scope.

---

## Move 1 — Dependency graph BEFORE dispatch

Before creating any worktree, write down the ownership table:

```
WT-A: owns files X, Y      (creates 2, modifies 1)
WT-B: owns files Z, W      (creates 4)
WT-C: owns files V         (modifies 1)
→ no overlap. Safe to parallelize.
```

- **Any overlap** → combine the chunks into one branch, or sequence
  them. Never "they'll probably touch different sections."
- **Interface additions go first.** One agent adds the shared method;
  everyone else branches after that merge lands (see merge ordering in
  [`practices/worktree-collaboration.md`](../practices/worktree-collaboration.md)).
- **Shared surfaces appear in NO branch's list.** CHANGELOG,
  QUICKCONTEXT, README, metrics/status docs are reserved for the
  parent's post-merge sweep (Move 5).
- Run the full pre-launch audit in
  [`practices/multi-agent-orchestration.md`](../practices/multi-agent-orchestration.md)
  — grep for existing implementations, verify docs against code, set
  GC protection.

The table is not bureaucracy — writing "WT-A owns these files, WT-B
owns those, no overlap" surfaces accidental couplings at planning time
instead of at merge time. If you can't write the table, you're not
ready to dispatch.

---

## Move 2 — One worktree, one branch, per chunk

The parent creates every worktree and branch. Subagents never create
their own isolation.

Three entry points, in order of preference:

1. **`rebar agent start`** (rebar-adopting repos) — wraps worktree
   creation, role permissions, and the sealed-envelope audit that
   `rebar agent finish` closes (Rules 1 and 10 in
   [`subagent-guidelines.md`](subagent-guidelines.md)).
2. **Harness isolation** — the Agent tool's `isolation: "worktree"`
   flag.
3. **Raw `git worktree add`** — the fallback below, when the harness
   flag fails.

### Worktree-isolation fallback

The harness `isolation: "worktree"` flag fails when the harness
doesn't detect the parent as a git workspace — observed on dapple-sdk
even though git itself worked fine. Don't fight the harness; go under
it:

```bash
# parent, from the repo root — once per chunk
git worktree add ../<proj>-wt-<name> -b feature/<name> main
```

Then dispatch each subagent with its `cwd` set to the worktree path.
The subagent contract is unchanged: commit to your own branch, don't
push, don't switch branches, never touch the main worktree. After all
branches merge (Move 4):

```bash
git worktree remove ../<proj>-wt-<name>   # per worktree
git worktree prune
git worktree list                          # must show only the main tree
```

---

## Move 3 — Dispatch with a strict allowlist

Every fanout prompt contains, non-negotiably:

1. **File allowlist** — exactly what this subagent may create/modify
   (Rule 3). Everything else is read-only.
2. **Explicit do-NOT-touch list** — the shared/hot files near its work
   (Rule 8), so "adjacent" never becomes "mine."
3. **Recent changes** — what moved on main since the worktree branched
   (Rule 9).
4. **Template reference + parameters** — which
   `agents/subagent-prompts/*.md` governs the task.
5. **The verify-before-relying clause** — verbatim, from
   [`subagent-guidelines.md`](subagent-guidelines.md) §Verify Before
   Relying.

Item 5 is the insurance policy. Parent briefs are recall, not ground
truth; without the clause, fanout multiplies the cost of every factual
error in the parent's planning. With it, fanout is self-correcting —
on dapple-sdk, three of nine subagents caught real errors in their
briefs (a hallucinated script path, unshipped speculation described as
shipped) and deviated correctly instead of building on them.

The same discipline binds the parent on the way back: subagent reports
of pre-existing state ("X already exists," "tests already pass") are
claims to re-verify, not facts to build on.

### Fanning out test writers

When the chunks are test specs, add the max-fidelity brief: **"MAX
FIDELITY: do not gate the spec on `test.skip(!BACKEND_REACHABLE)` or
any similar bailout. If the rig isn't running, fail loud."** A spec
that skips when the backend is down verifies nothing and reports
green. This was load-bearing on dapple-sdk — the cross-device-recovery
spec caught two real production bugs precisely because it didn't skip.
Each spec subagent runs against the shared rig with isolated test
data; if the data can't be isolated, that's the Move-0 shared-state
rule — sequence instead. Fidelity tiers are defined in
`practices/test-fidelity.md`.

---

## Move 4 — Parent-owned sequential merge

After **all** subagents complete — not as each finishes:

```bash
git merge --no-ff feature/<name>   # one branch at a time
# run the affected package's tests before merging the next branch
```

Merge order and conflict handling follow
[`practices/worktree-collaboration.md`](../practices/worktree-collaboration.md):
most-interface-changes first, new-files-only next, shared-file
modifiers last. Never optimistic-merge (all branches, test later) —
when the suite then fails, you can't tell which branch broke it. The
orchestrator's per-merge integration checklist lives in
[`practices/multi-agent-orchestration.md`](../practices/multi-agent-orchestration.md).

---

## Move 5 — Parent-owned post-merge sweep

The shared surfaces every branch would have fought over — CHANGELOG,
QUICKCONTEXT, README, metrics/status docs — get updated once, by the
parent, in a single commit after all merges land. One writer, zero
conflicts, by construction.

Then clean up: remove worktrees, `git worktree prune`, verify
`git worktree list` shows only the main tree, and restore GC defaults
if you changed them at session start.

---

## See also

- [`subagent-guidelines.md`](subagent-guidelines.md) — the behavioral
  contract every subagent reads first
- [`practices/multi-agent-orchestration.md`](../practices/multi-agent-orchestration.md)
  — pre-launch audit, GC protection, health signals, recovery protocol
- [`practices/worktree-collaboration.md`](../practices/worktree-collaboration.md)
  — conflict zones, merge ordering, cherry-pick resolution
- [`feedback/2026-04-28-multi-subagent-fanout-playbook.md`](../feedback/2026-04-28-multi-subagent-fanout-playbook.md)
  — the field report this doctrine distills
