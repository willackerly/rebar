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
CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}.md
```

| Prefix | Meaning | Example |
|--------|---------|---------|
| `S` | Service | `S1-AUTH`, `S4-STORAGE` |
| `C` | Component | `C1-BLOBSTORE`, `C2-RELAY` |
| `I` | Interface | `I1-SESSION`, `I2-KEY-EXCHANGE` |
| `P` | Protocol | `P1-WIRE-FORMAT`, `P2-SIGNALING` |
| `D` | Data Model | `D1-USER-SCHEMA`, `D2-RECEIPT` (frozen schemas + canonicalization) |
| `O` | Operational | `O1-PIPELINE-DAEMON`, `O2-API-GATEWAY` (SLOs, startup/shutdown, health) |
| `T` | Integration Seam | `T1-WIRE-CODEC`, `T2-TDFBOT-API` (cross-language type/error mappings) |

See [DESIGN.md §Contract System](../DESIGN.md#contract-system) for when to
reach for each prefix. The `D`, `O`, and `T` prefixes were promoted to
the canonical taxonomy after filedag's 2026-04-24 architectural-spike
retrospective demonstrated they partition cleanly under real
production pressure.

## Contract Lifecycle

Computed by the [Steward](../scripts/steward.sh), never declared manually:

| Status | Criteria |
|--------|----------|
| **DRAFT** | Missing required sections (Interfaces, Behavioral, Errors, Tests, Implementing) |
| **ACTIVE** | All sections present, no `CONTRACT:{id}` found in source |
| **TESTING** | Implementing files exist, no test files found |
| **VERIFIED** | Implementing files AND test files exist |

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
