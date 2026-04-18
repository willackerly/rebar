# Feedback: Parallel Agent Integration Protocol

**Date:** 2026-04-04
**Source:** agents/, practices/, AGENTS-QUICKSTART.md
**Type:** improvement
**Status:** proposed
**Template impact:** agents/subagent-guidelines, practices/agent-orchestration, AGENTS-QUICKSTART.md
**From:** filedag architecture migration — 24 agents, 27 commits, ~50 merge operations across Stages 1-4

## What Happened

During a full architecture migration of the filedag project (path-centric → content-addressed, 4 stages, ~12 hours), we ran 24 subagents in parallel worktrees. The agents were highly productive — each delivered working code with tests. But **integration at merge time consumed ~40% of orchestrator effort.** The same five failure patterns repeated across every merge:

| Failure | Frequency | Time Cost | Root Cause |
|---------|-----------|-----------|-----------|
| Missing mock stubs | Every merge (50+) | 15-30 min each | Agent added Store method, 6 test files need stubs |
| Migration version collision | 4 times | 10-15 min each | Two agents both used "v7" when main was at v12 |
| Type removal breaking callers | 3 times | 20-30 min each | Agent "cleaned up" types used by code outside its scope |
| Duplicate declarations | 2 times | 5 min each | Two files in same package define same helper |
| File divergence (App.tsx) | 3 times | 30+ min each | Two agents modify same file concurrently |

**Total estimated integration overhead:** ~8 hours across the session, or roughly 40% of the 20-hour total effort. Most of this was avoidable.

## What Was Expected

REBAR's current agent guidelines say "use worktree isolation" and "commit before completing." These are correct but insufficient for **parallel agent orchestration.** The guidelines assume agents work on independent tasks. When multiple agents touch a shared codebase with shared interfaces, types, and test infrastructure, worktree isolation prevents Git conflicts but not integration conflicts.

The missing guidance:
1. **Who owns shared files** (interfaces, types, mocks, router, migrations)?
2. **How should agents scope their changes** to minimize integration surface?
3. **What should the orchestrator do BEFORE launching agents** to prevent drift?
4. **How should migrations be coordinated** across parallel agents?

## Observations

### Observation 1: Interface Changes Create O(n) Ripple

