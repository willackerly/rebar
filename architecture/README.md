# Architecture Directory

Contracts, lifecycle state, and the contract registry.

See the [root README](../README.md) for how contracts fit into the overall system,
and [DESIGN.md](../DESIGN.md) for the full philosophy.

## Quick Reference

```bash
# Find all contracts
ls architecture/CONTRACT-*.md

# Find all code implementing a specific contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/ internal/

# Find what contract a code file implements
head -10 path/to/file.go

# Run a quality scan across all contracts
ask steward
# or: ./scripts/steward.sh

# Check a single contract
ask steward check C1
```

## What's In Here

```
architecture/
  README.md                         # this file
  CONTRACT-TEMPLATE.md              # annotated template for new contracts
  CONTRACT-REGISTRY.template.md     # contract index
  .state/                           # steward output (gitkeep tracked)
    <contract-id>.<version>.json    # per-contract lifecycle state
    steward-report.json             # aggregate quality report
```

## Naming Convention

```
CONTRACT-{NAME}.{MAJOR}.{MINOR}.md
```

**The full descriptive name is the key. Do not prefix it.**

```text
CONTRACT-AUTH.1.0.md            →  ID: AUTH
CONTRACT-BLOBSTORE.2.1.md       →  ID: BLOBSTORE
CONTRACT-KEY-EXCHANGE.1.0.md    →  ID: KEY-EXCHANGE
CONTRACT-WIRE-FORMAT.1.0.md     →  ID: WIRE-FORMAT
```

