# Repository Guidelines

## Read Before Coding

**The Cold Start Quad (every session, every agent, no exceptions):**
1. `README.md` — universal orientation (ALWAYS first)
2. `QUICKCONTEXT.md` — current state (verify against `git log --oneline -10`)
3. `TODO.md` — tasks + known issues + blockers
4. `AGENTS.md` (this file) — norms, contracts, collaboration

**Reference (read as needed):**
- `DESIGN.md` — the philosophy (contracts are the operating system)
- `architecture/CONTRACT-REGISTRY.md` — contract index
- `practices/` — specialized guidance (orchestration, E2E, deployment, worktrees, session lifecycle, red team)
- **ASK CLI** — query role-based agents: `ask architect "question"`, `ask product "question"`, `ask steward summary`. Persistent sessions save context across questions. See `bin/ask help`.
- **`rebar context [role]`** — cats role-relevant files in reading order (e.g., `rebar context architect` for contracts + DESIGN.md)

<!-- Add project-specific context files here, e.g.:
5. `docs/README.md` -> full documentation tree
6. `architecture/CONTRACT-REGISTRY.md` -> all contracts
-->

## Core Tenets

<!-- Mirror these from CLAUDE.md. Agents must internalize these before writing any code.
     These are non-negotiable architectural principles that override convenience. -->

1. **Offline-First** — Every feature must work without network access. Network is for enhancement, never a hard dependency. Test offline paths first.
2. **Client-Side Only** — Zero server dependencies for core functionality.
3. **Progressive Enhancement** — Render/function immediately with what's available, improve as resources load.

<!-- Customize for your project. The point: agents should check their work against these
     tenets before committing. "Does this feature work offline?" is a review checklist item. -->

---

## Agent Autonomy

**Maximum autonomy granted.** Act decisively. Ship code. Don't ask permission for routine work.

### Full Authority (no approval needed)
- Write, edit, refactor, delete code
- Run, write, fix tests
- Git: commit, push, branch, merge, rebase
- Deploy via configured deploy tooling
- Add/remove/upgrade dependencies
- Create, update, reorganize, archive documentation
- Fix bugs, improve error handling, optimize performance
- Implement features that follow existing patterns

### Requires Discussion (enter plan mode)
Only **fundamental architectural decisions** that are hard to reverse:
- New major dependencies (e.g., framework changes)
- Data model/schema changes (databases, API contracts)
- Security model changes (encryption, auth, key management)
- Creating new packages in the monorepo
- Protocol changes (inter-service communication, API versioning)
- Breaking changes affecting existing users/data

### Never Without Explicit Request
- `git push --force` to shared branches
- `git reset --hard` on commits others have
- Deleting production data
- Modifying production secrets

**Rule of thumb:** If it follows existing patterns and is reversible — just do it. If it establishes new patterns or is hard to undo — plan mode.

---

## Cold Start Methodology (MANDATORY for New Agent Sessions)

**When starting a new session, always perform this sanity check before acting:**

### Step 1: Verify Document Freshness (5 min)
Don't trust docs blindly. Cross-reference against actual state:

```bash
# 1. Check current branch (docs may reference wrong branch)
git branch --show-current
git log --oneline -10

# 2. Compare QUICKCONTEXT.md branch claim against reality
grep -i "branch" QUICKCONTEXT.md

# 3. Check TODO.md "Last synced" date
head -10 TODO.md

# 4. Verify recent commits match documented priorities
git log --oneline -20 | head -10
```

### Step 1b: Verify Ground Truth Metrics

If the project has a ground truth script and a `METRICS` file, verify
numeric claims match reality before trusting them:

```bash
# Compare documented metrics against codebase reality
[ -x scripts/check-ground-truth.sh ] && ./scripts/check-ground-truth.sh
```

If metrics have drifted, update `METRICS` BEFORE proceeding with your task.
Stale numeric claims cascade — an agent that trusts "126 tests" when there
are 586 will make wrong assumptions about coverage and maturity.

### Step 2: Identify Discrepancies
Look for these common drift patterns:
- **Branch mismatch**: Docs say one branch, you're on another
- **Phase status lag**: Code shows Phase N complete but docs say Phase N-1
- **Stale dates**: "Last Updated" > 2 weeks old warrants scrutiny
- **Missing features**: Grep for features in code vs docs

### Step 3: Update Before Acting
If you find discrepancies:
1. **Minor drift**: Update the doc inline while working
2. **Major drift**: Update docs FIRST, then proceed with task
3. **Conflicting signals**: Ask user for clarification

