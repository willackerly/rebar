# Learnings from OpenDocKit: Drift Prevention, Agent Patterns, and Contract-First Methodology

**Source project:** OpenDocKit — a progressive-fidelity, client-side OOXML document renderer. Monorepo with 8 packages, 5,824+ tests, developed extensively using Claude Code agents with worktree isolation, fan-out strategies, and multi-wave execution plans.

---

## Implementation Status

The actionable patterns from this document have been implemented in the
agent-templates kit. This table maps each lesson to where it lives now:

| Learning | Implemented In |
|----------|---------------|
| Cold Start Quad | `README.template.md`, `QUICKCONTEXT.template.md`, `TODO.template.md`, `AGENTS.template.md` |
| Testing Cascade (T0-T5) | `AGENTS.template.md` §Testing Cascade |
| Worktree Isolation | `AGENTS.template.md` §Agent Collaboration Patterns |
| Central Contracts / IR Types | `architecture/`, `methodology.md` §1-3 |
| TODO Two-Tag System | `AGENTS.template.md` §TODO Tracking, `TODO.template.md` |
| Fan-Out Patterns | `AGENTS.template.md` §Subagent Prompt Templates, `agents/` |
| Doc Drift | `agents/subagent-prompts/doc-drift-detector.md` |
| W6 / Feature Inventory | `agents/subagent-prompts/feature-inventory.md`, `AGENTS.template.md` §Feature Inventory Protocol |
| Contract-First Gaps | `architecture/` contract system, `methodology.md` §2-3 |
| Pre-Launch Audit | `AGENTS.template.md` §Pre-Launch Audit |
| Feature Inventories | `agents/subagent-prompts/feature-inventory.md` |
| Freshness Timestamps | All template files (freshness markers) |
| Cherry-Pick Resolution | `AGENTS.template.md` §Cherry-Pick Conflict Resolution |
| Trust-but-Verify | `AGENTS.template.md` §Post-Merge Integration, `methodology.md` §4 |
| Subagent Templates | `agents/`, `methodology.md` §6 |
| Visual Inspection / SBS | Project-specific (rendering), general principle in `methodology.md` |
| Branded Types | Project-specific (TypeScript), not templated |
| Metrics Bundle | Project-specific (font data), not templated |

This document remains as reference material — the "why" behind each pattern,
the war stories, and the failure analysis that motivated the templates.

---

## 1. Executive Summary

The central lesson from building OpenDocKit with AI agents is this: **the quality of agent output is bounded by the quality of the context agents receive, and context degrades over time unless actively maintained.** We built excellent cold-start documentation (QUICKCONTEXT.md, AGENTS.md, TODO.md, KNOWN_ISSUES.md), powerful parallelization patterns (9 simultaneous worktree agents completing major work in hours), and rigorous tracking systems (the TODO/TRACKED-TASK two-tag methodology). All of these worked well in the moment they were created. But documentation drifts from reality at the speed of code changes, and agents both suffer from and contribute to that drift. The fundamental challenge of agent-driven development is not making agents productive — it is preventing the information environment they depend on from decaying.

---

## 2. What Worked (and Why)

### The Cold Start Quad: QUICKCONTEXT / AGENTS / TODO / KNOWN_ISSUES

Every new agent session reads four files in order and knows where it stands within five minutes. This pattern works because it separates concerns cleanly: QUICKCONTEXT.md is the "what is true right now" file (current branch, recent changes, test counts), AGENTS.md is the "how we work" file (norms, parallelization guides, testing cascade), TODO.md is the "what needs doing" file, and KNOWN_ISSUES.md is the "what will bite you" file.

**Underlying principle:** Agents need layered context — orientation first, then norms, then tasks, then hazards. Forcing a reading order prevents agents from diving into code before they understand the project's current state.

### The Testing Cascade (T0-T5 Tiers)

The tiered testing approach — from T0 (typecheck, <5s) through T1 (single test, <10s) to T5 (full suite, <5min) — prevented agents from wasting time on full-suite runs during inner-loop development. Before we formalized this, agents would routinely run `pnpm test` (all 5,000+ tests) to validate a single function change, burning 3-5 minutes per iteration.

