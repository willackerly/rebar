# agent-templates

A complete starter kit for contract-driven, agent-powered software development.

---

### So you're a human:

Read [methodology.md](methodology.md) for the philosophy, then
[SETUP.md](SETUP.md) to adopt it in your project. Pick your
[project profile](profiles/) to know what to copy and what to skip.

### So you're an agent:

**a) Starting a new project?**
Read [methodology.md](methodology.md) first — contracts are the operating
system, everything flows from there. Then follow [SETUP.md](SETUP.md)
step by step. Pick the [profile](profiles/) that matches your project type.
Copy the templates, customize per the `<!-- comments -->`, write your first
contracts in `architecture/`, then start building.

**b) Aligning an existing project to this standard?**
Read [methodology.md](methodology.md), then diff your existing docs against
the templates. Start with the highest-leverage gaps: add `CONTRACT:` headers
to source files, create `architecture/` with your most important interface
contracts, adopt the [Cold Start Quad](#the-cold-start-quad) reading order.
You don't have to adopt everything at once — see your [profile](profiles/)
for what matters most for your project type.

**c) Improving this methodology based on your learnings?**
Drop a file in [feedback/](feedback/) describing what you learned, what's
missing, or what didn't work. Include the project type, the situation, and
a concrete suggestion. If you have domain-specific expertise (security,
distributed systems, ML pipelines, etc.) that should inform the templates
or methodology, document it as feedback — the maintainers will integrate
what's broadly applicable.

---

## What's Included

### The Methodology

| File | Purpose |
|------|---------|
| [methodology.md](methodology.md) | **The philosophy.** Contract-driven development, BDD-first, agent autonomy model, anti-drift mechanisms. Read this to understand *why* everything else is structured the way it is. |

### The Cold Start Quad

Four files every agent reads on session start, in order:

| Order | Template | Purpose |
|-------|----------|---------|
| 1 | [README.template.md](README.template.md) | Universal first-read — project orientation, architecture overview, cold start instructions |
| 2 | [QUICKCONTEXT.template.md](QUICKCONTEXT.template.md) | What's true right now — branch, test counts, active work, blockers |
| 3 | [TODO.template.md](TODO.template.md) | Tasks + known issues + blockers (two-tag tracking system) |
| 4 | [AGENTS.template.md](AGENTS.template.md) | How we work — norms, testing cascade, contracts, agent collaboration |

Plus: [CLAUDE.template.md](CLAUDE.template.md) — Claude Code-specific configuration.

### The Contract System

| File | Purpose |
|------|---------|
| [architecture/README.md](architecture/README.md) | How the contract system works — naming, linking, versioning |
| [architecture/CONTRACT-TEMPLATE.md](architecture/CONTRACT-TEMPLATE.md) | Annotated template for new contracts |
| [architecture/CONTRACT-REGISTRY.template.md](architecture/CONTRACT-REGISTRY.template.md) | Index of all contracts |

### Agent Orchestration

| Directory | Purpose |
|-----------|---------|
| [agents/](agents/) | Subagent guidelines, prompt index, and templates |
| [agents/subagent-prompts/](agents/subagent-prompts/) | UX review, security scan, code review, contract audit, doc drift, feature inventory, test sharding |

### Project Profiles

Different projects need different subsets. Pick your profile:

| Profile | Best For |
|---------|----------|
| [web-app](profiles/web-app.md) | SPA, SSR, web frontend + API |
| [api-service](profiles/api-service.md) | Backend API, microservice |
| [crypto-library](profiles/crypto-library.md) | Security-critical library |
| [cli-tool](profiles/cli-tool.md) | Command-line tool |

### Supporting

| Directory | Purpose |
|-----------|---------|
| [feedback/](feedback/) | Lightweight feedback mechanism — agents drop suggestions here |
| [profiles/](profiles/) | Project-type adoption guides |
| [learnings-from-opendockit.md](learnings-from-opendockit.md) | Battle-tested patterns and failure analysis from 5,800+ tests and 9 simultaneous agents |

## Quick Start

See [SETUP.md](SETUP.md) for the full adoption guide. The short version:

```bash
PROJECT=/path/to/your/project

# Core docs
cp README.template.md       "$PROJECT/README.md"
cp QUICKCONTEXT.template.md "$PROJECT/QUICKCONTEXT.md"
cp TODO.template.md         "$PROJECT/TODO.md"
cp AGENTS.template.md       "$PROJECT/AGENTS.md"
cp CLAUDE.template.md       "$PROJECT/CLAUDE.md"
cp methodology.md           "$PROJECT/methodology.md"

# Contract system
cp -r architecture/         "$PROJECT/architecture/"

# Agent orchestration (optional)
cp -r agents/               "$PROJECT/agents/"

# Customize everything (follow the <!-- comments --> in each template)
```

## Design Philosophy

See [methodology.md](methodology.md) for the complete philosophy. In brief:

1. **Contracts are the operating system** — don't implement without a contract
2. **BDD first** — encode who and why before writing contracts
3. **Max autonomy within contracts** — agents are unrestricted inside contract boundaries
4. **Trust but verify** — freshness markers, pre-launch audits, filesystem as truth
5. **Encode corrections as infrastructure** — if you've corrected an agent, put it in a template
6. **Fast inner loops** — Testing Cascade T0-T5, iterate at the speed of a single test

