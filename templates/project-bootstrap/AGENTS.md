# Agent Guidelines

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: 2026-03-21 -->

**How AI agents work effectively in this project.**

---

## Quick Start for New Agents

### Essential Reading Order (5 minutes)
1. **README.md** — what is this project?
2. **QUICKCONTEXT.md** — what's true right now? (branch, tests, active work)
3. **TODO.md** — what needs doing? (P0 tasks, blockers, discoveries)
4. **This file** — how do we work together?

### Project Context
- **Project type:** [Web app/API service/Library/CLI tool]
- **Team size:** [Solo/Small team/Department]
- **Rebar tier:** 1 (Partial) — contract refs + TODO tracking enforced
- **Quality standards:** Contract-first development, progressive testing cascade

---

## Contract-Driven Development

### Core Principle
**Don't implement without a contract. Don't modify code without checking its contract.**

Every component gets a behavioral specification in `architecture/CONTRACT-*.md` before implementation begins.

### The Four Contract Rules
1. **Don't implement without a contract** — write the spec first
2. **Don't modify code without checking its contract** — understand what you're changing
3. **Don't update a contract without searching all implementations** — `grep -rn "CONTRACT:C1-NAME"`
4. **Contract changes that break interfaces** → plan mode with architect + product + englead

### Contract Linking
```go
// filestore.go
// CONTRACT:C1-FILESTORE.1.0
package storage
```

Every source file gets a `CONTRACT:` header linking to its specification.

---

## Agent Coordination

### Role-Based Persistent Agents

**Use `ask <role> "<question>"` for guidance and coordination:**

```bash
ask architect "Should we add caching to the user service?"
ask product "Does this contract match our user requirements?"
ask englead "Are we ready to ship this feature?"
ask steward summary  # Automated quality health check
```

**Available roles:**
- **architect** — system design, contracts, technical architecture
- **product** — user requirements, BDD scenarios, feature prioritization
- **englead** — delivery coordination, quality gates, team management
- **steward** — automated quality scanning, contract health, compliance
- **tester** — test strategy, coverage analysis, quality assurance
- **merger** — branch integration, conflict resolution, post-development coordination

### Multi-Agent Coordination

**Before major changes:**
1. **Ask architect** for system design impact
2. **Ask product** for user experience considerations
3. **Ask englead** for implementation and timeline planning
4. **Coordinate implementation** using shared contracts as the interface

**During implementation:**
- **Commit frequently** — uncommitted work is lost work (login expires, crashes)
- **Update QUICKCONTEXT.md** as you change project state
- **Link to contracts** — maintain `CONTRACT:` headers in all source files
- **Run quality checks** — `scripts/check-contract-refs.sh` before commits

---

## Testing Cascade

### Progressive Quality Gates
- **T0: Unit tests** — test contract behavior directly
- **T1: Integration tests** — test component interactions
- **T2: Security tests** — test edge cases, input validation, error handling
- **T3: System tests** — full workflow validation
- **T4: Load tests** — performance and scalability
- **T5: Chaos tests** — failure mode validation

### For This Project
**Start with T0-T1**, add T2+ as system grows in complexity and criticality.

**Quality enforcement:**
```bash
scripts/check-contract-refs.sh  # Contract links valid
scripts/check-todos.sh         # No untracked TODOs
scripts/ci-check.sh           # Full quality scan
```

---

## Two-Tag System

### TODO Tracking
- **`TODO:` in code** = untracked = **blocks commit**
- **`TRACKED-TASK:` in code** = tracked in TODO.md = commit allowed

```go
// BAD: This blocks commit
// TODO: Handle edge case for concurrent access

// GOOD: This is tracked and commit-safe
// TRACKED-TASK:TODO.md#handle-concurrency Handle edge case for concurrent access
```

---

## Project-Specific Guidelines

### Domain Knowledge
- [Key domain concepts agents should understand]
- [Business logic or technical constraints specific to this project]

### Code Patterns
- [Coding conventions for this project]
- [Architectural patterns to follow]
- [Patterns to avoid]

### Integration Points
- [External APIs, services, databases this project connects to]
- [Data formats, protocols, authentication requirements]

### Deployment
- [How code gets deployed]
- [Environment differences (dev/staging/prod)]
- [Rollback procedures]

---

## Autonomy Levels

### Tier 1 (Current): Guided Development
- **READ** any project file to understand context
- **ASK** architect/product/englead before major design decisions
- **MODIFY** code within established contracts
- **CREATE** tests, documentation, implementation code
- **RUN** quality checks and enforcement scripts
- **UPDATE** QUICKCONTEXT.md, TODO.md, and other living documents

### What Requires Coordination
- **New contracts** — review with architect + product
- **Breaking changes** — plan mode with all relevant roles
- **Architecture decisions** — consult architect
- **Product decisions** — consult product
- **Quality standards** — consult englead

### Emergency Procedures
- **If agent crashes mid-work** — check for uncommitted changes, recover from git
- **If tests start failing** — stop new work, focus on getting back to green
- **If contracts drift** — run `ask steward summary` to identify issues

---

## Success Metrics

### Development Velocity
- Time from user story → deployed feature
- Contract specification → working implementation
- Bug report → fix deployed

### Quality Indicators
- Test coverage and pass rate
- Contract compliance (via steward scanning)
- Agent coordination effectiveness (merge conflicts, rework)

### Team Satisfaction
- Agent-human collaboration quality
- Documentation usefulness and accuracy
- System reliability and maintainability

---

**Remember:** Agents work best when they understand both the technical system AND the human context. Read the project files, ask questions, coordinate decisions, and maintain shared understanding through contracts and documentation.