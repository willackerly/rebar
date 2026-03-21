# Feedback: Migrating an Existing Production Project to rebar

**Source project:** Dapple SafeSign (pdf-signer-web) — local-first, zero-knowledge PDF signing with biometric identity binding
**Project type:** Web app monorepo (React/Vite client, Express API, 6 packages, shared types/utils)
**Scale:** 586 unit tests (38 files, 7 packages), 80 E2E specs, 15 Playwright configs, 14 API route modules, 48 endpoints, ~50k LOC
**Maturity:** 3+ months of heavy agent-driven development, production deployed, 5 workstreams shipped
**Date:** 2026-03-18

---

## Why This Feedback Exists

The first SafeSign feedback document (`dapple-safesign-feedback.md`) focused on gaps in the template system itself — ground truth enforcement, numeric drift, deploy safety. This document addresses a different problem: **the templates have no clear migration path for projects that already exist and already have extensive (but differently-structured) documentation.**

SETUP.md assumes you're starting fresh or near-fresh. For a project like ours — with 6 established documentation files, 14 architecture docs, 13 API specs, 4 CI workflows, and battle-tested enforcement scripts — the instructions "diff your existing docs against the templates" undersells the complexity by an order of magnitude.

This document captures what we learned evaluating (and partially executing) adoption, so the templates can provide a real migration guide.

---

## Part 1: The State Before Migration

### What We Already Had (Organically Built)

Our project independently developed most of what the templates prescribe. This is itself a data point — it validates the templates' patterns as convergent best practices rather than one team's opinions.

| Template Concept | Our Equivalent | Quality |
|---|---|---|
| Cold Start Quad (4 files) | 6 files: CLAUDE.md, QUICKCONTEXT.md, TODO.md, AGENTS.md, KNOWN_ISSUES.md, docs/README.md | Good — same function, different structure |
| `DESIGN.md` | `docs/plans/CONTRACT_FIRST_STRATEGY.md` | Partial — covers API contracts but not the full philosophy |
| `architecture/CONTRACT-*.md` | `docs/architecture/` (14 files, prose-based) | Strong content, but no CONTRACT: IDs, no versioning |
| API specs | `docs/specifications/api/` (13 files covering all 14 route modules) | Comprehensive — but no machine-readable format (OpenAPI is partial) |
| `agents/` subagent templates | None | Gap — we use subagents heavily but with inline prompts |
| `conventions.md` | Scattered across CLAUDE.md and AGENTS.md | Exists but not consolidated |
| `check-todos.sh` | Built into `check-doc-freshness.sh` | Identical two-tag system |
| `check-freshness.sh` | `check-doc-freshness.sh` (more comprehensive) | Ahead of templates |
| `check-ground-truth.sh` | `doc-ground-truth.sh` + ACID maintenance section | Ahead of templates |
| `check-contract-headers.sh` | Nothing | **Gap** — no CONTRACT: headers exist |
| `check-contract-refs.sh` | Nothing | **Gap** — no CONTRACT: refs exist |
| `ci-check.sh` | `.github/workflows/contracts.yml` | Equivalent (CI-based, not script-based) |
| `pre-commit.sh` | `scripts/pre-commit-hook.sh` (E2E crypto verification only) | Partial overlap |
| CONTRACT: headers in source | Nothing | **The biggest gap** |
| BDD features | `docs/product/PRODUCT_REQUIREMENTS.md` | Requirements exist but not in Gherkin |
| Deploy safety | `deploy-test.sh`, `promote-to-prod.sh`, TTY guard | Ahead of templates |
| E2E infrastructure | `test-stack.sh`, PID tracking, timeouts, vite preview | Ahead of templates |

### The 70/30 Problem

We're ~70% aligned with the templates, but the 30% that's missing is **the contract linking system** — the core differentiator. Our architecture docs have excellent content. Our API specs are comprehensive. But there's no bidirectional link between source code and these documents. An agent editing `packages/api/src/routes/modules/auth.ts` doesn't know `docs/specifications/api/auth-api.md` exists unless it happens to explore that directory.

