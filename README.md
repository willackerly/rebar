# rebar

**The dual-purpose foundation for AI-powered development teams.**

Rebar is **two things in one**:

## 1. Information Organization Model
How your team structures knowledge so both humans AND agents can navigate at scale:
- **Architecture contracts** — behavioral specifications that survive team turnover
- **Product requirements & BDD** — living documentation that drives implementation
- **Testing tiers (T0-T5)** — from unit tests to integration, each tier enforced
- **Role-based workflows** — architect, product, eng lead, developer boundaries
- **Cross-cutting discovery** — patterns, gotchas, technical debt tracked persistently

## 2. Swarm Coordination Platform
How 10+ agents work together without destroying each other's work:

### The ASK Tool — Persistent Multi-Role Agent System
**What it is:** Enterprise-grade CLI that maintains persistent agent sessions across projects and time. Instead of ephemeral subagents that cost 10x context on each question, ASK agents accumulate knowledge and coordinate through specialized roles.

**MCP-Enabled for Swarms:** Now supports Model Context Protocol (MCP), allowing a single ASK agent instance to answer questions from multiple users and agents simultaneously. This enables true swarm intelligence where one expert agent can serve an entire development team or agent collective.

**How it's used:**
```bash
ask architect "Should we add caching to the user service?"
ask product "Does this contract match our user stories?"
ask englead "Are we ready to merge this feature?"
ask steward "What contracts need attention?"
```

**What it accomplishes:**
- **Cross-agent** — shared progress tracking, conflict avoidance, incremental checkpointing
- **Cross-repo** — learnings in one repo propagate to siblings automatically
- **Cross-role** — architect agent findings surface to product agent, and vice versa
- **Cross-session** — today's agent learns what yesterday's agent discovered
- **Cross-failure** — when agents fail, failure modes become swarm knowledge

**The result:** 10x context efficiency + coordinated decision-making across your entire development workflow.

**The result:** Your codebase becomes a *living system* where information stays organized, agents coordinate seamlessly, and collective intelligence compounds across every session.

30 minutes to set up. Zero infrastructure. Everything is plain text — bash, markdown, grep, jq. No framework to install. Copy files into your project, and both your organization model AND agent coordination work immediately.

---

# Getting Started: Choose Your Path

## 🚀 **Try It** (5 minutes)
*Just want to see what rebar is about?*

**Solo developer quickstart:**
```bash
# Clone and bootstrap
git clone https://github.com/willackerly/rebar.git && cd rebar
cp -r templates/project-bootstrap/* ../my-project/ && cd ../my-project
# Edit your first contract → link it to code → see the magic
```

**What you get:** Contracts linked to code + agent coordination + immediate productivity boost
**Perfect for:** Solo devs, 1-3 repos, testing the waters
**[→ QUICKSTART GUIDE](QUICKSTART.md)**

## ❤️ **Love It** (1 hour)
*Ready to experience rebar's full value?*

**Guided feature development journey:**
1. **BDD scenario** → define what success looks like
2. **Contract first** → specify behavior before code
3. **Agent coordination** → architect + product + engineer perspectives
4. **Quality cascade** → T0-T5 testing with automated enforcement

**What you get:** Complete workflow that scales from solo to swarm
**Perfect for:** Teams ready to adopt AI-first development
**[→ FEATURE DEVELOPMENT GUIDE](FEATURE-DEVELOPMENT.md)**

## 🎯 **Master It** (ongoing)
*Need advanced patterns for scaling, coordination, or specific problems?*

**Navigate by what you need:**
- **[Scaling](profiles/)** — Solo → Small Team → Department (tier progression)
- **[Quality](DESIGN.md)** — Contracts → Testing → Enforcement
- **[Coordination](practices/)** — Multi-agent → Worktrees → Swarms
- **[Case Studies](CASE-STUDIES.md)** — Real-world solutions indexed by problem

**What you get:** Battle-tested patterns from 100+ agent launches
**Perfect for:** Teams hitting limits, complex coordination needs

---

# How It Works

## The 5-Minute Picture

### 1. Every source file declares its contract

```go
// CONTRACT:C1-BLOBSTORE.2.1
package blobstore
```

```bash
# Find all code implementing a contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/

# Find what contract a file implements
head -5 internal/blobstore/store.go
```

### 2. Contracts define behavior, not just interfaces

```markdown
# architecture/CONTRACT-C1-BLOBSTORE.2.1.md

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| `Get` on missing key | Returns `ErrNotFound` (not generic error) |
| `Put` with empty data | Returns `ErrInvalidInput` |
| `Delete` on missing key | No-op (idempotent), returns nil |
| Concurrent safety | All methods safe for concurrent use |
```