### Step 4: Strategic Assessment
Before diving into code, ask:
- What's the **actual** current state? (git log, file structure)
- What's the **documented** next step? (TODO.md priorities)
- Do they align? If not, which is authoritative?
- Are there **blocked** items I should avoid?

### Why This Matters
Multiple agents work async on this codebase. Docs drift when agents complete work but don't update all references. Taking 5 minutes to verify state prevents hours of wasted effort on outdated priorities.

---

## Session Lifecycle

Sessions have three stages. See `practices/session-lifecycle.md` for the full protocol.

| Stage | Trigger | Key Actions |
|-------|---------|-------------|
| **Start** | New session | Cold Start Quad + staleness verification |
| **Checkpoint** | Every 10 commits or 2 hours | Update QUICKCONTEXT, commit WIP, check context quality |
| **End** | Session closing | Update QUICKCONTEXT, update TODO, clean worktrees, write wrapup |

**Priority tracking rule:** QUICKCONTEXT.md "What's Next" is the single source of truth for priorities. TODO.md has task details but should NOT duplicate the priority ordering.

**Issue tracking rule:** One canonical entry per issue. Cross-reference, don't duplicate.

---

## Multi-Agent Orchestration

> For subagent templates, fan-out patterns, pre-launch audit, and feature
> inventory protocol, see **`practices/multi-agent-orchestration.md`**.
>
> **Key rules (always apply):** Subagents writing code MUST use worktree
> isolation. Subagents MUST commit before completing. Run a pre-launch audit
> before any parallel agent campaign.

### The 10 Rules (Mandatory for All Subagents)

Every subagent reads `agents/subagent-guidelines.md` which defines **The 10
Rules** — the non-negotiable protocol for parallel agent work. Summary:

| # | Rule | Why |
|---|------|-----|
| 1 | **Worktree isolation** for code changes | Prevents conflicts with parallel agents |
| 2 | **Commit after every logical chunk** | Uncommitted work = lost work on crash/logout |
| 3 | **Strict file ownership** — only modify your allowlist | Prevents merge conflicts |
| 4 | **No removals** without explicit authorization | Your worktree is stale; references exist you can't see |
| 5 | **Measure before AND after** | Catches regressions before they compound |
| 6 | **Run package tests** after each change | Fast (<10s), catches compile errors immediately |
| 7 | **Write progress** to shared file | Orchestrator sees what happened without reading transcripts |
| 8 | **Don't touch shared files** (types, router, App.tsx) | #1 source of merge conflicts |
| 9 | **Respect context briefing** — don't modify files in "Recent Changes" | Your snapshot doesn't have those changes |
| 10 | **Commit before completing** | Worktree is ephemeral; no commit = no work |

**Canonical source:** `agents/subagent-guidelines.md` — cite rules by number.

**Orchestrator protocol:** `practices/multi-agent-orchestration.md` — pre-launch
audit (9 steps), merge ordering, conflict zones, recovery.

---

## Testing Cascade (MANDATORY)

**Fast inner loops, rigorous outer gates.** Never run the full suite when a targeted test will do. Iterate at the speed of a single test file, promote through tiers of increasing rigor only when the current tier passes.

### The Tiers

<!-- Customize commands for your project's test runner and package manager -->

| Tier | Name | Target | Speed | When to Run | Command |
|------|------|--------|-------|-------------|---------|
| **T0** | Typecheck | Changed package | <5s | Every meaningful edit | `pnpm --filter <pkg> exec tsc --noEmit` |
| **T1** | Targeted | Single test file | <10s | Every change cycle | `npx vitest run path/to/test.ts` |
| **T2** | Package | One package's suite | <30s | Before committing | `pnpm --filter <pkg> test` |
| **T3** | Cross-package | All unit/integration | <60s | Before pushing | `pnpm test` |
| **T4** | Visual/E2E | Visual regression, E2E | <2min | UI/render changes | `pnpm test:e2e:smoke` |
| **T5** | Full suite | Everything | <10min | Release prep | `pnpm test && pnpm test:e2e && pnpm lint` |

### Rules for Agents