Earlier versions encoded a type letter and a sequence number in the ID
(`S1-AUTH`, `C1-BLOBSTORE`, `I2-KEY-EXCHANGE`). **Both halves are retired for new
contracts** — see [conventions.md §Contract Naming](../conventions.md#contract-naming)
for the full rationale and the no-migration rule.

The short version: the **number** was pure bookkeeping that collided on merge and
broke every reference when resolved, and the **letter** duplicated information the
contract already declares in its own `**Type:**` field. Two places to state the
same fact is how they drift apart.

The type taxonomy itself is unchanged and still meaningful — it just lives in the
`Type:` field rather than in the filename:

| `Type:` value | Covers |
|---------------|--------|
| Service | A deployable, independently-operated unit |
| Component | An internal module with a defined interface |
| Interface | A boundary contract between components |
| Protocol | A wire or message format |
| Data Model | Frozen schemas + canonicalization rules |
| Operational | SLOs, startup/shutdown, health |
| Integration Seam | Cross-language type/error mappings |

See [DESIGN.md §Contract System](../DESIGN.md#contract-system) for when to reach
for each type. Data Model, Operational, and Integration Seam were promoted to the
canonical taxonomy after filedag's 2026-04-24 architectural-spike retrospective
demonstrated they partition cleanly under real production pressure.

**Existing numbered contracts stay as they are.** Renaming them breaks references
for no functional gain — the exact cost that made numbering painful. Rebar's own
`S1-STEWARD`, `S2-ASK-CLI`, and `S3-MCP-SERVER` keep their names permanently.

## Contract Lifecycle

Computed by the [Steward](../scripts/steward.sh), never declared manually:

| Status | Criteria |
|--------|----------|
| **DRAFT** | Missing required sections (Interfaces, Behavioral, Errors, Tests, Implementing) |
| **ACTIVE** | All sections present, no `CONTRACT:{id}` found in source |
| **TESTING** | Implementing files exist, no test files found |
| **IMPL-PRESENT** | Implementing files AND test files exist |

> **v3 rename:** this top state was `VERIFIED` through v2.x — computed from
> file presence, no test necessarily executed, so the old name overclaimed.
> `verified` now belongs exclusively to the *declared* maturity vocabulary
> (`conventions.md` §Declared Maturity). Parsers: `verified` → `impl-present`.

## Companion Files

Contracts may have companion files for tribal knowledge (implementation notes,
debugging tips, performance characteristics). Companions don't affect lifecycle.

```
CONTRACT-C1-BLOBSTORE.2.1.md         # the contract
CONTRACT-C1-BLOBSTORE.impl.md        # the companion (no version in filename)
```

See [conventions.md](../conventions.md) for naming rules.

## Versioning

| Change | Version Bump | Autonomy |
|--------|-------------|----------|
| Doc fix (no behavior change) | None | Full |
| New optional method/field | Minor (1.0 → 1.1) | Full |
| Changed signature, removed method | Major (1.1 → 2.0) | **Plan mode** |
| New contract | New ID + 1.0 | **Plan mode** |

When bumping major:
1. Create the new version file
2. Mark old: `<!-- SUPERSEDED BY: CONTRACT-{ID}.{NEW} -->`
3. `grep -rn "CONTRACT:{ID}.{OLD}"` → update all code
4. Keep old version for history

## Code-to-Contract Linking

Every source file declares its contract in a header comment:

```go
// CONTRACT:C1-BLOBSTORE.2.1
package blobstore
```

```typescript
/** @contract CONTRACT:C3-CRYPTO-BRIDGE.1.0 */
```

For helpers that don't directly implement a contract:

```go
// Architecture: CONTRACT:S2-API-GATEWAY.1.0
package httputil
```

This creates doubly-linked traceability — searchable in either direction
with `grep`.

## Cross-Repo Contract Federation

When this repo's contracts are consumed by other rebar-adopting repos,
or when this repo consumes contracts from other repos, the federation
discipline applies (CHARTER §1.6):

### As an owner (your contracts are consumed)

- **Mandatory semver** for any contract that has external consumers.
  File renames `CONTRACT-<id>.<v>.md → CONTRACT-<id>.<v+1>.md` are the
  bump signal.
- The post-commit hook `scripts/check-version-bump.sh` detects bumps
  and queues a notification to `architecture/.state/pending-notifications.md`.
- Run `scripts/scan-consumers.sh <contract-id>` to see who declares
  consumption of your contract (greps `CONSUMES.md` across registered
  sibling repos).
- Run `scripts/flush-notifications.sh` to dispatch queued notifications
  via the existing `ask_<consumer>_featurerequest` gate. Manually flush
  on your schedule — non-blocking by design (CHARTER §2.8).

### As a consumer (you depend on other repos' contracts)

- Declare each cross-repo dep in a top-level `CONSUMES.md` (see
  `templates/project-bootstrap/CONSUMES.md` for the format).
- **Mandatory semver pin** in `version_pinned`. Optional
  `notify_on_change: true` opts in to upstream notification dispatch.
- Run `rebar contract drift-check` to compare your pins against current
  upstream. **Required in CI** when CONSUMES.md exists with at least
  one entry — `scripts/check-compliance.sh` enforces this.
- For local extensions of upstream contracts, write a *new* local
  contract (e.g., `CONTRACT-C2-AGENTS-MYAUDIT.1.0.md`) that documents
  in prose how it relates to the upstream. **Composition over
  inheritance** — no `extends:` field. List the extension contract IDs
  in the consumer entry's `extension_contracts:` field for owner
  visibility.
- To propose your local extension be absorbed upstream, run
  `rebar contract upstream <path-to-extension>` — files an FR via the
  owner's `ask_<owner>_featurerequest` gate. Owner triages on their
  schedule (owner-pulled reconciliation, never auto-merge).

### Where this lives in rebar's substrate

| Concern | Where |
|---------|-------|
| Who consumes what | `CONSUMES.md` (per-repo, opt-in declaration) |
| Outbox of pending change notifications | `architecture/.state/pending-notifications.md` |
| Outbox schema | `practices/federation-outbox.md` |
| Owner-side scripts | `scripts/{scan-consumers,flush-notifications,check-version-bump}.sh` |
| Consumer-side commands | `rebar contract drift-check`, `rebar contract upstream` |
| Status surfacing | `rebar status` shows pending notifications |
| Compliance gating | `scripts/check-compliance.sh` requires drift-check in CI when CONSUMES.md present |

This is "federation as discipline, not infrastructure" — no daemon, no
server, no central registry. Consumer self-declaration + owner-pulled
reconciliation through the existing featurerequest gate.
