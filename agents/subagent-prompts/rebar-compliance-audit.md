# Template: REBAR Compliance Audit

> Assess a project's compliance with REBAR v2.0.0. Produces a structured
> report covering structural presence, content accuracy, session lifecycle,
> agent coordination, and enforcement maturity. Writes a
> REBAR-COMPLIANCE-ASSESSMENT.md to the project root.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | analysis |
| **Mode** | single-invocation or fan-out (one per repo) |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~15K-30K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `PROJECT_ROOT` | yes | Absolute path to the project to audit | `/Users/will/dev/filedag` |
| `REBAR_ROOT` | no | Path to rebar repo (for reference docs) | `/Users/will/dev/rebar` |
| `OUTPUT` | no | Where to write the report (default: `PROJECT_ROOT/REBAR-COMPLIANCE-ASSESSMENT.md`) | — |

## Task

You are auditing `PROJECT_ROOT` for compliance with REBAR v2.0.0.

Examine the project systematically through each section below. For each
check, record PASS, PARTIAL, FAIL, or N/A with a brief explanation.
Do not guess — verify by reading files, running `ls`, checking `git log`.

### Section 1: Structural Presence (Cold Start Quad)

Check that these files exist and have substantive content (not just
placeholder text):

| File | Check | How to Verify |
|------|-------|---------------|
| `README.md` | Exists, has rebar badge | `head -5 README.md` — look for `> **rebar v` |
| `QUICKCONTEXT.md` | Exists, has `last-synced` date | `grep "last-synced" QUICKCONTEXT.md` |
| `TODO.md` | Exists, has open items section | `grep -c "^\- \[ \]" TODO.md` |
| `AGENTS.md` | Exists, references session lifecycle | `grep -i "session lifecycle" AGENTS.md` |
| `CLAUDE.md` | Exists (for Claude Code projects) | `ls CLAUDE.md` |
| `.rebar-version` | Exists, contains version | `cat .rebar-version` |
| `.rebarrc` | Exists, declares tier | `cat .rebarrc` |

### Section 2: Contract System

| Check | How to Verify |
|-------|---------------|
| `architecture/` directory exists | `ls architecture/` |
| At least 1 contract file exists | `ls architecture/CONTRACT-*.md` |
| Source files have `CONTRACT:` headers | `grep -rn "CONTRACT:" --include="*.go" --include="*.ts" --include="*.py" src/ internal/ packages/ lib/ 2>/dev/null \| head -5` |
| Contract references point to valid files | For each `CONTRACT:X` in code, check `architecture/CONTRACT-X*.md` exists |
| Seam contracts exist (if multi-language) | `ls architecture/CONTRACT-SEAM-*.md 2>/dev/null` |

### Section 3: Enforcement & Scripts

| Check | How to Verify |
|-------|---------------|
| `scripts/` directory exists | `ls scripts/` |
| Pre-commit hook installed | `ls -la .git/hooks/pre-commit` — should be symlink to scripts/ |
| `check-contract-refs.sh` exists | `ls scripts/check-contract-refs.sh` |
| `check-todos.sh` exists | `ls scripts/check-todos.sh` |
| `check-freshness.sh` exists (Tier 2+) | `ls scripts/check-freshness.sh` |
| `check-ground-truth.sh` exists (Tier 2+) | `ls scripts/check-ground-truth.sh` |
| `check-compliance.sh` exists (Tier 2+) | `ls scripts/check-compliance.sh` |
| `refresh-context.sh` exists | `ls scripts/refresh-context.sh` |
| `METRICS` file exists (Tier 2+) | `ls METRICS*` |
| Scripts exclude `.claude/worktrees/` | `grep -l "\.claude" scripts/*.sh` |

### Section 4: Agent Coordination

| Check | How to Verify |
|-------|---------------|
| `agents/` directory exists | `ls agents/` |
| `agents/subagent-guidelines.md` exists | Has The 10 Rules |
| `agents/subagent-prompts/` has templates | `ls agents/subagent-prompts/` |
| `agents/subagent-prompts-index.md` exists | Lists all templates |
| ASK CLI configured | `ls bin/ask` or reference in AGENTS.md |

### Section 5: Session Lifecycle (v2.0.0)

