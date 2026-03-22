# CLAUDE.md

Project instructions for Claude Code. These override defaults—follow them exactly.

## Project: <PROJECT_NAME>

<!-- One-liner: what this project is and its tech stack. -->

## Core Tenets

<!-- Define 3-5 non-negotiable architectural principles. These guide every decision.
     Examples below — customize for your project: -->

1. **Offline-First** — Every feature must work without network access. Network is for enhancement (sync, CDN assets, updates), never a hard dependency. Test offline paths first.
2. **Client-Side Only** — Zero server dependencies for core functionality. All processing runs in the browser or local runtime.
3. **Progressive Enhancement** — Render/function immediately with what's available, improve quality as optional resources load. Never block on optional resources.

<!-- Other common tenets:
- **Privacy by Default** — No telemetry, no external calls without explicit opt-in.
- **Zero Config** — Works out of the box. Configuration is for power users.
- **Backwards Compatible** — Never break existing users/data without migration path.
-->

## Cold Start (New Agent?)

**Read in order (5 min total):**
1. `README.md` → universal orientation (ALWAYS read first, every session)
2. `QUICKCONTEXT.md` → current state (verify against `git log --oneline -10`)
3. `TODO.md` → tasks + known issues + blockers
4. `AGENTS.md` → norms, testing cascade, contracts, collaboration

**Reference:**
- `DESIGN.md` → the philosophy (contracts, BDD, autonomy model)
- `architecture/` → contract documents
- `agents/` → subagent templates

<!-- Add project-specific deep-dive files here, e.g.:
5. `docs/README.md` → full documentation tree
6. `architecture/CONTRACT-REGISTRY.md` → contract index
-->

## Commands

```bash
# Fill in your project's commands. Common patterns:
# <pkg-manager> install        # bootstrap workspace
# <pkg-manager> dev            # run dev server
# <pkg-manager> build          # build all packages
# <pkg-manager> test           # run all tests
# <pkg-manager> test:e2e       # end-to-end tests
# <pkg-manager> lint           # lint + typecheck
# <pkg-manager> format         # auto-format
# <pkg-manager> clean          # nuke build artifacts / node_modules
```

## Structure

<!-- Map your project's directory layout. Example:
- `packages/web/` - Frontend client
- `packages/api/` - Backend API
- `shared/types/` - Shared TypeScript types
- `shared/utils/` - Common utilities
-->

## Coding Style

<!-- Describe your project's conventions. Example:
TypeScript strict mode. Prettier defaults (2 spaces, semicolons, single quotes).
Function components. Name files after primary export.
Keep changes minimal—don't over-engineer.
-->

### Contract Linking (MANDATORY)

Every source file must have a header comment declaring which contract it
implements or belongs to. See `architecture/README.md` for the full system.

```
// CONTRACT:C1-BLOBSTORE.2.1          — implements this contract directly
// Architecture: CONTRACT:S2-API.1.0   — belongs to this service (for helpers)
```

When editing a file: **read its contract first.** When updating a contract:
**search for all implementing code** (`grep -rn "CONTRACT:{id}"`).
See `DESIGN.md` for the full philosophy.

## Testing

<!-- Describe your testing conventions. Example:
Co-locate unit tests beside code or in `__tests__/`.
E2E specs in a dedicated test directory.
Tag E2E tests (`@critical`, `@visual`).
-->

### Testing Cascade

**Iterate at the speed of a single test, not the full pipeline.** See `AGENTS.md` "Testing Cascade" for full rules.

| Tier | Speed | What | When |
|------|-------|------|------|
| T0 Typecheck | <5s | Changed package | Every edit |
| T1 Targeted | <10s | Single test file | Every change |
| T2 Package | <30s | One package suite | Before commit |
| T3 Full unit | <60s | All unit tests | Before push |
| T4 Visual/E2E | <2min | E2E smoke | UI changes |
| T5 Full suite | <10min | Everything | Release prep |

### E2E Test Infrastructure

<!-- If your project has multi-server E2E tests, document the approach here.
See AGENTS.md "E2E Test Server Management" for the full pattern. Key points:

