# Feedback: OpenDocKit Edit-in-Place Marathon — Architecture Overhaul, Fidelity Infrastructure, and Session Lifecycle Gaps

**Date:** 2026-03-29 to 2026-04-01
**Source:** Full rebar methodology stress-tested across editing architecture, rendering fidelity, and test infrastructure
**Type:** improvement | missing-feature | validation
**Status:** implemented
**Template impact:** practices/session-lifecycle.md, practices/red-team-protocol.md, practices/visual-fidelity.md, agents/subagent-prompts/red-team.md, ux-review.md
**From:** Claude Code agent session on OpenDocKit (35+ commits, 8,000+ tests, ~20 hours of continuous work across 3 calendar days)

---

## Session Overview

This was the longest and most ambitious session to date on OpenDocKit. It spanned:

- **Complete architectural overhaul** of the SVG text editing pipeline (edit-in-place: host-provided rendering replaces library rendering)
- **5-persona red team** analysis producing 18 structured findings, all 18 resolved
- **6 monkey test bugs** found by human testing, all 6 fixed
- **15+ parallel worktree agent launches** across editing, DOCX, PDF, and fidelity workstreams
- **94 Playwright E2E tests** with **110 baseline screenshots** for visual regression
- **SVG fidelity comparison engine** built from scratch (RMSE improved 0.159 → 0.102)
- **Structural diff tool** comparing SVG DOM vs Canvas trace at the element level
- **Real-world tests** against NASA SEWP (30 slides) and CISA presentations

The session stress-tested rebar's methodology at extreme scale: marathon duration, rapid context switching between workstreams, aggressive parallelization, and visual fidelity work that rebar has no templates for. This feedback captures battle-tested observations from the trenches.

---

## Part 1: What Rebar Gets Absolutely Right

### 1.1 The Cold Start Quad Is the Single Best Contribution

Every time I spun up a fresh agent, switched workstreams, or recovered from context compaction, the sequence `QUICKCONTEXT → TODO → KNOWN_ISSUES → AGENTS` got me productive in seconds. Not minutes — seconds.

**Why it works:** The layered context is ordered by urgency: orientation → current state → blockers → coordination norms. Each layer can be skimmed or deep-read depending on the agent's task. A fidelity agent reads QUICKCONTEXT for orientation and KNOWN_ISSUES for gaps. An editing agent reads TODO for the next task.

**Quantified impact:** I estimate the Cold Start Quad saved 30% of what would otherwise be spent on "where am I, what's happening" context thrashing.

**Validation:** After 20+ hours, the Cold Start Quad was still working perfectly. It's the foundation that enables everything else.

### 1.2 The Testing Cascade Saved Enormous Time

The T0→T1→T2→T3 cascade from `CLAUDE.md` was followed religiously:

```
T1: npx vitest run path/to/specific.test.ts         # <10s, hundreds of iterations
T2: pnpm --filter @opendockit/<pkg> test             # <30s, before committing
T3: pnpm test                                        # <60s, before pushing
T4: npx playwright test e2e/svg-edit-lib.spec.ts     # <30s, per-suite
```

Agents iterated at T1 hundreds of times without running the full 8,000+ test suite. The cascade doctrine ("iterate at T1, promote on success only, background T3+") kept each agent focused and fast.

**Key metric:** The editing pipeline went from 115 → 200 unit tests this session, all verified at T1 speed. Without the cascade, agents would have run T3 after every change and the session would have taken 3x longer.

### 1.3 Worktree Isolation Is Proven at Scale

15+ worktree agents this session. Zero lost work. The isolation pattern is battle-hardened:
- Each agent gets its own working copy
- Commits are frequent (commit-per-chunk)
- Cherry-picks onto main happen sequentially
- Post-merge testing catches integration issues

**New validation:** Unlike the March 18 session, this session had agents modifying the SAME package (3 DOCX agents touching `packages/docx/`). Worktree isolation prevented conflicts during development. Conflicts only appeared at merge time (expected and manageable).

### 1.4 The Two-Tag TODO System Prevented Drift

`TODO:` / `TRACKED-TASK:` kept the backlog honest across 35 commits. Pre-commit hooks (`scripts/check-todos.sh`) caught untracked items. The system works exactly as designed.

