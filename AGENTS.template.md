# Repository Guidelines

## Read Before Coding

**The Cold Start Quad (every session, every agent, no exceptions):**
1. `README.md` → universal orientation (ALWAYS first)
2. `QUICKCONTEXT.md` → current state (verify against `git log --oneline -10`)
3. `TODO.md` → tasks + known issues + blockers
4. `AGENTS.md` (this file) → norms, contracts, collaboration

**Reference (read as needed):**
- `methodology.md` → the philosophy (contracts are the operating system)
- `architecture/CONTRACT-REGISTRY.md` → contract index
- `agents/subagent-prompts-index.md` → available subagent templates

<!-- Add project-specific context files here, e.g.:
5. `docs/README.md` → full documentation tree
6. `architecture/CONTRACT-REGISTRY.md` → all contracts
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

**Rule of thumb:** If it follows existing patterns and is reversible → just do it. If it establishes new patterns or is hard to undo → plan mode.

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

# 4. Verify Active Workstreams match recent commits
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
- What's the **documented** next step? (TODO.md, AGENTS.md workstreams)
- Do they align? If not, which is authoritative?
- Are there **blocked** items I should avoid?

### Why This Matters
Multiple agents work async on this codebase. Docs drift when agents complete work but don't update all references. Taking 5 minutes to verify state prevents hours of wasted effort on outdated priorities.

## Subagent Prompt Templates

If this project has an `agents/` directory, it contains reusable prompt
templates for subagent delegation. **Check the index before doing specialized
tasks** — there may be a template that encodes how we want it done.

```
agents/
  subagent-guidelines.md       # shared behavioral contract — every subagent reads this
  subagent-prompts-index.md    # catalog of available templates
  subagent-prompts/            # one .md per template (UX review, security scan, etc.)
  results/                     # subagent output files
  findings/                    # architectural/security findings from subagents
```

### Use Subagents Aggressively

**Default to delegation, not doing it yourself.** When there is a backlog of
work, consider fan-out strategies before deploying subagents — plan the
sharding, check for conflicts, then launch in parallel. A single orchestrator
doing 10 tasks sequentially is almost always slower than 10 subagents doing
them in parallel.

**Two hard rules:**
1. **Subagents writing code MUST use worktree isolation.** No exceptions.
2. **Subagents MUST commit before completing.** Uncommitted work in an
   ephemeral worktree is lost work.

### When to Use Templates

- **Single invocation:** Point one subagent at a template for a task you want
  done *your way*. A `ux-review.md` template encodes your definition of UX
  review — the agent doesn't guess.
- **Parallel fan-out:** Same template, N agents, different parameters (shard
  ranges, file subsets, package names).

### How to Invoke

```
Agent(prompt: "Read agents/subagent-guidelines.md for behavioral rules.
              Read agents/subagent-prompts/<template>.md for your task.
              Parameters: TARGET=<path> OUTPUT=agents/results/<name>.json")
```

For fan-out, add `isolation: "worktree"` and launch multiple in one message.

### Pre-Launch Audit (MANDATORY Before Fan-Out)

Before launching ANY parallel agent campaign, the orchestrating agent must
verify what the codebase actually contains — not what docs or memory say.
This prevents the 50% waste incident (see learnings-from-opendockit.md §7).

1. **Grep for existing implementations** in target packages. If planning an
   agent to "add feature X," first check if X already exists.
2. **Check test counts.** If docs say 129 tests but `pnpm test` shows 684,
   substantial work has happened since your last context.
3. **Read actual source directories.** `ls` and `wc -l` tell you what exists.
4. **Cross-reference "What's Next"** in QUICKCONTEXT.md against code — verify
   planned items haven't already been implemented.
5. **Check for overlap between planned agents.** List the files each agent
   will likely modify. If two agents touch the same file, either combine
   them into one or explicitly assign non-overlapping sections. Overlap
   causes merge conflicts that consume significant post-merge context.

This takes 2-3 minutes and prevents hours of wasted agent compute.

### Feature Inventory Protocol

Before assigning a worktree agent to modify a file with **>300 lines of
logic**, generate a feature inventory first: an explicit list of every
behavior the file implements, linked to its exercising test.

