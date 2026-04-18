# Template: Cleanroom Spec Audit

> Spawn an auditor with ONLY a spec document and the implementation files.
> The auditor has zero implementation context — it cannot rationalize deviations.
> Every discrepancy between spec and code is reported as a finding.
>
> Proven pattern: found 4 HIGH-severity bugs in one session that the author missed.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | review |
| **Mode** | single-invocation |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~10K-25K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `SPEC` | yes | Path to the spec/contract document (ground truth) | `docs/plans/AUTH_UX_SPEC.md` |
| `TARGET` | yes | Files or directory to audit against the spec | `src/auth/`, `packages/signing/` |
| `OUTPUT` | no | Results path | `agents/results/cleanroom-audit-auth.json` |

## Task

You are a cleanroom auditor. Your ONLY source of truth is the `SPEC`
document. You have NO implementation context, NO architecture knowledge,
NO known issues list. This isolation is deliberate and critical.

### Rules

1. **Read the spec end-to-end first.** Understand every requirement, every
   screen state, every edge case, every persona before reading any code.

2. **Read the implementation.** Walk through every file in `TARGET`.

3. **Report every discrepancy.** If the spec says X and the code does Y,
   that is a finding — regardless of whether Y might be intentional. You
   are not qualified to judge intent because you have no context. Report it.

4. **Do NOT rationalize deviations.** An auditor with full context will
   unconsciously excuse deviations ("oh, they probably did it this way
   because..."). You must not do this. If the spec and code disagree,
   that's a finding. Period.

5. **Do NOT read architecture docs, KNOWN_ISSUES, or TODO.** These would
   give you context that compromises your isolation. You may only read
   `SPEC` and `TARGET`.

### What to Look For

- **Missing implementations:** Spec describes behavior X, no code implements it
- **Wrong implementations:** Spec says X, code does Y
- **Extra implementations:** Code does Z, spec never mentions Z (DISCOVERY)
- **Persona coverage:** Spec defines personas — are all personas' journeys implemented?
- **Edge cases:** Spec lists edge cases — are they handled in code?
- **Copy/text mismatches:** Spec defines exact UI copy — does code use the exact text?
- **State handling:** Spec describes loading/error/empty states — do they exist?

## Context Files

**READ ONLY THESE:**
- `SPEC` parameter file — your ground truth
- Files in `TARGET` — the thing being audited
- `agents/subagent-guidelines.md` — behavioral contract for all subagents

**DO NOT READ:** QUICKCONTEXT.md, TODO.md, KNOWN_ISSUES.md, architecture docs,
design docs, or any file not explicitly listed above.

## Output Format

```json
{
  "template": "cleanroom-audit",
  "spec": "<SPEC path>",
  "target": "<TARGET path>",
  "status": "complete | partial | failed",
  "summary": "N discrepancies found: X high, Y medium, Z low",
  "discrepancies": [
    {
      "location": "src/auth/login.tsx:42",
      "severity": "HIGH | MEDIUM | LOW",
      "spec_says": "Login page shows biometric option for enrolled users",
      "code_does": "Login page always shows password form, no biometric check",
      "category": "missing | wrong | extra | copy-mismatch | state-missing | persona-gap | edge-case"
    }
  ],
  "persona_coverage": [
    {
      "persona": "Alice — power user",
      "journey_tested": true,
      "gaps": ["Missing: multi-device sync flow"]
    }
  ],
  "spec_completeness": {
    "requirements_total": 0,
    "requirements_implemented": 0,
    "requirements_missing": 0,
    "requirements_wrong": 0
  },
  "errors": []
}
```

## Success Criteria

- `OUTPUT` file exists and is valid JSON
- `status` is `complete`
- Every section of the spec was compared against the implementation
- Every persona (if defined in spec) has a coverage assessment
- Every finding has both `spec_says` and `code_does` — not just one
- The auditor maintained isolation (no architecture/design doc references)

## Anti-Patterns

- Do NOT guess at implementation intent. If you find yourself writing "they probably meant to..." — stop. Report the discrepancy as-is.
- Do NOT skip spec sections that seem minor. The 4 bugs found in the proof-of-concept session were all in "minor" spec details (button labels, error message copy, missing loading states).
- Do NOT read the spec and implementation simultaneously. Read the full spec first, then the implementation. This prevents anchoring on the code's approach.
- Do NOT soften severity because "it mostly works." If the spec says X and the code does 80% of X, the missing 20% is a finding.

## Why This Works

The auditor's lack of implementation context is a **feature**, not a bug.
An auditor with full context will unconsciously excuse deviations because
they understand *why* the deviation exists. A cleanroom auditor cannot
rationalize — it can only compare. The false positive rate is low because
specs are written with intent; deviations are usually real bugs.

**Measured ROI:** 4 HIGH-severity bugs found in one session that the
implementation author missed despite having built the system.
