# OpenDocKit Test Rot Post-Mortem & REBAR Gap Analysis

**Date:** 2026-04-02
**Project:** OpenDocKit (8,287 tests, 12 packages)
**Author:** Claude Code session
**REBAR Version:** v2.0.0

## Incident Summary

A fresh session inherited a repo claiming "~8,500 tests, 0 failures" in QUICKCONTEXT.md. On running `pnpm test`, we found **63 failures** across 4 packages. All were pre-existing — some for weeks. The handoff doc also listed 2 P1 stability failures that were already fixed by a prior commit.

**Time cost:** ~2 hours to track down, diagnose, and fix all 63 failures before starting actual work.

## The 5 Failure Classes

### 1. Moved External File (16 pdf-signer failures)
**What:** 7 test files referenced `/Users/will/dev/USG Briefing/USG Briefing Mar 7 - UNCLAS.pdf`. The directory was deleted; the file had moved to `~/dev/reference-files/ground-truth/`.

**REBAR coverage:** NONE. REBAR doesn't track external file dependencies.

**Root cause:** Absolute paths to user-local files. When the user reorganized their filesystem, the tests silently broke. 3 of the 7 files had `existsSync` skip guards, 4 didn't — inconsistent defensive coding.

### 2. Gitignored Test Fixtures (9 docx failures)
**What:** `embedded-fonts-integration.test.ts` pointed to `test-data/docx/` where fixtures were gitignored. The same files existed in `test-data/ground-truth/<name>/` (committed).

**REBAR coverage:** NONE. The TODO check wouldn't catch this. The freshness check wouldn't catch this. The ground-truth check could catch the test count drift (9 fewer passing) but wouldn't explain why.

**Root cause:** Test was written on a machine with local copies. The gitignored path was never corrected to the committed path. No CI to catch it.

### 3. Test Helper Bug (1 pptx failure)
**What:** `cleanroom-svg-helpers.ts` `getTextElementsAbsolute()` assumed SVG transforms were on `<text>` elements. SVGBackend changed to put them on parent `<g>`. The helper returned `transformTx: 0` for everything.

**REBAR coverage:** PARTIAL. A contract on SVGBackend's output format would have flagged this. MODULE.md for SVGBackend doesn't specify the transform encoding.

**Root cause:** SVGBackend's output format changed without updating consumers. The test helper was tightly coupled to SVGBackend's internal DOM structure.

### 4. Cross-Slide DOM Query Bug (1 editor E2E failure)
**What:** `getTextBBox()` in E2E test queried all `.svg-content-layer` elements across slides. Shape IDs aren't globally unique, so it found matching shapes on off-screen slides and clicked at y=1825 (off viewport).

**REBAR coverage:** NONE. This is a test implementation bug, not a doc/contract issue.

### 5. Stale Unit Tests (38 editor failures)
**What:** `svg-text/__tests__/` tests were written for the old text editing module. The module was partially superseded by `svg-interaction/`, and the tests were never updated. Missing jsdom env directives, stale mock paths, hard-coded position expectations.

**REBAR coverage:** PARTIAL. REBAR's zero-tolerance policy for `test.skip()` would catch explicitly skipped tests, but these tests weren't skipped — they were just broken. No mechanism detects "tests that fail silently in a workspace where `pnpm test` is rarely run end-to-end."

---

## What REBAR Currently Catches

| Failure Class | check-todos | check-freshness | check-ground-truth | steward | Contract System |
|---|---|---|---|---|---|
| Moved external file | No | No | Partial (count drift) | No | No |
| Gitignored fixtures | No | No | Partial (count drift) | No | No |
| Test helper bug | No | No | No | No | Partial (contract on output format) |
| Cross-slide DOM query | No | No | No | No | No |
| Stale unit tests | No | No | Partial (count drift) | Partial (test gap detection) | No |

**Summary:** Of the 5 failure classes, REBAR's current scripts catch **0 directly** and **2 partially** (via ground-truth count drift). The contract system could prevent 1 if the output format were specified.

---

## Proposed REBAR Improvements

### P0: `check-test-health.sh` — Zero-Failure Enforcement

**The gap:** No REBAR mechanism verifies that the test suite actually PASSES. Ground truth checks file counts, but 116 test files with 63 failures looks identical to 116 test files with 0 failures.

**Proposal:** New script that runs the test suite and verifies zero failures. Can be configured for speed:
- `--quick` mode: just `pnpm test 2>&1 | grep -c "failed"` and check for 0
- `--full` mode: parse test output for pass/fail/skip counts, compare against METRICS
- Pre-commit: too slow (tests take 30s+), but should run in CI
- Session-start: add to `refresh-context.sh` as "test baseline" step

**METRICS.md integration:**
```
# Test results (from last verified run)
total_tests_passing = 8287
total_tests_failing = 0
total_tests_skipped = 131
```

**Why this matters:** The #1 failure mode we saw was tests accumulating failures silently. The existing checks maintain doc freshness but not code health.

