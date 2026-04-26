# Agent Guidelines

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: YYYY-MM-DD -->
<!-- adopters: replace YYYY-MM-DD with the date you copied this template,
     and bump it whenever you edit. Stale dates >14 days fail check-freshness.sh. -->

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

### The Four Contract Principles
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

## Single Source of Truth for Metrics (W1-2)

Every quantitative claim in documentation must have ONE authoritative source.
All other documents cross-reference it. Numeric drift is the fastest-moving
form of doc rot — every commit that adds a test file silently invalidates
counts in 3-5 documents unless there's a single computed source.

The `METRICS` file is the canonical location for project-wide numbers.
`scripts/check-ground-truth.sh` verifies it against code. When the metric
changes, update `METRICS`; the ground truth script catches stale
cross-references.

| Metric | Authoritative Source | Verified By | Cross-Referenced In |
|--------|---------------------|-------------|---------------------|
| Test count | `METRICS` (computed via test runner) | `check-ground-truth.sh` | QUICKCONTEXT.md, CLAUDE.md |
| Contract count | `METRICS` (computed from `architecture/CONTRACT-*.md`) | `check-ground-truth.sh` | CONTRACT-REGISTRY.md |
| Endpoint count | `METRICS` (computed from route handlers) | `check-ground-truth.sh` | API specs |

**Anti-pattern:** hardcoding the same number in 5 documents. **Pattern:** put
the number in `METRICS`, reference it from prose, regenerate on commit.

---

## Production Deploy Confirmation (W1-3)

Deploy scripts that target production MUST require interactive confirmation.
Without this, agents with "maximum autonomy" can deploy autonomously —
autonomy grants are for development workflow, not production operations.

```bash
# In your production deploy script:
if [ -t 0 ]; then
  read -p "Deploy to PRODUCTION? Type 'yes' to confirm: " confirm
  [ "$confirm" = "yes" ] || { echo "Aborted."; exit 1; }
else
  echo "ERROR: Production deploy requires interactive terminal (TTY)."
  echo "This prevents automated/scripted deploys without human confirmation."
  exit 1
fi
```

The `-t 0` test ensures the script is running in an interactive terminal,
not piped or scripted. **This is a deliberate friction point** — the one
place where we want to slow agents down. Document for your project: which
deploy commands target production vs staging, which have this guard, and
how to bypass for CI/CD pipelines (e.g., `DEPLOY_CONFIRMED=1`).

For the full deployment-pattern catalog (origin allowlists, MIME types,
build-time env vars, etc.), see `practices/deployment-patterns.md`.

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

### Regression-Fix Gates H + L (W3-4)

When the prompt is "fix this regression," two doctrines bind:

**Gate H — Single-fix-isolation.** Each `fix:` commit must be paired with a
verify step in your reasoning: *"I applied X; the symptom is now Y;
therefore X was/wasn't the cause."* Don't apply Fix A + Fix B + cleanup all
at once and declare victory — when the result is good you can't attribute
it; when it's bad you can't isolate it.

**Gate L — Fix-your-own-test-drift.** When a test fails after your change,
the FIRST hypothesis is *"I broke the test's contract assumption"* —
investigate before bypassing. If the contract change is intended, the test
update is part of the same PR. "Test contract drift" is not a free pass.

Mechanical commit-msg gates (`scripts/check-fix-commit.sh` for Gate G,
`scripts/check-bypass-flags.sh` for Gate I) catch the related failure modes
that ARE script-detectable. See `practices/regression-fix-protocol.md` for
the full six-gate protocol.

### The Scout Rule: Zero Tolerance for Broken Tests (W1-4)

**You're a scout. Leave the camp cleaner than you found it.**

| Situation | Action |
|-----------|--------|
| Test fails after your change | Fix the code or fix the test |
| Test was already failing before your change | Fix it NOW — you found it, you own it |
| Test times out | Timeout is wrong OR product is broken — fix one |
| Skipped test | Fix the skip. Scope it properly or delete. Never leave a `skip`. |
| Flaky test | Stabilize or delete. Flaky = lying about coverage. |
| Obsolete test (OBE) | Remove carefully. Verify behavior is gone or covered elsewhere. |
| Platform-specific test | Use proper conditions (`if platform == X`), not blanket `skip`. |

**Forbidden phrases:**
- "Pre-existing failure" — that's a tracking failure, not an excuse
- "Not caused by our changes" — investigate every failure
- "Flaky" without a root cause in the commit message

**Why absolute:** A `skipped` test is invisible debt; it rots and gives false
confidence. Every session that walks past a broken test makes the problem
worse. Fixing a test you didn't break is not extra work — it's the cost of
working in a shared codebase. The 30-second fix you defer today becomes a
30-minute archaeology project next month.

**Practical:** before starting a task, run the relevant tier; if anything is
red or skipped, fix first. After finishing, run again — leave them greener
than you found them. If a fix would take >30 min and block your task,
create a P0 in TODO.md with a deadline — but this is the exception, not the
norm.

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