**Underlying principle:** Agents default to the most thorough validation they know about unless you explicitly tell them not to. The cascade gives them a clear escalation path: iterate at T1, promote on success, background the expensive tiers. The key insight is that "never run T5 in your inner loop" must be stated as a rule, not left as common sense.

### Worktree Agent Isolation

Using `git worktree` to give each parallel agent its own working directory was transformative. During the Fidelity Blitz, 9 agents worked simultaneously on different files — group transforms, gradients, effects, rotation, geometry arcs, DOCX rendering, table borders, font substitutions — without stepping on each other. The merge phase (cherry-picking commits to main) was manageable because each agent's changes were cleanly scoped.

**Underlying principle:** Agents cannot coordinate in real time. Isolation (separate working trees, explicit file ownership) is the only reliable way to prevent conflicts. Coordination happens at merge time, not during development.

### IR Types as Central Contract

The intermediate representation types (ShapePropertiesIR, FillIR, TextBodyIR, etc.) serve as the shared contract between parsers and renderers. Parsers produce IR, renderers consume IR, and they never call each other directly. This made parallelization possible — a parser agent and a renderer agent could work simultaneously because the IR type definitions were the stable interface between them.

**Underlying principle:** Parallel agent work requires explicit, stable interfaces. When you can point two agents at the same type definition and say "produce this" / "consume this," they can work independently. Without such a contract, agents must read each other's code to understand integration points.

### The TODO Two-Tag System

The `TODO:` / `TRACKED-TASK:` methodology — where untracked TODOs block commits and must be either fixed or promoted to TODO.md — prevented technical debt from accumulating invisibly. Before this system, agents would scatter TODO comments throughout the codebase with no tracking, and subsequent agents would never find them.

**Underlying principle:** Agents are prolific TODO generators but terrible TODO followers-up. Making the commit hook enforce tracking turns a natural agent behavior (leaving notes for future work) into a structured process.

### Fan-Out Patterns

We developed four distinct fan-out patterns: research fanout (2-4 agents searching different areas simultaneously), implementation fanout (worktree isolation for independent changes), validation fanout (typecheck + tests + lint in parallel background agents), and speculative fanout (try multiple approaches, keep the best). The implementation fanout was the most impactful — the Fidelity Blitz shipped 13 commits across 9 agents in a single session.

**Underlying principle:** Sub-agents are cheap; context is expensive. When you need to understand or change multiple independent things, sequential processing wastes the main agent's context on intermediate results. Fan out, then synthesize.

### Branded Types (EMU, Points)

Compile-time unit safety through TypeScript branded types (EMU for English Metric Units, Points for screen coordinates) caught an entire class of unit-conversion bugs at the type level. Agents could not accidentally pass EMU values where Points were expected.

**Underlying principle:** When agents work at speed, they make the same category errors humans make — but faster and in higher volume. Type-level safety catches systematic errors that testing alone would miss because tests only cover the cases you think to write.

---

## 3. What Didn't Work (and Why)

### Documentation That Drifts

Despite having a comprehensive documentation maintenance policy (doc update checklists, ownership tables, mandatory update rules), documentation still drifted significantly from reality. A full audit found: 6 out of 11 plan documents had stale status fields, COVERAGE.md was 653 tests behind the actual count, TODO.md and QUICKCONTEXT.md contradicted each other on font delivery phase completion, 3 packages had zero documentation despite active development, and the `docs/specifications/` directory — designated as the home for contract-first specs — was nearly empty.

**Root cause:** Documentation updates are a secondary task that agents deprioritize when focused on their primary coding objective. The policy said "update docs after every code change," but agents consistently shipped code without updating the corresponding plan docs, status fields, or cross-references. The policy was correct but unenforceable without tooling.

### Agent Knowledge Gaps in Worktrees

The W6 incident is the canonical failure case. A worktree agent was assigned to improve DOCX rendering in `doc-kit.ts`. It also modified `line-breaker.ts` and `page-layout-engine.ts`, deleting four features — alignment positioning, lineSpacingRule handling, pageBreak detection, and indent support — because it did not know those features existed. The files went from 617 lines to 422 lines. Fourteen tests caught the regression, but the agent never ran the full package suite.

