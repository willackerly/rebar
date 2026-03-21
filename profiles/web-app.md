# Profile: Web Application

For SPAs, SSR apps, web frontends with API backends.

## Core Files — Copy All

| File | Priority | Notes |
|------|----------|-------|
| `README.template.md` | Required | Add project description, setup instructions |
| `QUICKCONTEXT.template.md` | Required | Track branch, test counts, active work |
| `TODO.template.md` | Required | Full two-tag system |
| `AGENTS.template.md` | Required | All sections relevant |
| `CLAUDE.template.md` | Required | All sections relevant |
| `DESIGN.md` | Required | Reference — don't modify per-project |

## Architecture — Customize Heavily

| Item | Relevance |
|------|-----------|
| Contract system | **High** — especially for API contracts, component interfaces |
| BDD features | **High** — user-facing app, write scenarios for every flow |
| Contract linking | **Medium** — useful for API routes, less critical for UI components |

**Suggested contract categories:**
- `S*` services: API gateway, auth, CDN/static serving
- `C*` components: page layouts, shared UI components, data fetching layer
- `P*` protocols: API versioning, WebSocket messages, OAuth flows

## Subagent Templates — High Value

| Template | Relevance |
|----------|-----------|
| `ux-review.md` | **Essential** — customize dimensions for your design system |
| `code-review.md` | **Essential** |
| `security-surface-scan.md` | **High** — focus on XSS, CSRF, auth dimensions |
| `doc-drift-detector.md` | **High** — API docs drift fast |
| `test-shard-runner.md` | **Medium** — useful once test suite is large |
| `contract-audit.md` | **Medium** — for API contract conformance |
| `feature-inventory.md` | **Medium** — for large page components |

## AGENTS.template.md Sections — What to Customize

| Section | Action |
|---------|--------|
| Core Tenets | Add: "Mobile-first", "Accessible by default", "Progressive enhancement" |
| Testing Cascade | Add T4 visual/E2E tier commands (Playwright, Cypress) |
| Quality Gates | Add: Lighthouse score, accessibility audit |

**Practice files to customize (in `practices/`):**

| Practice | Action |
|----------|--------|
| `practices/e2e-testing.md` | **Essential** — fill in test-stack.sh, ports, timeouts |
| `practices/deployment-patterns.md` | **Essential** — fill in build-time env vars, CDN MIME types |
| `practices/worktree-collaboration.md` | Keep as-is |

## CLAUDE.template.md Sections — What to Customize

| Section | Action |
|---------|--------|
| Commands | `dev`, `build`, `test`, `test:e2e`, `lint`, `format` |
| Coding Style | Framework conventions (React/Vue/Svelte), CSS approach |
| Testing | Component tests, integration tests, E2E tiers |
| Allowed Commands | Browser tools, framework CLIs |
| Web Fetch Domains | Framework docs, component library docs, API docs |

## What You Can Skip

- Crypto rules section (unless handling encryption client-side)
- Wire protocol guidance (unless building WebSocket/WebRTC features)
- Ghost DB / database shim patterns

## Retrofitting an Existing Project

Don't add CONTRACT: headers to every file at once. Start with the highest-leverage boundaries:

1. **API route modules** — Each gets a CONTRACT: header and a corresponding spec in `docs/specifications/api/`. This is the highest-leverage target because route modules define the external interface. One spec per route file, verified by the ground truth script.
2. **Shared type definitions** — The types that cross package boundaries (shared/types/, API request/response schemas). These are the IR — the stable interface between packages.
3. **Core library entry points** — The main exports of each package (index.ts, public API surface).

These 3 areas cover ~80% of the contract system's value. Internal modules, utility functions, and UI components can be tagged incrementally as you touch them.

**Ground truth first step:** Set up `METRICS` with test counts and route counts. These are the numbers that drift fastest in web app monorepos.
