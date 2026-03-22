# Agent Orchestration Templates

Structured prompt templates for Claude Code subagent delegation — both
single-invocation specialized tasks and parallel fan-out.

## Concept

Instead of crafting one-off subagent prompts inline, we version-control
reusable prompt templates with parameterized slots. The orchestrating agent
(or human):

1. Picks a template from `subagent-prompts/`
2. Reads `subagent-guidelines.md` for shared behavioral contracts
3. Passes runtime parameters (file list, shard range, target branch, etc.)

This gives us **reviewable, composable, repeatable** agent orchestration.

## Directory Layout

```
agents/
  README.md                    # this file
  subagent-guidelines.md       # shared behavioral rules all subagents follow
  subagent-prompts-index.md    # catalog of available templates
  subagent-prompts/            # one .md per template
    _example-template.md       # annotated example showing the format
    ux-review.md               # UX audit — accessibility, interaction, responsive, visual
    security-surface-scan.md   # security audit — crypto, auth, injection, data exposure
    code-review.md             # multi-dimension code review — correctness, perf, security
    contract-audit.md          # interface conformance — methods, behavior, error contracts
    doc-drift-detector.md      # doc-vs-code consistency check
    feature-inventory.md       # behavioral inventory — prerequisite for worktree delegation
    test-shard-runner.md       # parallel test execution across shards
  results/                     # subagent output files (gitignored or ephemeral)
  findings/                    # architectural/security findings from subagents
```

## Two Usage Modes

### Single Invocation (Specialized Task)

Point one subagent at a template for a task you want done *your way*:

```
Agent(prompt: "Read agents/subagent-guidelines.md for behavioral rules.
              Read agents/subagent-prompts/ux-review.md for your task.
              Parameters: TARGET=client/src/components/ SCOPE=full",
      subagent_type: "general-purpose")
```

**Why this matters:** When you ask an agent to do a "UX review" without a
template, it guesses what you mean. The template encodes *your* definition —
your criteria, your heuristics, your output format. If you've ever corrected
an agent ("no, not like that"), that correction belongs in a template.

### Parallel Fan-Out (Sharded Work)

Same template, N agents, different parameters:

```
Agent(prompt: "Read agents/subagent-guidelines.md for behavioral rules.
              Read agents/subagent-prompts/test-shard-runner.md for your task.
              Parameters: SHARD=0 SHARD_SIZE=500 OUTPUT=agents/results/shard-00.json",
      isolation: "worktree")
```

Launch 10-20 of these in a single message with different shard parameters.

## Relationship to Claude Skills

Skills (`.claude/skills/<name>/SKILL.md`) and subagent templates serve
complementary purposes:

| | **Skills** | **Subagent Templates** |
|---|---|---|
| **Who invokes** | User (`/skill-name`) or Claude auto-trigger | Orchestrating agent delegates |
| **Context** | Main conversation (or forked) | Always separate subagent |
| **Best for** | User-facing workflows | Agent-to-agent delegation |
| **Fan-out** | One invocation | N parallel invocations |
| **Discovery** | Auto-discovered by framework | Manual (orchestrator reads index) |

**Rule of thumb:** Skills are the button the user presses. Templates are the
instruction manual the worker reads.

A skill *can orchestrate* template-driven fan-out — a `/fan-out` skill that
reads a template, computes shard boundaries, and launches N subagents.

## Design Principles

Borrowed from Claude Skills best practices, adapted for subagent delegation:

1. **Progressive disclosure** — Template body is concise (<300 lines). Move
   reference material, checklists, and examples to supporting files.
2. **Declarative, not procedural** — Describe the task, inputs, outputs, and
   success criteria. Let the agent decide *how*.
3. **Explicit output format** — The orchestrator needs to parse results.
   Specify JSON schema, markdown structure, or whatever enables aggregation.
4. **Context files as parameters** — Point to existing project docs rather
   than inlining domain knowledge. Prevents template drift.
5. **Testable success criteria** — "Output file exists and is valid JSON" is
   testable. "Do a good review" is not.
6. **Include anti-patterns** — If agents consistently make a mistake on this
   task, say so explicitly.
