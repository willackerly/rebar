# Feedback: OpenDocKit Fidelity Blitz — Context Pollution, Skill Gaps, and Architectural Recommendations

**Date:** 2026-03-18
**Source:** Full rebar methodology, ASK-SHELL.md, AGENT-RUNTIME.md, DESIGN.md
**Type:** improvement | missing-feature
**From:** Claude Code agent session on OpenDocKit (20 commits, 6,042 tests, 15+ hours of continuous work)

---

## Session Overview

This was a marathon session on OpenDocKit — a client-side OOXML document renderer. The session involved:
- 3 waves of parallel worktree agent fan-outs (4 + 1 + 4 agents = 9 worktree agents total)
- 20 commits spanning DOCX rendering, PPTX regression fix, tooling consolidation, and architectural doctrine
- Extensive visual SBS comparison driving prioritization
- Discovery of a fundamental debugging principle (PDF DOM as oracle for OOXML fidelity)
- Significant context pollution from merge conflict resolution

The session stress-tested nearly every aspect of the rebar methodology: fan-out patterns, worktree isolation, pre-launch audits, visual inspection over metrics, subagent prompt design, and post-merge integration. This feedback captures what worked, what didn't, and concrete recommendations for the templates, ASK CLI, and skill system.

---

## Part 1: What Polluted the L1 Cache (Context Waste Analysis)

### 1.1 Merge Conflict Resolution (~20% of context)

**What happened:** Every cherry-pick from a worktree agent produced conflicts because the worktree branched from an earlier commit. The resolution pattern was always the same: find conflict markers, read both sides, determine "keep both" vs "pick one," edit the file, test, commit. This consumed ~20% of total context across the session.

**The pattern is mechanical:** 80%+ of conflicts were "keep both" — the worktree agent added feature X, main already had feature Y, both are correct. The remaining 20% were genuine semantic conflicts requiring understanding of both sides.

**Recommendation — `/cherry-pick-resolve` skill:**
```bash
# Auto-resolve keep-both conflicts, flag genuine conflicts for human review
/cherry-pick-resolve worktree-agent-abc123 --strategy keep-both --test "pnpm --filter @opendockit/docx test"
```

Implementation:
1. `git cherry-pick --no-commit <branch>`
2. For each conflicted file, attempt keep-both resolution (concatenate both sides, remove markers)
3. Run the specified test command
4. If tests pass → stage and report success
5. If tests fail → revert, show the genuine conflicts for manual resolution
6. Never auto-commit — always let the orchestrator review

This would compress 5-8 tool calls per cherry-pick into 1, saving ~15% of context.

### 1.2 Intermediate File Reads (~15% of context)

**What happened:** To resolve conflicts and wire features together, I read dozens of file snippets — checking function signatures, tracing call sites, verifying imports. The conclusions mattered but the raw content was dead weight.

**Recommendation:** The methodology already addresses this ("sub-agents are cheap; context is expensive"). The gap is that conflict resolution doesn't lend itself to sub-agent delegation because it requires understanding the full merge context. A dedicated merge-resolution skill (1.1 above) is the right fix.

### 1.3 Worktree Agent Result Parsing (~10% of context)

**What happened:** Remembering which agent ID (`a60fdd7b`) did numbering indents vs which (`a5f4ba76`) did CHAPTER headings. Agent IDs are random hashes with no semantic meaning.

**Recommendation — semantic agent naming:**
When spawning worktree agents, the orchestrator should assign semantic names that persist in the result:
```
Agent W1-A (roman-numerals): worktree-agent-ae6c8bf8
Agent W1-B (numbering-fix): worktree-agent-a9262b73
```
The Agent tool already supports a `description` parameter. But the returned `agentId` and `worktreeBranch` are random hashes. If the worktree branch name could be semantic (e.g., `worktree-roman-numerals` instead of `worktree-agent-ae6c8bf8`), post-merge bookkeeping would be much cleaner.

### 1.4 Post-Merge Cleanup Boilerplate (~5% of context)

**What happened:** After every cherry-pick: `git add -A && pnpm test && git commit`. After all merges: `git worktree remove` + `git branch -D` for each of N worktrees. Pure mechanical waste.

