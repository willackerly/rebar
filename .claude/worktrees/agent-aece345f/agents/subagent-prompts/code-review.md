# Template: Code Review

> Multi-dimensional code review with structured findings, severity ratings,
> and concrete suggestions. Covers correctness, performance, security,
> maintainability, and style — calibrated to the project's standards, not
> generic best practices.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | review |
| **Mode** | either (single PR/file set or fan-out across packages) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~8K-20K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TARGET` | yes | Files, directory, or diff to review | `internal/relay/` |
| `DIFF` | no | Git diff to review (alternative to TARGET) | `git diff main..feature` |
| `STYLE_GUIDE` | no | Path to coding style docs | `CLAUDE.md` (Coding Style section) |
| `FOCUS` | no | Narrow the review focus | `correctness`, `performance`, `security` |
| `OUTPUT` | no | Results path | `agents/results/review-relay.json` |

## Task

You are conducting a code review of `TARGET` (or `DIFF`).

Review the code through every applicable dimension below. For each finding,
identify the specific location, explain the issue, assess its severity,
and provide a concrete fix — not just "this could be improved."

## Review Dimensions

### 1. Correctness
- Does the code do what it claims? Are edge cases handled?
- Are error paths correct (not just happy path)?
- Are return values checked? Are errors propagated with context?
- Are nil/null/zero-value cases handled?
- Do loops terminate? Are bounds correct (off-by-one)?

### 2. Performance
- Are there unnecessary allocations in hot paths?
- Are collections pre-sized when the length is known?
- Are there N+1 query patterns or unnecessary iterations?
- Is work done eagerly that could be lazy (or vice versa)?
- Are there blocking calls that should be async?

### 3. Security
- Is user input validated at the boundary?
- Are there injection risks (SQL, command, path traversal)?
- Are secrets handled safely (not logged, zeroized after use)?
- See `security-surface-scan.md` for a deep security audit — this
  dimension is a quick check, not a full audit.

### 4. Maintainability
- Is the code readable without comments? If not, are comments present?
- Are names descriptive and consistent with the codebase?
- Is complexity justified? Could this be simpler?
- Are abstractions at the right level (not over-engineered, not too concrete)?
- Is the code testable? Are dependencies injectable?

### 5. Style & Conventions
- Does the code follow the project's style guide?
- Are naming conventions consistent (casing, prefixes, suffixes)?
- Is formatting consistent with the codebase?
- Are imports organized per project convention?

### 6. Testing
- Are new behaviors covered by tests?
- Are edge cases tested (empty input, max values, error paths)?
- Are test names descriptive (readable as documentation)?
- Do tests test behavior, not implementation details?

## Context Files

Read these before starting:
- `QUICKCONTEXT.md` — project orientation
- `STYLE_GUIDE` parameter value (if provided) — coding standards
- Adjacent test files for `TARGET` — understand existing coverage

## Output Format

```json
{
  "template": "code-review",
  "target": "<TARGET or DIFF description>",
  "focus": "<FOCUS or 'full'>",
  "status": "complete | partial",
  "summary": "One-line overall assessment",
  "findings": [
    {
      "location": "relay.go:87",
      "dimension": "correctness",
      "severity": "P0 | P1 | P2 | P3",
      "finding": "Error from conn.Write is silently discarded",
      "impact": "Failed writes go unnoticed, client receives no feedback",
      "suggestion": "Return the error to the caller: `if _, err := conn.Write(data); err != nil { return fmt.Errorf(\"write to peer: %w\", err) }`"
    }
  ],
  "strengths": [
    "Acknowledge well-written code — pattern reinforcement matters"
  ],
  "verdict": "approve | request-changes | needs-discussion"
}
```

## Severity Guide

| Level | Meaning | Examples |
|-------|---------|---------|
| **P0** | Blocker — breaks functionality or security | Data loss, auth bypass, crash |
| **P1** | Important — design flaw or reliability risk | Race condition, missing error handling |
| **P2** | Improvement — code quality or minor perf | Unnecessary allocation, unclear naming |
| **P3** | Nit — style or polish | Formatting, typo, import order |

## Success Criteria

- Every file in scope was reviewed (none silently skipped)
- All P0/P1 findings have concrete `suggestion` with example code
- `strengths` section is populated
- `verdict` reflects the findings (don't approve with P0s outstanding)

## Anti-Patterns

- Do NOT nitpick style in code you're not otherwise reviewing — if the file
  has real issues, focus on those
- Do NOT suggest wholesale refactors as part of a review — flag
  maintainability concerns, but keep suggestions proportionate
- Do NOT review generated code, vendored dependencies, or lock files
- Do NOT repeat the same finding across multiple locations — group them:
  "Pattern: error not wrapped with context (relay.go:87, relay.go:134,
  relay.go:201)"
