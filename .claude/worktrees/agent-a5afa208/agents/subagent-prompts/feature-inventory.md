# Template: Feature Inventory

> Generate an exhaustive inventory of every behavior a file or module
> implements, linked to its exercising test. Essential before assigning
> worktree agents to modify large files — prevents the W6 incident
> (agent deletes features it doesn't recognize as intentional).

## Metadata

| Field | Value |
|-------|-------|
| **Category** | analysis |
| **Mode** | single-invocation (pre-delegation step) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~5K-12K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TARGET` | yes | File(s) to inventory | `internal/relay/relay.go` |
| `TEST_DIR` | no | Where to look for tests (auto-detected if omitted) | `internal/relay/` |
| `OUTPUT` | no | Results path | `agents/results/inventory-relay.json` |

## Task

You are generating a feature inventory of `TARGET`.

Read every source file in `TARGET` and produce a comprehensive list of
every distinct behavior, capability, or feature the code implements. For
each feature, identify the test that exercises it.

This inventory will be given to a worktree agent with the instruction:
"Preserve all listed features unless explicitly told to remove them."
Completeness is critical — any feature you miss may be silently deleted.

## What Counts as a "Feature"

- Each public function/method and its documented behavior
- Each code path in conditional logic (error handling, edge cases)
- Each protocol or wire format the code speaks
- Each configuration option the code respects
- Each integration point (what it calls, what calls it)
- Each behavioral invariant (ordering guarantees, atomicity, idempotency)

## Output Format

```json
{
  "template": "feature-inventory",
  "target": "internal/relay/relay.go",
  "status": "complete",
  "line_count": 617,
  "features": [
    {
      "name": "Broadcast to all connected peers",
      "location": "relay.go:45-67",
      "description": "Forwards encrypted messages to all peers in the session except the sender",
      "test": "relay_test.go:TestRelay_BroadcastToAllPeers (line 23)",
      "test_status": "found | not-found | partial"
    },
    {
      "name": "Session timeout cleanup",
      "location": "relay.go:120-145",
      "description": "Removes inactive sessions after TTL expiry, notifies remaining peers",
      "test": "relay_test.go:TestRelay_SessionTimeout (line 89)",
      "test_status": "found"
    }
  ],
  "untested_features": [
    {
      "name": "Graceful shutdown drain",
      "location": "relay.go:200-215",
      "description": "Drains in-flight messages before stopping the relay",
      "risk": "Modification could break shutdown behavior with no test safety net"
    }
  ],
  "summary": {
    "total_features": 12,
    "tested": 10,
    "untested": 2,
    "coverage_assessment": "Well-tested with two gaps in shutdown and reconnection paths"
  }
}
```

## Success Criteria

- Every public function/method in `TARGET` is listed
- Every conditional branch (error path, edge case) is captured
- Every feature links to a specific test (or is flagged as untested)
- `untested_features` explicitly calls out risk

## Anti-Patterns

- Do NOT only list function signatures — capture the *behavior* each
  function implements
- Do NOT skip "obvious" features — the point is exhaustiveness. The agent
  receiving this inventory has no prior knowledge of the file.
- Do NOT conflate multiple behaviors into one feature entry — if a function
  does three things, list three features
- Do NOT guess at test coverage — read the actual test files