**Root cause:** Agents operate on what they can see. A worktree agent sees only its assigned files and whatever context you give it in the prompt. If a file has 600 lines of logic implementing 8 features, and you tell the agent to "add table borders and heading styles," it will restructure the file around the new features and may discard existing logic it doesn't recognize as intentional.

### Contract-First Gaps

The project aspired to contract-first development — write specs, then implement. In practice, the `docs/specifications/` directory remained nearly empty. Agents building new parsers or renderers had no authoritative spec to code against. They reverse-engineered contracts from IR type definitions and existing parser implementations. This worked for experienced agents with good context, but led to subtle mismatches for agents working from cold starts.

**Root cause:** Writing specs is pure overhead in the moment — it produces no running code and no passing tests. Agents optimize for shipping features, not for enabling future agents. The contract-first policy was aspirational, not structural.

### Cross-Reference Rot

Broken links accumulated across the documentation tree. Architecture docs referenced files that had been moved or deleted. Plan docs referenced paths at the repo root when files had been relocated to `docs/plans/`. The architecture README referenced `ELEMENT_MODEL_BRIEF.md` which no longer existed at the expected path.

**Root cause:** When agents move or rename files, they update the obvious references (import statements, nearby README) but miss distant cross-references in architecture docs, plan docs, and navigation tables. No tooling validates link integrity.

---

## 4. The Drift Problem

The most dangerous pattern in agent-driven development is **intent-to-execution drift**: when documentation describes one reality and code implements another. This is not unique to AI agents — human teams experience it too — but agents amplify it in three ways:

**Agents produce drift faster.** A single agent session can ship 13 commits across 9 worktrees. Each commit potentially invalidates some documentation claim. Human developers might update one feature per day; agents update several per hour.

**Agents consume stale docs uncritically.** A human developer might notice that a plan doc marked "Phase 4 remaining" feels wrong because they remember shipping Phase 4 last week. An agent has no such memory. It reads the doc, trusts it, and makes decisions accordingly.

**Drift compounds across agent sessions.** Here is the specific failure chain we observed:

1. Agent A completes Font Delivery Phase 4 (CDN polish) as part of Wave 1 work.
2. Agent A updates QUICKCONTEXT.md to say "Phases 1-5 COMPLETE" but does not update TODO.md, which still shows Phase 4 and Phase 5 as unchecked items, or the plan doc (FONT_DELIVERY_EXECUTION.md), which still says "Phases 4-5 remaining."
3. Agent B starts a new session, reads TODO.md, sees unchecked Phase 4/5 items, and either (a) wastes time investigating already-completed work or (b) makes plans that assume these phases are incomplete.
4. AGENTS.md says "Phase 1-3 COMPLETE, Phase 4-5 remaining" — a third contradictory claim.
5. A documentation audit finds 3 sources disagree on the same fact.

The cost is not just wasted time. It is **lost trust in the documentation system itself.** Once agents learn (through experience or instruction) that docs may be stale, they start falling back to code as the source of truth. But code cannot tell you what was intended, what is planned, or what was deliberately deferred. Code only tells you what exists right now. Without trustworthy documentation, agents lose the ability to make strategic decisions.

---

## 5. Proposed Anti-Drift Mechanisms

### 5.1 Documentation Sentinel Agents

Run a dedicated agent post-merge whose sole job is to verify documentation consistency. This agent would:

- Compare test counts in COVERAGE.md and QUICKCONTEXT.md against `pnpm test` output
- Check plan doc status fields against recent git history (if a plan says "Phase N: Planning" but commits reference "Phase N" completion, flag it)
- Validate all cross-references (links in markdown files point to existing files)
- Verify TODO.md items against TRACKED-TASK comments in code
- Report discrepancies as a structured diff

This could be implemented as a Claude Code custom skill (`/doc-sentinel`) or a post-commit hook that spawns a verification agent.

### 5.2 Feature Inventories for Worktree Agents

Before assigning a worktree agent to modify a file with >300 lines of logic, generate a feature inventory: an explicit list of every behavior the file implements, with the test that exercises it. Include this inventory in the agent prompt with the instruction "preserve all listed features unless explicitly told to remove them."

