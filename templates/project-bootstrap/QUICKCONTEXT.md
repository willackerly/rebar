# Quick Context

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: 2026-03-21 -->
<!-- last-synced: 2026-03-21 — date this file was verified against code -->

**Current state of the project for agents starting a new session.**

---

## Branch & State

- **Active branch:** `main`
- **Last deploy:** [Not yet deployed]
- **Environment:** Development setup
- **Database:** [Local/staging/production as applicable]

## Test Status

- **Unit tests:** 0 passing, 0 failing (setup needed)
- **Integration tests:** Not yet implemented
- **E2E tests:** Not yet implemented
- **Coverage:** 0% (baseline)

## Active Work

**Current sprint/focus:** Project setup and first contract implementation

**In progress:**
- Initial rebar setup and configuration
- First contract definition
- Basic project structure

**Recently completed:**
- Rebar bootstrap installation
- Agent configuration

**Blocked:**
- None currently

## Key Decisions

**Architecture decisions:**
- Using rebar contract-driven development
- Tier 1 (partial) enforcement to start
- Role-based agent coordination

**Tech stack:**
- [Fill in your technology choices]
- [Database, framework, etc.]

**Process decisions:**
- Contract-first development approach
- Quality enforcement via rebar scripts
- Agent coordination for multi-perspective input

## Context for Agents

**Project scope:** [Brief description of what this project does]

**User personas:** [Who uses this system]

**Key constraints:**
- [Performance, security, compliance requirements]
- [Team size, timeline, budget constraints]

**Integration points:**
- [External services, APIs, databases]
- [Other systems this connects to]

## Current Architecture

**Contracts:**
- No contracts defined yet (first implementation needed)

**Components:**
- Project structure established
- Rebar tooling configured
- Ready for first contract implementation

**Dependencies:**
- Rebar framework v1.2.0
- [Your project dependencies]

---

## Agent Guidelines for This Project

**When working on this project:**

1. **Check this file first** — understand current state before making changes
2. **Update this file** — when you change project state, update relevant sections
3. **Follow contract-first approach** — define behavioral contracts before implementation
4. **Coordinate with other agents** — use `ask` command for cross-role decisions
5. **Maintain quality gates** — run `scripts/check-*.sh` before commits

**Project-specific considerations:**
- [Any special requirements for this project]
- [Domain knowledge agents should know]
- [Patterns to follow or avoid]

---

**Last updated by:** Initial setup
**Next review:** When first feature is implemented