This is the exact failure mode the contract system prevents: "agents making locally-correct but globally-wrong decisions because they didn't understand the architectural context" (DESIGN.md §1).

---

## Part 2: The Migration Challenge

### Why "Diff and Merge" Doesn't Work

SETUP.md says: "If you already have `README.md`, `AGENTS.md`, or `CLAUDE.md`, diff the templates against yours and merge the sections you're missing."

In practice, this underestimates the problem:

1. **Structural divergence.** Our AGENTS.md is 750 lines with project-specific sections (Active Workstreams with 12 detailed entries, E2E Test Server Lifecycle with Playwright config tables, Dapple Biometric Testing with TestRig patterns). The template's AGENTS.md has different sections and different ordering. A diff produces hundreds of lines of noise.

2. **Existing functionality in different locations.** Our cold start instructions are split across CLAUDE.md ("Cold Start" section) and docs/README.md (navigation hub). The template puts everything in README.md. Our coding style is in CLAUDE.md; the template has `conventions.md`. Our architecture docs are in `docs/architecture/`; the template expects `architecture/` at the root. These aren't gaps — they're structural choices that work for our project.

3. **Content that shouldn't be replaced.** Our CLAUDE.md has extensive security architecture rules (zero-knowledge, no server-side signing, no plaintext keys) that are critical and project-specific. The template's CLAUDE.md has generic placeholders. We can't replace ours; we can only add template sections we're missing.

4. **Scale of the CONTRACT: header task.** Adding headers to ~200 source files isn't a "merge" — it's a codebase-wide annotation project that requires classifying every file by contract category.

### The Real Migration Steps (What SETUP.md Should Say)

For projects like ours, the migration has four distinct phases with very different effort profiles:

**Phase A: Additive (hours, no risk)**
- Copy `DESIGN.md` as reference material
- Create `agents/` directory with subagent templates
- Consolidate coding conventions into `conventions.md`
- Create `METRICS` file from ground truth script output

**Phase B: CONTRACT: headers on high-leverage files (a day, low risk)**
- Categorize source files by contract tier (see Part 3)
- Add headers to Tier 1 files (~30-40 files)
- Create a CONTRACT-REGISTRY.md indexing existing architecture docs

**Phase C: Contract document creation for uncovered areas (multi-day, medium effort)**
- Identify source files with no corresponding contract document
- Create contract docs for the gaps
- This is where the categorization system (Part 3) is essential

**Phase D: BDD Gherkin scenarios (multi-day, high effort, do last)**
- Convert product requirements into `product/features/` Gherkin files
- Link contracts to their source BDD scenarios
- This is the capstone — it completes the chain: who/why → contract → code

---

## Part 3: The Contract Categorization Problem

### Why Categorization Matters

When we looked at our ~200 source files and asked "what contract does this implement?", we immediately hit a classification problem. The template's ID prefixes (S=Service, C=Component, I=Interface, P=Protocol) are a good start, but they don't capture the full spectrum of file types in a real project.

Specifically: **not every file needs the same depth of contract.** A route handler defining a public API needs a full contract document with behavioral specs, error contracts, and test requirements. A utility function that formats dates needs a one-line header linking it to the service it supports. The template's "direct implementation" vs "Architecture: belonging to" distinction (from conventions.md) handles this, but needs more guidance on where to draw the line.

### Proposed Tiering System

We propose a three-tier classification that determines how much contract infrastructure each file needs:

#### Tier 1: Contract-Owning Files (need full CONTRACT: header + contract document)

These files define or implement a public interface. Changes to them can break consumers. They MUST have a contract document in `architecture/` and a `CONTRACT:{ID}` header.

| Category | Template ID Prefix | Examples from our project |
|---|---|---|
| **API route modules** | `S` (Service) | `auth.ts`, `envelopes.ts`, `signing-link.ts`, `templates.ts` |
| **Shared type definitions** | `I` (Interface) | `shared/types/src/envelope.ts`, `api.ts`, `template.ts` |
| **Wire protocols** | `P` (Protocol) | PostMessage protocol (dapple-postmessage.md) |
| **Core crypto operations** | `C` (Component) | `encryption.ts`, `password-crypto.ts`, `ecdsa-keypair-signer.ts` |
| **Database schema** | `I` (Interface) | `schema.sql`, `db/types.ts` |
| **SDK public API** | `S` (Service) | `dapple-sdk/src/client.ts` |