For the W6 incident, this would have looked like:

```
line-breaker.ts feature inventory:
- Line breaking with word wrap (test: line-breaker.test.ts#L12)
- Alignment positioning (left/center/right/justify) (test: line-breaker.test.ts#L45)
- lineSpacingRule: exact and atLeast (test: line-breaker.test.ts#L78)
- Page break detection (test: page-layout.test.ts#L120)
- Paragraph indent (firstLine, hanging) (test: line-breaker.test.ts#L95)
DO NOT remove any of these features.
```

### 5.3 Contract-First Enforcement via MODULE.md

Rather than asking agents to write standalone spec docs (which they consistently deprioritize), make MODULE.md files the enforceable contract. Require that every directory with >3 source files has a MODULE.md that documents: purpose, inputs (types received), outputs (types produced), dependencies, and key behavioral constraints.

The enforcement mechanism: a pre-commit check that verifies MODULE.md exists for directories above the size threshold, and a `/contract-check` skill that validates MODULE.md claims against actual exports.

### 5.4 Post-Merge Doc Verification Checklist

Add a machine-readable checklist to the commit message or PR template:

```
docs-updated: [yes/no/not-needed]
plan-status-updated: [yes/no/not-needed]
test-count-checked: [yes/no]
cross-refs-validated: [yes/no]
```

A post-merge hook or CI step can flag commits where `docs-updated: no` but the diff touches files that have associated documentation (detected via a doc-ownership mapping).

### 5.5 Freshness Timestamps with Automated Staleness Detection

Every documentation file that contains status claims should have a machine-readable freshness marker:

```markdown
<!-- freshness: 2026-03-16 -->
```

A weekly cron job or `/freshness-audit` skill scans for files where the freshness date is more than 2 weeks old relative to the last commit that modified the related source code. This turns "docs might be stale" into "these specific docs are probably stale."

---

## 6. Template Improvement Recommendations

### AGENTS.template.md

1. **Add a "Feature Inventory Protocol" section** under the worktree agent guidance. Template text should instruct agents to generate feature inventories for files >300 lines before assigning them to worktree agents.

2. **Add a "Documentation Sentinel" section** describing the post-merge verification pattern. Include a sample `/doc-sentinel` skill definition.

3. **Strengthen the Cold Start methodology** by adding a "trust but verify" step: after reading QUICKCONTEXT.md, cross-reference its claims against `git log --oneline -10` and the actual file tree. The current template includes this but it should be more prominent — make it Step 1, not a sub-step.

4. **Add an "Agent Collaboration Patterns" section** covering: worktree isolation rules, cherry-pick conflict resolution, post-merge integration planning, and the "never modify files outside your assigned list" rule.

5. **Add freshness markers** to the template's status-bearing sections (Active Workstreams, etc.) with a comment explaining the convention.

### CLAUDE.template.md

1. **Add a "Contract-First" section** in the Testing block that explicitly requires MODULE.md for directories above a size threshold.

2. **Add the feature inventory protocol** to the Allowed Commands or Agent Autonomy section — agents should know they can and should generate inventories before delegating to sub-agents.

3. **Add a "Documentation Freshness" command** to the pre-commit checklist alongside the TODO tracking check.

4. **Add "Worktree Agent Scoping" guidance** — when spawning worktree agents, always specify: assigned files, feature inventory for large files, testing tier to run before committing, and post-merge integration notes.

---

## 7. Agent Collaboration Patterns

### Pre-Launch Audit: The 50% Waste Incident (2026-03-16)

The most expensive lesson came from launching 6 worktree agents without verifying what the codebase already contained. Three out of six agents (50%) did completely redundant work:

- **OffscreenCanvas worker agent** — Created a duplicate implementation. `packages/pptx/src/workers/` already had a full worker scaffold with 72 tests from a prior wave. Zero value.
- **Three DOCX feature agents** (images, headers/footers, columns) — All features already existed in mature forms in the DOCX package. Only test files were salvageable.
- The other two agents (font fidelity deep dive, build config check) provided genuine value.