**New observation:** The red team report (18 issues) was tracked as a document (`docs/plans/EDIT_IN_PLACE_RED_TEAM_2026-03-29.md`) rather than as TODO items. This worked well — structured reports are better than TODO lists for prioritized backlogs with severity levels.

### 1.5 The Oracle Pattern Drove the Fidelity Work

The "PDF DOM as oracle" principle from the March 18 session directly drove two major deliverables:
1. The SVG-Canvas structural diff tool (`scripts/svg-canvas-structural-diff.mjs`)
2. The SVG fidelity comparison engine (`scripts/svg-fidelity-runner.mjs`)

Both tools exist because of the insight: "use the closest-to-ground-truth path as an oracle for debugging the others." This session validated the pattern at production scale — it's not just a debugging trick, it's an infrastructure design principle.

---

## Part 2: What Rebar Misses (The Ugly)

### 2.1 No Session Lifecycle Protocol

**The gap:** Rebar's `SETUP.md` says "every session end → update QUICKCONTEXT" but provides no structured wrapup protocol. After a 35-commit marathon spanning 3 workstreams, "update QUICKCONTEXT" is woefully insufficient.

**What actually happened:** I built ad hoc wrapup artifacts:
- Memory files for cross-session persistence
- A monkey test findings doc
- A red team report
- Inline doc updates to TODO.md and KNOWN_ISSUES.md

But these were improvised, not systematic. The next session has to figure out what's relevant from a sprawl of changes. There's no "here's the one thing to read first" handoff document.

**What's needed — a session wrapup template:**

```markdown
# Session Wrapup — [date]

## What Shipped (commits, not aspirations)
[List of commits with one-line descriptions]

## Test State
- Passing: X/Y (list failures with repro commands)
- New tests: N (list test files added)

## Known Failures (with exact repro)
- `npx playwright test -g "RW-ST-cisa101-s3"` — text content changes on click
- PI-15 — field codes not editable (pre-existing)

## Next Session Entry Point
1. Run: [exact command]
2. Read: [exact file]
3. Fix: [specific bug with file:line pointer]

## Memory Updates
[What was saved to .claude/memory and why]
```

This would take 5 minutes to fill out and save the next session 30 minutes of context archaeology.

### 2.2 No Protocol for Marathon Sessions

Rebar assumes sessions are 1-3 hours. This session was 20+ hours with multiple context compactions. The methodology has no guidance for:

**When to force a break:** Context quality degrades noticeably after ~80 tool calls. The agent starts re-reading files it read 50 calls ago. The right move is to break, commit, and start fresh — but there's no metric or trigger for this.

**How to checkpoint mid-session:** We did this ad hoc with memory writes and doc updates. A structured checkpoint protocol would help:
```bash
# Every 2 hours or 50 tool calls:
/checkpoint  # Updates QUICKCONTEXT, saves memory, commits WIP, reports test state
```

**How to hand off mid-flight:** When work is in progress across multiple worktree agents, the session state is complex: which agents are running, which committed, which need cherry-picks. There's no handoff protocol for this.

**Recommendation:** Add `practices/marathon-sessions.md` with:
- Context quality signals (re-reading files, repeating searches, losing track of agent IDs)
- Checkpoint protocol (every 2h or 50 tool calls)
- Break triggers ("if you're compacting context, it's time to break")
- Handoff template for mid-flight state

### 2.3 Cherry-Pick Merge Pain Is STILL Real

The March 18 feedback identified this perfectly and recommended `/cherry-pick-resolve`. Two weeks later, this session hit the EXACT same problem — worse, because 15 agents meant 15 potential cherry-picks.

**Specific pain points this session:**
1. Agents frequently committed on the wrong branch (`feat/svg-rendering-fidelity` instead of `main`) because the working directory had that branch checked out
2. Cherry-picks from worktree branches often had conflicts with other agents' work that had already been merged
3. Worktree cleanup was manual (`git worktree remove --force` × N)
4. Agent IDs are random hashes (`a9b5ba76082611d91`) — impossible to remember which did what

**What we did instead:** Cherry-pick the commit hash directly, resolve conflicts manually, push. This consumed ~15% of total context.

**The recommendation from March 18 still stands.** Until `/cherry-pick-resolve` exists, this will keep burning context in every parallel session.

