# Profile: Solo Developer

For individual developers or very small projects (1-3 repos).
Rebar's lightest configuration — zero overhead beyond what helps you directly.

## Recommended Tier: 1 (Partial)

Set in `.rebarrc`:
```
tier = 1
```

Only `contract-refs` and `TODO` tracking are enforced. Everything else is available but optional.

## What to Copy

| File | Priority | Notes |
|------|----------|-------|
| `README.template.md` | Required | Your project's front door |
| `QUICKCONTEXT.template.md` | Required | Where you are right now |
| `TODO.template.md` | Required | What needs doing |
| `AGENTS.template.md` | Required | Already slim — use as-is |
| `CLAUDE.template.md` | Required | Agent configuration |
| `DESIGN.md` | Optional | Read once, reference later |

## What to Skip

- **`agents/` directory** — you're the only agent. Add later if you start using multi-agent.
- **`practices/` files** — read if you need them, but don't set up infrastructure for them.
- **Most enforcement scripts** — `check-todos.sh` and `check-contract-refs.sh` are all you need at Tier 1.
- **`METRICS` file** — you know your project. Add when the numbers start mattering.
- **Steward** — useful once you have 5+ contracts. Before that, just grep.

## What to Use

- **Cold Start Quad** — README, QUICKCONTEXT, TODO, AGENTS. Read them each session.
- **Contract headers** — `// CONTRACT:C1-WHATEVER.1.0` in every source file. This costs nothing and gives you grep-based discovery instantly.
- **Two-tag TODOs** — `TODO:` for stuff you need to track, convert to `TRACKED-TASK:` after adding to TODO.md.
- **`ask` CLI** — even solo, `ask architect "should I split this?"` saves context and gives focused answers.

## When to Level Up

Move to [small-team.md](small-team.md) when:
- A second developer joins
- You have 5+ contracts and want automated health checks
- You're about to ship to users and want enforcement gates
- You find yourself losing track of what's documented vs. what's real

## Setup Time

~15 minutes. Copy the Cold Start Quad, fill in your project details, start coding.
