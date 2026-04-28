# rebar

> **rebar v2.0.0** | **Tier 3: ENFORCED**

[![rebar v2.0.0](https://img.shields.io/badge/rebar-v2.0.0-orange)](DESIGN.md)
[![Tier 3: Enforced](https://img.shields.io/badge/tier-3_enforced-brightgreen)](DESIGN.md)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

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

**MCP-Enabled for Claude Code:** ASK exposes every agent as an MCP tool
(`ask_<repo>_<role>`), so Claude Code instances working in your project
can call the architect, product, or englead agent as a first-class tool —
no shell-out, no context cost. `rebar init` and `rebar adopt` write a
project-local `.mcp.json` automatically.
**[→ MCP Setup Guide](docs/MCP-SETUP.md)**

**How it's used:**
```bash
# From your shell
ask architect "Should we add caching to the user service?"
ask product "Does this contract match our user stories?"

# From Claude Code (after MCP wiring, tools appear automatically)
# → agent reaches for ask_rebar_architect the same way it reaches for Grep
```

**What it accomplishes:**
- **Cross-agent** — shared progress tracking, conflict avoidance, incremental checkpointing
- **Cross-repo** — learnings in one repo propagate to siblings automatically
- **Cross-role** — architect agent findings surface to product agent, and vice versa
- **Cross-session** — today's agent learns what yesterday's agent discovered
- **Cross-failure** — when agents fail, failure modes become swarm knowledge

**The result:** Your codebase becomes a *living system* where information stays organized, agents coordinate seamlessly, and collective intelligence compounds across every session. 10x context efficiency. Coordinated decision-making across your entire workflow.

**Setup time** depends on your profile (see [SETUP.md](SETUP.md)):
solo dev ~15 min, small team ~45 min, department ~2 hours. The substrate
is plain text — markdown, bash, grep, jq — plus optional Go and Python
binaries (`rebar` CLI, ASK CLI, MCP server) when you want them. The 5-minute
[QUICKSTART](QUICKSTART.md) gets you a working contract without any
binaries built; everything else is opt-in.

---

# Getting Started: Choose Your Path

## 🚀 **Try It** (5 minutes)
*Just want to see what rebar is about?*

**Solo developer quickstart:**
```bash
# Option A: Copy templates
git clone https://github.com/willackerly/rebar.git && cd rebar
cp -r templates/project-bootstrap/* ../my-project/ && cd ../my-project

# Option B: Use the CLI (builds v2 scaffolding automatically)
cd my-project && ../rebar/bin/rebar init
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

**What you get:** Battle-tested patterns from 200+ agent launches
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

**The four contract principles** (the foundation everything else builds on):

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
| **2 - Adopted** | + headers, freshness, ground truth, compliance, doc-refs, soft-hardening decay | + `check-contract-headers.sh`, `check-freshness.sh`, `check-ground-truth.sh`, `check-compliance.sh`, `check-doc-refs.sh`, `check-decay-patterns.sh` |
| **3 - Enforced** | + strict steward, full lifecycle | + full Steward with computed lifecycles |

*Run all: `scripts/ci-check.sh` | Pre-commit: `scripts/pre-commit.sh`*

`check-doc-refs.sh` catches the class of drift where a doc cites a file
that's never `git add`-ed — green on the author's machine, broken on a
fresh clone. `check-decay-patterns.sh` flags soft-hardening patterns
(silenced failures, inverted assertions, magic-string project gating,
"keep in sync" comments) that survive code review and decay six months
later. See `feedback/2026-04-21-...`, `2026-04-22-...`, and
`2026-04-24-fidelity-decay-...` for the source incidents.

</details>

---

# Next Steps

**New to rebar?** → [🚀 Try It (5 min)](QUICKSTART.md)
**Ready to adopt?** → [❤️ Love It (1 hour)](FEATURE-DEVELOPMENT.md)
**Need advanced patterns?** → [🎯 Master It](CASE-STUDIES.md)

**Questions?** → [Ask the architect agent](bin/README.md)

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
├── # Getting started
│   QUICKSTART.md                # 5-minute solo dev setup
│   FEATURE-DEVELOPMENT.md       # 1-hour guided BDD → Contract → Code workflow
│   CASE-STUDIES.md              # Real-world solutions indexed by problem
│   SETUP.md                     # Full adoption guide (per profile)
│
├── # Methodology
│   DESIGN.md                    # The philosophy (read first for depth)
│   conventions.md               # Branch naming, commits, headers, discoveries
│   CHANGELOG.md                 # Version history + migration notes
│
├── templates/
│   ├── project-bootstrap/       # `cp -r project-bootstrap/* ../my-project/` is one-shot
│   │   └── README.md, QUICKCONTEXT.md, TODO.md, AGENTS.md, CLAUDE.md, METRICS.md,
│   │       architecture/, scripts/ (synced from /scripts/), .rebarrc
│   ├── component-templates/     # Individual file templates for advanced use
│   └── scripts/                 # Optional Node.js checks (e.g., check-tag-ci-coverage)
│
├── architecture/                # Contract system + CONTRACT-TEMPLATE.md
├── agents/                      # Role agents (architect/product/englead/steward/tester/merger)
│                                #   + subagent prompt templates
├── bin/                         # ASK CLI (Python) + ask-mcp-server
├── cli/                         # rebar CLI (Go binary — init, commit, verify, audit)
├── scripts/                     # Enforcement + quality scanning (canonical bash)
├── practices/                   # Session lifecycle, orchestration, red team, fidelity
├── profiles/                    # Adoption guides (by project type × team size)
├── feedback/                    # Adoption reports — active in root, decided in processed/
└── docs/                        # Proposals + design documents (maintainer-facing)
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

---

## License

Copyright 2026 Will Ackerly.

Licensed under the [Apache License, Version 2.0](LICENSE). You may not use this
project except in compliance with the License. Unless required by applicable law
or agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND.