**Root cause:** Memory and documentation captured project milestones ("Wave 3 complete", "DOCX Endstate Waves 1-3 done") but not operational state ("what specific features exist in which files"). QUICKCONTEXT.md listed features as "What's Next" that had already been implemented. The orchestrating agent trusted its own memory and docs over the actual codebase.

**The fix — Pre-Launch Audit protocol:**

Before launching ANY fan-out campaign, the orchestrating agent must:

1. **Grep for existing implementations** in the target packages. If you're planning an agent to "add header/footer rendering," first `grep -rn "HeaderFooter\|header-footer\|renderHeader" packages/docx/src/` to see if it's already there.

2. **Check test counts.** Run `pnpm --filter @opendockit/<pkg> test 2>&1 | tail -3`. If the DOCX package has 684 tests but your memory says 129, substantial work has been done since your last context.

3. **Read actual source directories.** `ls packages/<pkg>/src/<subdir>/` and `wc -l *.ts` tell you what exists. Don't rely on memory or docs alone.

4. **Cross-reference "What's Next" against code.** If QUICKCONTEXT says a feature is "planned" or "ready," verify it's not already implemented.

**Template recommendation:** Add this as a mandatory section in AGENTS.template.md under the fan-out patterns section. The protocol takes 2-3 minutes and prevents hours of wasted agent compute.

**Underlying principle:** Agents trust context they are given. If your memory says "Feature X is planned," the agent believes it. But memory drifts from reality at the speed of code changes. The pre-launch audit forces a reality check: trust your memory, but verify against the filesystem.

### Fan-Out Strategies That Worked

**Wave-based execution** was the most effective pattern. We organized work into waves of 6-9 parallel worktree agents, where each wave's agents had non-overlapping file assignments. Quick wins (small, independent fixes) were merged to main first, giving wave agents a clean base. Each wave had a conflict matrix predicting which agents might touch adjacent code.

The key success factor: **merge quick wins before launching wave agents.** When we skipped this step, cherry-pick conflicts increased dramatically because wave agents were working against a base that was about to change.

**Speculative fanout** — trying multiple approaches in parallel worktrees and keeping the best — was surprisingly effective for debugging. When the root cause of a rendering bug was unclear, launching one agent to fix the parser and another to fix the renderer often produced a solution faster than sequential investigation.

### Conflict Resolution Patterns

Cherry-pick conflicts between worktree agents were expected, not exceptional. The resolution protocol: (1) understand which version is the superset (don't blindly take "theirs" or "ours"), (2) merge manually with understanding of both agents' intent, (3) run T2 (package-level tests) immediately after resolution. The W4/W5 incident taught us that when a fix involves a common pattern across multiple files, all affected files should be assigned to the same agent.

### Context Boundary Management

Worktree agents cannot see each other's work. This is a feature (isolation prevents conflicts) and a liability (agents may duplicate or contradict each other's changes). The mitigation: post-merge integration as an explicit planned step, not an afterthought. The fan-out plan should include a "post-merge wiring" section that lists which cross-file connections need to be made after all worktrees merge.

### When to Use Worktrees vs. Main-Thread Agents

**Use worktrees for:** implementation work that modifies files, any change that might conflict with parallel work, speculative approaches where you want to compare results.

**Use main-thread sub-agents for:** read-only research and exploration, validation (running tests, typechecks, linting), synthesizing information from multiple files.

