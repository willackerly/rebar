# Feedback: Role Definitions for ASK CLI — What's Missing from the Templates

**Source project:** Dapple SafeSign (pdf-signer-web)
**Context:** Implemented all 4 role AGENT.md files for the ASK CLI agent hierarchy
**Date:** 2026-03-18

---

## The Gap

The rebar repo has `agents/product/AGENT.md` and `agents/architect/AGENT.md` as 12-line skeletons. Two of the four roles in the hierarchy (`englead` and `engineer`) don't exist at all. For the ASK CLI to power role-based reasoning, each role needs a **substantive** definition — not a stub.

We built full role files for SafeSign (50-150 lines each). This feedback captures what we learned about what makes a role definition useful vs decorative, and proposes changes to the templates.

---

## What a Role Definition Needs

After writing all 4 roles, a clear pattern emerged. Every effective AGENT.md has these sections:

### 1. Core Identity (2-3 sentences)
Not just "you are the X agent" — a statement of **what you care about** and **how you think**. This shapes the agent's reasoning posture:

- Product: "You think in terms of personas, workflows, and outcomes — not implementations."
- Architect: "You are the guardian of the contract system."
- Eng Lead: "You bridge product intent and working code."
- Engineer: "You are a contractor working from blueprints."

### 2. Context Loading Order (explicit, numbered)
Which files to read, in what order. This is critical — it determines what the agent knows before it starts reasoning. Each role reads **different files first**:

| Role | First Reads | Why |
|------|------------|-----|
| Product | QUICKCONTEXT → Product Requirements | Needs current state + user needs |
| Architect | DESIGN.md → CONTRACT-REGISTRY → Security Rules | Needs philosophy + contract inventory |
| Eng Lead | QUICKCONTEXT → TODO → AGENTS → CLAUDE | Needs full operational picture |
| Engineer | subagent-guidelines → assigned template → assigned contract | Needs task spec + contract |

The template skeletons don't specify this order. Without it, every role reads the same files in the same order and loses its differentiated perspective.

### 3. Decision Framework (project-specific filters)
A numbered checklist the agent applies to every decision. This is where zero-knowledge, offline-first, and other constraints get encoded as **role-specific filters**:

- Product asks: "Does it serve Sarah, Alex, or Jordan?" before "Is it technically feasible?"
- Architect asks: "Does it violate zero-knowledge?" before "Is it a clean design?"
- Eng Lead asks: "Does this follow existing patterns?" before "Should I build something new?"
- Engineer asks: "Does my contract cover this?" before "Should I expand scope?"

### 4. What You Own / What You Don't
Explicit boundaries. The Product agent doesn't write code. The Engineer doesn't make architectural decisions. The Architect doesn't set priorities. These boundaries are what make the hierarchy work — without them, every role tries to do everything.

### 5. Permissions Matrix
Read/write/ask permissions per role. This maps directly to the table in AGENT-RUNTIME.md but needs to be concrete and project-specific:

```
Product:   Read: all  | Write: product/, TODO (priorities only) | Ask: any
Architect: Read: all  | Write: architecture/, conventions       | Ask: any
Eng Lead:  Read: all  | Write: all code, all docs, deploy test  | Ask: Product, Architect
Engineer:  Read: all  | Write: assigned scope only              | Ask: via findings only
```

### 6. How to Find Things (role-specific)
Each role needs different `grep`/`ls` commands. The Architect greps for `CONTRACT:`. The Engineer reads file headers. The Eng Lead runs test suites. Including these as concrete shell commands makes the role immediately operational.

---

## What's Missing from the Template Skeletons

### `agents/product/AGENT.md` (current: 12 lines)

**Missing:**
- Personas (who are the users?)
- Decision framework (how to evaluate features)
- Domain knowledge (what does the market look like?)
- Context loading order (what to read first)
- What you DON'T decide (explicit boundaries)

**Recommended template size:** 50-70 lines with `<!-- customize -->` comments for project-specific personas and domain knowledge.

### `agents/architect/AGENT.md` (current: 12 lines)

**Missing:**
- Contract inventory (which contracts exist, grouped by type)
- Architecture patterns to enforce (project-specific)
- Decision framework (how to evaluate design proposals)
- Security invariants (if applicable)
- `grep` commands for finding contracts and implementations

**Recommended template size:** 70-100 lines. The Architect role has the most structure because it owns the contract system.

