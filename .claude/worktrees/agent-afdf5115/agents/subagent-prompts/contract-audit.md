# Template: Contract Audit

> Verify that implementations conform to their declared interfaces/contracts.
> Check that every method is implemented, types match, error contracts are
> honored, and tests cover the contract surface.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | analysis |
| **Mode** | either (single interface or fan-out across contracts) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~8K-15K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `CONTRACT` | yes | Interface/type definition file or directory | `pkg/contracts/` |
| `IMPL` | no | Implementation to verify (auto-detected if omitted) | `internal/relay/` |
| `OUTPUT` | no | Results path | `agents/results/contract-relay.json` |

## Task

You are auditing implementations against their declared contracts.

For each interface in `CONTRACT`:

1. **Locate all implementations** — find types that implement the interface
2. **Method conformance** — verify every method is implemented with correct
   signatures (params, returns, error types)
3. **Behavioral conformance** — check that documented behavioral contracts
   (preconditions, postconditions, invariants) are honored
4. **Error contract conformance** — verify that implementations return the
   documented error types/codes, not ad-hoc errors
5. **Test coverage** — verify that contract tests exist and exercise the
   contract surface

## Context Files

Read these before starting:
- `QUICKCONTEXT.md` — project orientation
- The contract/interface files themselves
- Implementation source files
- Adjacent test files

## Output Format

```json
{
  "template": "contract-audit",
  "contracts_audited": ["BlobStore", "Relay", "SignalServer"],
  "status": "complete | partial",
  "summary": "One-line conformance assessment",
  "contracts": [
    {
      "name": "BlobStore",
      "defined_in": "pkg/contracts/blobstore.go:15",
      "implementations": [
        {
          "type": "FileBlobStore",
          "location": "internal/blobstore/file.go:22",
          "method_conformance": "full | partial | missing",
          "missing_methods": [],
          "signature_mismatches": [],
          "behavioral_violations": [
            {
              "method": "Get",
              "contract": "Returns ErrNotFound if key doesn't exist",
              "actual": "Returns generic error without wrapping ErrNotFound",
              "severity": "high"
            }
          ],
          "test_coverage": {
            "contract_tests_exist": true,
            "contract_test_file": "pkg/contracts/blobstore_test.go",
            "methods_tested": ["Get", "Put"],
            "methods_untested": ["Delete", "List"]
          }
        }
      ]
    }
  ],
  "orphaned_implementations": [
    "Types that appear to implement an interface but aren't registered/used"
  ]
}
```

## Success Criteria

- Every interface in `CONTRACT` was audited
- Every known implementation was located and checked
- Missing methods and signature mismatches are exhaustively listed
- Test coverage assessment is based on actual test file contents, not guesses

## Anti-Patterns

- Do NOT only check method signatures — behavioral conformance (error
  contracts, preconditions) matters more
- Do NOT assume an implementation is correct because tests pass — the tests
  might not cover the contract edge cases
- Do NOT flag Go interface satisfaction as a behavioral guarantee — the
  compiler checks signatures, you check behavior
- Do NOT audit generated code (protobuf stubs, mock implementations)