**Recommendation — `/merge-worktree` skill:**
```bash
/merge-worktree W1-A --message "feat(docx): Roman numeral page numbers" --test "pnpm --filter @opendockit/docx test"
```
Handles: cherry-pick, conflict resolution attempt, test, commit, worktree cleanup. One skill call replaces 6-8 tool calls.

### 1.5 SBS Image Analysis (~10% of context)

**What happened:** Reading rendered PNGs and reference PNGs to visually compare them. Each comparison was 2 Read tool calls. Across 20+ page comparisons, this consumed significant context.

**Recommendation:** This is inherent to the visual inspection methodology and shouldn't be eliminated — it's the most valuable part of the session. But a dedicated comparison sub-agent could do the visual assessment and return a structured report, keeping the orchestrator's context clean:
```
/sbs-assess --fixture acp240 --pages 1,5,8,10 --report structured
```
Returns: per-page structured findings (what matches, what's missing, root cause hypothesis, fix priority) without polluting the orchestrator's context with raw image data.

---

## Part 2: What Worked Extremely Well

### 2.1 Visual SBS Inspection Over RMSE

The learnings doc (§10) nails this perfectly. Today's session proved it again: RMSE said ConOps was at 0.2175, but visually the TOC was near-perfect while the section headings had numbering gaps. RMSE can't distinguish between "everything slightly off" and "one thing completely missing." The SBS viewer drove every prioritization decision.

**Status in templates:** Well-documented in learnings §10. Should be elevated to a first-class methodology principle, not just a project-specific lesson.

### 2.2 Pre-Launch Audit (Partially)

We ran 4 worktree agents in Wave 1 and all 4 produced genuine value — no redundant work. This is because we explicitly checked what existed before designing the fan-out. The audit protocol from learnings §7 prevented the 50% waste pattern.

However, Wave 1's W1-B (numbering) and W1-C (CHAPTER headings) independently solved overlapping problems (both added style-level `numPr` parsing). This created merge conflicts that consumed significant context. The pre-launch audit checked for *existing* implementations but didn't check for *overlap between planned agents*.

**Recommendation — overlap detection in fan-out planning:**
The pre-launch audit should include a "conflict matrix" step: for each pair of planned agents, list the files they'll likely modify. If two agents touch the same file, either combine them or explicitly assign non-overlapping sections. Add to `AGENTS.template.md` §Fan-Out Patterns.

### 2.3 The PDF Oracle Discovery

The most important discovery of the session: our PDF renderer (NativeRenderer) is 37% closer to Office ground truth (0.063 RMSE) than our OOXML renderer (0.100 RMSE). This means **the PDF DOM is a verified-correct oracle for debugging OOXML rendering**.

This emerged from running the unified 3-pair SBS viewer for the first time. Without the tooling to compare all three representations simultaneously, this insight would never have surfaced.

**Recommendation for templates:** Add a "cross-representation oracle" pattern to DESIGN.md. When you have multiple rendering paths for the same input, the path closest to ground truth becomes an oracle for the others. This generalizes beyond document rendering — any system with multiple implementations of the same spec can use this pattern.

### 2.4 Worktree Isolation

9 worktree agents, zero lost work. The isolation pattern from learnings §2 and §7 is proven. Every agent committed its work, every cherry-pick preserved the changes, and the post-merge integration was manageable (if tedious — see Part 1).

---

## Part 3: Recommendations for ASK CLI

### 3.1 `ask` as Merge Coordinator

The biggest gap in today's session was merge coordination. The `ask` primitive is designed for question-answer, but the most painful workflow was imperative: "cherry-pick this branch, resolve conflicts, test, commit."

**Proposal:** Extend `ask` with a `do` variant for imperative operations:
```bash
ask engineer "cherry-pick worktree-W1-A and resolve conflicts"
# vs
do engineer "cherry-pick worktree-W1-A with keep-both strategy"
```

Or keep `ask` purely interrogative and add merge coordination as a separate tool:
```bash
merge W1-A --strategy keep-both --test "pnpm test" --message "feat: ..."
```

### 3.2 `ask diff` for Agent Output Comparison

Today we had agents W1-B and W1-C independently solve overlapping problems. If we could have done:
```bash
ask diff W1-B W1-C  # Compare what each agent changed
```
We would have detected the overlap before attempting to merge both, saving significant conflict resolution time.

### 3.3 `ask trace` for Provenance

When debugging why a rendered page looks wrong, the key question is "what decisions led to this output?" The pincer methodology is a manual version of tracing. An `ask trace` command that follows the decision chain (OOXML parse → IR → layout → render) would formalize this:
```bash
ask trace "why is slide 11 group displaced?" --from ooxml-parse --to canvas-render
```

### 3.4 Agent-to-Agent via `ask` in Fan-Outs

Currently, worktree agents are fire-and-forget — they can't ask each other questions. This is correct for isolation, but it means they can't discover overlap. A read-only `ask peek` that lets an agent see what other agents are working on (without modifying anything) could help:
```bash
# Inside worktree agent W1-C's context:
ask peek W1-B  # "W1-B is modifying numbering.ts and styles.ts"
# → Agent W1-C knows to avoid those files or coordinate
```

---

## Part 4: Skill Recommendations

Based on today's session, these skills would have the highest impact:

### 4.1 `/cherry-pick-resolve` (Critical)

See §1.1. Handles 80% of merge conflicts automatically, flags the remaining 20% for human review. Single biggest context saver.

**Implementation sketch:**
```markdown
# Skill: cherry-pick-resolve
Trigger: after worktree agent completes, orchestrator wants to merge

Steps:
1. `git cherry-pick --no-commit <branch>`
2. If no conflicts → stage all, run test command, report success
3. If conflicts:
   a. For each conflicted file, try keep-both resolution
   b. Stage resolved files
   c. Run test command
   d. If tests pass → report success with "auto-resolved N conflicts"
   e. If tests fail → `git cherry-pick --abort`, report which files need manual resolution
4. Never auto-commit — orchestrator decides
```

### 4.2 `/fanout-audit` (High)

Runs the pre-launch audit protocol from learnings §7, extended with overlap detection:

```markdown
# Skill: fanout-audit
Input: list of planned agent tasks with target files/packages

Steps:
1. For each planned agent:
   a. Grep for existing implementations of the planned feature
   b. Check test counts in target packages
   c. List source files in target directories
2. Build conflict matrix: which agents touch which files
3. Flag: redundant agents (feature already exists), overlapping agents (same files)
4. Output: go/no-go for each agent, recommended merges for overlapping agents
```

### 4.3 `/sbs` (High)

Wraps `generate-sbs.mjs` with intelligent defaults:

```markdown
# Skill: sbs
Trigger: user says "run SBS", "compare", "show me the rendering"

Steps:
1. Auto-detect format from file extension
2. Find ground truth from manifest.json
3. Find Office-exported PDF if available
4. Run generate-sbs.mjs with all available layers
5. Open the HTML viewer
6. Report summary RMSE for each pair
```

### 4.4 `/sbs-assess` (Medium-High)

Delegates visual inspection to a sub-agent:

```markdown
# Skill: sbs-assess
Input: fixture name, page numbers to inspect

Steps:
1. Generate SBS PNGs for specified pages
2. Spawn sub-agent that reads rendered + reference PNGs
3. Sub-agent produces structured assessment:
   - Per-page: what matches, what's missing, root cause hypothesis
   - Prioritized fix list with effort estimates
4. Return assessment to orchestrator (not the raw images)
```

### 4.5 `/pincer-investigate` (Medium)

The PDF oracle debugging methodology as a repeatable skill:

```markdown
# Skill: pincer-investigate
Input: document, page/slide number, specific element or region to investigate

Steps:
1. Run 3-pair SBS for the specified page
2. Check GT↔PDF RMSE — if PDF is correct, the PDF DOM has the answer
3. Extract PDF trace (CanvasTreeRecorder) for the element
4. Extract OOXML trace (TracingBackend) for the element
5. Diff the two traces: position, font, size, color, transform
6. Report: "PDF says X, OOXML says Y, the fix is in Z"
```

### 4.6 `/worktree-cleanup` (Low but Annoying)

```bash
/worktree-cleanup  # removes all completed worktrees + branches
```

---

## Part 5: Architectural Recommendations

### 5.1 Subagent Templates for Rendering Work

Today's session revealed a repeatable pattern for rendering fidelity work that should be templated:

```markdown
# Template: rendering-fidelity-fix.md

## Context
You are fixing a rendering fidelity gap in [PPTX|DOCX] rendering.

## Inputs
- Specific page/slide with the issue
- Visual description of the gap (from SBS assessment)
- PDF DOM trace showing the correct rendering (from pincer investigation)

## Steps
1. Read the relevant renderer/parser/layout files
2. Compare our output against the PDF DOM oracle values
3. Identify the specific code path that produces the wrong result
4. Fix it with minimal changes
5. Write a targeted test
6. Run T1 (specific test) then T2 (package test)

## Output
- Commit with descriptive message
- Before/after RMSE for the affected page
```

### 5.2 The "Information Environment" as First-Class Concern

The methodology doc's central thesis ("agent output quality is bounded by information environment quality") was viscerally proven today. Every breakthrough came from improving the information environment:

- **SBS tooling** → agents could see what was wrong visually
- **3-pair comparison** → revealed the PDF oracle
- **Ground truth doctrine** → prevented garbage-in-garbage-out

**Recommendation:** The templates should treat information environment tooling (comparison viewers, oracle pipelines, ground truth management) as **first-class infrastructure**, not project-specific tooling. Every project that produces visual output needs SBS comparison. Every project with multiple rendering paths can use the oracle pattern. Template these.

### 5.3 Memory as Context Compression

The auto-memory system in Claude Code was essential for persisting insights across the session and for future sessions. But the memory files themselves became part of the L1 pollution — reading MEMORY.md (134 lines) on every context refresh consumed tokens.

**Recommendation:** Memory should be tiered:
- **Hot memory** (MEMORY.md index): just pointers, <50 lines, always loaded
- **Warm memory** (individual .md files): loaded on-demand when relevant
- **Cold memory** (archived entries): moved out of the index after 2+ weeks of irrelevance

The current system is roughly this shape but the index grows unbounded. A periodic `/memory-compact` skill that archives stale entries would help.

### 5.4 The "Oracle Pattern" as a Methodology Primitive

The PDF-as-oracle discovery generalizes beyond rendering:

**Any system with multiple implementations of the same specification can use the closest-to-ground-truth implementation as an oracle for debugging the others.**

Examples beyond OpenDocKit:
- Two parsers for the same format → the more accurate one is oracle for the other
- A reference implementation and a production implementation → reference is oracle
- Test doubles and real implementations → real implementation is oracle for test double accuracy

**Recommendation:** Add the "Oracle Pattern" to `DESIGN.md` as a debugging primitive alongside the existing patterns (contracts, BDD, testing cascade). It's applicable to any project where you can measure distance from ground truth across multiple paths.

---

## Part 6: What the Templates Get Right

### 6.1 The Cold Start Quad
Works exactly as designed. Every fresh agent reads QUICKCONTEXT → KNOWN_ISSUES → TODO → AGENTS and is productive within minutes. The layered context (orientation → state → tasks → norms) is the right order.

### 6.2 The Testing Cascade
T1 inner loop saved enormous time today. Agents iterated on `npx vitest run path/to/specific.test.ts` dozens of times without ever running the full 6,000-test suite unnecessarily.

### 6.3 Worktree Isolation
9 agents, zero conflicts during development. All conflicts were at merge time (expected and manageable). The pattern is proven.

### 6.4 The Two-Tag TODO System
The `TODO:` / `TRACKED-TASK:` system prevented tech debt accumulation. Every agent checked for untracked TODOs before committing.

### 6.5 The Learnings Doc
`learnings-from-opendockit.md` is the single most valuable document in the templates repo. The war stories (W6 incident, 50% waste incident) are compelling and actionable. The fact that each lesson maps to a specific template location makes it practical.

---

## Summary: Top 5 Recommendations by Impact

| Rank | Recommendation | Impact | Effort |
|------|---------------|--------|--------|
| 1 | `/cherry-pick-resolve` skill | Saves ~20% of context in merge-heavy sessions | Medium |
| 2 | `/fanout-audit` skill with overlap detection | Prevents redundant agents + merge conflicts | Low-Medium |
| 3 | Oracle Pattern in DESIGN.md | Generalizable debugging primitive | Low |
| 4 | `/sbs` + `/sbs-assess` skills | Streamlines visual fidelity workflow | Medium |
| 5 | Semantic worktree branch naming | Reduces post-merge bookkeeping confusion | Low |