### P1: External Dependency Manifest

**The gap:** Tests depend on files outside the repo (`~/dev/USG Briefing/`, `~/dev/reference-files/`). When those files move, tests break silently. No mechanism tracks these dependencies.

**Proposal:** Convention for declaring external test dependencies in a `.rebar/external-deps.json`:
```json
{
  "test-fixtures": [
    {
      "path": "~/dev/reference-files/ground-truth/pptx-usg-briefing/usg-briefing.pdf",
      "used-by": ["packages/pdf-signer/src/render/__tests__/*.test.ts"],
      "skip-if-missing": true,
      "description": "USG Briefing PDF for NativeRenderer comparison tests"
    }
  ]
}
```

A `check-external-deps.sh` would verify these exist (or confirm tests have skip guards).

### P2: Contract Coverage for Render Output Formats

**The gap:** SVGBackend changed its DOM structure (transform on `<g>` vs `<text>`) and nothing flagged that consumers needed updating.

**Proposal:** REBAR contracts should cover **output format** for renderers, not just input/method interfaces:

```markdown
## Output Format Contract

SVGBackend emits SVG with this structure:
- `<g data-shape-id="N">` — shape wrapper
  - `<g transform="matrix(...)">` — position/rotation wrapper
    - `<g data-para="N">` — paragraph wrapper
      - `<text x="..." y="..." data-run="N">` — individual runs
```

When the output structure changes, all consumers (test helpers, DOM inspectors, E2E tests) must be found via `CONTRACT:` references and updated.

### P3: Module Supersession Tracking

**The gap:** `svg-text/` was partially superseded by `svg-interaction/`, but its 38 unit tests were left broken. No mechanism tracks module lifecycle (active vs. superseded vs. deprecated).

**Proposal:** Add a `status` field to MODULE.md:
```markdown
---
status: active | superseded | deprecated
superseded-by: svg-interaction/
---
```

`steward.sh` could then flag modules that are `superseded` but still have failing tests, and suggest either fixing or removing them.

### P4: Session-Start Test Baseline in refresh-context.sh

**The gap:** `refresh-context.sh` checks QUICKCONTEXT freshness, TODO state, and worktrees, but doesn't verify that tests pass. The session lifecycle protocol says "establish baseline" but doesn't enforce it.

**Proposal:** Add a `--test-baseline` flag to `refresh-context.sh`:
```bash
scripts/refresh-context.sh --test-baseline
```

This would run `pnpm test` (or project-configured test command), capture pass/fail/skip counts, and compare against METRICS.md. If there's a drift, it warns before the session starts — saving the 2-hour discovery we experienced.

### P5: Gitignore-Aware Test Path Validation

**The gap:** Tests referenced paths that were gitignored. No mechanism catches "test file X imports fixture Y, but Y is in .gitignore."

**Proposal:** A steward check that:
1. Finds all `readFileSync`/`open`/`import` in test files
2. Resolves relative paths
3. Checks if any resolved path matches a .gitignore pattern
4. Flags: "Test X references Y which is gitignored — will fail on fresh clone"

This is hard to make fully general, but even a simple regex-based scan would catch the most common cases.

---

## What We Did to Fix It

1. **Created `check-ground-truth.sh`** — ported from REBAR, customized with OpenDocKit metrics
2. **Created `METRICS.md`** — machine-verified ground truth (12 metrics)
3. **Installed pre-commit hook** — was NOT installed before (scripts existed but hook wasn't linked)
4. **Added ground truth to pre-commit** — catches metric drift at commit time
5. **Ported `refresh-context.sh`** — session-start freshness verification
6. **Fixed all 63 test failures** — path corrections, skip guards, helper fixes, mock updates
7. **Updated PDF path references** — 7 files pointed to deleted directory, now point to `~/dev/reference-files/`

## Lessons for REBAR

1. **Scripts without hooks are decorative.** Our repo had `check-todos.sh` and `check-freshness.sh` for weeks, but the pre-commit hook wasn't installed. The scripts were doing nothing. REBAR's `SETUP.md` should emphasize: "If you don't run `ln -sf`, nothing is enforced."

2. **File counts are necessary but not sufficient.** Ground truth that says "116 test files" doesn't tell you if they pass. REBAR needs a "test health" metric alongside the structural metrics.

3. **Handoff docs lie.** The session wrapup said "2 stability failures expected" but they'd already been fixed. Freshness markers show the doc was updated the same day as the fix — the doc was "fresh" but wrong. Freshness checks catch staleness but not inaccuracy.

4. **Contract coverage gaps cause ripple failures.** SVGBackend's output format wasn't contracted. One change broke a helper that broke a test that was then reported as "test framework issue" by the next agent. The actual fix was 5 lines; the diagnosis took 30 minutes.

5. **Superseded modules need lifecycle tracking.** The svg-text module was "mostly replaced" but its tests were left to rot. REBAR's MODULE.md convention needs a status field to prevent this.