Adding one method to a Go interface requires updates in:
- The interface definition (1 file)
- Each backend implementation (2 files: SQLite, PostgreSQL)
- The shared mock (1 file)
- Every test file with an inline mock (5+ files, if the shared mock isn't used)

If an agent adds a Store method, it modifies store.go. But the agent's worktree doesn't have the other agents' recently-added methods, so the mock stubs it adds are incomplete. At merge time, the orchestrator must reconcile N agents' views of the Store interface.

**This was the #1 time sink.** Every merge required manually adding mock stubs to 6 test files for methods the agent didn't know existed.

### Observation 2: Agents Over-Clean

Agents with a "refactoring" mandate tend to remove code they perceive as unused. Three times, agents removed types (ContentRating, ScanResult.Rating, ColumnData.parentId) that were used by code in other packages or files the agent didn't read. The agent's worktree compiled clean because the callers were in its stale snapshot — they'd been modified by another agent.

**Pattern:** Agent reads a type, sees no references in its scope, removes it. But references exist in files modified by a concurrent agent.

### Observation 3: Migration Versioning Is Inherently Sequential

Database migrations are ordered. Two agents both writing "migration v7" will collide. The orchestrator must renumber one of them. This happened 4 times despite clear instructions in agent prompts. The problem: agents don't know what version main is at because their worktree branched before the other agent's migration was merged.

### Observation 4: Frontend Files Are a Conflict Magnet

`App.tsx` was modified by 4 different agents (routing, settings, filter behavior, recents). Each agent's changes were correct in isolation but incompatible with each other. The final reconciliation required manually reading all four versions and assembling a combined file.

### Observation 5: Shared Mock Store Was the Biggest Quality-of-Life Win

Midway through, we created `internal/metadata/mockstore/` — a single shared mock implementing the full Store interface. This immediately eliminated the "add stubs to 6 test files" problem for all subsequent agents. Every project with interface-heavy test mocking should do this FIRST.

## Suggestions

### S1: Add "Parallel Agent Protocol" to REBAR Agent Guidelines

Extend `agents/subagent-guidelines.md` (or create a new `practices/parallel-agent-protocol.md`) with these rules:

**Rule 1: Strict File Ownership.** Each agent prompt includes an explicit allowlist of files it may create or modify. Everything else is read-only. The orchestrator handles shared files (interfaces, types, mocks, router, migrations).

**Rule 2: Interface Changes Are Orchestrator-Only.** Agents NEVER modify shared interface definitions. They write code that ASSUMES methods exist. The orchestrator adds methods to all required files before launching the agent, or handles it at merge time.

**Rule 3: Migration Versions Are Assigned at Merge Time.** Agents use `version: 0` as a placeholder. The orchestrator assigns the correct sequential version during merge.

Alternatively: switch to timestamp-based migration naming (like Django/Rails) to eliminate version conflicts entirely.

**Rule 4: No Removals Without Explicit Authorization.** Agents may ADD types, functions, and methods. They may NOT REMOVE or RENAME anything unless their prompt explicitly says "delete X." If something appears unused, add a `// DEPRECATED` comment and note it in the summary.

**Rule 5: Agent Context Briefing.** Each agent prompt includes a "Recent Changes" section listing what has changed on main since the worktree branched. This gives the agent awareness of concurrent work even though its snapshot is stale.

**Rule 6: Integration Checklist.** The orchestrator follows a standard checklist at merge: build, test, no duplicates, no removals, migration version correct, mock stubs present, router wired, contract headers.

### S2: Add "Shared Mock Store" to Project Bootstrap

REBAR project templates should include guidance for creating a shared mock package early in the project. The pattern:

```
internal/{domain}/mockstore/mockstore.go
  - Single struct implementing the full interface
  - Data fields for simple stub behavior (maps, slices)
  - Function fields for test-specific overrides
  - compile-time interface satisfaction check
```

This should be created as soon as the project has >10 interface methods or >3 test files using mocks.

### S3: Add "Conflict Zone" Documentation

The AGENTS.md file should identify **conflict zones** — files that multiple agents are likely to modify simultaneously:

```
## Conflict Zones (do not assign to multiple agents)
- web/src/App.tsx — main orchestration component
- internal/metadata/store.go — interface definition
- internal/server/router.go — route registration
- internal/metadata/migrations.go — schema migrations
```

This already exists partially in filedag's AGENTS.md ("Key conflict zones: App.tsx, handlers.go, router.go, index.css") but should be expanded to a general REBAR practice.

### S4: Orchestrator Pre-Flight Checklist

Before launching a batch of parallel agents, the orchestrator should:

```
□ All shared interfaces are stable (no pending method additions)
□ Shared mock store is up to date with ALL current methods
□ Migration version counter is known (agents will use placeholders)
□ Conflict zone files are assigned to at most ONE agent
□ Each agent prompt has: file allowlist, recent changes, context briefing
□ Integration time budget estimated (5 min per agent as target)
```

### S5: Post-Merge Verification Standard

After merging each agent:
```
□ go build ./... (or equivalent)
□ go test ./... (unit tests)
□ Frontend build (npm run build)
□ No duplicate declarations
□ No removed types/functions
□ Migration version correct
□ Mock stubs complete
□ E2e tests (if UI changed)
```

## Quantified Impact

If these rules had been in place from the start of the filedag migration:

| Metric | Actual | Estimated with Protocol |
|--------|--------|------------------------|
| Integration time per merge | ~15 min avg | ~5 min avg |
| Total integration overhead | ~8 hours | ~2.5 hours |
| Build failures at merge | ~80% of merges | ~10% of merges |
| Manual mock stub additions | ~96 (16 methods × 6 files) | ~0 (orchestrator pre-flight) |
| Migration renumbering | 4 times | 0 (placeholder pattern) |
| Type restoration from removal | 3 times | 0 (no-removal rule) |

**Estimated time savings: 5.5 hours (69% reduction in integration overhead).**

## Relationship to Existing Feedback

This builds on `2026-03-21-swarm-orchestration-sop.md` which identified:
- Login expiration (solved by worktree model)
- Worktree pruning (solved by commit-before-complete rule)
- No progress observability (partially solved by output files)
- **Shared file conflicts (the focus of THIS feedback)**

The swarm SOP solved the "agents losing work" problem. This protocol solves the "agents creating integration debt" problem — the next layer up.

---

*Developed from the filedag architecture migration: 4 stages, 24 agents, 27 commits,
content-addressed identity model + ABAC + entity resolution + tiered AI pipeline.
Full protocol document at `filedag/agents/PARALLEL-AGENT-PROTOCOL.md`.*
