# Profile: API Service / Backend

For REST/gRPC APIs, microservices, data pipelines, backend systems.

## Core Files — Copy All

| File | Priority | Notes |
|------|----------|-------|
| `README.template.md` | Required | Include API overview, quickstart for consumers |
| `QUICKCONTEXT.template.md` | Required | Track deploy status, migration status |
| `TODO.template.md` | Required | Full two-tag system |
| `AGENTS.template.md` | Required | All sections relevant |
| `CLAUDE.template.md` | Required | All sections relevant |
| `methodology.md` | Required | Reference — contracts are the API contracts |

## Architecture — High Value

| Item | Relevance |
|------|-----------|
| Contract system | **Essential** — API contracts define the product surface |
| BDD features | **High** — API behavior maps directly to BDD scenarios |
| Contract linking | **Essential** — every handler must reference its contract |

**Suggested contract categories:**
- `S*` services: API gateway, auth, background workers, message queue consumers
- `C*` components: middleware, validators, serializers, cache layer
- `I*` interfaces: database repositories, external service clients
- `P*` protocols: REST API versioning, gRPC service definitions, event schemas

**API-specific contract additions:**
- Include OpenAPI/protobuf snippets in the contract
- Define rate limits, pagination, error response format
- Define auth requirements per endpoint group

## Subagent Templates — High Value

| Template | Relevance |
|----------|-----------|
| `code-review.md` | **Essential** |
| `contract-audit.md` | **Essential** — API contracts must match implementation |
| `security-surface-scan.md` | **Essential** — input validation, auth, injection |
| `test-shard-runner.md` | **High** — backend test suites grow fast |
| `doc-drift-detector.md` | **High** — API docs must match endpoints |
| `feature-inventory.md` | **Medium** — for large handler/service files |
| `ux-review.md` | **N/A** — no UI (unless admin dashboard) |

## AGENTS.template.md Sections — What to Customize

| Section | Action |
|---------|--------|
| Core Tenets | Add: "API-first", "Backwards compatible", "Observable" |
| Contract-First Policy | **Essential** — OpenAPI/protobuf before implementation |
| Testing Cascade | Add: integration test tier (real DB), load test tier |
| Deployment Traps | Fill in: migration order, rollback procedure, health checks |
| Quality Gates | Add: API contract conformance check, migration reversibility |

## What You Can Skip

- E2E Test Server Management (Playwright-specific patterns)
- UX review template
- Visual/E2E testing tier (T4)
- Frontend-specific deployment traps (MIME types, build-time env vars, etc.)

## Retrofitting an Existing Project

1. **Route handlers / controllers** — Each gets a CONTRACT: header + spec. These ARE the API contract.
2. **Database schema / migrations** — The data model contract. Tag schema files and migration directories.
3. **Middleware chain** — Auth, validation, rate limiting. These define cross-cutting behavioral contracts.

These 3 areas cover the external surface (routes), data surface (schema), and cross-cutting surface (middleware). Internal utilities and helpers can be tagged incrementally.

**Ground truth first step:** Set up `METRICS` with endpoint counts and test counts. Run `check-ground-truth.sh` in CI from day one — it catches drift before it compounds.

## What to Add

- **Migration checklist** — database migrations, backwards compatibility
- **API versioning strategy** — how to deprecate, how to sunset
- **Observability contracts** — what metrics, logs, traces each service must emit
- **Runbook template** — incident response for each service