### 2.4 No Red Team Protocol

We invented the "red team then fix" pattern on the fly:
1. Launch a red team agent with 5 personas (adversarial user, performance, concurrency, fidelity, API contract)
2. Agent produces a structured report with severity levels
3. Parse the report, create a DAG of fixes
4. Fan out agents to fix each cluster

This worked brilliantly — 18 issues found, 18 fixed, across 5 workstreams. But it's not in any rebar template. It should be a first-class practice.

**Recommended template: `practices/red-team-protocol.md`**

```markdown
# Red Team Protocol

## When to Use
- After a major architectural change
- Before a release
- When you suspect hidden quality issues

## How to Run
1. Define 3-5 personas (adversarial user, performance engineer, security analyst, etc.)
2. Each persona gets: scope, files to read, what to look for
3. Output: structured report with severity (Critical/High/Medium/Low), file:line refs, suggested fixes

## How to Act on Results
1. Group findings by root cause (not by persona)
2. Prioritize: Critical → High → Medium → Low
3. Create fix DAG: which fixes are independent, which depend on others
4. Fan out agents on independent fixes
5. Verify: run the relevant test suite after each fix

## Template Prompt
[Include the actual prompt template used in this session]
```

### 2.5 No Visual Fidelity Methodology

Rebar handles behavioral contracts well but has zero guidance for projects that produce visual output. This session built an entire visual fidelity infrastructure from scratch:

- Ground truth management (Office-exported PNGs, symlinks, manifest.json)
- RMSE measurement (per-slide, per-document, with regression thresholds)
- Screenshot-based regression testing (Playwright `toHaveScreenshot`)
- Structural element diffing (SVG DOM vs Canvas trace)
- Edit stability testing (before/after click comparison)
- The oracle pattern (PDF as verified reference for OOXML debugging)

**None of this is templated.** Any project that renders documents, generates images, produces PDFs, or has a visual UI could benefit from:

**Recommended template: `practices/visual-fidelity.md`**

```markdown
# Visual Fidelity Methodology

## Ground Truth
- How to generate reference images
- Storage structure (document/slides/slide-N.png)
- Manifest format for document indexing

## Measurement
- RMSE for pixel-level comparison
- Structural diffing for element-level comparison
- The oracle pattern: use the most accurate rendering path as reference

## Regression Prevention
- Screenshot baselines (Playwright toHaveScreenshot)
- Before/after stability testing (detect visual changes from interactions)
- Tolerance levels: 1% for text stability, 3% for visual regression, 5% for layout

## Triage
- RMSE > 0.2: investigate immediately
- RMSE 0.1-0.2: schedule for next sprint
- RMSE < 0.1: acceptable, monitor for regression
```

### 2.6 Feedback → Template Pipeline Is Manual and Slow

The March 18 feedback identified `/cherry-pick-resolve` as the #1 context saver. Two weeks and another marathon session later, it still doesn't exist. The feedback was captured, reviewed, and acknowledged — but nothing changed in the templates.

**The gap:** There's no mechanism to track which feedback items have been actioned. The `feedback/` directory is write-only — entries go in but there's no process to extract them into template changes.

**Recommendation:** Add a status field to feedback entries:
```markdown
**Status:** proposed | in-progress | implemented | wontfix
**Template impact:** [which templates need updating]
```

And a periodic review: `ask steward "which feedback items are still open?"` that scans `feedback/*.md` for `Status: proposed`.

---

## Part 3: What Worked Surprisingly Well (The Good)

### 3.1 The "Steering Wheel vs Engine" Metaphor

The user described the editing architecture as "the library should be the steering wheel and drivetrain, the PPTX pipeline should be the engine." This metaphor drove every design decision for the edit-in-place architecture.

**Rebar connection:** This is exactly what contracts are for — defining boundaries. The `SvgEditRenderCallback` interface IS a contract: "the library handles input/cursor/selection, the host handles rendering." If we'd written a formal contract before implementing, the boundary would have been even cleaner.

**Takeaway:** Metaphors from the user should be captured as architectural doctrine, not just conversation notes. They're the highest-signal input for interface design.

### 3.2 The "Human Emulator" Test Pattern