Use the `agents/subagent-prompts/feature-inventory.md` template, then include
the output in the worktree agent's prompt with: "Preserve all listed features
unless explicitly told to remove them."

**Why:** Without an inventory, agents restructure files around their assigned
task and may silently delete existing features they don't recognize as
intentional (see learnings-from-opendockit.md §3, the W6 incident).

---

## Project Structure & Module Ownership

<!-- Describe your project's layout and ownership conventions. Example:
`packages/web` is the client app; `packages/api` is the backend;
`shared/{types,utils}` hold cross-cutting contracts.
`docs/` mirrors the repo hierarchy with "When to read / Key files" per subfolder.
-->

## Active Workstreams

<!-- List current priorities by workstream. Example:
- **Feature A (complete YYYY-MM-DD)**
  - Summary of what shipped
  - **Remaining:** what's left

- **Feature B (in progress)**
  - Current state
  - Pending work

- **Feature C (maintenance mode)**
  - What's done, what occasionally needs attention
-->

## Build, Test & Development Commands

```
# Fill in your project's commands. Common patterns:
# <pkg-manager> install        # bootstrap workspace
# <pkg-manager> dev            # run dev server
# <pkg-manager> build          # build across workspaces
# <pkg-manager> test           # run test suites
# <pkg-manager> lint           # lint + typecheck
# <pkg-manager> format         # auto-format
# <pkg-manager> clean          # clean build artifacts + node_modules
```

<!-- Add project-specific commands as needed -->

## Coding Style & File Summaries

<!-- Describe conventions. Example:
TypeScript everywhere with strict configs. Follow Prettier defaults
(2 spaces, semicolons, single quotes). Function components.
Name files after their primary export. For complex modules, add a
two-line header comment or update the folder README so others can
skim responsibilities without reading implementations.
-->

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

1. **Iterate at T1.** Your inner loop is: edit → run the specific test → edit. This should take <10 seconds.
2. **Promote on success.** Only escalate to the next tier when the current one passes. Never skip tiers.
3. **Background the expensive tiers.** T3+ should run in background sub-agents while you keep coding.
4. **Use `--related` when unsure.** Most test runners support running tests related to changed files — use this to auto-detect affected tests.
5. **Never run T5 in your inner loop.** T5 is a release gate, not a development tool. Running the full suite to check one function is like deploying to prod to test a button color.
6. **T4 only for visual/UI changes.** If you changed business logic or a utility, T1-T3 are sufficient.
7. **Fan out validation.** After finishing a body of work, launch T3, T4, and lint as parallel background agents.

### Anti-Patterns

- **Run T5 after every change** — You'll spend more time waiting than coding.
- **Skip T1 and go straight to T3** — T3 runs hundreds of tests. T1 runs one. The feedback delay kills iteration speed.
- **Block on T3 while coding** — Run T3 in a background agent. Keep working on the next thing.
- **Run `pnpm test` to check one function** — Find the exact test file. Run that one.

### Why This Matters

Traditional CI/CD pipelines are designed for correctness, not velocity. They run everything, every time, sequentially. That's appropriate for merge gates but catastrophic for development loops. The cascade inverts this: start with the fastest possible validation that covers your change, and only expand scope when you have reason to. This means 10x faster feedback, unblocked iteration, and proportional rigor.

<!-- Create a docs/testing/TESTING_CASCADE.md with full details including:
     - Concrete commands for each tier
     - Agent workflow patterns (single-package feature, cross-package refactor, UI change)
     - Sub-agent validation fanout patterns
     - Detailed anti-patterns with explanations
-->

---

## Testing Expectations

<!-- Describe your testing approach. Example:
Unit/integration coverage via your test runner; co-locate specs
beside code or inside `__tests__`. E2E specs should be tagged
(`@critical`, `@regression`) for CI selection. Keep coverage docs
updated when coverage changes >=2 pts.
-->

### Contract-Driven Development

**Contracts are the operating system.** See `methodology.md` for the full
philosophy and `architecture/README.md` for the naming/linking system.

