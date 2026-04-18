# Template: Product Review

> Review implementation from the product perspective — does this match what
> was intended? Does it serve the defined personas? Are the user flows
> complete? Bridges the gap between what was built and what was specified.

## Metadata

| Field | Value |
|-------|-------|
| **Category** | review |
| **Mode** | single-invocation |
| **Isolation** | none (read-only analysis) |
| **Estimated tokens** | ~10K-20K |

## Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TARGET` | yes | Component, feature, or directory to review | `src/auth/`, `packages/signing/` |
| `BDD_FEATURES` | no | Path to BDD feature files | `product/features/` |
| `PERSONAS` | no | Path to persona definitions | `product/personas/` |
| `USER_STORIES` | no | Path to user stories | `product/user-stories/` |
| `OUTPUT` | no | Results path | `agents/results/product-review-auth.json` |

## Task

You are reviewing `TARGET` from the product perspective. Your goal is to
answer: **"Does this implementation serve the users it was built for?"**

Examine the implementation through every dimension below. For each finding,
identify the specific gap between intent and implementation, explain the
user impact, and suggest a concrete improvement.

### 1. BDD Alignment

If `BDD_FEATURES` are provided, compare each scenario against the
implementation:

- Does the code satisfy every Given/When/Then clause?
- Are edge cases from the scenarios handled?
- Are there implemented behaviors NOT covered by any scenario? (These are
  DISCOVERYs — undocumented features that may or may not be intentional.)
- Are there scenarios NOT implemented? (These are gaps.)

### 2. Persona Fit

If `PERSONAS` are provided, evaluate the implementation from each
persona's perspective:

- Would this persona find the workflow intuitive?
- Does the implementation respect this persona's constraints (technical
  skill level, time pressure, accessibility needs)?
- Are there assumptions about user capability that don't match the persona?
- Does the vocabulary in the UI match what this persona would expect?

### 3. User Flow Completeness

Walk through the complete user journey for the feature:

- **Happy path:** Does the intended workflow complete without friction?
- **Empty states:** What does the user see before any data exists?
- **Error states:** When things go wrong, is the message actionable?
  ("Unable to save — check your network connection" vs "Error 500")
- **Loading states:** Is there feedback during async operations?
- **First-use experience:** Is there onboarding or is the user dropped
  into a blank screen?
- **Exit points:** Can the user cancel, undo, or go back at every step?

### 4. Scope Assessment

- **Under-built:** Is the feature missing capabilities that the personas
  need? Are there TODO comments or stub implementations?
- **Over-built:** Does the feature do more than what was asked? Extra
  configurability, unnecessary settings, features no persona requested?
- **Misaligned:** Does the feature solve a different problem than what
  the personas actually need?

### 5. Consistency

- Does this feature follow the same patterns as other features in the
  app? (Navigation, layout, terminology, interaction patterns)
- Are similar actions named consistently? (Save/Submit/Confirm — pick one)
- Does the data model make sense from the user's perspective, or does it
  leak implementation details?

## Context Files

Read these before starting:
- `agents/subagent-guidelines.md` — behavioral contract for all subagents
- `QUICKCONTEXT.md` — project orientation
- `BDD_FEATURES` parameter files (if provided)
- `PERSONAS` parameter files (if provided)
- `USER_STORIES` parameter files (if provided)
- Relevant `architecture/CONTRACT-*.md` files for the target area

## Output Format

```json
{
  "template": "product-review",
  "scope": "<TARGET value>",
  "status": "complete | partial | failed",
  "summary": "One-line summary of product alignment",
  "bdd_coverage": {
    "scenarios_satisfied": 0,
    "scenarios_partial": 0,
    "scenarios_missing": 0,
    "undocumented_behaviors": 0
  },
  "findings": [
    {
      "location": "file.ts:42",
      "dimension": "persona-fit | flow-completeness | scope | consistency | bdd-alignment",
      "severity": "Critical | High | Medium | Low",
      "finding": "Login form has no 'forgot password' flow — Sarah persona needs this",
      "user_impact": "Users who forget passwords are completely blocked with no recovery path",
      "suggestion": "Add password reset flow via email — see product/features/auth.feature scenario 3"
    }
  ],
  "persona_assessment": [
    {
      "persona": "Sarah — security analyst",
      "fit_score": "good | adequate | poor",
      "notes": "Workflow matches persona needs. Vocabulary is too technical for non-crypto users."
    }
  ],
  "errors": []
}
```

## Success Criteria

- `OUTPUT` file exists and is valid JSON
- `status` is `complete`
- Every BDD scenario (if provided) is assessed
- Every persona (if provided) has a fit assessment
- Findings distinguish between under-built, over-built, and misaligned
- User impact is described in user terms, not implementation terms

## Anti-Patterns

- Do NOT evaluate code quality — that's the code-review template's job. Focus on whether the implementation serves the user, not whether the code is clean.
- Do NOT suggest technical refactors unless they directly affect the user experience. "This function should be extracted" is not a product finding.
- Do NOT assume your own preferences are the user's preferences. Judge against the defined personas, not your intuition about what users want.
- If no BDD features or personas are provided, explicitly note that the review is less rigorous and recommend creating them.
