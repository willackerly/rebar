# agent-templates

A complete system for contract-driven, agent-powered software development.

This is not a framework you install. It's a set of files you copy into your
project that give AI agents (and humans) the structure they need to build
software correctly. Contracts define what components should do. Agents read
contracts before writing code. Automated scans catch drift between intent and
reality. Everything is plain text — bash, markdown, grep, jq.

---

## Table of Contents

- [The Problem](#the-problem)
- [The Core Idea: Contracts](#the-core-idea-contracts)
- [How Contracts Work](#how-contracts-work)
- [The Cold Start Quad](#the-cold-start-quad)
- [The Agent Team (ASK)](#the-agent-team-ask)
- [Quality Infrastructure](#quality-infrastructure)
- [Anti-Drift Mechanisms](#anti-drift-mechanisms)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Design Decisions](#design-decisions)
- [Related Work](#related-work)
- [Future Concepts](#future-concepts)

---

## The Problem

AI coding agents are fast, capable, and confidently wrong. They break things
in a specific way: they make changes that are **locally correct but globally
wrong**. The function works. The tests pass. But the change violates an
architectural boundary, contradicts a design decision made weeks ago, or
introduces behavior that conflicts with another component's assumptions.

This happens because agents don't have access to **architectural intent**.
Code tells you what exists right now. Tests tell you if it works. But neither
tells you:

- What was **intended** vs. accidentally implemented
- What was **deliberately excluded** vs. forgotten
- Where the **boundaries** are vs. what just hasn't been tested yet
- What the **dependencies** are vs. what happens to work
- **Who** this serves and **why** they need it

Without this information, agents fill in the gaps with assumptions. Those
assumptions compound across sessions, across agents, and across time. The
result is a codebase that technically works but has lost its architectural
coherence — the design equivalent of a game of telephone.

**The fix is simple in concept:** write down the architectural intent in a
format agents can find and understand, link it bidirectionally to the code
that implements it, and automate the verification that they stay in sync.

That's what this repo provides.

---

## The Core Idea: Contracts

A **contract** is a versioned markdown document that defines what a component
does, who it serves, why it exists, and how it interfaces with other
components. Contracts live in `architecture/` and are the single source of
truth for system behavior.

Here's what makes contracts different from regular documentation:

**1. They're doubly-linked to code.** Every source file has a header comment
declaring which contract it implements. Every contract lists its implementing
files. You can go from code to spec or spec to code with a single `grep`.

```go
// Package blobstore implements encrypted blob storage.
//
// CONTRACT:C1-BLOBSTORE.2.1
package blobstore
```

```bash
# Find all code implementing a contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/ internal/

# Find what contract a file implements
head -10 internal/blobstore/file.go
```

**2. They're versioned.** Breaking changes bump the major version. Additive
changes bump the minor. Old versions are kept (marked superseded) so you can
trace evolution. When a contract version bumps, `grep` finds every file that
needs updating.

**3. They define behavior, not just interfaces.** A contract doesn't just list
function signatures — it specifies what happens on edge cases, what errors are
returned, what guarantees are made. These behavioral contracts are what tests
verify.

| Behavior | Specification |
|----------|--------------|
| `Get` on missing key | Returns `ErrNotFound` (not generic error) |
| `Put` with empty data | Returns `ErrInvalidInput` |
| `Delete` on missing key | No-op (idempotent), returns nil |
| Concurrent safety | All methods safe for concurrent use |

**4. They have a computed lifecycle.** The [Steward](#the-steward) scans
the codebase and derives each contract's status from what actually exists:

| Status | How It's Determined |
|--------|-------------------|
| **DRAFT** | Contract file exists but is missing required sections |
| **ACTIVE** | All sections present, no implementing code found yet |
| **TESTING** | Implementing code exists, but no test files found |
| **VERIFIED** | Implementing code AND test files exist |

Status is never declared manually — it's always computed from reality.

**5. They're the unit of agent autonomy.** Agents have full authority to
write, edit, refactor, delete, test, commit, and push — as long as they're
working within existing contracts. Creating or breaking a contract requires
discussion (plan mode). This makes autonomy safe: agents move fast within
boundaries, and the system forces a pause when boundaries need to change.

### The Rules

These are non-negotiable:

1. **Don't implement without a contract.** If there's no contract for what
   you're building, write the contract first.
2. **Don't modify code without checking its contract.** Every source file
   declares which contract it implements. Read it before changing behavior.
3. **Don't update a contract without searching implementations.**
   `grep -rn "CONTRACT:{id}"` finds all implementing code. Update them all.
4. **Contract changes that break interfaces require plan mode.**

### What a Contract Contains

Every contract has these required sections:

| Section | Purpose |
|---------|---------|
| **Interfaces** | The public API — function signatures, types, protocols |
| **Behavioral Contracts** | Edge cases, guarantees, ordering — what tests verify |
| **Error Contracts** | Which errors, when, with what codes |
| **Test Requirements** | What must be tested for the contract to be satisfied |
| **Implementing Files** | Which source files implement this contract |

Optional sections include **Scenarios** (Gherkin-style Given/When/Then for
UI/API contracts) and a **Companion File** (tribal knowledge that supports
the contract but doesn't define behavior — see [conventions.md](conventions.md)).

See [architecture/CONTRACT-TEMPLATE.md](architecture/CONTRACT-TEMPLATE.md)
for the annotated template.

---

## How Contracts Work

### The Full Chain: Who/Why → Contract → Code → Verify

The methodology flows in one direction:

```
1. BDD First    → Who needs this? Why? What does success look like?
2. Contract     → Formalize into a versioned architecture document
3. Implement    → Write code that references the contract
4. Verify       → Tests validate contract conformance
5. Evolve       → Update contract, search implementations, propagate changes
```

**BDD first** means every contract traces back to a user need. Before writing
a contract, you answer: who is the persona? What scenario are they in? What
does success look like? This lives in `product/` (personas, epics, features)
and contracts reference their source: `**Source:** product/features/encrypted-storage.feature`.

This chain prevents the most common failure mode in agent-driven development:
building technically correct things that nobody asked for.

### Naming Convention

```
CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}.md
```

| Prefix | Meaning | Example |
|--------|---------|---------|
| `S` | Service (top-level system boundary) | `S1-AUTH`, `S4-STORAGE` |
| `C` | Component (internal module) | `C1-BLOBSTORE`, `C2-RELAY` |
| `I` | Interface (shared between components) | `I1-SESSION`, `I2-KEY-EXCHANGE` |
| `P` | Protocol (wire format, messaging) | `P1-WIRE-FORMAT`, `P2-SIGNALING` |

### The Discovery Taxonomy

When reality diverges from contracts, that gap has a name:

| Type | What It Means | Who Resolves |
|------|---------------|-------------|
| **BUG** | Behavior contradicts a contract | Developer fixes the code |
| **DISCOVERY** | Behavior exists but no contract covers it | Architect writes a contract |
| **DRIFT** | Behavior matches contract literally but misses intent | Architect + Developer refine both |
| **DISPUTE** | The contract itself is wrong | Architect + Product update the contract |

Discoveries live in the `## Discoveries` section of `TODO.md` and are parsed
by the Steward. They're the feedback loop that keeps contracts honest.

---

## The Cold Start Quad

Every agent session starts by reading four files, in this order:

| Order | File | Purpose | Drift Risk |
|-------|------|---------|-----------|
| 1 | `README.md` | What this project is, how to navigate | Low |
| 2 | `QUICKCONTEXT.md` | What's true right now — branch, test counts, active work | **High** |
| 3 | `TODO.md` | Tasks, known issues, blockers, discoveries | Medium-High |
| 4 | `AGENTS.md` | How we work — norms, testing cascade, contracts | Low |

Plus `CLAUDE.md` for Claude Code-specific configuration.

**Why this order:** Orientation first (README), then current state
(QUICKCONTEXT), then tasks (TODO), then norms (AGENTS). An agent that
understands what the project is before diving into what's happening now makes
better decisions. The order goes from stable/strategic to volatile/tactical.

After the quad, agents should verify what they read:

```bash
# Cross-reference QUICKCONTEXT claims against reality
git log --oneline -10
grep -i "branch" QUICKCONTEXT.md

# Verify ground truth metrics
[ -x scripts/check-ground-truth.sh ] && ./scripts/check-ground-truth.sh
```

This takes 2 minutes and prevents hours of wasted effort on stale information.

This repo provides templates for all four files:
[README.template.md](README.template.md),
[QUICKCONTEXT.template.md](QUICKCONTEXT.template.md),
[TODO.template.md](TODO.template.md),
[AGENTS.template.md](AGENTS.template.md),
[CLAUDE.template.md](CLAUDE.template.md).

---

## The Agent Team (ASK)

**ASK** (Agent Scoped Knowledge) is a CLI that gives each agent role its
own persona, memory, and bounded context. It's a bash script in `bin/ask`.

### Built-In Roles

| Role | What It Owns | Default Command |
|------|-------------|----------------|
| **Architect** | Contracts, system design, architecture/ | `ask architect` → contract audit |
| **Product** | Requirements, BDD scenarios, product/ | `ask product` → gap analysis |
| **Eng Lead** | Implementation, QA, TODO, QUICKCONTEXT | `ask englead` → enforcement status |
| **Steward** | Quality scanning, architecture/.state/ | `ask steward` → full health scan |

### Two Interaction Modes

**Questions** — ask an agent something in natural language:

```bash
ask architect "What contract governs authentication?"
ask product "What are the current requirements for file storage?"
ask -v architect "Why was RSA chosen over ECDH?"   # verbose: answer + rationale + refs
```

**Commands** — run an agent's built-in action (unquoted single word):

```bash
ask steward              # full quality scan
ask steward summary      # one-line health check
ask steward json         # aggregate JSON to stdout
ask steward check C1     # scan a single contract

ask architect            # contract audit (DRAFTs, DISPUTEs, action items)
ask architect audit      # same thing, explicit

ask englead              # enforcement results, TESTING contracts
ask englead check        # run ci-check.sh
ask englead qa           # full QA flow: steward scan + enforcement

ask product              # gap analysis (DISCOVERYs, missing BDD refs)
ask product gaps         # same thing, explicit
```

The convention: **quoted = question, unquoted = command.** Under the hood,
each agent has a `commands/` directory with executable scripts. Drop a new
`.sh` file in `agents/<role>/commands/` and it's immediately available as
`ask <role> <command-name>`.

### Other ASK Commands

```bash
ask who                  # list available agents
ask init                 # initialize directory structure + create agents
ask status architect     # check if an agent is running
ask log architect        # view interaction history
ask reset architect      # clear session, start fresh next question
ask register myproject   # register project for cross-project queries
ask myproject:architect "what is the architecture?"   # cross-project query
```

### Session Persistence

Agents maintain sessions across questions. The first question pays the full
context cost; follow-ups resume the session. When context usage hits 70%
(configurable via `ASK_CONTEXT_LIMIT`), the session auto-resets.

### How Agents Are Structured

Each agent is a directory under `agents/` with:

```
agents/architect/
  AGENT.md              # Role definition, responsibilities, context loading rules
  commands/             # Executable command scripts (default.sh, audit.sh, etc.)
  memory.md             # Distilled current state (persists across sessions)
  memory.log.md         # Append-only interaction history
  inbox/                # Incoming messages (when running via `ask up`)
  outbox/               # Outgoing responses
```

See [bin/README.md](bin/README.md) for the full ASK reference.

### Subagent Templates

For delegation (one agent assigning work to others), the system includes
reusable prompt templates in `agents/subagent-prompts/`:

| Template | Purpose |
|----------|---------|
| [code-review.md](agents/subagent-prompts/code-review.md) | Multi-dimension code review |
| [contract-audit.md](agents/subagent-prompts/contract-audit.md) | Interface conformance check |
| [security-surface-scan.md](agents/subagent-prompts/security-surface-scan.md) | Security audit |
| [ux-review.md](agents/subagent-prompts/ux-review.md) | UX, accessibility, responsive |
| [doc-drift-detector.md](agents/subagent-prompts/doc-drift-detector.md) | Doc-vs-code consistency |
| [feature-inventory.md](agents/subagent-prompts/feature-inventory.md) | Behavioral inventory for safe refactoring |
| [test-shard-runner.md](agents/subagent-prompts/test-shard-runner.md) | Parallel test execution |

Templates encode **your** definition of how a task should be done. If you've
ever corrected an agent ("no, not like that — here's how we do reviews"),
that correction belongs in a template. See [agents/README.md](agents/README.md).

---

## Quality Infrastructure

### The Steward

The Steward is the project's technical program manager in code form. It scans
the contract system, runs enforcement checks, and produces per-role action
items. It reports facts — it doesn't prescribe solutions.

```bash
scripts/steward.sh             # full scan → JSON + markdown report
scripts/steward.sh --json      # aggregate JSON to stdout
scripts/steward.sh --summary   # one-line summary
scripts/steward.sh --check C1  # single contract

# Or through ASK:
ask steward                    # same as scripts/steward.sh
ask steward summary            # one-liner
```

**What it checks per contract:**
- Spec gate: are all required sections present?
- Impl gate: do any source files reference this contract? Are there tests?
- Lifecycle: computed from the above (DRAFT → ACTIVE → TESTING → VERIFIED)
- Discoveries: any BUG/DISCOVERY/DRIFT/DISPUTE entries in TODO.md?

**What it checks globally:**
- Enforcement: runs all `check-*.sh` scripts, captures pass/fail
- Ground truth: runs metric verification against the METRICS file

**What it produces:**
- Per-contract JSON: `architecture/.state/<id>.<version>.json`
- Aggregate JSON: `architecture/.state/steward-report.json`
- Human-readable: `STEWARD_REPORT.md`

The JSON schema is designed so a future single-file HTML dashboard could
`fetch()` the report and render it — no server, no build step.

**Role-based action items:**

| Role | Sees | Acts On |
|------|------|---------|
| Architect | DRAFT contracts, DISPUTEs | Complete contracts, resolve disputes |
| Eng Lead | Enforcement failures, TESTING contracts | Fix failures, coordinate verification |
| Product | DISCOVERYs, missing BDD refs | Write requirements, fill coverage gaps |
| Developer | ACTIVE contracts, BUGs | Implement contracts, fix bugs |

### Enforcement Scripts

Individual checks, each standalone and fast (<5s):

| Script | What It Checks |
|--------|---------------|
| `check-contract-headers.sh` | Every source file has a `CONTRACT:` header |
| `check-contract-refs.sh` | Every `CONTRACT:` ref points to a real contract file |
| `check-todos.sh` | No untracked `TODO:` comments (two-tag system) |
| `check-freshness.sh` | Doc freshness dates aren't stale (>14 days) |
| `check-registry.sh` | Contract registry matches actual files |
| `check-ground-truth.sh` | METRICS file matches codebase reality |

Composite runners:

| Script | When |
|--------|------|
| `ci-check.sh` | CI pipeline — runs all checks, reports summary |
| `pre-commit.sh` | Git hook — runs fast checks before commit |
| `steward.sh` | Full quality scan — everything above + contract lifecycle |

See [scripts/README.md](scripts/README.md) for installation and configuration.

### Ground Truth Metrics

Quantitative claims (test counts, contract counts, endpoint counts) drift
faster than any other documentation. The `METRICS` file is the single source
of truth for project-wide numbers, and `check-ground-truth.sh` verifies it
against code. This catches the failure mode where everything works but
documented numbers describe a different reality.

---

## Anti-Drift Mechanisms

Documentation drifts from reality at the speed of code changes. Agents both
suffer from and contribute to drift. These mechanisms fight it:

| Mechanism | How It Works |
|-----------|-------------|
| **Freshness timestamps** | Every status-bearing doc has a date. >2 weeks old = treat with skepticism. |
| **Two-tag TODO system** | `TODO:` in code = untracked = blocks commit. `TRACKED-TASK:` = tracked in TODO.md = allowed. |
| **Doubly-linked contracts** | Code references contracts, contracts list implementing files. Either direction is searchable. |
| **Contract version bumps** | Changing a contract forces you to grep all implementing code and update it. |
| **Ground truth script** | Computes metrics from code and compares against documented claims. |
| **Pre-launch audits** | Before fan-out, grep for existing implementations. Prevents 50% waste. |
| **Cold start verification** | Every new session verifies QUICKCONTEXT claims against `git log`. |
| **Steward scans** | Automated quality scan catches lifecycle gaps, missing tests, stale contracts. |
| **Discovery taxonomy** | BUG/DISCOVERY/DRIFT/DISPUTE captures the full spectrum of spec-reality gaps. |

The underlying principle: **the filesystem is the source of truth.** When docs
say one thing and `ls` + `grep` say another, the filesystem wins. Docs
describe intent; the filesystem describes reality. Both matter, but reality
takes precedence when they conflict.

---

## Getting Started

### For Humans

1. Read [methodology.md](methodology.md) — the philosophy
2. Pick your [project profile](profiles/) — tells you what to adopt
3. Follow [SETUP.md](SETUP.md) — step-by-step adoption guide
4. Customize the templates per the `<!-- comments -->` in each file

### For Agents

**Starting a new project:** Read [methodology.md](methodology.md) first, then
follow [SETUP.md](SETUP.md) step by step. Contracts are the operating system —
everything flows from there.

**Aligning an existing project:** Diff your docs against the templates. Start
with `CONTRACT:` headers in source files and an `architecture/` directory with
your most important contracts. See your [profile](profiles/) for priorities.

**Improving this methodology:** Drop a file in [feedback/](feedback/)
describing what you learned, what's missing, or what didn't work.

### Quick Start

```bash
PROJECT=/path/to/your/project

# Core docs (the Cold Start Quad + Claude config)
cp README.template.md       "$PROJECT/README.md"
cp QUICKCONTEXT.template.md "$PROJECT/QUICKCONTEXT.md"
cp TODO.template.md         "$PROJECT/TODO.md"
cp AGENTS.template.md       "$PROJECT/AGENTS.md"
cp CLAUDE.template.md       "$PROJECT/CLAUDE.md"
cp methodology.md           "$PROJECT/methodology.md"

# Contract system
cp -r architecture/         "$PROJECT/architecture/"
mkdir -p "$PROJECT/architecture/.state"

# Agent orchestration
cp -r agents/               "$PROJECT/agents/"

# Enforcement scripts
cp -r scripts/              "$PROJECT/scripts/"
cp conventions.md           "$PROJECT/conventions.md"
cp METRICS.template         "$PROJECT/METRICS"
chmod +x "$PROJECT/scripts/"*.sh

# ASK CLI
cp -r bin/                  "$PROJECT/bin/"
chmod +x "$PROJECT/bin/"*

# Pre-commit hook
ln -sf ../../scripts/pre-commit.sh "$PROJECT/.git/hooks/pre-commit"

# Initialize ASK agents
cd "$PROJECT" && bin/ask init
```

See [SETUP.md](SETUP.md) for the detailed guide with customization steps.

---

## Project Structure

```
agent-templates/
│
├── methodology.md                  # The philosophy — read this first
├── conventions.md                  # Branch naming, commits, file headers, reviews
├── SETUP.md                        # Step-by-step adoption guide
├── learnings-from-opendockit.md    # War stories from 5,800+ tests & 9 agents
│
├── # Templates (copy these into your project)
├── README.template.md              # Cold Start Quad #1 — project orientation
├── QUICKCONTEXT.template.md        # Cold Start Quad #2 — current state
├── TODO.template.md                # Cold Start Quad #3 — tasks + discoveries
├── AGENTS.template.md              # Cold Start Quad #4 — norms + collaboration
├── CLAUDE.template.md              # Claude Code config
├── METRICS.template                # Ground truth metrics (key=value)
├── STEWARD_REPORT.template.md      # Steward report format reference
│
├── architecture/                   # The contract system
│   ├── README.md                   # How contracts work
│   ├── CONTRACT-TEMPLATE.md        # Annotated contract template
│   ├── CONTRACT-REGISTRY.template.md  # Contract index
│   └── .state/                     # Steward output (per-contract + aggregate JSON)
│
├── agents/                         # Agent orchestration
│   ├── README.md                   # How agent templates work
│   ├── subagent-guidelines.md      # Shared behavioral rules for all subagents
│   ├── subagent-prompts-index.md   # Catalog of available templates
│   ├── subagent-prompts/           # Reusable prompt templates
│   │   ├── code-review.md
│   │   ├── contract-audit.md
│   │   ├── security-surface-scan.md
│   │   ├── ux-review.md
│   │   ├── doc-drift-detector.md
│   │   ├── feature-inventory.md
│   │   └── test-shard-runner.md
│   ├── architect/                  # Architect agent
│   │   ├── AGENT.md                #   Role definition
│   │   └── commands/               #   audit, default
│   ├── product/                    # Product agent
│   │   ├── AGENT.md
│   │   └── commands/               #   gaps, default
│   ├── englead/                    # Engineering Lead agent
│   │   ├── AGENT.md
│   │   └── commands/               #   check, qa, default
│   ├── steward/                    # Steward agent (quality scanner)
│   │   ├── AGENT.md
│   │   └── commands/               #   scan, json, summary, check, default
│   ├── findings/                   # Architectural findings from subagents
│   └── results/                    # Subagent output files
│
├── bin/                            # ASK CLI
│   ├── README.md                   # Full ASK reference
│   ├── ask                         # The CLI script
│   └── ask-agent-loop              # Background agent loop runner
│
├── scripts/                        # Enforcement + quality
│   ├── README.md                   # Script reference
│   ├── steward.sh                  # Full quality scan
│   ├── ci-check.sh                 # CI entrypoint (runs all checks)
│   ├── pre-commit.sh               # Git pre-commit hook
│   ├── check-contract-headers.sh   # Source files have CONTRACT: headers
│   ├── check-contract-refs.sh      # CONTRACT: refs point to real files
│   ├── check-todos.sh              # No untracked TODO: comments
│   ├── check-freshness.sh          # Doc freshness dates aren't stale
│   ├── check-registry.sh           # Registry matches actual files
│   └── check-ground-truth.sh       # METRICS matches codebase reality
│
├── profiles/                       # Project-type adoption guides
│   ├── README.md
│   ├── web-app.md
│   ├── api-service.md
│   ├── crypto-library.md
│   └── cli-tool.md
│
├── feedback/                       # Agent/human feedback on methodology
│   └── README.md
│
├── # Design documents (proposals, roadmap)
├── AGENT-RUNTIME.md                # Proposal: multi-agent execution runtime
├── ASK-SHELL.md                    # Proposal: Unix shell for agent interaction
└── IMPLEMENTATION.md               # Implementation roadmap: bash v0 → Go v2
```

---

## Design Decisions

This repo was built iteratively from real agent-driven development. Each
decision emerged from a failure that the previous approach didn't prevent.

### Why contracts became the center, not docs or tests

We started with documentation templates (QUICKCONTEXT, AGENTS, TODO) and they
worked — agents oriented faster, drift decreased. But agents kept changing
code that *technically worked* but violated architectural intent. They'd
refactor a function in a way that broke an implicit contract with another
module, or add a feature that contradicted a design decision from weeks ago.

The problem: no document answered "what is this code *supposed to do*
according to the architecture?" Tests answer "does it work?" Code answers
"what does it do right now?" But neither answers "what was intended, and what
boundaries must be respected?" That's what contracts do.

### Why grep-based linking over tooling

`// CONTRACT:C1-BLOBSTORE.2.1` in code headers + `grep -rn` to find
implementations. No build plugins, no custom linters, no databases.

- **Zero adoption cost.** Any project can start using it today.
- **Tool-agnostic.** Works with any editor, any AI agent, any CI.
- **Transparent.** The linking mechanism is visible in the code.
- **Resilient.** No tool to break, update, or configure.

The value comes from the *practice* of writing and referencing contracts, not
from tooling. Start with grep. Add tooling later if scale demands it.

### Why subagent templates matter for single invocations

We built `agents/subagent-prompts/` for parallel fan-out. But the bigger
insight: **templates are just as valuable for single tasks.** When you ask an
agent to do a "UX review" without a template, it guesses what you mean. A
`ux-review.md` template encodes *your* definition — your criteria, heuristics,
and output format. If you've ever corrected an agent, that correction belongs
in a template. This is how agents learn across sessions.

### Why TODO absorbed KNOWN_ISSUES

We had QUICKCONTEXT, KNOWN_ISSUES, TODO, AGENTS, CLAUDE — five files. In
practice, KNOWN_ISSUES and TODO overlapped and agents had to maintain both.
Every additional file is a drift surface. Merging known issues into TODO as a
section reduced maintenance without losing information. Fewer files actually
maintained beats more files that drift.

### Why README is the universal first-read

Previously QUICKCONTEXT was first. But it answers "what's happening now," not
"what is this project?" An agent that dives into current state without
understanding the project's identity makes worse decisions. README provides
stable orientation; QUICKCONTEXT is volatile and tactical. The reading order
goes from stable/strategic to volatile/tactical.

### Why lifecycle is computed, not declared

We adopted lifecycle tracking from [Purlin](#related-work), but with a key
difference: status is derived from what exists in the codebase (do implementing
files exist? do test files exist? are all spec sections present?) rather than
manually declared. Computed status can't drift — it's always accurate because
it's always recomputed from reality.

### Why agent commands are unquoted

`ask steward "what needs attention?"` asks a question. `ask steward` runs a
scan. The convention — quoted = question, unquoted = command — emerged from
wanting each role to own a slice of project health without requiring the full
persona invocation. Each agent has a `commands/` directory; drop a `.sh` file
in it and it's immediately available. Zero changes to `bin/ask` needed.

### How this repo was built

This repo emerged from a single conversation about Claude Code subagent
concurrency limits, evolved through prompt templates as version-controlled
infrastructure, and grew into a complete methodology for contract-driven
agent development. Each idea built on the last:

- Subagent fan-out → reusable prompt templates
- Templates → single-invocation value (not just fan-out)
- Templates need shared rules → behavioral contracts for agents
- Agent contracts → why not contracts for code?
- Code contracts → BDD first (who/why before what/how)
- Docs needed → Cold Start Quad
- Docs drifted → anti-drift mechanisms
- Different projects → profiles
- Quality gaps → Steward + enforcement scripts
- Agent roles → ASK CLI with commands

The [learnings document](learnings-from-opendockit.md) captures the raw
failure analysis and war stories from 5,800+ tests and 9 simultaneous agents.

---

## Adoption Profiles

Different projects need different subsets. Pick your profile:

| Profile | Best For |
|---------|----------|
| [web-app](profiles/web-app.md) | SPA, SSR, web frontend + API |
| [api-service](profiles/api-service.md) | Backend API, microservice |
| [crypto-library](profiles/crypto-library.md) | Security-critical library |
| [cli-tool](profiles/cli-tool.md) | Command-line tool |

Profiles tell you what to copy, what to customize, and what to skip. If your
project spans multiple types, combine the relevant parts.

---

## Related Work

### Purlin

[Purlin](https://github.com/purlin) is a spec-driven development framework
that influenced several concepts in agent-templates.

**Where we align:** specs before code, contract lifecycle, quality gates,
companion docs, discovery taxonomy.

**Where we diverge:**

| Aspect | Purlin | agent-templates | Why |
|--------|--------|-----------------|-----|
| Role rigidity | Fixed hierarchy, formal handoffs | Fluid roles | Small teams wear multiple hats |
| Scenarios | Gherkin-only, required | Gherkin optional | Infrastructure contracts don't benefit from Given/When/Then |
| Code philosophy | "Code is disposable, specs are permanent" | Code and contracts are co-equal | Contracts become stale when not grounded in implementation |
| Repo structure | Git submodules | Flat vendoring (proposed) | Submodules add complexity small teams can't absorb |
| Dashboard | Required web UI | JSON-first, dashboard optional | `jq` should answer what a dashboard can |
| Tooling | Purpose-built CLI | bash + jq + grep | Zero dependencies beyond a Unix shell |

**What we adopted and adapted:**
- Computed lifecycle status (derived from code, never declared)
- Discovery taxonomy (BUG/DISCOVERY/DRIFT/DISPUTE)
- Companion files (`.impl.md` for tribal knowledge)
- Role-based action items (Steward routes findings to the right role)
- Quality scanning as infrastructure (Steward = TPM in code form)

---

## Future Concepts

Ideas we believe are directionally correct but haven't built yet.

### Contract Vendoring

When multiple repos implement the same contracts, each needs a local copy.
Copies drift. **Concept:** Vendor contracts like Go vendors dependencies —
`architecture/vendor/` contains read-only copies with a lock file pointing
to the upstream source. `scripts/vendor-contracts.sh` pulls updates.

### Cryptographic Contract Signing

Vendored contracts could be tampered with. **Concept:** Sign contracts when
they pass a trust checklist (BDD reviewed, architect approved, security
reviewed, tests passing, reference implementation exists). Child repos can
vendor but not modify without invalidating the signature. Matters for
high-assurance environments (financial, healthcare, defense).

### Expert Agent Hierarchy

At scale, a single orchestrating agent can't deeply understand product intent,
architecture, and all code simultaneously. **Concept:** A hierarchy of expert
agents — Product (reads BDD, answers "what and why"), Architect (reads
contracts, answers "how should it be structured"), Eng Lead (reads code,
answers "how to implement"), Engineers (subagents, do the work). Each level
reads summaries from above, detail at its own level. The Eng Lead is the
default persona you talk to.

This is partially realized through ASK roles and agent commands, but the full
hierarchy with structured inter-agent communication is future work.

See [AGENT-RUNTIME.md](AGENT-RUNTIME.md), [ASK-SHELL.md](ASK-SHELL.md), and
[IMPLEMENTATION.md](IMPLEMENTATION.md) for detailed proposals.
