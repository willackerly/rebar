📍 **You are here:** **Try It** → [Love It](FEATURE-DEVELOPMENT.md) → [Master It](CASE-STUDIES.md)
**Part of:** 5-minute setup flow
**Next step:** [Complete workflow](FEATURE-DEVELOPMENT.md) or [agent coordination](AGENTS-QUICKSTART.md)

# Contract Quickstart

**Write your first rebar contract in 5 minutes**

Contracts are rebar's secret weapon: versioned markdown documents that specify exactly what a component does, how it behaves, and how it integrates. They're what make it safe for multiple agents to work the same codebase in parallel.

---

## The 2-Minute Concept

### What's a contract?
A contract is **behavioral specification** that survives team turnover:
- **Interfaces** — what methods/functions are available
- **Behaviors** — what happens in edge cases (empty input, missing keys, concurrent access)
- **Dependencies** — what other contracts this depends on
- **Testing** — what tiers of tests are required

### Why contracts?
```go
// Without a contract, agents guess:
func Get(key string) ([]byte, error) {
    // What happens if key doesn't exist?
    // Return error? Return empty slice? Panic?
    // How does an agent know without reading implementation?
}

// With a contract, agents know:
// CONTRACT:C1-BLOBSTORE.2.1 specifies:
// - Get on missing key returns ErrNotFound (not generic error)
// - Get never returns nil slice
// - Get is safe for concurrent use
```

**The result:** Any agent can implement or modify this component correctly without guessing.

---

## Quick Template

### Step 1: Name your contract
```
CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}.md
```

| Prefix | Meaning | Examples |
|--------|---------|----------|
| `S` | Service (business logic) | `S1-AUTH`, `S4-STORAGE` |
| `C` | Component (building block) | `C1-BLOBSTORE`, `C2-RELAY` |
| `I` | Interface (protocol) | `I1-SESSION`, `I2-KEY-EXCHANGE` |
| `P` | Process (workflow) | `P1-DEPLOY`, `P2-BACKUP` |

**Example:** `CONTRACT-C1-BLOBSTORE.2.1.md`

### Step 2: Fill the template

```markdown
# CONTRACT-C1-BLOBSTORE.2.1

**Version:** 2.1
**Status:** active
**Owner:** platform team
**Source:** product/features/file-storage.feature

## Purpose

Key-value blob storage for application files. Provides S3-compatible interface
with local filesystem backend. Motivated by "As a user, I want to upload
profile pictures" scenario.

## Interface

```go
type BlobStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Put(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
}
```

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| `Get` on missing key | Returns `ErrNotFound` (not generic error) |
| `Put` with empty data | Returns `ErrInvalidInput` |
| `Delete` on missing key | No-op (idempotent), returns nil |
| `List` with no matches | Returns empty slice (never nil) |
| Concurrent safety | All methods safe for concurrent use |
| Large files | Handles up to 10MB per blob |

## Dependencies

- **Filesystem**: Local storage backend
- **Context**: Cancellation and timeout support
- **Logging**: Error events via structured logger

## Testing Requirements

- **T0**: Unit tests for each public method
- **T1**: Integration tests with real filesystem
- **T2**: Concurrent access stress tests
- **T3**: File size limit validation

## Implementation Notes

- Keys must be valid filenames (no path separators)
- Storage location: `${DATA_DIR}/blobs/{key}`
- Cleanup: Temporary files removed on error
```

### Step 3: Link to code

```go
// storage/blobstore.go
// CONTRACT:C1-BLOBSTORE.2.1
package storage

type FileBlobStore struct {
    dataDir string
    logger  Logger
}

func (fs *FileBlobStore) Get(ctx context.Context, key string) ([]byte, error) {
    // Implementation follows contract exactly...
}
```

### Step 4: Verify linkage

```bash
# Find all code implementing this contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/

# Make sure contract file exists
ls architecture/CONTRACT-C1-BLOBSTORE.2.1.md

# Run quality check
scripts/check-contract-refs.sh
```

---

## Common Patterns

### Error Handling
```markdown
| Behavior | Specification |
|----------|--------------|
| Invalid input | Returns `ErrInvalidInput` with descriptive message |
| Resource not found | Returns `ErrNotFound` (not generic error) |
| System failure | Returns `ErrInternal` with logged details |
| Timeout | Respects context cancellation |
```

### State Management
```markdown
| Behavior | Specification |
|----------|--------------|
| Idempotent operations | PUT/DELETE safe to retry |
| State transitions | Only valid state changes allowed |
| Cleanup on error | No partial state left behind |
| Concurrent access | Operations are atomic |
```

### Integration Points
```markdown
## Dependencies
- **CONFIG:S2-SETTINGS** — Database connection settings
- **CONTRACT:I1-LOGGER** — Structured logging interface
- **CONTRACT:C3-METRICS** — Performance instrumentation

## Dependents
- **CONTRACT:S1-USER-SERVICE** — Uses for profile storage
- **CONTRACT:S4-FILE-API** — Exposes via HTTP endpoints
```

---

## Quick Reference

### Discovery Commands
```bash
# Find all contracts
ls architecture/CONTRACT-*.md

# Find implementations of a contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/

# Check what contract a file implements
head -10 src/storage/blobstore.go

# Get contract health report
ask steward check C1
```

### Contract Lifecycle
```markdown
draft → active → testing → verified → deprecated → superseded
```

- **draft**: Being written, may change
- **active**: Stable interface, implementation in progress
- **testing**: Implementation complete, tests running
- **verified**: Production ready, tests passing
- **deprecated**: Being phased out, don't use for new code
- **superseded**: Replaced by newer version

### Versioning Rules
- **Minor version** (2.0 → 2.1): Backward compatible additions
- **Major version** (2.x → 3.0): Breaking changes
- **Cross-references**: Always include version (`CONTRACT:C1-BLOBSTORE.2.1`)

---

## What's Next?

### Just getting started?
- **[FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md)** — See contracts in action with full BDD → Contract → Code workflow

### Need more depth?
- **[architecture/CONTRACT-TEMPLATE.md](architecture/CONTRACT-TEMPLATE.md)** — Complete annotated template
- **[DESIGN.md](DESIGN.md)** — Full philosophy: why contracts are rebar's foundation
- **[architecture/README.md](architecture/README.md)** — Advanced naming, lifecycle, registry management

### Ready to scale?
- **[Contract versioning](architecture/README.md#versioning)** — Breaking changes and upgrade paths
- **[Cross-repo contracts](CASE-STUDIES.md)** — Namespacing for shared dependencies
- **[Contract enforcement](practices/multi-agent-orchestration.md)** — Automated quality gates

**Remember:** Every contract follows the same pattern — Purpose, Interface, Behaviors, Dependencies, Testing. Start simple, add complexity as you need it. The five-minute contract you write today can grow into the foundation for coordinated multi-agent development.