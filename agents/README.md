# Agent Orchestration

Role-based agents with commands, subagent templates for delegation, and
structured prompt templates for parallel fan-out.

See the [root README](../README.md) for how agents fit into the overall system.

## Quick decision tree

| You want… | Use | Example |
|-----------|-----|---------|
| **An answer to a question** | Role-based agent (`ask <role>`) | `ask architect "should this contract handle encryption?"` |
| **A focused task done** | Subagent template (`agents/subagent-prompts/*.md`) | Code review, security scan, contract audit |
| **Cross-cutting decision** | A sequence of role calls | `ask architect` → `ask product` → `ask englead` |
| **Parallel work distribution** | Subagent templates fanned out across worktrees | See `practices/multi-agent-orchestration.md` |

**Rule of thumb:** Questions accumulate knowledge in a persistent ASK
session (10 questions cost 1× context). Subagent templates execute
ephemeral, focused work. Use both — they're complementary, not
alternatives.

## The 6 core roles

| Role | Owns | Best for |
|------|------|----------|
| 🏗️ **architect** | Contracts, system design, integration patterns | "Should we add caching here?" "Review CONTRACT:S3-AUTH for integration issues." |
| 📋 **product** | BDD scenarios, personas, requirements gaps | "Does this contract serve the user need?" "What scenarios are missing?" |
| 👥 **englead** | Delivery coordination, QA flow, fan-out planning | "Are we ready to ship?" "Coordinate these 3 parallel branches." |
| 🔍 **steward** | Automated quality scanning, contract health | `ask steward summary`, `ask steward check C1` |
| 🧪 **tester** | Test strategy, coverage, fidelity ladder | "What's the coverage gap?" "E2E plan for this flow?" |
| 🔄 **merger** | Branch integration, conflict resolution (actor) | "Merge worktree-X and worktree-Y onto main." |

Each role has its own `AGENT.md` (role definition), an optional
`commands/` directory for unquoted invocations (e.g. `ask steward summary`
→ `commands/summary.sh`), and a memory file (`memory.md`) that persists
across sessions.

## Directory Layout

```
agents/
  README.md                    # this file
  subagent-guidelines.md       # shared behavioral rules all subagents follow
  subagent-prompts-index.md    # catalog of available templates
  subagent-prompts/            # one .md per template
    code-review.md             # multi-dimension code review
    contract-audit.md          # interface conformance check
    security-surface-scan.md   # security audit
    ux-review.md               # UX, accessibility, responsive
    doc-drift-detector.md      # doc-vs-code consistency
    feature-inventory.md       # behavioral inventory for safe refactoring
    test-shard-runner.md       # parallel test execution
    merge-coordinator.md       # post-worktree merge coordination
  architect/                   # Architect agent
    AGENT.md                   #   role definition
    commands/                  #   audit, default
  product/                     # Product agent
    AGENT.md
    commands/                  #   gaps, default
  englead/                     # Engineering Lead agent
    AGENT.md
    commands/                  #   check, qa, default
  steward/                     # Steward agent (quality scanner)
    AGENT.md
    commands/                  #   scan, json, summary, check, default
  merger/                      # Merger agent (branch integration)
    AGENT.md
    commands/                  #   default, report
  findings/                    # architectural findings from subagents
  results/                     # subagent output files
```

## Agent Commands

Each agent can have a `commands/` directory containing executable scripts.
These are invoked via ASK with unquoted words:

```bash
ask steward              # runs commands/default.sh (full scan)
ask steward summary      # runs commands/summary.sh
ask architect            # runs commands/default.sh (contract audit)
ask englead qa           # runs commands/qa.sh (steward + enforcement)
ask product              # runs commands/default.sh (gap analysis)
```

To add a command: create `agents/<role>/commands/<name>.sh`, make it
executable. It's immediately available as `ask <role> <name>`. The second
line of the script (after the shebang) is used as the description in help
output — start it with `# `.

## Actor Agents

Most agents are **readers** — they answer questions about the project
(architect, product, steward). Some agents are **actors** — they perform
actions on the repository.

| Agent | Type | What It Does |
|-------|------|-------------|
| architect | Reader | Answers architecture questions, audits contracts |
| product | Reader | Answers product questions, finds gaps |
| englead | Reader + Actor | Answers questions, runs enforcement checks |
| steward | Reader | Scans quality, produces reports |
| merger | Actor | Merges branches, resolves conflicts, produces reports |

Actor agents are invoked the same way as readers, but they modify state:

```bash
# Reader: answers a question
ask architect "what contracts are in DRAFT?"

# Actor: performs an action
ask merger "merge worktree-agent-abc worktree-agent-def onto main"

# Actor: performs action, then answer follow-up
ask merger "what conflicts did you resolve?"
```

## Subagent Templates

Reusable prompt templates for delegating work to subagents — both single
invocations and parallel fan-out.

### Single Invocation

Point one subagent at a template for a task done *your way*:

```
Agent(prompt: "Read agents/subagent-guidelines.md for behavioral rules.
              Read agents/subagent-prompts/ux-review.md for your task.
              Parameters: TARGET=client/src/components/ SCOPE=full")
```

### Parallel Fan-Out

Same template, N agents, different parameters:

```
Agent(prompt: "Read agents/subagent-guidelines.md.
              Read agents/subagent-prompts/test-shard-runner.md.
              Parameters: SHARD=0 SHARD_SIZE=500 OUTPUT=agents/results/shard-00.json",
      isolation: "worktree")
```

### Why Templates Matter

When you ask an agent to do a "UX review" without a template, it guesses what
you mean. A template encodes your definition — your criteria, your heuristics,
your output format. **If you've ever corrected an agent, that correction
belongs in a template.** This is how agents learn across sessions.

## Design Principles

1. **Progressive disclosure** — Template body <300 lines. Move reference
   material to supporting files.
2. **Declarative, not procedural** — Describe the task, inputs, outputs, and
   success criteria. Let the agent decide how.
3. **Explicit output format** — Specify JSON schema or markdown structure so
   the orchestrator can parse results.
4. **Context files as parameters** — Point to existing docs rather than
   inlining domain knowledge.
5. **Testable success criteria** — "Output file exists and is valid JSON" is
   testable. "Do a good review" is not.
6. **Include anti-patterns** — If agents consistently make a mistake on this
   task, say so explicitly.