An agent reading this knows *exactly* what edge cases to handle — and what
tests to write. No guessing. No assumptions.

### 3. Automated scans catch drift

```bash
$ scripts/steward.sh --summary
Steward: 12 contracts (0d/2a/3t/7v), 1 discovery, 6/6 enforcement passing
```

The [Steward](#quality-infrastructure) scans your codebase and derives each
contract's lifecycle from what actually exists — draft, active, testing, or
verified. Status is computed from reality, never declared manually.

### 4. Agents query role-based experts

```bash
$ ask architect "should the blobstore handle encryption or delegate to the caller?"
# Persistent session — 10 questions cost 1x context, not 10x

$ ask steward summary
# Automated health check — which contracts need attention?

$ ask product "does offline sync need a new contract?"
# Product perspective — requirements before implementation
```

---

# Battle-Tested Results

Rebar has been proven across 200+ worktree agent launches in five production projects:

| Project | Swarm Scale | What Happened |
|---------|------------|--------------|
| **Dapple SafeSign** | 18 agents, 3 phases | 17 contracts, 169 headers, **0 merge conflicts**, 3 hours wall clock |
| **blindpipe** | Selective adoption | Crypto-critical ZK suite. ASK sessions save **10x context** vs ephemeral subagents |
| **OpenDocKit** | 15+ parallel agents | 8,000+ tests, 20hr marathon. Red team protocol: 18 issues found, 18 fixed. Visual fidelity RMSE 0.159→0.102 |
| **filedag** | 40+ parallel agents | 62 commits in 48hrs, 28K lines Go + 10K TypeScript, 87 Playwright tests. Proved session lifecycle need |
| **Office 180** | Multi-repo swarm | Cross-repo namespacing, AI-native contract frontmatter |

**Key insight from OpenDocKit:** 100% of committed agent work was recoverable after login expiration incidents. Uncommitted work was the only true loss — which the commit-per-chunk protocol minimizes.

*[See complete adoption reports →](feedback/)*

---

# Architecture Overview

*For teams who want to understand the foundation before diving in*

## The Core Idea

Rebar is a **swarm coordination framework** built on three layers:

### Layer 1: Contracts (shared truth)

A **contract** is a versioned markdown document that defines what a component
does, who it serves, why it exists, and how it interfaces with other
components. Contracts live in `architecture/` and are doubly-linked to code
via `CONTRACT:` headers — `grep` gets you from code to spec or spec to code.

Contracts are the operating system of the swarm. They're what make it safe
for 10 agents to work the same codebase in parallel — each agent knows what
it can and can't change by reading the contracts, not by asking you.

### Layer 2: Coordination (shared work)

When agents run in parallel, they need more than contracts — they need
protocols that prevent collisions and survive failures:

- **Worktree isolation** — every coding agent gets its own working copy
- **Commit-per-chunk** — uncommitted work is lost work (login expires, agent crashes)
- **Shared progress tracking** — the parent knows what each agent accomplished without reading transcripts
- **Kill-before-spin** — always clean up stale processes before starting new ones
- **Post-fanout merge** — cherry-pick one agent at a time, test between each

### Layer 3: Collective learning (shared knowledge)

The swarm gets smarter over time. What one agent discovers, all agents benefit from:

- **Persistent agent sessions** (ASK CLI) — 10 questions cost 1x context, not 10x
- **Cross-session memory** — today's agent learns what yesterday's discovered
- **Role-based expertise** — architect, product, tester each accumulate domain knowledge
- **Failure pattern library** — every crash, conflict, and regression becomes a mitigation for the next run

**The four contract rules** (the foundation everything else builds on):

1. Don't implement without a contract
2. Don't modify code without checking its contract
3. Don't update a contract without searching all implementations
4. Contract changes that break interfaces → plan mode

*[Complete philosophy → DESIGN.md](DESIGN.md) | [Contract template → architecture/](architecture/CONTRACT-TEMPLATE.md)*

---

<details>
<summary><strong>🔧 Quality Infrastructure</strong> <em>(expand to see automation details)</em></summary>

### The Steward

Automated quality scanner that produces per-contract health reports:

```bash
scripts/steward.sh --summary # One-line health check
ask steward summary          # Same via ASK CLI
```

Routes action items by role: draft contracts → architect, testing gaps → eng lead, discoveries → product.

### Enforcement (Progressive by Tier)

| Tier | What's Enforced | Scripts |
|------|----------------|---------|
| **1 - Partial** | Contract refs + TODOs | `check-contract-refs.sh`, `check-todos.sh` |
| **2 - Adopted** | + headers, freshness, ground truth, compliance | + `check-contract-headers.sh`, `check-freshness.sh`, `check-ground-truth.sh`, `check-compliance.sh` |
| **3 - Enforced** | + strict steward, full lifecycle | + full Steward with computed lifecycles |

*Run all: `scripts/ci-check.sh` | Pre-commit: `scripts/pre-commit.sh`*

</details>

---

<details>
<summary><strong>📁 Project Structure</strong> <em>(expand to see full layout)</em></summary>

```
rebar/
├── # Getting Started Files
├── QUICKSTART.md               # 5-minute solo dev setup
├── FEATURE-DEVELOPMENT.md      # 1-hour guided workflow
├── CASE-STUDIES.md             # Problem-indexed war stories
│
├── # Core Philosophy
├── DESIGN.md                   # Complete methodology
├── conventions.md              # Standards & naming
├── SETUP.md                    # Full adoption guide
│
├── # Copy Into Your Project
├── *.template.md               # Cold Start Quad templates
├── architecture/               # Contract system + templates
├── scripts/                    # Enforcement & scanning
│
├── # Advanced Patterns
├── practices/                  # Specialized workflows
├── profiles/                   # Team size & project type guides
├── agents/                     # Role definitions & subagent templates
├── bin/                        # ASK CLI for persistent sessions
├── feedback/                   # Real-world adoption reports
└── docs/                       # Proposals & design documents
```

</details>

---

# Next Steps

**New to rebar?** → [🚀 Try It (5 min)](QUICKSTART.md)
**Ready to adopt?** → [❤️ Love It (1 hour)](FEATURE-DEVELOPMENT.md)
**Need advanced patterns?** → [🎯 Master It](CASE-STUDIES.md)

**Questions?** → [Ask the architect agent](bin/README.md#ask-cli-reference)

### Pick Your Profile

**By project type:** [web-app](profiles/web-app.md) | [api-service](profiles/api-service.md) | [crypto-library](profiles/crypto-library.md) | [cli-tool](profiles/cli-tool.md)

**By team size:** [solo-dev](profiles/solo-dev.md) | [small-team](profiles/small-team.md) | [department](profiles/department.md)

### Compliance

Every rebar repo declares its version and tier at the top of README.md:

```markdown
> **rebar v2.0.0** | **Tier 2: ADOPTED**
```

This is validated by `scripts/check-compliance.sh` and the Steward. It tells
anyone looking at your repo: "this project speaks rebar, here's what's enforced."

---

## Project Structure

```
rebar/
├── DESIGN.md                    # The philosophy (read first for depth)
├── conventions.md               # Branch naming, commits, headers, reviews
├── SETUP.md                     # Step-by-step adoption guide
├── CHANGELOG.md                 # Version history + migration guides
│
├── templates/
│   ├── project-bootstrap/       # Copy this into your project to get started
│   │   ├── README.md, QUICKCONTEXT.md, TODO.md, AGENTS.md, CLAUDE.md
│   │   ├── architecture/, scripts/
│   │   └── .rebarrc, METRICS.md
│   └── component-templates/     # Individual file templates for advanced use
│
├── architecture/                # Contract system + templates
├── agents/                      # Role definitions + subagent templates
├── bin/                         # ASK CLI (persistent agent sessions)
├── cli/                         # Rebar CLI (Go binary — commit, verify, context)
├── scripts/                     # Enforcement + quality scanning
├── practices/                   # Session lifecycle, orchestration, red team, fidelity, etc.
├── profiles/                    # Adoption guides (by project type + team size)
├── feedback/                    # Real-world adoption reports
└── docs/                        # Proposals + design documents
```

---

## Learn More

- [DESIGN.md](DESIGN.md) — the full philosophy: contracts, BDD, lifecycle, autonomy, testing cascade
- [conventions.md](conventions.md) — branch naming, commit format, file headers, discovery taxonomy
- [architecture/README.md](architecture/README.md) — contract naming, linking, and lifecycle reference
- [bin/README.md](bin/README.md) — ASK CLI reference (commands, sessions, cross-project queries)
- [docs/learnings-from-opendockit.md](docs/learnings-from-opendockit.md) — war stories from 5,800+ tests and 9 simultaneous agents
- [feedback/](feedback/) — adoption reports from blindpipe, Dapple SafeSign, Office 180
- [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) — ASK CLI roadmap (bash v0 → Go v2)
