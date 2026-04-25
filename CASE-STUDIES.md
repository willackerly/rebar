📍 **You are here:** [Try It](QUICKSTART.md) → [Love It](FEATURE-DEVELOPMENT.md) → **Master It**
**Prerequisites:** Basic rebar experience
**Deep dive:** [DESIGN.md](DESIGN.md) for complete philosophy

# Case Studies & Real-World Patterns

**Find solutions to problems you're facing, indexed by problem type and context**

This directory contains 12 detailed adoption reports from production systems that have used rebar across 100+ agent launches. Each report includes specific problems encountered, solutions implemented, and lessons learned.

**Use this when:** You're facing a challenge and want to see how others solved similar problems.

---

## Quick Problem Navigation

### 🔍 **"I have this specific problem"**

| Problem | See This Case Study | Key Solution |
|---------|---------------------|--------------|
| **Numbers in docs drift from reality** | [Human-based Digital Signer](feedback/digital-signer-feedback.md) | Ground truth enforcement layer |
| **Want to adopt rebar in mature codebase** | [blindpipe Adoption](feedback/processed/blindpipe-adoption-2026-03-19.md) | Selective adoption strategy |
| **Scaling beyond solo development** | [Scalability Assessment](feedback/processed/scalability-assessment-2026-03-20.md) | Tier progression patterns |
| **Version migration and compatibility** | [Versioning & Upgrade Path](feedback/versioning-and-upgrade-path-2026-03-20.md) | Backwards compatibility strategies |
| **Multi-agent coordination failures** | [OpenDocKit Fidelity Session](feedback/processed/2026-03-18-opendockit-fidelity-session.md) | Swarm orchestration protocols |
| **Role boundaries unclear** | [Role Definitions Feedback](feedback/processed/role-definitions-feedback.md) | Agent responsibility clarification |
| **Testing everything vs. testing enough** | [Zero-Tolerance Testing](feedback/zero-tolerance-testing-feedback.md) | Pragmatic quality gates |
| **Contract design for AI agents** | [AI-Native Contracts](feedback/processed/ai-native-contracts-2026-03-20.md) | Machine-readable specifications |
| **Legacy system migration** | [PDF Signer Migration](feedback/processed/pdf-signer-migration-feedback.md) | Incremental adoption patterns |

### 🏗️ **"I'm building this type of project"**

| Project Type | Case Study | Key Insights |
|--------------|------------|--------------|
| **Web App** (React + API) | [Human-based Digital Signer](feedback/digital-signer-feedback.md) | Monorepo patterns, E2E testing with 586 unit tests |
| **Crypto Library** | [blindpipe](feedback/processed/blindpipe-adoption-2026-03-19.md) | Security-critical development, selective adoption |
| **Document Processing** | [OpenDocKit](feedback/processed/2026-03-18-opendockit-fidelity-session.md) | 5,824 tests, progressive fidelity rendering |
| **Enterprise Platform** | [Scalability Assessment](feedback/processed/scalability-assessment-2026-03-20.md) | Multi-repo coordination, governance patterns |

### 👥 **"My team looks like this"**

| Team Size | Case Study | Coordination Patterns |
|-----------|------------|---------------------|
| **Solo Developer** | [blindpipe](feedback/processed/blindpipe-adoption-2026-03-19.md) | ASK CLI for 10x context efficiency |
| **Small Team (2-10)** | [Human-based Digital Signer](feedback/digital-signer-feedback.md) | 18 agents, 3 phases, 0 merge conflicts |
| **Department (10+)** | [Scalability Assessment](feedback/processed/scalability-assessment-2026-03-20.md) | Cross-repo contracts, breaking change detection |
| **Multi-Agent Swarms** | [OpenDocKit](feedback/processed/2026-03-18-opendockit-fidelity-session.md) | 9 simultaneous agents, failure recovery |

---

# Complete Case Study Catalog

## Production Success Stories

### 🎯 **Human-based Digital Signer** — Web App Excellence
**File:** [feedback/digital-signer-feedback.md](feedback/digital-signer-feedback.md) • **Size:** 20.3K

**Project:** React/Vite + Express API monorepo with biometric identity
**Scale:** 586 unit tests, 80 E2E specs, ~50k LOC
**Agent Usage:** 18 agents across 3 phases, maximum autonomy, worktree isolation