1. **Iterate at T1.** Your inner loop is: edit — run the specific test — edit. This should take <10 seconds.
2. **Promote on success.** Only escalate to the next tier when the current one passes. Never skip tiers.
3. **Background the expensive tiers.** T3+ should run in background sub-agents while you keep coding.
4. **Use `--related` when unsure.** Most test runners support running tests related to changed files.
5. **Never run T5 in your inner loop.** T5 is a release gate, not a development tool.
6. **T4 only for visual/UI changes.** If you changed business logic or a utility, T1-T3 are sufficient.
7. **Fan out validation.** After finishing a body of work, launch T3, T4, and lint as parallel background agents.

### Anti-Patterns

- **Run T5 after every change** — You'll spend more time waiting than coding.
- **Skip T1 and go straight to T3** — T3 runs hundreds of tests. T1 runs one. The feedback delay kills iteration speed.
- **Block on T3 while coding** — Run T3 in a background agent. Keep working on the next thing.
- **Run `pnpm test` to check one function** — Find the exact test file. Run that one.

---

## Testing Expectations

<!-- Describe your testing approach. Example:
Unit/integration coverage via your test runner; co-locate specs
beside code or inside `__tests__`. E2E specs should be tagged
(`@critical`, `@regression`) for CI selection.
-->

### Regression-Fix Gates H + L

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

### The Scout Rule: Zero Tolerance for Broken Tests

**"You're a scout. You leave the camp cleaner than when you came."**

| Situation | Action |
|-----------|--------|
| Skipped test | Fix the skip. Scope it properly or delete it. Never leave a `skip`. |
| Failing test | P0. Fix it before continuing your task. No exceptions. |
| Flaky test | Stabilize it or delete it. Flaky = lying about coverage. |
| Obsolete test (OBE) | Remove it carefully. Verify the behavior is gone or covered elsewhere. |
| Platform-specific test | Use proper conditions (`if platform == X`), not `skip`. |

**Why this is absolute:**
- A skipped test is invisible debt. It rots. It gives false confidence.
- A test suite with 50 skipped tests is lying about coverage.
- Every agent session that encounters a broken test and walks past it makes the problem worse.
- Fixing a test you didn't break is not extra work — it's the cost of working in a shared codebase.

**The rule in practice:**
1. Before starting your task, run the relevant test tier. If anything is red or skipped, fix it first.
2. After finishing your task, run the tests again. Leave them greener than you found them.
3. If fixing a broken test would take >30 minutes and block your current task, create a P0 entry in TODO.md and flag it — but this is the exception, not the norm.

### Contract-Driven Development

**Contracts are the operating system.** See `DESIGN.md` for the full
philosophy and `architecture/README.md` for the naming/linking system.

**The rules:**
1. **Don't implement without a contract.** Write the contract first.
2. **Don't modify code without checking its contract.** Read the `CONTRACT:`
   header comment, then read the contract document.
3. **Don't update a contract without searching implementations.**
   `grep -rn "CONTRACT:{id}" src/ internal/` finds all implementing code.
4. **Contract changes that break interfaces — plan mode.**

**Every source file** has a header declaring its contract:
```
// CONTRACT:C1-BLOBSTORE.2.1
```

**Contract tests are king.** If a contract test fails, nothing ships.

---

## E2E Testing

> For managed test stacks, Playwright configuration, port ranges, environment
> variable hygiene, and tier timeouts, see **`practices/e2e-testing.md`**.

## Deployment

> For deploy patterns, origin allowlists, MIME type issues, build-time env
> vars, and production deploy confirmation, see **`practices/deployment-patterns.md`**.

## Agent Collaboration

> For worktree isolation rules, cherry-pick conflict resolution, post-merge
> integration, role flows, and discovery flows, see **`practices/worktree-collaboration.md`**.

---

## Documentation Maintenance Policy

**Principle**: Code and docs must stay in sync. Outdated docs are worse than no docs — they mislead future agents and create compounding confusion.

**After every code change or task completion**, walk the doc tree and update affected files:

| Change Type | Docs to Update |
|-------------|----------------|
| **New feature/module** | Package README, architecture docs if structural |
| **API change** | Specifications first (contract-first!), then implementation |
| **Bug fix** | Relevant README if it clarifies behavior; remove stale warnings |
| **Config/env change** | Getting-started docs, package README, `.env.example` |
| **Test change** | Coverage docs if coverage shifts >=2pts |
| **Phase/milestone complete** | Plan docs status table, status docs |
| **New file/module** | Parent folder's README or header comment |

**Doc Update Checklist** (include in PR/commit):

1. **Local**: Did you update the nearest README (package, folder)?
2. **Specifications**: Did you update specs if interfaces changed?
3. **Plans**: Did you update plan docs if a task/phase completed?
4. **Status**: Did you update status docs for milestones or blockers?
5. **Breadcrumbs**: Are new files linked from parent READMEs so they're discoverable?

