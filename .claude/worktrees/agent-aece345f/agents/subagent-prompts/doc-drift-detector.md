# Template: Doc-Drift Detector

> Compare documentation claims against actual code behavior. Flag stale docs,
> missing docs, contradictions between docs, and docs that reference
> non-existent files or interfaces.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | analysis |
| **Mode** | either (single doc or fan-out across doc tree) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~8K-15K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `DOCS` | yes | Doc file(s) or directory to audit | `docs/architecture/` |
| `CODE_ROOT` | no | Source root to verify against | `internal/` |
| `OUTPUT` | no | Results path | `agents/results/drift-architecture.json` |

## Task

You are auditing `DOCS` for drift from the actual codebase.

For each document, verify its claims against the code. A "claim" is any
statement about: what exists, how it works, what interfaces are defined,
what the current status is, or what files/paths are referenced.

## Drift Categories

### 1. Stale Status Claims
- Doc says "Phase N: in progress" but code shows it's complete (or vice versa)
- Doc says "X tests passing" but actual count differs significantly
- Doc says "not yet implemented" for something that exists

### 2. Broken References
- Links to files that don't exist (moved, renamed, or deleted)
- References to interfaces, types, or functions that don't exist
- Import paths or package names that have changed

### 3. Behavioral Contradictions
- Doc describes behavior X but code implements behavior Y
- Doc says "uses algorithm A" but code uses algorithm B
- Doc describes an API contract that doesn't match the implementation

### 4. Cross-Document Contradictions
- Two docs disagree on the same fact (status, architecture, ownership)
- One doc says feature is complete, another says it's planned

### 5. Missing Documentation
- Packages/modules with >5 source files and no README or MODULE.md
- Exported interfaces with no doc comments
- Complex logic with no architectural documentation

## Context Files

Read these before starting:
- `QUICKCONTEXT.md` — current project state (also a drift candidate itself)
- The actual source files referenced by the docs you're auditing

## Output Format

```json
{
  "template": "doc-drift-detector",
  "docs_audited": ["docs/architecture/OVERVIEW.md", "docs/architecture/THREAT_MODEL.md"],
  "code_root": "internal/",
  "status": "complete | partial",
  "summary": "One-line drift assessment",
  "drift_items": [
    {
      "doc": "docs/architecture/OVERVIEW.md",
      "line": 42,
      "category": "stale-status | broken-reference | behavioral-contradiction | cross-doc-contradiction | missing-doc",
      "severity": "high | medium | low",
      "claim": "What the doc says",
      "reality": "What the code actually shows",
      "suggested_fix": "Update line 42 to say '...'"
    }
  ],
  "docs_healthy": [
    "docs/architecture/THREAT_MODEL.md — all claims verified"
  ],
  "drift_summary": {
    "stale_status": 0,
    "broken_references": 0,
    "behavioral_contradictions": 0,
    "cross_doc_contradictions": 0,
    "missing_docs": 0
  }
}
```

## Success Criteria

- Every file in `DOCS` was audited (none skipped)
- Every broken reference was verified by checking the filesystem
- Claims about code behavior were verified by reading the actual source
- `docs_healthy` list confirms which docs passed audit (not just failures)

## Anti-Patterns

- Do NOT just check that files exist — verify the *content* matches claims
- Do NOT flag docs as drifted because of minor wording differences — focus
  on factual inaccuracies
- Do NOT rewrite the docs — report the drift and suggest fixes. The
  orchestrator decides what to update
- Do NOT assume your knowledge of the codebase is current — read the actual
  files to verify claims