**The rules:**
1. **Don't implement without a contract.** Write the contract first.
2. **Don't modify code without checking its contract.** Read the `CONTRACT:`
   header comment, then read the contract document.
3. **Don't update a contract without searching implementations.**
   `grep -rn "CONTRACT:{id}" src/ internal/` finds all implementing code.
4. **Contract changes that break interfaces → plan mode.**

**Every source file** has a header declaring its contract:
```
// CONTRACT:C1-BLOBSTORE.2.1
```

**Contract tests are king.** If a contract test fails, nothing ships.

### Documentation Maintenance Policy

**Principle**: Code and docs must stay in sync. Outdated docs are worse than no docs—they mislead future agents and create compounding confusion.

**After every code change or task completion**, walk the doc tree and update affected files:

| Change Type | Docs to Update |
|-------------|----------------|
| **New feature/module** | Package README, architecture docs if structural, AGENTS.md workstreams if major |
| **API change** | Specifications first (contract-first!), then implementation |
| **Bug fix** | Relevant README if it clarifies behavior; remove stale warnings |
| **Config/env change** | Getting-started docs, package README, `.env.example` |
| **Test change** | Coverage docs if coverage shifts >=2pts |
| **Phase/milestone complete** | Plan docs status table, AGENTS.md workstreams, status docs |
| **New file/module** | Parent folder's README or header comment |

**Doc Update Checklist** (include in PR/commit):

1. **Local**: Did you update the nearest README (package, folder)?
2. **Specifications**: Did you update specs if interfaces changed?
3. **Plans**: Did you update plan docs if a task/phase completed?
4. **Workstreams**: Did you update AGENTS.md Active Workstreams if priorities shifted?
5. **Status**: Did you update status docs for milestones or blockers?
6. **Breadcrumbs**: Are new files linked from parent READMEs so they're discoverable?

**Why this matters**: Multiple agents work on this codebase asynchronously. Each agent relies on docs to understand context without reading the full history. Stale docs cause wasted effort, duplicate work, and architectural drift.

**Enforcement**: PRs that change code without corresponding doc updates should be flagged. When in doubt, over-document—it's cheaper to trim than to reconstruct context.

#### Metric-Bearing Changes (High Drift Risk)

Quantitative claims (test counts, contract counts, endpoint counts) drift
faster than prose. These code changes invalidate the `METRICS` file and
any doc that cross-references those numbers:

| Code Change | What to Update |
|-------------|----------------|
| Add/remove test file | `METRICS` file, QUICKCONTEXT.md |
| Add/remove contract | `METRICS` file, CONTRACT-REGISTRY.md |
| Add/remove API route | `METRICS` file, corresponding spec |
| Change dependency version | Architecture docs referencing it |

**Convention:** All tests live in `tests/`. All contracts live in
`architecture/`. These known locations make metric computation reliable —
no guesswork about where to count.

#### Single Source of Truth for Metrics

Every quantitative claim must trace to ONE authoritative source.
The `METRICS` file is the canonical location for project-wide numbers.
`scripts/check-ground-truth.sh` verifies it against code.

| Metric | Computed From | Verified By | Referenced In |
|--------|--------------|-------------|---------------|
| Test count | `tests/` directory | `check-ground-truth.sh` | QUICKCONTEXT.md |
| Contract count | `architecture/CONTRACT-*.md` | `check-ground-truth.sh` | CONTRACT-REGISTRY.md |
| Contract coverage | `CONTRACT:` headers in source | `check-ground-truth.sh` | AGENTS.md |

<!-- Customize this table for your project's metrics. -->

#### Doc Ownership by Workstream

<!-- Map workstreams to their owned docs. Example:
| Workstream | Owned Docs | Responsibility |
|------------|-----------|----------------|
| **Feature A** | `packages/feature-a/`, `docs/plans/FEATURE_A_PLAN.md` | Active development |
| **Feature B** | `docs/architecture/FEATURE_B.md` | Maintenance mode |
| **Cross-cutting** | `AGENTS.md`, `QUICKCONTEXT.md`, `TODO.md` | All agents share |
-->

#### Archive Policy

**When to archive:**
- Feature/phase 100% complete and no longer changing
- Status snapshot > 3 months old AND newer snapshot exists
- Planning doc for approach not implemented