**🔥 Key Achievement:** 0 merge conflicts across 18 agents and 3 development phases

**Key Insights:**
- **Ground truth enforcement** prevents docs from drifting (numbers, metrics, test counts)
- **Quantitative claims** need machine verification, not just structural checks
- **Heavy agent usage** requires robust conflict avoidance and progress tracking

**Best for:** Web application teams, monorepos, heavy automation, quality obsessed teams

### 🔐 **blindpipe** — Crypto-Critical Selective Adoption
**File:** [feedback/processed/blindpipe-adoption-2026-03-19.md](feedback/processed/blindpipe-adoption-2026-03-19.md) • **Size:** 6.1K

**Project:** Zero-knowledge collaborative office suite (Go + TypeScript)
**Context:** Mature codebase with existing contract system
**Approach:** Selective adoption rather than wholesale replacement

**🔥 Key Achievement:** 10x context efficiency via persistent ASK sessions

**Key Insights:**
- **Selective adoption** works better for mature projects than full bootstrap
- **Enforcement scripts** provide highest immediate value
- **Role-based agents** create natural knowledge partitioning for complex domains
- **ASK CLI persistence** dramatically reduces context switching costs

**Best for:** Mature projects, security-critical systems, incremental adoption

### 📄 **OpenDocKit** — Massive Multi-Agent Coordination
**File:** [feedback/processed/2026-03-18-opendockit-fidelity-session.md](feedback/processed/2026-03-18-opendockit-fidelity-session.md) • **Size:** 17.6K

**Project:** Progressive-fidelity OOXML renderer
**Scale:** 5,824 tests, 9 simultaneous agents
**Focus:** Proving swarm coordination protocols under stress

**🔥 Key Achievement:** 100% work recovery after login expiration incidents

**Key Insights:**
- **Commit-per-chunk protocol** minimizes lost work during agent failures
- **6-rule agent protocol** scales to 9 simultaneous agents without conflicts
- **Failure recovery patterns** can be systematized and automated
- **Cross-agent learning** amplifies individual agent discoveries

**Best for:** Large codebases, complex coordination, failure-tolerant systems

## Strategic & Scaling Patterns

### 📈 **Scalability Assessment** — Solo to Enterprise
**File:** [feedback/processed/scalability-assessment-2026-03-20.md](feedback/processed/scalability-assessment-2026-03-20.md) • **Size:** 19.7K

**Scope:** Tier 1 (solo) → Tier 2 (team) → Tier 3 (department) → Enterprise
**Focus:** What breaks at each scale transition and how to evolve gracefully

**🔥 Key Achievement:** Gradual scaling path with zero rewrites

**Key Insights:**
- **Core abstractions** (contracts, testing cascade, agents) work at any scale
- **Infrastructure evolution** from files → services as teams grow
- **Governance emerges** naturally through contract catalogs and breaking change detection
- **Enterprise coordination** requires cross-repo namespacing and shared knowledge

**Best for:** Growing teams, scaling strategies, organizational evolution planning

### 🔧 **Scalability Deep Review** — Technical Implementation
**File:** [feedback/processed/scalability-deep-review-2026-03-20.md](feedback/processed/scalability-deep-review-2026-03-20.md) • **Size:** 20.7K

**Companion to:** Scalability Assessment (focuses on technical implementation details)

### 🔄 **Versioning & Upgrade Path** — Change Management
**File:** [feedback/versioning-and-upgrade-path-2026-03-20.md](feedback/versioning-and-upgrade-path-2026-03-20.md) • **Size:** 4.5K

**Focus:** Backwards compatibility, migration strategies, version management
**Key Insights:** Compliance markers, tier enforcement, upgrade automation

## Quality & Testing Patterns

### 🧪 **Zero-Tolerance Testing** — Quality Extremes
**File:** [feedback/zero-tolerance-testing-feedback.md](feedback/zero-tolerance-testing-feedback.md) • **Size:** 4.4K

**Focus:** When to apply maximum testing rigor vs. pragmatic quality gates
**Key Insights:** Testing cascade application, risk-based quality decisions