**Count for our project:** ~30-40 files. These are the highest leverage — they define the system's boundaries.

#### Tier 2: Architecture-Belonging Files (need `Architecture: CONTRACT:{ID}` header, NO separate contract doc)

These files implement logic within a contract's boundary. They're important but don't define interfaces themselves. They reference their parent contract.

| Category | Header Pattern | Examples from our project |
|---|---|---|
| **Internal service logic** | `Architecture: CONTRACT:S1-AUTH` | `db/users.ts`, `db/otps.ts` (belong to auth service) |
| **React components** | `Architecture: CONTRACT:S5-ENVELOPE-UI` | `EnvelopeComposer.tsx`, `FieldPlacement.tsx` |
| **Hooks** | `Architecture: CONTRACT:{parent}` | `useAppRouter.ts`, `useFileActions.ts` |
| **Middleware** | `Architecture: CONTRACT:{parent}` | `auth-middleware.ts`, `rate-limiter.ts` |
| **Service implementations** | `Architecture: CONTRACT:{parent}` | `audit-service.ts`, `notification-service.ts` |
| **Database operations** | `Architecture: CONTRACT:{parent}` | `db/envelopes.ts`, `db/templates.ts` |

**Count for our project:** ~80-100 files. Bulk of the codebase.

#### Tier 3: Utility/Infrastructure (NO contract header needed)

These files are generic utilities, configuration, build infrastructure, or test helpers. They don't implement domain logic and don't need contract annotations.

| Category | Examples from our project |
|---|---|
| **Config/env** | `config/env.ts`, `vite.config.ts` |
| **Build scripts** | `scripts/deploy-test.sh`, `scripts/test-stack.sh` |
| **Test utilities** | `tests/e2e/utils/test-id.ts`, `utils/envelope.ts` |
| **Generic helpers** | `shared/utils/src/formatters.ts`, `validators.ts` |
| **Type re-exports** | `packages/pdf-core/src/index.ts` (thin re-export layer) |
| **Generated files** | `packages/api/src/generated/api-types.ts` |

**Count for our project:** ~60-80 files. No contract overhead.

### Tracking Contract Gaps

When adding CONTRACT: headers to Tier 1 files, you'll discover files that SHOULD have a contract document but don't. This needs a tracking mechanism.

**Proposed pattern:** A `CONTRACT-GAPS.md` file (or section in CONTRACT-REGISTRY.md) that tracks files awaiting contract documents:

```markdown
## Contract Gaps (Pending Documents)

Files classified as Tier 1 that don't yet have a contract document.
These need contracts written before the contract system is complete.

| File | Proposed Contract ID | Priority | Notes |
|------|---------------------|----------|-------|
| `packages/api/src/services/pdf-form-fill.ts` | C4-FORM-FILL | P1 | Merges field values into PDF — complex behavioral rules |
| `packages/web/src/lib/sync/sync-queue.ts` | C5-SYNC-QUEUE | P2 | Retry logic, persistence format |
| `packages/api/src/lib/s3.ts` | C6-S3-STORAGE | P1 | Presigned URL generation, dual-read |
| `packages/web/src/lib/db/file-store.ts` | C7-FILE-STORE | P2 | Encrypted IndexedDB, MEK wrapping |
```

An enforcement script can then check: "every Tier 1 file has either a CONTRACT: header pointing to a real doc, or an entry in CONTRACT-GAPS.md."

This prevents the common failure mode where you start adding headers and then stop halfway. The gap tracker makes incomplete adoption visible and trackable.

---

## Part 4: What the Templates Should Add

### 1. A Dedicated "Migrating an Existing Project" Section in SETUP.md

Not a paragraph — a full section parallel to the new-project setup. Structure:

```markdown
## Migrating an Existing Project

### Step 1: Inventory (30 min)

Map your existing documentation to the template system:

| Template File | Your Equivalent | Action |
|---|---|---|
| README.md | ? | Merge missing sections (architecture overview, cold start) |
| QUICKCONTEXT.md | ? | Merge or keep yours if it covers the same ground |
| TODO.md | ? | Merge Known Issues if they're in a separate file |
| AGENTS.md | ? | Add missing sections (testing cascade, contracts, subagents) |
| CLAUDE.md | ? | Add contract linking section, verify commands |
| DESIGN.md | ? | Copy as-is — it's reference material |
| architecture/ | ? | Map existing architecture docs to contract format |
| agents/ | ? | Likely new — create from templates |

### Step 2: Classify Source Files (1-2 hours)

Walk your source tree and assign each directory to a contract tier:

- **Tier 1 (Contract-Owning):** Public APIs, shared types, protocols, core crypto
- **Tier 2 (Architecture-Belonging):** Internal logic, components, hooks, middleware
- **Tier 3 (No header needed):** Config, build, test utils, generated code

### Step 3: Additive Adoption (hours, zero risk)

Copy what doesn't conflict with existing docs:
- DESIGN.md (reference material, no conflict possible)
- agents/ directory (new capability, no existing equivalent)
- conventions.md (consolidate scattered style guidance)
- METRICS file (captures ground truth output)

### Step 4: CONTRACT: Headers on Tier 1 Files (a day)

Start with the highest-leverage files. See your profile for recommended order.
Use CONTRACT-GAPS.md to track files that need contract documents written.

### Step 5: Contract Document Backfill (ongoing)

Work through CONTRACT-GAPS.md, creating contract docs for uncovered Tier 1 files.
This can be incremental — write contracts as you touch the code.

### Step 6: BDD Scenarios (when ready)

Convert product requirements to Gherkin feature files.
Link contracts to their source scenarios.
This completes the chain: who/why → contract → code.

### What NOT to Do

- **Don't restructure working docs.** If your KNOWN_ISSUES.md works as a
  separate file, keep it. The template's "merge into TODO" is a recommendation,
  not a requirement.
- **Don't force the 4-file cold start** if you have 6 files that work.
  The reading order matters more than the file count.
- **Don't add CONTRACT: headers to Tier 3 files.** It's pure noise.
  Focus on the ~30-40 files that define system boundaries.
- **Don't replace your enforcement scripts** if they're ahead of the
  templates. Adopt the template scripts for capabilities you don't have
  (contract headers, contract refs) and keep yours for everything else.
```

### 2. The Tiering System in conventions.md

Add a section on contract tier classification. The existing "Direct Implementation" vs "Architecture: Belonging To" distinction in conventions.md is the right idea but needs the tiering framework to make it actionable:

```markdown
## Contract Tier Classification

Not every file needs the same depth of contract annotation.
Classify your source files into three tiers:

### Tier 1: Contract-Owning
- Defines a public interface, API, protocol, or core algorithm
- Changes can break consumers
- MUST have: full `CONTRACT:{ID}` header + contract document in `architecture/`
- Examples: API routes, shared type defs, crypto ops, DB schema, SDK public API

### Tier 2: Architecture-Belonging
- Implements logic within a Tier 1 contract's boundary
- Important but doesn't define interfaces itself
- MUST have: `Architecture: CONTRACT:{ID}` header (no separate doc)
- Examples: React components, hooks, middleware, DB operations, internal services

### Tier 3: No Header Needed
- Generic utilities, config, build scripts, test helpers, generated code
- No domain logic, no contract relationship
- NO header needed
- Examples: vite.config.ts, formatters.ts, test-id.ts, generated types

### Tracking Contract Gaps

When classifying Tier 1 files, some will lack contract documents.
Track these in `CONTRACT-GAPS.md`:

| File | Proposed ID | Priority | Notes |
|------|------------|----------|-------|
| path/to/file.ts | C4-NAME | P1 | Why this needs a contract |

An enforcement script can verify: every Tier 1 file has either a valid
CONTRACT: header or a CONTRACT-GAPS.md entry. Incomplete adoption is
visible, not silent.
```