**Never archive:** `AGENTS.md`, `QUICKCONTEXT.md`, `TODO.md`, `CLAUDE.md`, `methodology.md`, latest architecture contracts

**How to archive:**
1. Move to `docs/archive/YYYY-MM-DD-description/`
2. Add header: `ARCHIVED: [DATE] | REASON: [reason] | CURRENT: [link to replacement]`
3. Update `docs/archive/README.md` index
4. Remove link from parent README

#### Navigation: Where to Document What

| I Need to Document | Go Here |
|--------------------|---------|
| **Feature being built** | `docs/plans/[NAME]_PLAN.md` |
| **System design** | `docs/architecture/[TOPIC].md` |
| **API or data format** | `docs/specifications/` |
| **Testing approach** | `docs/testing/` |
| **Current state** | `docs/current-status/STATUS_YYYY-MM-DD.md` |
| **Blockers** | `TODO.md` "Known Issues & Blockers" section |
| **Tasks** | `TODO.md` (track with TRACKED-TASK: in code) |
| **File purpose** | Nearest README |
| **Historical context** | `docs/archive/` |

### E2E Test Server Management

When E2E tests require multiple servers (API, frontend, mock services), use a **managed test stack** approach for reliability. The core problems with Playwright's built-in `webServer` are: sequential startup (slow), no PID tracking (orphans), no hard timeouts (hangs), and opaque failures (no logs).

#### Architecture: `test-stack.sh`

Create a single shell script that manages the full server lifecycle:

```bash
#!/usr/bin/env bash
# scripts/test-stack.sh — Managed test server stack
# Usage:
#   ./scripts/test-stack.sh run <tier> [playwright-args...]   # start → test → stop
#   ./scripts/test-stack.sh start <tier>                      # start servers only
#   ./scripts/test-stack.sh stop                              # kill everything
#   ./scripts/test-stack.sh status                            # show running servers
```

**Key design principles:**

1. **PID tracking** — Write each server's PID to a temp file (`/tmp/<project>-test-stack.pids`). Read it back for reliable cleanup.

2. **EXIT/INT/TERM trap** — `trap 'cleanup' EXIT INT TERM` ensures servers die on Ctrl+C, crash, or normal exit. Never leave orphans.

3. **Parallel startup** — Start all servers with `&`, then health-check each. Much faster than Playwright's sequential `webServer`.

4. **Per-server health checks with dead-process detection** — Poll the health URL, but also check if the PID is still alive. If the process died, show last N lines of its log file immediately instead of waiting for timeout:
    ```bash
    wait_for_health() {
      local name="$1" url="$2" timeout_s="${3:-20}"
      local start_time=$(date +%s)
      while true; do
        local elapsed=$(( $(date +%s) - start_time ))
        [ "$elapsed" -ge "$timeout_s" ] && { show_log "$name"; return 1; }
        curl -sf "$url" >/dev/null 2>&1 && return 0
        # Fail fast if process died
        local pid=$(grep "^${name}=" "$PIDFILE" | cut -d= -f2)
        kill -0 "$pid" 2>/dev/null || { show_log "$name"; return 1; }
        sleep 0.5
      done
    }
    ```

5. **Hard execution timeouts** — Wrap the Playwright command in `gtimeout` (GNU coreutils on macOS) or a background watchdog:
    ```bash
    # macOS: brew install coreutils → provides gtimeout
    gtimeout --signal=TERM --kill-after=5 90 npx playwright test --config ...
    # Exit code 124 = timeout
    ```

6. **Server logs to files** — Redirect all server output to `/tmp/<project>-test-logs/<server>.log`. On failure, the health check shows the last 15 lines.

7. **Orphan sweep** — After killing tracked PIDs, also kill anything on known test ports. Catches processes that escaped PID tracking:
    ```bash
    for port in 8820 8821 8822 8823; do
      pids=$(lsof -ti :"$port" 2>/dev/null || true)
      [ -n "$pids" ] && echo "$pids" | xargs kill 2>/dev/null || true
    done
    ```