The user insisted: "I want full monkey emulators trying to find ways to break all 3 formats." This led to:
- 20-test visual interaction suite with 41 screenshots
- 5-test edit stability suite (ghost text detection)
- 22-test real-world suite against SEWP/CISA decks

The key insight was the stability test pattern: screenshot before click, click to edit, screenshot after click, demand they're pixel-identical. This catches the exact class of bugs (ghost text, double vision, text shift) that are invisible to unit tests and hard to catch in manual testing.

**Recommendation:** Add the "human emulator" pattern to `practices/testing.md`:
```markdown
## Human Emulator Tests
When building interactive features, create Playwright tests that:
1. Perform the exact sequence a human would (mousedown, mousemove, mouseup — not synthetic events)
2. Take screenshots at each interaction step
3. Compare against baselines with tight tolerance
4. Test stability: "does the visual state change when it shouldn't?"
```

### 3.3 Aggressive Parallelization Worked

This session launched 5 agents simultaneously multiple times:
- 4 editing agents (FF-1, AU-2/3/4, MT-2, MT-5) — all independent, no conflicts
- 5 cross-package agents (editing polish, DOCX headers, DOCX tables, DOCX paragraphs, PDF fonts)
- 3 fidelity agents (SVG parse errors, RMSE grind, structural diff)

**Success rate:** ~90% of parallel launches produced useful work. The 10% that didn't were due to agents committing on wrong branches or producing duplicate work.

**Key factor:** The DAG was carefully designed so agents touched different files. When two agents needed the same file (e.g., `editing-session.ts`), they were sequenced, not parallelized.

**Rebar connection:** The fan-out patterns in `AGENTS.md` should emphasize the file-level DAG: "two agents can run in parallel if and only if they modify non-overlapping files."

### 3.4 Memory as Cross-Session Bridge

The auto-memory system in Claude Code was essential. Key memories saved:
- Edit-in-place architecture decision (library = steering wheel, host = engine)
- Red team findings pointer
- Next-session pickup instructions

Without these, the next session would start cold and spend 30+ minutes rediscovering context.

**But:** The memory index (`MEMORY.md`) grows unbounded. It's now 177 lines. A periodic compaction that archives stale entries would help.

---

## Part 4: Detailed Recommendations

### 4.1 Session Wrapup Template (Critical — biggest gap)

**Add to:** `practices/session-wrapup.md`

A session should never end without producing a structured handoff. The template should be fillable in 5 minutes and contain everything the next session needs:

```markdown
# Session Wrapup — [YYYY-MM-DD]

## Duration & Scope
[1-2 sentences: what was the session about, how long, how many commits]

## What Shipped
| Commit | Category | Impact |
|--------|----------|--------|
| abc123 | feat | Description |

## Test State at Session End
```bash
# Run these first thing next session:
npx playwright test e2e/svg-realworld-editing.spec.ts  # 2 failures expected
pnpm test  # should be clean
```

## Known Failures (exact repro for each)
1. `npx playwright test -g "RW-ST-cisa101-s3"` — [root cause hypothesis]
2. `npx playwright test -g "PI-15"` — [pre-existing, low priority]

## Decisions Made (that future sessions must respect)
- Edit-in-place: library NEVER renders shapes for PPTX (host callback does)
- SVG fidelity target: < 0.08 RMSE (currently 0.102)

## Next Session Entry Point
1. Run: [exact test command to see current state]
2. Read: [exact file for context]
3. Fix: [specific bug with file:line]
```

### 4.2 Red Team as First-Class Practice (High)

**Add to:** `practices/red-team-protocol.md`

Include the actual prompt template that worked:

```markdown
You are a red team of 5 personas stress-testing [component]:
1. Adversarial User — tries to break it
2. Performance Engineer — looks for O(n^2), leaks, GC pressure
3. Concurrency/State Bug Hunter — race conditions, state corruption
4. Format Fidelity Analyst — data loss, lossy conversions
5. API/Contract Reviewer — interface design, error handling, type safety

For each persona: 3-5 issues, severity, file:line, suggested fix.
```

### 4.3 Visual Fidelity Practice (High)

**Add to:** `practices/visual-fidelity.md`

Template the entire ground truth → measurement → regression → triage pipeline that this session built from scratch.