### 3. Profile-Specific Retrofit Ordering

Each profile should specify which files are Tier 1 for that project type and in what order to add headers. From our web-app experience:

```markdown
### Contract Retrofit Order for Web Apps

1. **API route modules** (P0) — Each gets `CONTRACT:S{N}-{NAME}` + spec
   These are your external contract surface. One spec per route.
   Start here because API consumers (frontend, tests, external clients)
   depend on these interfaces being stable.

2. **Shared type definitions** (P0) — Each gets `CONTRACT:I{N}-{NAME}`
   The types that cross package boundaries. These ARE your internal
   contracts — formalizing them prevents the drift where two packages
   evolve incompatible assumptions about the same data shape.

3. **Core crypto / security modules** (P1) — `CONTRACT:C{N}-{NAME}`
   Any module that handles encryption, signing, key management, or auth.
   Security-critical code benefits most from behavioral contracts because
   the consequences of misunderstanding intent are highest.

4. **Database schema + access layer** (P1) — `CONTRACT:I{N}-{NAME}`
   schema.sql + db/*.ts. The data model is the most expensive thing to
   change — formalizing it prevents casual schema drift.

5. **Wire protocols** (P1) — `CONTRACT:P{N}-{NAME}`
   PostMessage protocols, WebSocket messages, inter-service communication.
   If two systems exchange messages, the message format is a contract.

6. **SDK / library public APIs** (P2) — `CONTRACT:S{N}-{NAME}`
   Anything consumed by external code or other packages.

After Tier 1 is covered, add `Architecture: CONTRACT:{parent}` headers to
Tier 2 files incrementally as you touch them. Don't batch-annotate.
```

### 4. CONTRACT-GAPS.md Template

A new template file for tracking uncovered Tier 1 files:

```markdown
# Contract Gaps

Tier 1 source files that need contract documents but don't have them yet.
Track here so incomplete adoption is visible, not silent.

<!-- freshness: YYYY-MM-DD -->

## Pending Contracts

| File | Proposed Contract ID | Priority | Blocking? | Notes |
|------|---------------------|----------|-----------|-------|
<!-- Add rows as you discover Tier 1 files without contracts -->

## Recently Resolved

| File | Contract Created | Date |
|------|-----------------|------|
<!-- Move rows here as contracts are written -->
```

### 5. check-contract-gaps.sh Enforcement Script

A new script that verifies Tier 1 contract coverage:

```bash
#!/usr/bin/env bash
# check-contract-gaps.sh — verify every Tier 1 file has a contract or gap entry
#
# For each file in TIER1_DIRS that lacks a CONTRACT: header,
# check that it appears in CONTRACT-GAPS.md.
# Files with neither are "silent gaps" — the worst kind.

# Customize: directories containing Tier 1 files
TIER1_DIRS="packages/api/src/routes/modules shared/types/src"
GAPS_FILE="architecture/CONTRACT-GAPS.md"

uncovered=0
for dir in $TIER1_DIRS; do
  for file in $(find "$dir" -name '*.ts' -not -name '*.test.ts' -not -name 'index.ts'); do
    has_header=$(head -15 "$file" | grep -c 'CONTRACT:' || true)
    if [ "$has_header" -eq 0 ]; then
      in_gaps=$(grep -c "$(basename "$file")" "$GAPS_FILE" 2>/dev/null || true)
      if [ "$in_gaps" -eq 0 ]; then
        echo "UNCOVERED: $file — no CONTRACT: header and not in $GAPS_FILE"
        ((uncovered++))
      fi
    fi
  done
done

if [ "$uncovered" -gt 0 ]; then
  echo ""
  echo "ERROR: $uncovered Tier 1 files have no contract and no gap entry."
  echo "Fix: add CONTRACT: header, or add to $GAPS_FILE"
  exit 1
fi

echo "OK: all Tier 1 files have contracts or tracked gaps"
```

---

## Part 5: BDD Gherkin — The Capstone (Do Last)

### Why Last