### `agents/englead/AGENT.md` (DOESN'T EXIST)

This is the most important role — it's the **default persona**. It needs:
- Codebase map (package structure, key directories)
- Test infrastructure summary (commands, counts, tiers)
- Testing cascade (T0-T5, inline — not just a reference)
- Delegation patterns (subagent templates, worktree rules, pre-launch audit)
- Session protocol (what to do on start, what to do on end)
- Deploy safety rules

**Recommended template size:** 100-150 lines. This role needs the most context because it's the one humans actually interact with.

### `agents/engineer/AGENT.md` (DOESN'T EXIST)

Needs:
- "Read your contract first" as hard rule #1
- Scope discipline rules (don't expand, don't redesign)
- Finding protocol (how to report architectural discoveries)
- Testing-before-commit matrix (which test tier for which change scope)
- Security context (project-specific hard rules, condensed)

**Recommended template size:** 50-70 lines. Shorter because Engineers are scoped — they don't need the full codebase map.

---

## Proposed Template Structure

Each role's `AGENT.md` template should follow this structure:

```markdown
# Agent: {Role}

## Role
<!-- One paragraph: who you are and how you think -->

## Core Identity
<!-- 2-3 sentences: what you care about, your reasoning posture -->

## Context Loading Order
<!-- Numbered list: which files to read, in what order -->

## Responsibilities
<!-- Bullet list: what you do -->

## Decision Framework
<!-- Numbered checklist: filters applied to every decision -->
<!-- Customize with project-specific constraints -->

## What You Own
<!-- Tables of contracts/files/directories this role manages -->

## What You Don't Decide
<!-- Explicit boundaries — prevents role overlap -->

## Permissions
<!-- Read/Write/Ask matrix -->

## Memory
<!-- Path to memory.md and memory.log.md -->
```

---

## The Eng Lead Problem

The biggest gap in the templates is the missing `englead` role. In practice, this is the role that 90% of human↔agent interaction goes through. It's the one that:

- Reads the Cold Start Quad on session start
- Decides how to implement features
- Runs tests and interprets results
- Delegates to Engineer subagents
- Manages deployments
- Updates documentation

Without a rich `englead/AGENT.md`, the ASK CLI has no default persona. The human types `ask englead "what's the current blocker?"` and gets a generic response instead of one informed by the full codebase context, test counts, deploy status, and active workstreams.

**Recommendation:** The `englead` template should be the most detailed of the four roles. It should include the testing cascade inline (not just a reference to AGENTS.md), the codebase structure map, delegation patterns, and session protocol. This is the role that makes the hierarchy useful to humans.

---

## Key Insight: Roles Are Context Scoping Mechanisms

The hierarchy isn't about rank or authority — it's about **context scoping**. Each role loads different context, applies different filters, and owns different artifacts:

| Role | Primary Context | Primary Filter | Primary Artifact |
|------|----------------|----------------|------------------|
| Product | User needs, personas | "Does this serve a user?" | Requirements, priorities |
| Architect | Contracts, patterns | "Does this respect boundaries?" | Contract documents |
| Eng Lead | Code, tests, deploys | "Does this work and is it safe?" | Working software |
| Engineer | Assigned contract + files | "Does this match the spec?" | Code changes |

The `ask` primitive enables this scoping: `ask architect "should we change the encryption algorithm?"` routes to a context that has all 17 contracts loaded and knows the security invariants. `ask product "should we add this feature?"` routes to a context that has personas and requirements loaded.

Without rich role definitions, every `ask` goes to the same generic context, and the hierarchy is decorative.

---

## Summary: Changes Needed in rebar

| File | Current State | Action |
|------|--------------|--------|
| `agents/product/AGENT.md` | 12-line skeleton | Expand to 50-70 line template with sections above |
| `agents/architect/AGENT.md` | 12-line skeleton | Expand to 70-100 line template with contract inventory section |
| `agents/englead/AGENT.md` | **Doesn't exist** | Create 100-150 line template (most important role) |
| `agents/engineer/AGENT.md` | **Doesn't exist** | Create 50-70 line template with scope discipline rules |
| `AGENT-RUNTIME.md` | References all 4 roles | Update to reflect template expectations |
| `SETUP.md` | No role setup instructions | Add "Step: Customize role definitions" |
| `agents/README.md` | No role directory docs | Add role directory structure explanation |