**Never use worktrees for:** changes that require modifying a single shared file (the merge will conflict), changes that require real-time coordination between agents, changes where the scope is unclear (agents will expand scope into each other's territory).

---

## 8. Trust-but-Verify: The Agent Correction Pattern (2026-03-16)

A second fan-out session (3 parallel worktree agents) revealed a distinct failure mode: **agents complete their task but against wrong assumptions about the codebase state.** This is different from the pre-launch audit problem (launching redundant agents). Here, the agents do genuinely new work, but their output needs correction before merge.

### The Cases

**PDF export test agent** — Tasked with adding test coverage for connectors, tables, and groups in `pdf-slide-renderer.ts`. The agent read the code, confirmed the implementations exist, then wrote 24 tests. But 14 of 24 tests asserted that connectors are "silently skipped (not yet implemented)" and tables render as "placeholder rectangles." Both were fully implemented. The agent wrote tests for a codebase state that didn't match reality — likely confused by comments, TODOs, or its own assumptions about what "new test coverage" means.

**Inter/Aptos font agent** — Tasked with downloading Inter TTFs and regenerating the metrics bundle. Succeeded at the primary task but its `regenerate-metrics.sh` lost the dual-name entries (Carlito, Caladea, Liberation families) that main already had. The worktree branched from an older commit that predated those entries. The agent regenerated the file from its worktree's state, not main's.

**DOCX field evaluation agent** — Clean success. All 39 new tests passed on main without modification. The difference: this agent created new files (`field-evaluator.ts`) rather than modifying existing ones, so there was no state mismatch.

### The Pattern

| Agent task type | Success rate without correction |
|-----------------|-------------------------------|
| Create new files | High (no existing state to misunderstand) |
| Modify existing files | Medium (worktree state may diverge from main) |
| Write tests for existing code | Low (agents assume wrong behavior 50%+ of the time) |

### Prevention

1. **Always review agent test assertions against actual code behavior.** Run the tests on main before committing — if they fail, the agent's assumptions were wrong.
2. **For worktree agents modifying existing files, check the diff against main** (not just the worktree base). `diff main_file worktree_file` catches lost changes.
3. **Agents creating new files are safest** — no existing state to conflict with, easy cherry-pick.
4. **The orchestrating agent must plan post-merge correction time.** Budget ~30% of agent time for fix-up, not 0%.

### Underlying Principle

Worktree agents are like contractors working from blueprints: they build exactly what the blueprints say, but if the blueprints are from last month and the building has changed, the new work won't fit. The orchestrator's job is to be the site foreman who checks each delivery against the actual building, not the old blueprints.

---

## 9. Metrics Bundle as High-Blast-Radius Asset (2026-03-16)

The font metrics bundle (`metrics-bundle.ts`) emerged as the single highest-blast-radius file in the monorepo. Every time it changes, tests break across 3+ packages:

- **core**: decoder test thresholds (family count, face count)
- **render**: metrics-sync test (compares core dist vs render import)
- **pptx**: font census snapshots (Aptos→Inter changes snapshot output)

In this session, adding Inter/Aptos metrics required: rebuild core (`pnpm build`), update 3 test files, update 1 snapshot. Each step was discoverable only by running `pnpm test` and seeing what broke.

### Recommendation

Create a `pnpm fonts:sync-tests` command that:
1. Reads the metrics bundle's actual family/face counts
2. Updates `metrics-decoder.test.ts` thresholds automatically
3. Rebuilds core dist so render package sees the update
4. Runs `--update` on snapshot tests that reference font data

This turns a 4-step manual process (where missing any step breaks CI) into a single command.

### Underlying Principle

When a single file change reliably breaks N>2 other files, the fix-up should be automated. Manual multi-file coordination is exactly the kind of task agents (and humans) forget steps on.

---

## 10. Visual Inspection Over Metrics — The SBS Protocol (2026-03-16)

A critical discovery during DOCX rendering development: **RMSE scores are nearly useless for guiding rendering improvements.** A page with RMSE 0.26 could mean "slight font antialiasing differences across every pixel" or "the entire cover page banner, logo, and title are completely missing." These are radically different problems requiring radically different solutions, but the metric treats them identically.

### What We Found

When we stopped looking at RMSE numbers and started viewing rendered pages side-by-side with Word reference output, three things happened:

1. **Feature gaps became immediately obvious.** Tab stops + dot leaders were completely missing (breaking every TOC page), headers/footers weren't rendering, cover page backgrounds were absent, heading sizes were wrong. None of these showed up clearly in RMSE.

2. **Priority ordering became clear.** Body text layout was actually 80-85% correct — lines breaking at nearly the same points, paragraph spacing approximately right. The remaining 20% was discrete missing features, each of which would produce a visible quality jump when added.

3. **Root cause analysis became trivial.** Seeing that the TOC page has text but no dot leaders immediately tells you "tab stop rendering isn't implemented." Seeing that body text wraps slightly differently tells you "font metrics are close but column width might be slightly off." The visual tells you *what kind of fix* is needed.

### The Protocol

After any rendering change:

1. Generate SBS reports for representative fixtures
2. **Extract and view key pages directly** (cover, TOC, body, tables) using the Read tool on image files
3. Write a structured assessment: what matches, what's missing, root causes, bang-for-buck priorities
4. Use visual gaps — not RMSE deltas — to set the next round of priorities
5. Cast progressively wider nets: fix the worst pages, re-render, inspect the next tier

### Underlying Principle

Quantitative metrics are useful for regression detection (did this change make things worse?), but qualitative visual inspection is essential for **direction setting** (what should I work on next?). A number can tell you the magnitude of error but not its *character*. For rendering work, the character of the error is more important than its magnitude — a missing feature is categorically different from an imprecise feature, and the fix is categorically different too.

### Applicability Beyond Rendering

This principle generalizes: whenever you're evaluating the quality of a complex output, look at the output directly before looking at aggregate metrics. Metrics compress information; that compression discards the structure of errors. Looking directly preserves structure and enables better decisions about what to fix.

---

## 11. Subagent Prompt Templates: Curated Context as Infrastructure

### The Problem

Every time you spawn a subagent, you either: (a) write an inline prompt from scratch (error-prone, non-repeatable, drifts between sessions), or (b) rely on the agent's general knowledge and hope it does the right thing. Both produce inconsistent results. The orchestrating agent wastes tokens re-explaining what a "UX review" or "security audit" means every single time.

### The Solution: Version-Controlled Prompt Templates

Store reusable subagent prompts as `.md` files in an `agents/subagent-prompts/` directory. Each template defines: the task, required parameters, output format, success criteria, and context files to read. A shared `subagent-guidelines.md` defines behavioral contracts (worktree usage, result checkpointing, architectural change detection).

Directory structure:

```
agents/
  subagent-guidelines.md         # shared behavioral contract
  subagent-prompts-index.md      # catalog of available templates
  subagent-prompts/
    ux-review.md                 # UX audit methodology + criteria
    security-surface-scan.md     # crypto/auth/input validation audit
    contract-audit.md            # interface conformance verification
    test-shard-runner.md         # parallel test execution
    doc-drift-detector.md        # doc-vs-code consistency check
```

### Single-Invocation Templates (Not Just Fan-Out)

The critical insight: **templates are just as valuable for single invocations as for parallel fan-out.** When you ask an agent to do a "UX review," you're implicitly relying on the agent to know what a good UX review covers. But the agent's definition of "UX review" may not match yours — it might miss accessibility, or skip mobile responsiveness, or not check against your design system.

A `ux-review.md` template encodes *your* definition of UX review: the specific criteria, the heuristics, the reference documents, the output format. The agent doesn't guess — it follows the template. If the template is wrong, you fix it once and every future review benefits.

**Examples of high-value single-invocation templates:**

| Template | What it encodes |
|----------|----------------|
| `ux-review.md` | Your specific UX criteria, design system references, accessibility standards, output format |
| `security-review.md` | OWASP checklist tailored to your stack, crypto rules, known vulnerability patterns |
| `code-review.md` | Your team's review standards, style guide references, architectural principles to check against |
| `incident-postmortem.md` | Your postmortem format, required sections, blameless language guidelines |
| `api-design-review.md` | REST/gRPC conventions, naming standards, versioning policy, error format |
| `onboarding-brief.md` | How to explain the codebase to a new agent (or human), what to cover, what order |

The pattern: **if you've ever said "no, not like that — here's how we do X," that correction belongs in a template.** Next time, the agent reads the template and gets it right the first time. This is the feedback loop that makes agents learn across sessions.

### Relationship to Claude Skills

Claude Code has a native concept called "skills" (`.claude/skills/<name>/SKILL.md`) that serves a related but distinct purpose:

| | **Skills** | **Subagent Templates** |
|---|---|---|
| **Invocation** | `/skill-name` or auto-triggered | Orchestrator tells agent to read the file |
| **Discovery** | Framework auto-discovers by description | Manual — orchestrator must know to use them |
| **Context** | Injected into main context (or forked) | Always separate subagent context |
| **Parameterization** | `$ARGUMENTS` substitution built in | Convention-based (template defines its params) |
| **Best for** | Extending Claude's repertoire for single tasks | Fan-out, specialized reviews, repeatable workflows |

**They're complementary.** A skill can orchestrate template-driven fan-out: a `/shard-tests` skill that reads a template, computes shard boundaries, and launches N subagents. For single-invocation specialized tasks (UX review, security audit), either mechanism works — skills are more discoverable, templates are more portable across projects and tools.

**Recommendation:** Use skills for workflows the *user* invokes directly (`/review`, `/deploy`, `/audit`). Use subagent templates for workflows the *orchestrating agent* delegates to subagents. The skill is the button; the template is the instruction manual the worker reads.

### Template Design Principles

1. **Declarative, not procedural.** Describe the task, inputs, outputs, and success criteria. Let the agent decide *how* to accomplish it.

2. **Explicit output format.** The orchestrator needs to parse results. Specify JSON schema, markdown structure, or whatever format enables aggregation.

3. **Context files as parameters.** Instead of inlining domain knowledge, point to existing project files: "Read `docs/design-system.md` for our component standards." This prevents the template from drifting from the source of truth.

4. **Success criteria are testable.** "Output file exists and is valid JSON" is testable. "Do a good review" is not.

5. **Include anti-patterns.** If agents consistently make a specific mistake on this task, say so: "Do NOT mock the database in these tests — use the integration test harness."

### Fan-Out Patterns

For parallel execution, templates support four patterns:

| Pattern | Description | Example |
|---------|-------------|---------|
| **Shard** | Same task, different data slices | Test runner: agent N runs tests 500-999 |
| **Map-Reduce** | Parallel map, single reduce | Each agent audits one package, orchestrator merges findings |
| **Speculative** | Same task, different approaches | Two agents fix a bug from different angles, keep the better fix |
| **Progressive** | Wide first pass, narrow second pass | Round 1: coarse analysis of all files. Round 2: deep dive on flagged areas |

### The File System as Message Bus

Coordinating parallel agents doesn't require infrastructure:

- **Inputs:** shared template files, data files, guidelines
- **Outputs:** predictable paths (`agents/results/shard-NN.json`)
- **Persistence:** git commit from worktree branches
- **Merge:** orchestrator reads all shard results, produces merged output

No queues, no databases, no coordination services. Git is the transport layer.

### Behavioral Contracts via Shared Guidelines

A `subagent-guidelines.md` file defines rules all subagents follow:

- **Isolation:** Always use worktree; commit before completing
- **Result format:** Write to specified output path in specified format
- **Architectural findings:** If you discover something that affects contracts, interfaces, or security, document it in `agents/findings/` — don't silently act on it
- **Scope discipline:** Do your assigned work, nothing more. Don't "helpfully" expand scope
- **Quality gates:** Run relevant tests before committing. Follow project coding style.

This prevents the inconsistency problem: without shared guidelines, each subagent makes its own decisions about commit conventions, test running, scope boundaries. With them, every agent behaves predictably.

### Template Recommendations for agent-templates

**Add to AGENTS.template.md:**
- A "Subagent Prompt Templates" section under fan-out patterns
- Reference the `agents/` directory structure
- Include the single-invocation pattern explicitly — templates aren't just for parallelism

**Add to CLAUDE.template.md:**
- Under "Agent Autonomy," note that agents should check for existing templates before doing specialized tasks (reviews, audits, etc.)
- A "Subagent Templates" command section showing how to invoke

---

## Closing Thought

The deepest lesson from OpenDocKit is that agent-driven development requires a **different kind of engineering discipline** than traditional development. The traditional discipline is about code quality — clean abstractions, test coverage, performance. The agent discipline is about **information environment quality** — accurate documentation, stable contracts, explicit context boundaries, and automated verification that the map still matches the territory. The code can be excellent while the information environment decays, and when that happens, agents become powerful tools working from flawed premises. Invest in the information environment at least as much as you invest in the code.