## Design Decisions

This repo was built iteratively — each decision emerged from the last. If
you're evaluating whether to adopt it, these are the non-obvious choices
and why we made them.

### Why contracts became the center, not docs or tests

We started with documentation templates (QUICKCONTEXT, AGENTS, TODO) and they
worked — agents oriented faster, drift decreased. But we kept hitting the same
failure mode: agents would change code that *technically worked* but violated
the architectural intent. They'd refactor a function in a way that broke an
implicit contract with another module, or add a feature that contradicted a
design decision made months ago.

The problem wasn't missing docs. It was that no document answered the question
"what is this code *supposed to do* according to the architecture?" Tests
answer "does it work?" Code answers "what does it do right now?" But neither
answers "what was intended, and what boundaries must be respected?" That's
what contracts do. Once we made contracts explicit, versioned, and
doubly-linked to code, agents stopped making locally-correct but
globally-wrong decisions.

### Why grep-based linking over tooling

The contract system uses `// CONTRACT:C1-BLOBSTORE.2.1` in code headers and
`grep -rn "CONTRACT:C1-BLOBSTORE"` to find implementations. No build plugins,
no custom linters, no databases. This is deliberate:

- **Zero adoption cost.** Any project can start using it immediately.
- **Tool-agnostic.** Works with any editor, any AI agent, any CI system.
- **Transparent.** The linking mechanism is visible in the code itself.
- **Resilient.** No tool to break, update, or configure.

A dedicated tool could provide richer features (link validation, dependency
graphs, automatic registry updates), but the value of the contract system
comes from the *practice* of writing and referencing contracts, not from
tooling. Start with grep. Add tooling later if the scale demands it.

### Why subagent templates matter for single invocations, not just fan-out

We originally built the `agents/subagent-prompts/` system for parallel
fan-out — sharding test runs across 20 agents, auditing packages in parallel.
That works well, but the bigger insight was simpler: **templates are just as
valuable when you invoke a single agent for a single task.**

When you ask an agent to do a "UX review" without a template, it uses its
general knowledge of what UX reviews cover. That general knowledge may not
match your standards — it might skip accessibility, or not check against your
design system, or format findings differently each time. A `ux-review.md`
template encodes *your* definition: your criteria, your heuristics, your
output format. The agent doesn't guess.

The pattern generalizes: **if you've ever corrected an agent ("no, not like
that — here's how we do X"), that correction belongs in a template.** Next
time, the agent reads the template and gets it right. This is how agents
learn across sessions.

### Why TODO absorbed KNOWN_ISSUES

We originally had five core files: QUICKCONTEXT, KNOWN_ISSUES, TODO, AGENTS,
CLAUDE. In practice, KNOWN_ISSUES and TODO had overlapping concerns (both
track "things that need attention") and agents had to maintain both. Every
additional file is a drift surface — a place where reality and documentation
can diverge. Merging known issues into TODO as a section (blockers, gotchas,
workarounds) reduced the maintenance burden without losing information.

The principle: fewer files that are actually maintained beats more files that
drift.

### Why README is the universal first-read

Previously, QUICKCONTEXT was the first file agents read. But QUICKCONTEXT
answers "what's happening now" — it doesn't answer "what is this project?"
An agent that jumps straight into current state without understanding the
project's identity, architecture, and core tenets makes worse decisions.

README provides orientation: what the project is, how it's structured, what
the core tenets are, where the contracts live. It's stable (changes rarely)
and foundational (everything else depends on understanding it). QUICKCONTEXT
is volatile and tactical. The reading order should go from stable/strategic
to volatile/tactical: README → QUICKCONTEXT → TODO → AGENTS.

### Why profiles exist

A crypto library team and a web app team have fundamentally different needs.
The crypto team needs strict algorithm auditing, cross-validation reviews,
and interop testing. The web app team needs UX reviews, responsive testing,
and deployment gotcha documentation. Without profiles, every team copies
all templates and then guesses which sections to customize vs. skip.

Profiles are adoption guides: "for your project type, here's what to copy,
what to customize, what to skip, and what to add." They reduce time-to-value
and prevent teams from ignoring the templates because they feel too heavy.

### How this repo was built

This repo emerged from a single conversation. It started with a question
about Claude Code subagent concurrency limits, evolved into a discussion of
prompt templates as version-controlled infrastructure, then grew into a
complete methodology for contract-driven agent development.

Each idea built on the previous one:
- Subagent fan-out → reusable prompt templates
- Prompt templates → single-invocation value (not just fan-out)
- Templates need shared guidelines → behavioral contracts for agents
- Behavioral contracts → why not contracts for the code itself?
- Code contracts → BDD first (who and why before what and how)
- All of this needs documentation → Cold Start Quad
- Documentation drifts → anti-drift mechanisms
- Different projects need different things → profiles

The iterative nature is the point. This isn't a top-down framework designed
in isolation — it's a bottom-up collection of patterns that proved their
value in real agent-driven development, organized into a coherent system
after the fact. The [learnings document](learnings-from-opendockit.md)
captures the raw failure analysis and war stories that motivated each pattern.
