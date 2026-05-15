# Feedback: Interaction-Class Fixes Need a Dedicated Protocol — False Positives Across Multiple Projects

**Date:** 2026-04-20
**Source:** opendockit (editor), pdf-signer (signature dialog interactions), filedag (drag/drop, playback controls)
**Type:** missing-feature / anti-pattern
**Status:** proposed
**Template impact:** `TESTING_CONTRACT.md` (add interaction-class section) — or new `INTERACTION_FIX_PROTOCOL.md` template to reference. Agent instructions (`AGENTS.md` template) should add a core tenet linking to it.
**From:** opendockit session, 2026-04-20

## What Happened

Across three projects over the past several months, agents have been shipping "interaction-class" fixes based on passing unit tests + passing E2E on sanitized fixtures + structural DOM inspection (MCP / browser devtools), and the fixes don't actually work end-to-end on the user's real workflow. Each bug typically takes 3–4 commits before it actually works, because the early commits pass all the gates the agent thought counted.

Specific incidents:

### opendockit R5-F1 (2026-04-18 through 2026-04-19)
A 2× move/resize overshoot on every Retina display user. Reported by MCP live probe. "Fixed" three times:
- commit `37d7b2b` — adapter-level DPI plumbing. Unit tests passed. E2E on `basic-shapes.pptx` passed. Missed: the adapter was passed a shim kit object with no `dpiScale` property.
- commit `8ea3cd5` — eliminated a `DPI_SCALE=2` constant in 13 consumer sites. Tests passed. Didn't fix R5-F1.
- commit `67a057f` — added Playwright project matrix at `deviceScaleFactor: 1` AND `2`. This is what finally caught the residual, and `1ae8174` (a one-line fix threading live dpiScale via a getter) was the actual fix.

**498 existing E2E tests all passed green while the bug was 100% reproducible for Retina users.** Playwright's default `deviceScaleFactor: 1` hid it. The MCP live-browser probe was correct three times in a row while the test suite said "green."

### opendockit picture click/select (2026-04-20)
- commit `323f525` — added `<g data-shape-id>` wrappers around pictures. MCP confirmed "54/54 `<image>` elements now have `[data-shape-id]` ancestors." 88 interactive E2E tests green at both DPRs. Committed. User reported picture moves snapped back on click-away. The structural change was correct but the edit-builder was using `name` as the element key while the SVG emitted the numeric `id` — `onShapeMoveEnd` silently early-exited because `_resolveElementId("316")` looked for `#316` but the model had `#Picture 5`.
- commit `bfd18c6` — one-line fix in `editable-builder.getShapeId`. The one-liner could have landed with the first commit if there had been a single end-to-end test running the full click → drag → click-away → persist sequence on a real deck with overlapping pictures.

### Cursor positioning (2026-04-19)
"Fixed" three times by prior session commits before the actual root cause was found. All prior attempts passed their tests. The real bug was two-sided: a product bug in mouse-handler + a test-helper bug in `getCursorRect`. Standard unit tests couldn't see either because they don't run the full click-and-measure chain.

### pdf-signer signature-dialog issues (earlier months)
Multiple rounds where a structural/DOM change "looked right" but the user-visible outcome on a real PDF was wrong. Exact commits not indexed here, but the pattern — "passing tests, broken UX, three commits before it worked" — was the same.

### filedag drag/drop + playback
Same pattern reported in session logs. Not detailed here; pointing at the cross-project prevalence.

## What Was Expected

The template's testing guidance distinguishes unit / integration / E2E. That's not the dimension that's failing.

**The dimension that's failing is:**

- **Interaction-class changes** (anything affecting what happens when the user clicks, drags, types, scrolls, resizes) don't get a dedicated protocol.
- Unit tests, structural DOM inspection, and E2E on synthetic fixtures all pass while the user's real workflow on a real deck is broken.

Agents read the template, see "we have a testing contract, we have zero tolerance for failures," and conclude that passing those gates means the change is good. It doesn't. For interaction-class specifically, it means the **correlates of correctness** passed — and the correlates routinely disagree with the thing being measured (user workflow on user deck).

## Recommendation for Templates

### Add a dedicated "Interaction Fix Protocol" to the testing contract

Proposed protocol (copy from opendockit's `docs/testing/INTERACTION_FIX_PROTOCOL.md`):

1. **Reproduce on the user's real deck/fixture first.** Their file is the test fixture; don't substitute a simpler one "because the repro is cleaner there."
2. **Write a failing Playwright (or equivalent) test** against that file. Assert user-visible outcomes, not structural proxies.
3. **Make the fix.**
4. **Re-run at multiple DPRs** — interaction bugs hide at `deviceScaleFactor: 1` and only show at `2` (or vice versa).
5. **Visual verification with the agent's own vision** — take screenshots at each state transition, describe what was seen. This is the step that's been consistently skipped and is the single biggest source of false confidence.
6. **Commit message must include a `VERIFIED:` block** naming the deck, the sequence, the before/after observations, and the test that now passes.

### Update `AGENTS.md` template to add a core tenet

```markdown
N. **Interaction fixes require visual verification on a real workflow.**
Unit tests, synthetic-fixture E2E, and structural DOM inspection are
correlates of correctness, not proof. For any change that affects what
happens in response to user input, follow the Interaction Fix Protocol.
No "fixed" without visual evidence in the commit.
```

### Template-level guardrails to consider

- **Project-matrix DPR** in `playwright.config.ts` template: `chromium-dpr1` + `chromium-dpr2` as default projects, not a workflow flag. Interaction bugs routinely hide at one DPR and appear at the other.
- **`real-world-interactions.spec.ts` scaffold** — a spec file that, by convention, uses user-provided or real-world-ground-truth fixtures for interaction-class tests. Not `basic-shapes.pptx` / `empty.pdf`.
- **`VERIFIED:` block requirement** enforced by `check-todos.sh` / `check-freshness.sh` analogue: a pre-commit hook that looks for keywords like `interaction`, `click`, `drag`, `mouse-handler`, `hit-test` in the diff and requires a `VERIFIED:` block in the commit message when present.

### Why this matters structurally

REBAR's strength is it makes the invisible visible — `METRICS.md` catches "silent success" drift, `TODO.md` catches "let me track that later," freshness markers catch "that doc was true last quarter." Interaction-class false positives are the next category of silent success: **the tests say green, the agent ships, the user finds the bug, and the agent's confidence was fully justified by everything the agent checked.** The gap is structural, not individual.

A dedicated protocol + a core-tenet framing + a scaffold for real-world-interactions tests would close it.

## Why I'm Formalizing This Now

The user explicitly asked: "I think we need to lock in stone method that doesn't result in false confidence as often." This has been the same pattern for months — they see it across pdf-signer, opendockit, filedag. Different agents, different sessions, same failure mode. That's a template-level gap, not a project-level one.

## Artifacts

- opendockit's version of the protocol: `docs/testing/INTERACTION_FIX_PROTOCOL.md`
- AGENTS.md tenet addition (opendockit): commit at 2026-04-20
