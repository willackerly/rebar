# Template: UX Review

> Structured UX audit against explicit criteria. Ensures reviews are
> consistent, thorough, and aligned with the project's standards — not
> whatever the agent's general notion of "good UX" happens to be.

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
| `TARGET` | yes | Component, page, or directory to review | `src/components/Dashboard/` |
| `SCOPE` | no | Focus area (omit for full review) | `accessibility`, `mobile`, `forms` |
| `DESIGN_SYSTEM` | no | Path to design system docs | `docs/design-system.md` |
| `OUTPUT` | no | Results path | `agents/results/ux-review-dashboard.json` |

## Task

You are conducting a UX review of `TARGET`.

Examine the implementation through every applicable dimension below. For
each finding, identify the specific file and line, explain what's wrong,
why it matters to users, and suggest a concrete fix.

## Review Dimensions

### 1. Information Architecture
- Is content hierarchy clear? Do headings, grouping, and layout communicate
  importance correctly?
- Can users find what they need without guessing? Is navigation predictable?
- Are related actions grouped? Are destructive actions separated from
  constructive ones?

### 2. Interaction Design
- Do interactive elements have clear affordances (buttons look clickable,
  inputs look editable)?
- Is feedback immediate for every user action (loading states, success/error
  confirmations, progress indicators)?
- Are there unnecessary steps? Can any workflow be shortened?
- Do forms validate inline (not just on submit)? Are error messages
  actionable ("Email must include @" not "Invalid input")?

### 3. Visual Design & Consistency
- Does the implementation follow the project's design system / component
  library? Flag any one-off styles.
- Is spacing consistent (margins, padding, gaps)?
- Is typography hierarchy clear (headings, body, captions, labels)?
- Do colors meet contrast ratios? Are colors used consistently for meaning
  (red=error, green=success)?

### 4. Accessibility (WCAG 2.1 AA)
- Can every interactive element be reached and operated via keyboard alone?
- Do all images have meaningful alt text (or empty alt for decorative)?
- Are form inputs associated with labels (`<label>`, `aria-label`,
  `aria-labelledby`)?
- Do color-only indicators have a non-color alternative (icon, text, pattern)?
- Is focus order logical? Is focus visible?
- Do dynamic content changes announce to screen readers (`aria-live`,
  `role="alert"`)?

### 5. Responsive & Adaptive
- Does the layout work at 320px, 768px, 1024px, 1440px widths?
- Do touch targets meet 44x44px minimum?
- Is text readable without horizontal scrolling at any width?
- Do images and media scale appropriately?

### 6. Error & Edge States
- What happens with empty data? Is there a meaningful empty state (not blank)?
- What happens with too much data (long names, large lists, overflow)?
- What happens during loading? Is there a skeleton/spinner/progress indicator?
- What happens on network failure? Is there a retry path?
- Are error messages user-friendly and actionable?

### 7. Performance Perception
- Does the UI feel fast? Are there unnecessary spinners or delays?
- Do interactions feel immediate (<100ms response)?
- Is above-the-fold content prioritized?

### 8. Interaction Stability (Human Emulator)

When reviewing interactive features, evaluate visual stability through
the lens of exact human interaction sequences:

- **Before/after stability:** Would a screenshot before a click be
  identical to a screenshot after clicking (when the visual state shouldn't
  change)? Look for ghost text, double rendering, layout shift.
- **Hover/focus/selection composition:** Do hover states, focus rings, and
  selection highlights compose correctly without visual glitches?
- **Real interaction sequences:** Consider the exact sequence a human would
  perform (mousedown → mousemove → mouseup, not just synthetic click events).
  Are there intermediate states that look wrong?
- **Edit-mode transitions:** When entering edit mode (clicking to type,
  opening a modal, expanding an accordion), does the transition preserve
  visual stability? No jumps, no flashes, no content reflow?

This dimension catches the class of bugs that are invisible to unit tests
and hard to spot in static code review — they only appear when a real user
interacts with the UI in real time.

## Context Files

Read these before starting:
- `QUICKCONTEXT.md` — project orientation
- `DESIGN_SYSTEM` parameter value (if provided) — component standards
- Any `*.stories.*` or storybook files in `TARGET` — understand intended usage

## Output Format

```json
{
  "template": "ux-review",
  "target": "<TARGET>",
  "scope": "<SCOPE or 'full'>",
  "status": "complete | partial",
  "summary": "One-line overall assessment",
  "dimensions_reviewed": ["information-architecture", "interaction", "visual", "accessibility", "responsive", "errors", "performance", "interaction-stability"],
  "findings": [
    {
      "location": "ComponentName.tsx:42",
      "dimension": "accessibility",
      "severity": "P0 | P1 | P2 | P3",
      "finding": "Submit button has no accessible label",
      "user_impact": "Screen reader users cannot identify the button's purpose",
      "suggestion": "Add aria-label='Submit form' to the button element"
    }
  ],
  "strengths": [
    "Note things done well — reinforces good patterns"
  ],
  "overall_score": {
    "information_architecture": "strong | adequate | needs-work",
    "interaction_design": "strong | adequate | needs-work",
    "visual_consistency": "strong | adequate | needs-work",
    "accessibility": "strong | adequate | needs-work",
    "responsive": "strong | adequate | needs-work",
    "error_states": "strong | adequate | needs-work",
    "performance_perception": "strong | adequate | needs-work",
    "interaction_stability": "strong | adequate | needs-work"
  }
}
```

## Success Criteria

- Every file in `TARGET` was examined
- At least 5 of 7 dimensions reviewed (note which were skipped and why)
- Every finding has a concrete `suggestion` (not just "fix this")
- `strengths` section is populated (don't only report problems)

## Anti-Patterns

- Do NOT review backend logic, API design, or data modeling — this is a
  UX review, not a code review
- Do NOT suggest redesigns or new features — flag what exists and how to
  improve it
- Do NOT assume a specific framework — check what's actually used before
  suggesting framework-specific fixes
- Do NOT flag style differences that are intentional design system deviations
  (check the design system docs first)
