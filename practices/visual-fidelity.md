# Visual Fidelity Methodology

**Referenced from AGENTS.md. Read when working on projects that produce visual output.**

---

## When to Use

This practice applies to any project that renders visual output:
- Document renderers (PDF, OOXML, HTML)
- Image processors or generators
- UI component libraries
- Data visualization tools
- PDF signing or stamping
- Any system where "does it look right?" is a quality criterion

---

## Core Concepts

### Ground Truth

**Ground truth** is the reference that defines "correct." Every fidelity
measurement is a comparison against ground truth.

| Source | When to Use | Trust Level |
|--------|------------|-------------|
| Original application output (e.g., PowerPoint export) | Format conversion projects | Highest |
| Designer mockup / Figma export | UI components | High |
| Specification-defined rendering | Standards compliance | High |
| Previous version output | Regression detection | Medium |
| Manual screenshot | Quick baseline | Low |

**Storage structure:**
```
tests/ground-truth/
  document-name/
    slides/           # or pages/, frames/, etc.
      slide-1.png
      slide-2.png
    manifest.json     # document metadata, slide count, source
```

### The Oracle Pattern

When a system has multiple rendering paths for the same input, the path
closest to ground truth becomes the **oracle** — the reference for
debugging the others.

**Example:** A document renderer has:
- SVG rendering (fast, interactive)
- Canvas rendering (pixel-accurate, non-interactive)
- PDF rendering (print-quality)

If Canvas rendering matches the original document at RMSE 0.02 but SVG
is at RMSE 0.15, Canvas is the oracle. To debug SVG issues, compare SVG's
intermediate state (DOM, computed styles) against Canvas's equivalent.

**The pattern generalizes:** Measure each implementation's distance from
ground truth. The closest one reveals what the others should be doing.
Instead of guessing why Implementation B produces wrong output, compare
its internals against the oracle's.

---

## Measurement

### Pixel-Level: RMSE

Root Mean Square Error between two images. Lower is better.

```
RMSE = sqrt(mean((pixel_a - pixel_b)^2))
```

**Thresholds (adjust for your project):**

| RMSE | Interpretation | Action |
|------|---------------|--------|
| < 0.05 | Excellent | Monitor for regression |
| 0.05 - 0.10 | Good | Schedule improvement |
| 0.10 - 0.20 | Needs work | Investigate and fix |
| > 0.20 | Broken | Fix immediately |

**Per-element vs per-image:** Per-image RMSE can hide localized problems
(one badly-rendered element in an otherwise-perfect page). When RMSE is
high, break down to per-element comparison to find the root cause.

### Structural: Element-Level Diff

Compare the logical structure, not just pixels:
- SVG DOM vs Canvas drawing calls
- HTML DOM vs expected component tree
- PDF content stream vs expected operations

Structural diff catches issues that RMSE misses:
- Correct appearance but wrong DOM (accessibility broken)
- Z-order issues (overlapping elements render correctly by accident)
- Missing elements that happen to be transparent or off-screen

### Stability: Before/After Comparison

The most insidious visual bugs are **interaction-triggered regressions**:
the page looks correct until you click, hover, or scroll, and then
something changes that shouldn't.

**The stability test pattern:**
1. Screenshot the initial state
2. Perform an interaction (click, hover, focus)
3. Screenshot the result
4. If the interaction shouldn't change the visual state, demand the
   screenshots are pixel-identical

**What this catches:**
- Ghost text (duplicate rendering on edit)
- Layout shift on interaction
- Double vision (original + edited text both visible)
- State leaks between components

---

## Regression Prevention

### Screenshot Baselines

Use your test framework's screenshot comparison (e.g., Playwright's
`toHaveScreenshot`, Cypress's `matchImageSnapshot`):

```typescript
// Playwright example
test('slide renders correctly', async ({ page }) => {
  await page.goto('/viewer?doc=test-deck&slide=3');
  await expect(page.locator('.slide-container')).toHaveScreenshot(
    'test-deck-slide-3.png',
    { maxDiffPixelRatio: 0.01 }
  );
});
```

**Tolerance levels (starting points):**
- Text stability (before/after click): 0% — pixel-identical
- Visual regression (between versions): 1% pixel diff ratio
- Layout comparison (responsive breakpoints): 3% pixel diff ratio
- Cross-browser: 5% pixel diff ratio

### Real-World Test Documents

Synthetic test documents catch known issues. Real-world documents catch
unknown ones. Include 3-5 real documents (with appropriate licensing) in
your test suite:

```
tests/fixtures/real-world/
  nasa-sewp-30-slides.pptx      # Complex government deck
  cisa-advisory-15-slides.pptx  # Dense text + diagrams
  quarterly-report.xlsx          # Charts + formatting
  legal-contract.docx            # Headers, footers, tracked changes
```

Test each against ground truth at every CI run.

---

## Human Emulator Tests

When building interactive features, create tests that simulate exact
human interaction sequences:

1. **Use real browser events**, not synthetic dispatches:
   ```typescript
   // Good: actual mouse interaction
   await element.click({ position: { x: 50, y: 20 } });
   
   // Bad: synthetic event
   await element.dispatchEvent('click');
   ```

2. **Screenshot at each interaction step**, not just the final state.

3. **Test stability:** screenshot before interaction, interact, screenshot
   after — demand they're identical when the interaction shouldn't change
   the visual state.

4. **Test composition:** multiple interactions in sequence (click, type,
   click elsewhere, undo). Each step should leave the visual state
   consistent.

---

## Triage Workflow

When visual issues are found:

1. **Isolate:** Is this a rendering engine issue, a data parsing issue, or
   a layout/CSS issue? Compare against the oracle to narrow down.

2. **Measure:** What's the RMSE before and after? Track improvement
   quantitatively, not just "it looks better."

3. **Prioritize:** User-visible issues in common documents > edge cases
   in unusual documents > pixel-perfect accuracy in rare scenarios.

4. **Regress:** After fixing, add a screenshot baseline test that catches
   this specific issue if it returns.

---

## Tooling Patterns

### Fidelity Runner Script

A script that processes a batch of test documents and produces an RMSE
report:

```bash
# Example: scripts/fidelity-runner.sh
for doc in tests/ground-truth/*/manifest.json; do
  dir=$(dirname "$doc")
  name=$(basename "$dir")
  echo "=== $name ==="
  for gt in "$dir"/slides/*.png; do
    slide=$(basename "$gt" .png)
    rendered="output/$name/$slide.png"
    rmse=$(compare -metric RMSE "$gt" "$rendered" null: 2>&1)
    echo "  $slide: RMSE $rmse"
  done
done
```

### Structural Diff Tool

Compare the logical structure of two rendering paths:

```bash
# Example: compare SVG DOM structure vs Canvas drawing calls
node scripts/structural-diff.mjs \
  --svg output/deck/slide-3.svg \
  --canvas output/deck/slide-3.canvas-trace.json \
  --output reports/structural-diff-slide-3.json
```

These tools are project-specific but the pattern is reusable. Build them
early — they pay for themselves within the first debugging session.
