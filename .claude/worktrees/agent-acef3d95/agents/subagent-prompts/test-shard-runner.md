# Template: Test Shard Runner

> Execute a subset of tests in an isolated worktree and report results.
> Designed for parallel fan-out: launch N agents, each running a different
> shard, merge results afterward.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | testing |
| **Mode** | fan-out |
| **Isolation** | worktree (runs tests in isolation) |
| **Estimated tokens** | ~3K-8K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TEST_CMD` | yes | Base test command | `go test -v -race` |
| `PACKAGES` | yes | Space-separated packages/paths for this shard | `./internal/relay/ ./internal/signal/` |
| `SHARD_ID` | yes | Shard identifier (for result file naming) | `03` |
| `OUTPUT` | no | Results path | `agents/results/test-shard-03.json` |
| `TIMEOUT` | no | Per-test timeout | `60s` |
| `RETRY_FLAKY` | no | Retry failed tests once to detect flakes | `true` |

## Task

You are running tests for shard `SHARD_ID`.

1. Execute `TEST_CMD` against each package in `PACKAGES`
2. Capture pass/fail/skip status for every test
3. If `RETRY_FLAKY` is `true`, re-run failed tests once — if they pass
   on retry, mark as `flaky` not `failed`
4. Write structured results to `OUTPUT`

Do not fix failing tests. Do not modify any code. Your job is to run and
report, not to change.

## Output Format

```json
{
  "template": "test-shard-runner",
  "shard_id": "03",
  "status": "complete | partial",
  "packages": ["./internal/relay/", "./internal/signal/"],
  "summary": {
    "total": 42,
    "passed": 38,
    "failed": 2,
    "skipped": 1,
    "flaky": 1
  },
  "failures": [
    {
      "package": "./internal/relay/",
      "test": "TestRelay_BroadcastToAllPeers",
      "output": "First 20 lines of failure output",
      "flaky": false
    }
  ],
  "flaky": [
    {
      "package": "./internal/signal/",
      "test": "TestSignal_ConcurrentOffers",
      "note": "Failed first run, passed on retry"
    }
  ],
  "duration_seconds": 34.2
}
```

## Success Criteria

- Every package in `PACKAGES` was tested (none skipped)
- `OUTPUT` file exists and is valid JSON
- `summary.total == summary.passed + summary.failed + summary.skipped + summary.flaky`
- Failure output is included (not just "FAIL")

## Anti-Patterns

- Do NOT modify source code or test files
- Do NOT skip packages that fail to compile — report the compilation error
  as a failure
- Do NOT run tests outside your assigned `PACKAGES`
- Do NOT retry more than once (we're detecting flakes, not hiding them)