8. **Playwright config dual-mode** — When `__TEST_STACK_RUNNING=1` is set, skip `webServer[]` entirely:
    ```typescript
    const stackRunning = !!process.env.__TEST_STACK_RUNNING;
    export default defineConfig({
      ...(stackRunning ? {} : { webServer: [...] }),
    });
    ```

#### Fixed Port Ranges

Use deterministic port ranges so cleanup is simple and tiers don't conflict:

```typescript
// tests/e2e/utils/find-ports.ts
const PORT_BASES: Record<string, number> = {
  'unit':       8810,  // fast browser-only tests
  'core':       8820,  // golden-path (API + Web + services)
  'integration': 8830, // full integration suite
  'e2e':        8840,  // complete E2E
};
```

#### Playwright Configuration Gotchas

These are hard-won lessons. Follow them exactly:

1. **HTML reporter: `open: 'never'`** — Without this, Playwright starts an HTTP server after test failure and blocks forever waiting for Ctrl+C. This is the #1 cause of "test hangs for 10 minutes" in non-interactive contexts:
    ```typescript
    reporter: [
      ['html', { outputFolder: 'playwright-report', open: 'never' }],
      ['list'],  // streaming progress to terminal
    ],
    ```

2. **Zero retries locally** — `retries: 0` in dev. Retries hide bugs and triple execution time. Only enable in CI:
    ```typescript
    retries: process.env.CI ? 1 : 0,
    ```

3. **`globalSetup` vs `webServer` ordering** — Playwright starts `webServer` entries BEFORE running `globalSetup`. If your globalSetup kills servers on test ports, it kills the servers Playwright just launched. Use globalSetup only for non-destructive initialization.

4. **Per-step evaluate timeouts** — A single `page.evaluate()` can hang for the full test timeout (30-90s) with zero output. Wrap in a race:
    ```typescript
    async function evalWithTimeout<T>(
      page: Page, fn: () => Promise<T>, label: string, ms = 10_000
    ): Promise<T> {
      return Promise.race([
        page.evaluate(fn),
        new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error(`${label} hung for ${ms}ms`)), ms)
        ),
      ]);
    }
    ```

5. **`strictPort` for Vite servers** — Without `--strictPort`, Vite silently picks the next available port if the configured one is busy. Your tests then connect to the wrong server or an old orphan:
    ```bash
    npx vite --port 8821 --strictPort  # fails if 8821 is taken
    ```

#### Environment Variable Hygiene for Tests

**dotenv auto-loading is a footgun.** Many frameworks (Express, Next.js, etc.) auto-load `.env` files at startup via `dotenv.config()`. This means environment variables you set in the test runner can be silently overridden by values in `.env`. Common symptoms:

- Tests fail with auth errors because `.env` has `DATABASE_URL` set, which changes the auth middleware behavior
- Tests hit production services because `.env` has production URLs
- Tests fail with "already in use" because `.env` sets a port that conflicts with the test port

**Defenses:**
1. Keep `.env` minimal — comment out anything not needed for local dev
2. In test startup scripts, explicitly `unset` dangerous env vars before launching servers
3. Use a separate `.env.test` and configure your framework to load it in test mode
4. Never put secrets in `.env` files committed to git (use `.env.local` or `.env.example`)

#### Template: Tier Timeouts

| Tier | Servers | Timeout | Rationale |
|------|---------|---------|-----------|
| Unit/component | 1 (web only) | 30s | Fast, isolated tests |
| Core/golden | 2-4 | 90s | Critical path, should be fast |
| Integration | 2-4 | 120s | More complex flows |
| Full E2E | All | 300s | Complete system tests |

#### Dev Server Memory for Long Test Runs

When running E2E suites with many specs (50+), dev servers can leak
memory and get OOM-killed mid-run. Common with Vite, Webpack HMR, and
Next.js dev mode.

**Pattern:** Use production-like servers for E2E, not dev servers.

| Server Type | Memory | Stability |
|------------|--------|-----------|
| Dev server (`vite dev`) | Growing (~500MB+) | OOM-killed during long runs |
| Preview/static (`vite preview`) | Constant (~50MB) | Stable indefinitely |