BDD Gherkin scenarios are the philosophical foundation of the contract system — they answer "who needs this and why" before contracts answer "what it does." But for an existing project with working code, extensive tests, and proven architecture, writing Gherkin AFTER the fact is working backward from the ideal sequence. It's still valuable, but it's the lowest-urgency adoption step:

- Contracts without BDD can still link code to architecture docs
- CONTRACT: headers without BDD still enable "read the contract before editing"
- The testing cascade works without BDD
- The enforcement scripts work without BDD

### When to Do It

Write Gherkin when you want to:
1. **Onboard a new team member** who needs to understand user intent, not just code
2. **Validate that contracts match user needs** (not just developer assumptions)
3. **Generate acceptance tests** from scenarios
4. **Make the contract system complete** (the full who/why → contract → code chain)

### How to Do It for an Existing Project

1. Start with the `product/` directory structure from the templates:
   ```
   product/
     personas/         # who uses this
     epics/            # high-level goals
     features/         # .feature files with scenarios
     user-stories/     # bridge between personas and features
   ```
2. Extract personas from existing product docs (we have `PRODUCT_REQUIREMENTS.md`)
3. For each Tier 1 contract, write 2-3 Gherkin scenarios that capture the *intent*
4. Link contracts to their features: `**Source:** product/features/encrypted-storage.feature`
5. Don't try to write scenarios for every edge case — focus on the core behaviors that motivated the contract

### Suggested Personas for SafeSign

From our product requirements, we'd extract:
- **Sarah (sender)** — Creates envelopes, adds recipients, tracks completion
- **Alex (signer)** — Receives signing links, reviews documents, applies signatures
- **Jordan (admin)** — Manages templates, monitors audit trail, downloads certificates
- **The System (zero-knowledge)** — The server never sees plaintext (this is an unusual "persona" but captures the privacy-first constraint as a stakeholder)

---

## Part 6: Summary for Template Authors

### What SETUP.md Needs

A full "Migrating an Existing Project" section (~100 lines) that acknowledges:
1. Existing projects have documentation that works differently — don't break it
2. The CONTRACT: linking system is the unique value — prioritize it
3. Source files need tiering (Tier 1/2/3) before headers can be added
4. Incomplete adoption should be tracked (CONTRACT-GAPS.md), not ignored
5. BDD is the capstone, not the starting point for existing projects

### What conventions.md Needs

The contract tier classification system (Tier 1: Contract-Owning, Tier 2: Architecture-Belonging, Tier 3: No Header Needed) with clear examples and the rule: "when in doubt, Tier 2."

### What Each Profile Needs

A "Retrofitting an Existing Project" subsection with project-type-specific Tier 1 file identification and recommended annotation order.

### New Template Files

- `architecture/CONTRACT-GAPS.md` — tracks Tier 1 files pending contract documents
- `scripts/check-contract-gaps.sh` — enforces that every Tier 1 file has a contract or gap entry

### Key Insight

The templates are designed for greenfield adoption: write contracts → implement → verify. Existing projects need the reverse path: classify existing code → annotate with headers → backfill missing contracts → optionally add BDD. The methodology is the same; the sequence is different. Acknowledging this in the setup guide would make adoption accessible to the projects that need it most — ones with real code, real users, and real technical debt that would benefit from contract-driven development.

---

## Appendix: Our Planned Adoption Sequence

For reference, here's the order we intend to adopt the remaining template components in SafeSign:

1. **Copy DESIGN.md** — reference material, zero risk
2. **Create agents/ directory** — formalize our subagent prompts
3. **Create conventions.md** — consolidate scattered style guidance
4. **Create METRICS file** — formalize ground truth output
5. **Add CONTRACT: headers to API route modules** (14 files) — link to existing specs
6. **Add CONTRACT: headers to shared/types/** (10 files) — link to architecture docs
7. **Add CONTRACT: headers to crypto/identity modules** (~15 files) — highest security leverage
8. **Create CONTRACT-REGISTRY.md** — index all contracts
9. **Create CONTRACT-GAPS.md** — track Tier 1 files without contracts
10. **Backfill contract docs** for gaps (ongoing, as we touch the code)
11. **BDD Gherkin scenarios** (last — capstone completion)