### 4.4 Marathon Session Protocol (Medium)

**Add to:** `practices/marathon-sessions.md`

Key content:
- Break after context compaction (the quality signal)
- Checkpoint protocol: update QUICKCONTEXT + save memory + commit WIP
- Agent state tracking: which agents are running, which completed, what needs cherry-picking
- The "commit-per-chunk" rule is even more critical in marathon sessions

### 4.5 Semantic Worktree Naming (Low but annoying)

When spawning worktree agents, assign meaningful names:
```
Agent "DOCX-headers" → branch worktree-docx-headers (not worktree-agent-a9b5ba76)
```

This makes post-merge bookkeeping humane. Currently I track `a9b5ba76` → "the headers agent" in my working memory, which is exactly the kind of ephemeral context that shouldn't consume working memory.

### 4.6 File-Level DAG Enforcement for Fan-Outs (Medium)

Before launching N parallel agents, build an explicit file-level conflict matrix:

```
Agent A: editing-session.ts, shape-dom.ts
Agent B: hit-testing.ts, cursor-positioning.ts
Agent C: pptx-svg-adapter.ts, pptx-editor.ts
→ No overlaps. Safe to parallelize.

Agent D: editing-session.ts, hit-testing.ts
Agent A: editing-session.ts, shape-dom.ts
→ OVERLAP on editing-session.ts! Sequence D after A, or combine.
```

This prevents the #1 cause of merge conflicts: two agents modifying the same file.

---

## Part 5: Metrics from This Session

| Metric | Value |
|--------|-------|
| Calendar duration | ~3 days (2026-03-29 to 2026-04-01) |
| Active work time | ~20 hours |
| Commits | 35+ |
| Parallel agent launches | 15+ |
| E2E tests created | 94 (72 editing + 22 real-world) |
| Unit tests created | 85+ (115 → 200 svg-interaction) |
| Baseline screenshots | 110 |
| Red team issues found | 18 |
| Red team issues resolved | 18 (100%) |
| Monkey test bugs found | 6 |
| Monkey test bugs fixed | 6 (100%) |
| SVG RMSE improvement | 0.159 → 0.102 (-36%) |
| DOCX fixes | 2 (header inheritance, table spacing) |
| PDF fixes | 1 (preLoadFonts API) |
| Dead code removed | 2,231 lines |
| Context compactions | 3+ |

---

## Part 6: Summary — Top Recommendations by Impact

| Rank | Recommendation | Impact | Effort | Status |
|------|---------------|--------|--------|--------|
| 1 | Session wrapup template | Saves 30min per session start | Low | **New** |
| 2 | Red team protocol | Structured quality assurance | Low | **New** |
| 3 | Visual fidelity practice | Templatable for any visual project | Medium | **New** |
| 4 | `/cherry-pick-resolve` skill | Saves ~15% context in parallel sessions | Medium | Repeated from March 18 |
| 5 | Marathon session protocol | Prevents context degradation | Low | **New** |
| 6 | File-level DAG for fan-outs | Prevents merge conflicts | Low | **New** |
| 7 | Semantic worktree naming | Reduces cognitive overhead | Low | Repeated from March 18 |
| 8 | Feedback → template pipeline | Closes the improvement loop | Medium | **New** |

---

## Appendix: Key Artifacts Produced

- `docs/plans/SVG_EDIT_IN_PLACE_PLAN.md` — architecture plan (Steps 1-5, all complete)
- `docs/plans/EDIT_IN_PLACE_RED_TEAM_2026-03-29.md` — 18 issues, all resolved
- `docs/plans/MONKEY_TEST_2026-03-29.md` — 6 bugs found, all fixed
- `scripts/svg-fidelity-runner.mjs` — SVG RMSE comparison engine
- `scripts/svg-canvas-structural-diff.mjs` — element-level SVG vs Canvas diff
- `tools/editor/e2e/svg-visual-interaction.spec.ts` — 20 tests, 41 screenshots
- `tools/editor/e2e/svg-edit-stability.spec.ts` — ghost text detection
- `tools/editor/e2e/svg-realworld-editing.spec.ts` — SEWP/CISA real-world tests
- `tools/editor/e2e/svg-edit-host-rendered.spec.ts` — edit-in-place proof