For Vite: `vite build --mode test && vite preview --port $PORT` gives a
static server with constant memory. The 10-15s build time pays for itself
by eliminating OOM kills. Adjust health check timeouts (60s vs 20s) to
account for the build step.

### Deployment Traps & Lessons

These patterns cause recurring production incidents in monorepo projects. Document the specifics for your project below.

#### Static Frontend vs Backend Deploy

In monorepos with separate frontend and backend packages, the default deploy command often targets the wrong service. Common trap: `railway up` (or equivalent PaaS CLI) deploys the backend/API, not the frontend static site.

**Document for your project:**
- How production frontend deploys (e.g., deploy script, CI/CD pipeline, git-push-to-deploy)
- How staging/test frontend deploys (often manual — build + upload)
- How backend deploys and why it is a different command/flow
- What happens if you accidentally run the wrong deploy command

<!-- Example:
- **Production frontend:** `scripts/deploy.sh` pushes to a separate git repo that the PaaS auto-deploys
- **Test frontend:** Manual build with test env vars → `railway up --path-as-root dist`
- **Backend:** `railway up` from repo root (or with appropriate service linked)
-->

#### Origin Allowlists for Cross-Origin Popups/iframes

If your app uses popup windows or iframes with `postMessage` (auth surfaces, payment gateways, OAuth providers, identity verification), the target window must allowlist the parent origin. The failure mode is **silent and devastating**: the popup completes its flow successfully but the `postMessage` response is blocked by origin checking. The parent window hangs indefinitely with no error visible to the user — only a console warning in the popup's DevTools.

**Document for your project:**
- Which files contain `ALLOWED_ORIGINS` or equivalent allowlists
- The exact steps to add a new origin (add to list + redeploy the service)
- How to debug: check the popup/iframe's console, not the parent's

<!-- Example:
- `reference-implementations/biometric3/surface-handler.js` has `ALLOWED_ORIGINS`
- New frontend URLs must be added + Surface redeployed
- Debug: open DevTools on the popup window, look for postMessage origin errors
-->

#### MIME Type Issues on CDN/PaaS

Some hosting platforms serve non-standard file extensions with incorrect MIME types. Common problems:
- `.mjs` served as `application/octet-stream` (breaks ES module imports)
- `.wasm` served without `application/wasm` (breaks WebAssembly instantiation)
- `.map` served with wrong type (breaks source maps)

**Workaround pattern — blob URL for web workers:**
```typescript
// Instead of: import workerUrl from './worker.mjs?url'
// Use:
import workerSource from './worker.mjs?raw';
const blob = new Blob([workerSource], { type: 'application/javascript' });
const workerUrl = URL.createObjectURL(blob);
```

**Document for your project:**
- Which files use this workaround and why
- Which hosting platform causes the issue
- Do NOT revert to `?url` imports without verifying MIME types on the deployed platform

#### Environment Variables Baked at Build Time

Vite, Next.js, and Create React App all **bake environment variables into the bundle at build time**. They are string-replaced during the build and become literal constants in the output JavaScript. This means:

- Building with the wrong `API_URL` and deploying will point the frontend at the wrong backend **permanently** until rebuilt
- There is no way to change these values after build without rebuilding
- Deploy scripts that default to production values will silently use those defaults if you forget to override

**Document for your project:**
- Which env vars are build-time (Vite: `VITE_*`, Next.js: `NEXT_PUBLIC_*`, CRA: `REACT_APP_*`)
- Which env vars are runtime (server-side only, read from `process.env` at request time)
- How to verify the correct values after deploy (e.g., check the built JS bundle, check network requests in DevTools)
- What the deploy script defaults to if no override is provided

<!-- Example:
- `VITE_API_URL` is build-time — always verify before `vite build`
- `JWT_SECRET` is runtime — only needs to be set on the server
- After deploy, open DevTools Network tab and verify API requests go to the expected URL
-->

#### Production Deploy Confirmation

Deploy scripts targeting production MUST require interactive confirmation.
Without this, agents with max autonomy can and will deploy autonomously —
autonomy grants are for development workflow, not production operations.

**Pattern:**
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