**Enforcement**: PRs that change code without corresponding doc updates should be flagged. When in doubt, over-document — it's cheaper to trim than to reconstruct context.

### Metric-Bearing Changes (High Drift Risk)

Quantitative claims (test counts, contract counts, endpoint counts) drift
faster than prose. These code changes invalidate the `METRICS` file and
any doc that cross-references those numbers:

| Code Change | What to Update |
|-------------|----------------|
| Add/remove test file | `METRICS` file, QUICKCONTEXT.md |
| Add/remove contract | `METRICS` file, CONTRACT-REGISTRY.md |
| Add/remove API route | `METRICS` file, corresponding spec |
| Change dependency version | Architecture docs referencing it |

### Single Source of Truth for Metrics

Every quantitative claim must trace to ONE authoritative source.
The `METRICS` file is the canonical location for project-wide numbers.
`scripts/check-ground-truth.sh` verifies it against code.

| Metric | Computed From | Verified By | Referenced In |
|--------|--------------|-------------|---------------|
| Test count | `tests/` directory | `check-ground-truth.sh` | QUICKCONTEXT.md |
| Contract count | `architecture/CONTRACT-*.md` | `check-ground-truth.sh` | CONTRACT-REGISTRY.md |
| Contract coverage | `CONTRACT:` headers in source | `check-ground-truth.sh` | AGENTS.md |

<!-- Customize this table for your project's metrics. -->

### Archive Policy

**When to archive:**
- Feature/phase 100% complete and no longer changing
- Status snapshot > 3 months old AND newer snapshot exists
- Planning doc for approach not implemented

**Never archive:** `AGENTS.md`, `QUICKCONTEXT.md`, `TODO.md`, `CLAUDE.md`, `DESIGN.md`, latest architecture contracts

**How to archive:**
1. Move to `docs/archive/YYYY-MM-DD-description/`
2. Add header: `ARCHIVED: [DATE] | REASON: [reason] | CURRENT: [link to replacement]`
3. Update `docs/archive/README.md` index
4. Remove link from parent README

---

## Quality Gates (run before every push)

<!-- Describe your quality gates. Example:
Follow `docs/testing/TEST_MATRIX.md` for the authoritative checklist
(lint, unit tests, contract tests, critical E2E, visual review, deploy smoke).
Document every run in your PR summary.
-->

**Skip Policy**

- No skipping core contract tests. If they flake, fix or revert.
- Any temporary skip must link to a tracking issue and include a removal date in the test file header. CI should fail if the deadline passes.

## Commit & PR Guidelines

Use conventional prefixes (`feat:`, `fix:`, `ui:`, `docs:`, `build:`). PRs must describe the user-facing impact, list touched packages/folders, link to docs or issues, and include screenshots/logs for UI or CLI changes. Call out new tests (or explain gaps) and note any follow-up work. Never commit secrets.

**Documentation in every PR**: List which docs were updated (or confirm none needed). Use the Doc Update Checklist above.

---

## TODO Tracking (MANDATORY PRE-COMMIT)

**This is a hard requirement for all agents.**

### Two-Tag System

| Tag | Meaning | Commit Allowed? |
|-----|---------|-----------------|
| `TODO:` | Untracked work | No - must track first |
| `TRACKED-TASK:` | In TODO.md/docs | Yes |

### Before Every Commit

```bash
# 1. Find untracked TODOs (should be 0 before commit)
# Adjust file extensions and directories for your project
grep -rn "TODO:" --include="*.ts" --include="*.tsx" --include="*.py" --include="*.go" src/ packages/ shared/

# 2. If untracked TODOs found:
#    - Add to TODO.md
#    - Convert TODO: -> TRACKED-TASK: in code
#    - Re-run check

# 3. Only commit when untracked TODOs = 0
```

### When Adding Code Comments

**Wrong (blocks commit):**
```
// TODO: Handle edge case for X
```

**Right (after tracking in TODO.md):**
```
// TRACKED-TASK: Handle edge case for X - see TODO.md "Code Debt"
```

### Periodic Scrub

Weekly or per-sprint, audit `TRACKED-TASK:` comments:
1. Verify each is still documented in TODO.md
2. Remove completed items from both code and docs
3. Update stale references

**See:** `CLAUDE.md` "TODO Tracking Methodology" for full details.
