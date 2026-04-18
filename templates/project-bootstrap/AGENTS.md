# Agent Guidelines

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: 2026-03-21 -->

**How AI agents work effectively in this project.**

---

## Quick Start for New Agents

### Essential Reading Order (5 minutes)
1. **README.md** — what is this project?
2. **QUICKCONTEXT.md** — what's true right now? (branch, tests, active work)
3. **VERIFY:** Run `git log --since='7 days' --oneline | head -20` and
   cross-reference against QUICKCONTEXT claims. Flag any discrepancies.
4. **TODO.md** — what needs doing? (open items only, scannable in 10 seconds)
5. **This file** — how do we work together?

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

### Parallel Agent Protocol

When launching subagents for parallel work, every agent follows **The 10 Rules**
in `agents/subagent-guidelines.md`. The orchestrator follows the pre-launch audit
and merge strategy in `practices/multi-agent-orchestration.md`.

**The 3 absolute invariants (never violated):**
1. **Worktree isolation** for any agent writing code
2. **Commit before completing** — no commit = no work
3. **Strict file ownership** — each agent gets an explicit allowlist; shared
   files (interfaces, types, router, App entry point) are orchestrator-only

---

## Testing Cascade

**Fast inner loops, rigorous outer gates.** Never run the full suite when a
targeted test will do. Iterate at the speed of a single test file.

### The Tiers

<!-- Customize commands for your project's test runner -->

| Tier | Name | Speed | When to Run |
|------|------|-------|-------------|
| **T0** | Typecheck | <5s | Every meaningful edit |
| **T1** | Targeted | <10s | Every change cycle (single test file) |
| **T2** | Package | <30s | Before committing |
| **T3** | Cross-package | <60s | Before pushing |
| **T4** | Visual/E2E | <2min | UI/render changes |
| **T5** | Full suite | <10min | Release prep |

**Rules:** Iterate at T1. Promote on success. Background T3+. Never run T5
in your inner loop.

### For This Project
**Start with T0-T2**, add T3+ as the project grows.

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

## Session Lifecycle

Sessions have three stages. The Cold Start Quad covers START. Checkpoints
and wrapup are equally important.

**See `practices/session-lifecycle.md` for the full protocol.**

### Checkpoint (every 10 commits or 2 hours)
- Update QUICKCONTEXT.md (at minimum: timestamp + what shipped)
- Commit work-in-progress
- Check context quality — if re-reading files or repeating searches, break and restart

### Session End
- Update QUICKCONTEXT.md with current state (not aspirational)
- Update TODO.md (check completed items, add discovered items)
- Clean up worktrees: `git worktree list` → `git worktree prune`
- Write a session wrapup (see template in `practices/session-lifecycle.md`)

---

## Priority and Issue Tracking Rules

### Priority Tracking
- **QUICKCONTEXT.md "What's Next"** = the single source of truth for priorities
- **TODO.md** = detailed task list with context, NOT a separate priority list
- If both files have a priority ordering, QUICKCONTEXT wins

### Issue Tracking
- **TODO.md "Known Issues"** = what's broken + workaround + fix tracking
- **Cross-reference, don't duplicate.** One canonical entry per issue.
- If an issue appears in multiple files, pick ONE as canonical and add
  "See TODO.md §Known Issues" to the others

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