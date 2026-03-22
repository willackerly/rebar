# CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}

<!-- Copy this file to create a new contract.
     Replace all {placeholders} with actual values.
     Remove these HTML comments when done. -->

<!-- VERSIONING:
     - When this contract is superseded, add: SUPERSEDED BY: CONTRACT-{ID}-{NAME}.{NEW}
     - When this contract supersedes another, add: SUPERSEDES: CONTRACT-{ID}-{NAME}.{OLD}
-->

**Version:** {MAJOR}.{MINOR}
**Status:** draft | active | deprecated
**Owner:** [team or person responsible]
**Source:** [link to BDD feature file, e.g., `product/features/encrypted-storage.feature`]

## Purpose

<!-- One paragraph: what this contract defines and why it exists.
     Reference the persona and scenario that motivated it. -->

## Interfaces

<!-- Define the public interface(s) this contract specifies.
     Use your language's idiom — Go interfaces, TypeScript types,
     Python protocols, etc. -->

```go
// Example: Go interface
type BlobStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Put(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
}
```

## Behavioral Contracts

<!-- Define behaviors that the type system can't enforce.
     These are the things contract tests verify. -->

| Behavior | Specification |
|----------|--------------|
| `Get` on missing key | Returns `ErrNotFound` (not generic error) |
| `Put` with empty data | Returns `ErrInvalidInput` |
| `Delete` on missing key | No-op (idempotent), returns nil |
| `List` with no matches | Returns empty slice, not nil |
| Concurrent safety | All methods safe for concurrent use |

## Error Contracts

<!-- Define the error types/codes this contract uses.
     Implementations MUST use these — not ad-hoc errors. -->

| Error | When | Code |
|-------|------|------|
| `ErrNotFound` | Key does not exist | `NOT_FOUND` |
| `ErrInvalidInput` | Invalid key or data | `INVALID_INPUT` |
| `ErrStorageFull` | Storage capacity exceeded | `STORAGE_FULL` |

## Dependencies

<!-- What does this component depend on? Other contracts, external services,
     configuration. -->

- Depends on: `CONTRACT:I2-KEY-EXCHANGE.1.0` for encryption keys
- Configuration: `BLOBSTORE_PATH` environment variable
- External: none (self-contained)

## Implementing Files

<!-- List all files that implement this contract.
     Keep updated — or regenerate with:
     grep -rn "CONTRACT:{ID}-{NAME}" src/ internal/ client/
-->

- `internal/blobstore/file.go` — file-backed implementation
- `internal/blobstore/memory.go` — in-memory implementation (tests)
- `internal/blobstore/blobstore_test.go` — contract tests

## Test Requirements

<!-- What must be tested? Contract tests are king. -->

- [ ] Every method in the interface has at least one happy-path test
- [ ] Every behavioral contract in the table above has a test
- [ ] Every error contract has a test (trigger the error, verify type/code)
- [ ] Concurrent safety verified with `-race` flag
- [ ] Contract tests run against EVERY implementation (file, memory, etc.)

## Change History

| Version | Date | Change | Migration |
|---------|------|--------|-----------|
| 1.0 | YYYY-MM-DD | Initial contract | — |
