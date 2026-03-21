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

This takes 2-3 minutes and prevents hours of wasted agent compute.

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
