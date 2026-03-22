# Architecture Directory

This directory contains the project's **contracts** — versioned architecture
documents that define component interfaces, behaviors, and boundaries.

**Contracts are the operating system of this project.** See
[methodology.md](../methodology.md) for the full philosophy.

## Quick Reference

```bash
# Find all contracts
ls architecture/CONTRACT-*.md

# Find all code implementing a specific contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/ internal/ client/

# Find what contract a code file implements
head -10 path/to/file.go    # read the CONTRACT: header comment

# Find all contract references in the codebase
grep -rn "CONTRACT:" --include="*.go" --include="*.ts" --include="*.py" .

# Check the registry for an overview
cat architecture/CONTRACT-REGISTRY.md
```

## Naming Convention

```
CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}.md
```

| Part | Meaning | Examples |
|------|---------|---------|
| `CONTRACT` | Searchable prefix | Always `CONTRACT` |
| `{ID}` | Unique short identifier | `S1`, `C3`, `I2`, `P1` |
| `{NAME}` | Descriptive name (SCREAMING-KEBAB) | `STORAGE`, `API-GATEWAY` |
| `{MAJOR}.{MINOR}` | Version | `1.0`, `2.1` |

**ID prefix conventions:**
- `S` = Service (top-level system boundary)
- `C` = Component (internal module)
- `I` = Interface (shared contract between components)
- `P` = Protocol (wire format, messaging)

## File Template

See [CONTRACT-TEMPLATE.md](CONTRACT-TEMPLATE.md) for the annotated template.

## Contract Registry

See [CONTRACT-REGISTRY.md](CONTRACT-REGISTRY.md) for the index of all contracts.

## Versioning Rules

| Change Type | Version Bump | Agent Autonomy |
|-------------|-------------|----------------|
| Bug fix in contract doc (no behavior change) | None | Full autonomy |
| New optional method, field, or capability | Bump minor (1.0 → 1.1) | Full autonomy |
| Changed signature, removed method, new requirement | Bump major (1.1 → 2.0) | **Plan mode** |
| New contract for new component | New ID + 1.0 | **Plan mode** |

When bumping:
1. Create the new version file
2. Mark the old version: `<!-- SUPERSEDED BY: CONTRACT-{ID}-{NAME}.{NEW} -->`
3. `grep -rn "CONTRACT:{ID}-{NAME}.{OLD}"` → update all implementing code
4. Keep the old version for historical reference

## Code-to-Contract Linking

**Every source file** must have a header comment declaring which contract(s)
it implements or belongs to:

```go
// Package blobstore implements encrypted blob storage.
//
// CONTRACT:C1-BLOBSTORE.2.1
package blobstore
```

```typescript
/**
 * @contract CONTRACT:C3-CRYPTO-BRIDGE.1.0
 */
```

For utility/helper code that doesn't directly implement a contract, reference
the parent service:

```go
// Package httputil provides HTTP helpers for the API gateway.
//
// Architecture: CONTRACT:S2-API-GATEWAY.1.0
package httputil
```

This creates **doubly-linked traceability**: code points to contracts,
contracts list their implementing files. Either direction is searchable
with grep.