**Managed test stack** (recommended over Playwright's webServer):
```bash
./scripts/test-stack.sh run <tier>     # start servers → run tests → stop
./scripts/test-stack.sh start <tier>   # start servers only
./scripts/test-stack.sh stop           # kill everything
```

**Why not Playwright's webServer?**
- Sequential startup (slow with 3+ servers)
- No PID tracking (orphaned servers after crashes)
- No hard execution timeouts (tests hang indefinitely)
- Can't fail fast when a server process dies

**Hard timeouts** (nothing hangs):
- Server health check: 20s per server
- Test execution: 90-300s depending on tier
- Per-step evaluate: 10s via evalWithTimeout()
- HTML report: open: 'never' (prevents blocking)

**Prerequisites:**
- macOS: `brew install coreutils` (provides `gtimeout`)
-->

---

## Allowed Commands

The following command patterns are pre-approved for autonomous execution:

### Package Management & Build
<!-- Adjust for your package manager (pnpm / npm / yarn / bun) -->
- `pnpm install`, `pnpm add`, `pnpm remove`, `pnpm rebuild`
- `pnpm build`, `pnpm dev`, `pnpm clean`
- `pnpm --filter <pkg> build`, `pnpm --filter <pkg> dev`
- `npm run build`, `npm rebuild`

### Testing
- `pnpm test`, `pnpm test:watch`, `pnpm test:unit`
- `pnpm test:e2e`, `pnpm test:e2e:critical`, `pnpm test:e2e:headed`
- `npx vitest`, `npx playwright test`
<!-- Add project-specific test commands here -->

### Type Checking & Linting
- `pnpm lint`, `pnpm format`, `pnpm typecheck`
- `pnpm tsc`

### Git Operations
- `git status`, `git diff`, `git log`, `git show`
- `git add`, `git commit`, `git push`, `git pull`
- `git checkout`, `git branch`, `git fetch`
- `git stash`, `git reset`, `git restore`
- `git cherry-pick`, `git merge`, `git rebase`
- `git rm`, `git mv`, `git ls-tree`, `git show-ref`
- `git worktree`, `git remote`

### File & System Utilities
- `ls`, `tree`, `find`, `cat`, `head`, `tail`
- `echo`, `tee`, `stat`, `chmod`
- `strings`, `awk`, `xargs`, `test`
- `timeout`, `gtimeout`, `lsof`, `pkill`, `killall`, `jobs`
- `curl`, `node`

### Deployment
<!-- Add your deployment tool commands here. Examples:
- `vercel deploy`, `vercel logs`
- `fly deploy`, `fly logs`
- `aws s3 sync`, `aws ecs update-service`
- `<platform> login`, `<platform> up`, `<platform> logs`
-->

### Scripts
- `node <script>.mjs`, `node <script>.js`
- `bash <script>.sh`
<!-- Add project-specific scripts here -->

### Diagnostics
- Any script under `scripts/diagnostics/`
- Temp scripts in `/tmp/`

## Web Fetch Domains

Pre-approved for fetching:
<!-- List domains the agent may fetch from. Examples:
- `docs.your-framework.dev`
- `your-app.production.example.com`
-->

## Agent Autonomy

**This project grants MAXIMUM autonomy.** Act decisively. Ship code. Don't ask permission for routine work.

### DO WITHOUT ASKING

1. **All coding tasks** - Write, edit, refactor, delete code freely
2. **All testing** - Run, write, fix, skip tests as needed
3. **All builds** - Build, rebuild, clean as needed
4. **All git operations** - Add, commit, push, branch, merge, rebase
5. **All deployments** - Run deploy scripts and deploy commands
6. **All dependency changes** - Add, remove, upgrade packages
7. **All documentation** - Create, update, reorganize, archive docs
8. **All file operations** - Create, move, rename, delete files
9. **Bug fixes** - Fix bugs immediately without discussion
10. **Refactoring** - Improve code quality, reduce duplication
11. **Test fixes** - Update broken tests, add missing coverage
12. **Config changes** - Update configs, env vars, build settings
13. **Minor features** - Small enhancements that follow existing patterns
14. **Error handling** - Add/improve error handling and logging
15. **Performance fixes** - Optimize slow code paths

### CHECK TEMPLATES FIRST

Before doing specialized tasks (UX review, security audit, code review, etc.),
check `agents/subagent-prompts-index.md` for a matching template. Templates
encode how we want the task done — use them instead of guessing.

### ASK ONLY FOR

**Fundamental architectural decisions** that would be hard to reverse:

1. **New major dependencies** - Adding a framework (e.g., switching UI libraries)
2. **Data model changes** - Modifying database schemas, API contracts
3. **Security model changes** - Altering encryption, auth, or key management approach
4. **New services/packages** - Creating entirely new packages in the monorepo
5. **Protocol changes** - Modifying inter-service protocols, API versioning
6. **Breaking changes** - Changes that break existing users or data

**When in doubt:** If a change follows existing patterns and is reversible, just do it. If it establishes a new pattern or is hard to undo, enter plan mode.

### Force Operations (require explicit user request)

- `git push --force` to shared branches
- `git reset --hard` on commits others might have
- Deleting production data or databases
- Modifying secrets/credentials in production

## Active Workstreams

<!-- Reference `AGENTS.md` for current priorities. Example:
- **P0** - Core feature in active development
- **P1** - Recently shipped, polish pending
- **P2** - Maintenance mode
-->

## Environment Variables

```bash
# List project-specific env vars here. Example:
# API_URL=http://localhost:4000/v1       # local API
# API_URL=https://your-app.example.com   # production
# DATABASE_URL=postgres://...            # database
```

<!-- IMPORTANT: If using dotenv, be aware that .env files are auto-loaded
by many frameworks (Express, Next.js, etc.) even when you don't explicitly
load them. This can silently override env vars set by test scripts.
Keep .env minimal and comment out anything that could interfere with tests
(DATABASE_URL, production API URLs, etc.). -->

---

## TODO Tracking Methodology (MANDATORY)

**This is a hard requirement. Follow exactly.**

### The Two-Tag System

| Tag | Meaning | Action Required |
|-----|---------|-----------------|
| `TODO:` | Untracked work item | Must be tracked before commit |
| `TRACKED-TASK:` | Already in TODO.md/docs | Periodically verify still documented |

### Workflow

**When you add a TODO in code:**
```
// TODO: Handle edge case for X
```

**Before committing, you MUST either:**
1. Fix it immediately (remove the TODO), OR
2. Track it in `TODO.md` and convert to:
```
// TRACKED-TASK: Handle edge case for X - see TODO.md "Code Debt"
```

### Pre-Commit Checklist

**Run before every commit:**
```bash
# Find untracked TODOs (should be 0 before commit)
# Adjust file extensions and directories for your project
grep -rn "TODO:" --include="*.ts" --include="*.tsx" --include="*.py" --include="*.go" src/ packages/ shared/

# Find tracked tasks (audit these periodically)
grep -rn "TRACKED-TASK:" --include="*.ts" --include="*.tsx" --include="*.py" --include="*.go" src/ packages/ shared/
```

<!-- If you have a freshness check script:
./scripts/check-doc-freshness.sh
-->

**If untracked TODOs exist, you must:**
1. Add each to `TODO.md` under appropriate section
2. Convert `TODO:` → `TRACKED-TASK:` in source
3. Re-run check to confirm zero untracked TODOs

### Periodic Scrub (Weekly or Per-Sprint)

For `TRACKED-TASK:` items:
1. Verify each is still in `TODO.md` or relevant doc
2. If completed, remove from both code and docs
3. If stale (no longer relevant), remove from both
4. If doc reference is wrong, update the comment

### Why This Matters

- **TODOs get lost.** Scattered comments with no tracking = technical debt amnesia.
- **Agents work async.** Next agent needs to know what's pending without reading all code.
- **State consistency.** Code comments and TODO.md must agree.

---

<!--
## EXAMPLES (filling in the template for a hypothetical project)

### Project Description
> Monorepo with frontend client, backend API, and shared packages.

### Structure
> - `packages/web/` - Frontend client
> - `packages/api/` - Backend API server
> - `packages/core/` - Core business logic library
> - `shared/types/` - Shared TypeScript types
> - `shared/utils/` - Common utilities

### Coding Style
> TypeScript strict mode. Prettier defaults (2 spaces, semicolons, single quotes).
> Function components. Name files after primary export.

### Testing
> Co-locate unit tests beside code or in `__tests__/`.
> E2E specs in `packages/web/tests/`.
> Tag E2E tests (`@critical`, `@visual`).
> Update `docs/testing/COVERAGE.md` when coverage shifts >=2pts.

### Commands
> ```bash
> pnpm install                # bootstrap workspace
> pnpm dev                    # run dev server
> pnpm build                  # build all packages
> pnpm test                   # run all tests
> pnpm test:contracts         # API contract tests
> pnpm test:e2e               # E2E tests
> pnpm lint                   # lint + typecheck
> pnpm format                 # auto-format
> pnpm typecheck              # typecheck packages
> pnpm clean                  # nuke node_modules
> pnpm stack:up               # docker compose up
> pnpm stack:down             # docker compose down
> ```

### E2E Test Infrastructure
> ```bash
> # Managed test stack (recommended)
> ./scripts/test-stack.sh run core              # golden-path (90s timeout)
> ./scripts/test-stack.sh run integration       # full integration (120s timeout)
> ./scripts/test-stack.sh start core            # start servers only
> ./scripts/test-stack.sh stop                  # kill everything
>
> # pnpm aliases
> pnpm test:e2e:core                           # → test-stack.sh run core
> pnpm test:e2e:core:headed                    # → test-stack.sh run core --headed
> ```

### Deployment Commands
> - `<platform> login`, `<platform> link`, `<platform> up`
> - `<platform> logs`, `<platform> status`

### Environment Variables
> ```bash
> API_URL=http://localhost:4000/v1    # local
> API_URL=https://my-app.example.com  # production
> DATABASE_URL=postgres://...         # database
> ```

### Web Fetch Domains
> - `docs.example.com`
> - `my-app.example.com`

### Active Workstreams
> - **P0** - User authentication (in progress)
> - **P1** - Dashboard redesign (shipped, polish pending)
> - **P2** - Analytics pipeline (maintenance mode)
> - **P3** - Mobile API (deferred)
-->