### 📋 **PDF Signer Migration** — Legacy Integration
**File:** [feedback/processed/pdf-signer-migration-feedback.md](feedback/processed/pdf-signer-migration-feedback.md) • **Size:** 24.6K

**Project:** Migrating legacy PDF signing system to rebar patterns
**Focus:** Incremental adoption in brownfield systems

## Framework Evolution

### 🤖 **AI-Native Contracts** — Agent-Optimized Design
**File:** [feedback/processed/ai-native-contracts-2026-03-20.md](feedback/processed/ai-native-contracts-2026-03-20.md) • **Size:** 4.8K

**Focus:** Contract frontmatter and structure optimized for AI agent consumption
**Key Insights:** Machine-readable specifications, agent-friendly formats

### 👥 **Role Definitions** — Agent Responsibility Boundaries
**File:** [feedback/processed/role-definitions-feedback.md](feedback/processed/role-definitions-feedback.md) • **Size:** 9.3K

**Focus:** Clarifying architect vs. product vs. englead vs. steward responsibilities
**Key Insights:** Role boundary design, coordination handoff patterns

### 🔄 **Swarm Orchestration SOP** — Advanced Coordination
**File:** [feedback/processed/2026-03-21-swarm-orchestration-sop.md](feedback/processed/2026-03-21-swarm-orchestration-sop.md) • **Size:** 16.9K

**Focus:** Collective learning frameworks, cross-agent knowledge sharing
**Key Insights:** P0-P5 enhancement roadmap for swarm intelligence

---

# Practices & Workflow Guides

**When you need specific tactical guidance for ongoing work**

## Current Practice Guides

### 🔀 **Multi-Agent Orchestration**
**File:** [practices/multi-agent-orchestration.md](practices/multi-agent-orchestration.md)

**When to read:** Planning fan-out work across multiple agents, pre-launch coordination audits
**Covers:** Sharding strategies, conflict avoidance, progress tracking, post-integration

### 🧪 **E2E Testing Infrastructure**
**File:** [practices/e2e-testing.md](practices/e2e-testing.md)

**When to read:** Setting up or debugging end-to-end test environments
**Covers:** Managed test stacks, health checks, timeout strategies, test data management

### 🚀 **Deployment Patterns**
**File:** [practices/deployment-patterns.md](practices/deployment-patterns.md)

**When to read:** Deploying to production environments, debugging deployment failures
**Covers:** Production deployment strategies, rollback procedures, environment management

### 🌿 **Worktree Collaboration**
**File:** [practices/worktree-collaboration.md](practices/worktree-collaboration.md)

**When to read:** Coordinating parallel agent development, resolving merge conflicts
**Covers:** Worktree isolation, cherry-pick strategies, conflict resolution protocols

---

# How to Use This Guide

## 📚 **For Learning**
- **Start with:** [Human-based Digital Signer](feedback/digital-signer-feedback.md) or [blindpipe](feedback/processed/blindpipe-adoption-2026-03-19.md) — see rebar in action
- **Then read:** [Scalability Assessment](feedback/processed/scalability-assessment-2026-03-20.md) — understand growth patterns
- **Deep dive:** [OpenDocKit](feedback/processed/2026-03-18-opendockit-fidelity-session.md) — see complex coordination patterns

## 🔧 **For Solving Problems**
1. **Check the problem navigation table** above for your specific issue
2. **Read the relevant case study** to see how others solved it
3. **Adapt the solution** to your context and constraints
4. **Refer to practice guides** for tactical implementation details

## 📈 **For Planning**
- **Team scaling:** [Scalability Assessment](feedback/processed/scalability-assessment-2026-03-20.md)
- **Adoption strategy:** [blindpipe](feedback/processed/blindpipe-adoption-2026-03-19.md) (mature projects) or [SETUP.md](SETUP.md) (new projects)
- **Quality standards:** [Zero-Tolerance Testing](feedback/zero-tolerance-testing-feedback.md)
- **Migration planning:** [PDF Signer Migration](feedback/processed/pdf-signer-migration-feedback.md)

---

**Remember:** These are real production experiences, not theoretical examples. Every solution has been battle-tested across hundreds of agent launches. Adapt the patterns to your context, but trust the underlying insights — they've been validated by teams building complex systems under real constraints.