The `-t 0` check ensures the script runs in an interactive terminal, not
piped or called from another script. This is a deliberate friction point —
the one place where we want to slow agents down.

**Document for your project:**
- Which deploy commands target production vs. staging
- Which commands have this guard and which don't
- How to bypass for CI/CD pipelines (e.g., `DEPLOY_CONFIRMED=1`)

### Agent Collaboration Patterns

#### Worktree Isolation Rules

- **Use worktrees for:** implementation work that modifies files, speculative
  approaches, any change that might conflict with parallel agents.
- **Use main-thread sub-agents for:** read-only research, validation (tests,
  typechecks, lint), synthesizing information from multiple files.
- **Never use worktrees for:** changes to a single shared file (merge will
  conflict), changes requiring real-time coordination, changes with unclear
  scope (agents will expand into each other's territory).

#### Cherry-Pick Conflict Resolution

Conflicts between worktree agents are expected, not exceptional:

1. Understand which version is the superset — don't blindly take "theirs" or "ours"
2. Merge manually with understanding of both agents' intent
3. Run T2 (package-level tests) immediately after resolution
4. When a fix involves a common pattern across multiple files, assign all affected files to the same agent

#### Post-Merge Integration

Plan post-merge integration as an explicit step, not an afterthought:

- Fan-out plans should include a "post-merge wiring" section listing which
  cross-file connections need to be made after all worktrees merge
- Budget ~30% of agent time for fix-up, not 0%
- Agents creating new files are safest (no existing state to conflict with)
- Agents modifying existing files need diff-against-main review
- Agents writing tests for existing code have ~50% wrong-assumption rate —
  always run on main before committing

#### Freshness Markers

<!-- freshness: YYYY-MM-DD -->

Status-bearing sections in this file (Active Workstreams, etc.) should
include a freshness timestamp. Agents should check this date and treat
claims in sections >2 weeks stale with skepticism.

---

### Quality Gates (run before every push)

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
#    - Convert TODO: → TRACKED-TASK: in code
#    - Re-run check

# 3. Only commit when untracked TODOs = 0
```

<!-- If you have a freshness check script:
./scripts/check-doc-freshness.sh
-->

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

---

<!--
## EXAMPLES (filling in the template for a hypothetical project)

### Project Structure
> `packages/web` is the frontend client; `packages/api` is the backend;
> `packages/core` is the shared business logic library;
> `shared/{types,utils}` hold cross-cutting contracts.
> `docs/` mirrors the repo hierarchy.

### Active Workstreams
> - **User Auth (complete 2025-12-01)**
>   - OAuth2 + session management, 15 contract tests passing
>   - **Remaining:** deploy, cross-browser E2E
>
> - **Dashboard Redesign (in progress)**
>   - New component library, responsive layouts
>   - Pending: accessibility audit, performance benchmarks
>
> - **Data Export (maintenance mode)**
>   - CSV/JSON export, scheduled jobs, retry queue

### Testing
> Unit/integration via test runner of choice.
> E2E specs in `packages/web/tests/`, tagged `@critical`, `@regression`.
> Contract tests required for all API interface changes.

### Contract-First Policy
> - Specs in `docs/specifications/` (OpenAPI, data formats, contract testing plan)
> - API routes reference `packages/api/openapi.yaml`

### Doc Ownership
> | Workstream | Owned Docs | Responsibility |
> |------------|-----------|----------------|
> | **Auth** | `packages/api/src/auth/`, `docs/plans/AUTH_PLAN.md` | Active development |
> | **Dashboard** | `packages/web/src/dashboard/`, `docs/architecture/DASHBOARD.md` | Active development |
> | **Cross-cutting** | `AGENTS.md`, `QUICKCONTEXT.md`, `TODO.md` | All agents share |

### Quality Gates
> Follow `docs/testing/TEST_MATRIX.md` for the checklist:
> lint, unit tests, contract tests, critical E2E, deploy smoke.
> Deploy script invokes smoke tests automatically.

### Commit Style
> feat: add OAuth2 login flow
> fix: resolve race condition in job queue retry
> docs: update architecture diagram for auth pipeline
> build: upgrade bundler, fix HMR config
-->