| Check | How to Verify |
|-------|---------------|
| AGENTS.md mentions session lifecycle | `grep -i "session" AGENTS.md` |
| QUICKCONTEXT has "What's Next" section | `grep "What's Next" QUICKCONTEXT.md` |
| TODO.md is forward-looking (<50 open lines) | `grep -c "^\- \[ \]" TODO.md` |
| Completed items are collapsed or archived | `grep -c "<details>" TODO.md` or check completed section is short |
| Session wrapup docs exist | `ls docs/session-wrapups/ 2>/dev/null` |

### Section 6: Content Accuracy (the "ceremony vs truth" check)

This is the most important section. Structural presence without content
accuracy is ceremony, not methodology.

| Check | How to Verify |
|-------|---------------|
| QUICKCONTEXT test count matches reality | Run test suite, compare count against QUICKCONTEXT claim |
| QUICKCONTEXT `last-synced` date is <7 days old | `grep "last-synced" QUICKCONTEXT.md` |
| TODO.md open items are actually open | Cross-reference P0 items against `git log --oneline -20` |
| Contract lifecycle is computed, not declared | Check if CONTRACT-REGISTRY has lifecycle status matching actual state |
| METRICS file matches code reality | If METRICS exists, spot-check 2-3 values against actual counts |

### Section 7: Testing Cascade

| Check | How to Verify |
|-------|---------------|
| Testing tiers defined (T0-T5) | `grep -i "T0\|T1\|T2" AGENTS.md` |
| Tiers use granularity model (Typecheck→Targeted→Package→Cross→E2E→Full) | NOT type model (Unit→Integration→Security→System→Load→Chaos) |
| Test suite passes | Run the project's test command |
| No skipped tests (`test.skip`, `xit`, `xdescribe`) | `grep -rn "\.skip\|xit\|xdescribe" --include="*.test.*" --include="*_test.*"` |

## Output Format

Write `REBAR-COMPLIANCE-ASSESSMENT.md` to the project root:

```markdown
# REBAR Compliance Assessment

**Project:** {project name}
**Date:** {YYYY-MM-DD}
**REBAR Version:** v2.0.0
**Declared Tier:** {from .rebarrc or "none"}
**Assessed By:** Claude (automated audit)

## Score: {X}/10

## Summary

{2-3 sentences: overall state, biggest gap, biggest strength}

## Detailed Results

### Section 1: Structural Presence — {PASS|PARTIAL|FAIL}
| Check | Status | Notes |
|-------|--------|-------|
| README.md with badge | {PASS/FAIL} | {detail} |
...

### Section 2: Contract System — {PASS|PARTIAL|FAIL}
...

### Section 3: Enforcement & Scripts — {PASS|PARTIAL|FAIL}
...

### Section 4: Agent Coordination — {PASS|PARTIAL|FAIL}
...

### Section 5: Session Lifecycle — {PASS|PARTIAL|FAIL}
...

### Section 6: Content Accuracy — {PASS|PARTIAL|FAIL}
...

### Section 7: Testing Cascade — {PASS|PARTIAL|FAIL}
...

## Top 5 Recommendations (priority order)

1. {Most impactful fix}
2. ...

## Compliance Score Breakdown

| Section | Weight | Score | Weighted |
|---------|--------|-------|----------|
| Structural Presence | 15% | {0-10} | {weighted} |
| Contract System | 20% | {0-10} | {weighted} |
| Enforcement & Scripts | 15% | {0-10} | {weighted} |
| Agent Coordination | 10% | {0-10} | {weighted} |
| Session Lifecycle | 10% | {0-10} | {weighted} |
| Content Accuracy | 20% | {0-10} | {weighted} |
| Testing Cascade | 10% | {0-10} | {weighted} |
| **Total** | **100%** | | **{total}/10** |
```

## Success Criteria

- `REBAR-COMPLIANCE-ASSESSMENT.md` written to project root
- Every section has a status (PASS/PARTIAL/FAIL/N/A)
- Content accuracy checks are verified against actual state, not just doc claims
- Top 5 recommendations are actionable with specific file paths
- Score is honest — don't inflate for structural presence when content is stale

## Anti-Patterns

- Do NOT give credit for files that exist but contain placeholder text
- Do NOT skip the content accuracy section — it's the most important one
- Do NOT run `pnpm test` or `go test` if it would take >2 minutes — note "not verified (time constraint)" instead
- Do NOT penalize projects that don't use REBAR at all — give them a baseline score of 0/10 with a note "REBAR not adopted" and skip detailed checks
