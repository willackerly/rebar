# REBAR Feedback: Continuous Daemon Pattern + REBAR Script Gaps

> **Date:** 2026-04-05
> **Project:** filedag
> **Author:** Claude (session with Will)
> **Scope:** Compliance check after v0.2.0-alpha (Pipeline v2, ABAC, Dashboard)

---

## 1. REBAR Scripts Must Exclude Worktrees

**Finding:** `check-contract-refs.sh` and `check-contract-headers.sh` scanned `.claude/worktrees/` directories, producing 9 false-positive errors from stale agent worktree copies that had older CONTRACT headers.

**Impact:** CI would fail on clean main tree due to worktree artifacts. Developers waste time investigating false positives.

**Fix applied:** Added `-not -path "./.claude/*"` and `grep -v ".claude/worktrees"` exclusions to both scripts. Also excluded `node_modules` (vendored Go files in npm packages triggered false headers).

**REBAR recommendation:** All REBAR enforcement scripts should exclude:
- `.claude/worktrees/` (agent isolation directories)
- `node_modules/` (vendored third-party code)
- `.git/` (already excluded)
- `vendor/` (already excluded)

Consider adding a shared `REBAR_EXCLUDES` pattern or a `.rebarignore` file that all scripts read.

---

## 2. Continuous Daemon vs Cron: Contract Implications

**Finding:** filedag evolved from cron-triggered batch maintenance (`maintain` command) to a continuous daemon (`pipeline` command). This changes the operational model:

| Aspect | Cron Batch (old) | Continuous Daemon (new) |
|--------|-----------------|------------------------|
| Invocation | `0 3 * * *` | Always running, `nice 10` |
| Status | Check report JSON after run | Live status file + HTTP endpoint |
| Health | Cron exit code | PID check + process signal |
| Reports | Every run | Every 12 hours |
| Failure recovery | Next cron invocation | Keep-alive cron restarts daemon |

**REBAR recommendation:** The Contract system should consider **operational contracts** alongside component contracts. A daemon process has different invariants than a CLI command:
- Liveness guarantee (must be running)
- Graceful shutdown (SIGTERM handling)
- Status reporting (machine-readable, polled by other components)
- Resource budgets (CPU priority, memory limits)
- Inter-process communication (status file, shared DB)

Consider a new contract prefix: `O` (Operational) for daemon/service contracts. Example: `O1-PIPELINE-DAEMON.1.0` covering startup, shutdown, status reporting, and health check behaviors.

---

## 3. ABAC Enforcement Completeness

**Finding:** ABAC (content rating filter) was initially implemented on 3 of 38 endpoints. After this session, it covers all content-returning endpoints. The gap existed because the original C9-ABAC contract didn't specify which endpoints MUST enforce it.

**REBAR recommendation:** Security contracts should include an **enforcement matrix** — a table listing every endpoint and whether the security control applies. This makes gaps discoverable by inspection rather than audit.

Example addition to C9-ABAC.1.0:

```markdown
## Enforcement Matrix

| Endpoint | ABAC Required | Status |
|----------|--------------|--------|
| GET /api/v1/nav/{path} | Yes | Enforced |
| GET /api/v1/search | Yes | Enforced |
| GET /api/v1/stats | No (aggregate) | N/A |
...
```

---

## 4. Parallel Agent File Ownership

**Finding:** Running 3 parallel agents (main + 2 worktrees) required strict file ownership assignment to prevent merge conflicts. This session's pattern:

- **Main:** `web/src/App.tsx`, `web/src/components/Toolbar.tsx`, `web/src/index.css`
- **Worktree 1:** `web/e2e/dashboard.spec.ts` (new file only)
- **Worktree 2:** `internal/server/handlers.go`, `internal/server/dupes.go`, etc.

**REBAR recommendation:** The existing parallel agent protocol (`agents/PARALLEL-AGENT-PROTOCOL.md`) should add a **file ownership declaration format** that agents declare at launch:

```yaml
agent: dashboard-tests
owns:
  - web/e2e/dashboard.spec.ts (create)
reads:
  - web/e2e/*.spec.ts (patterns only)
  - web/src/components/StatusView.tsx
```

This makes conflicts detectable before agents start, not after they collide.

---

## 5. Compliance Score

**Current REBAR Tier 2 compliance: 9.0/10** (up from 8.5)

| Check | Status |
|-------|--------|
| Contract headers on all source files | PASS |
| Contract references valid | PASS |
| Cold Start Quad current | PASS |
| TODO.md reflects reality | PASS |
| ABAC enforced on all content endpoints | PASS (was FAIL) |
| CORS environment-aware | PASS (was FAIL) |
| Test coverage for shipped features | PASS |
| REBAR scripts exclude worktrees | PASS (fixed this session) |
| Operational contract for daemon | MISSING (recommendation) |
| ABAC enforcement matrix in contract | MISSING (recommendation) |
