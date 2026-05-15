# Feedback: Multi-Subagent Fanout Playbook (Worktree + Strict-Allowlist Pattern)

**Date:** 2026-04-28
**Source:** dapple-sdk — two consecutive multi-subagent cycles totaling 9 fanned-out branches (Wave 1 = 5, Wave 2 Phase 1+2 = 4) + 3 sequential Phase 3 specs. All merged into main with **zero conflict resolution**.
**Type:** missing-feature / improvement
**Status:** proposed
**Template impact:** `AGENTS.template.md` (subagent guidelines section), `agents/subagent-guidelines.md`, possible new `FANOUT_PATTERN.md` artifact in REBAR profiles. Cross-cuts with `2026-04-22-testing-rigor-six-moments.md` and `2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md`.
**From:** Claude Opus 4.7 (1M), dapple-sdk, 2026-04-28. Pattern emerged organically across two cycles in one ~16hr session; worth codifying so it's not re-derived per project.

**Related prior feedback:**
- `2026-04-22-testing-rigor-six-moments.md` — testing rigor across multiple projects.
- `2026-04-27-user-at-keyboard-story-tier.md` — UAKS tier surfaces real production bugs that mock-tier misses. Today's dapple-sdk session observed the same pattern from a different angle: MAX-FIDELITY e2e surfaced 5 real production bugs that mock-tier surface-api unit tests had missed. The two reports are mutually reinforcing.
- `2026-04-26-sessionstart-hook-cold-start-enforcement.md` — REBAR's enforcement should be runtime-mandatory, not prose. Today's fanout pattern leaned on the SessionStart hook for state continuity.

---

## What Happened

dapple-sdk ran two multi-subagent fanouts in one session:

**Cycle 1 (Wave 1, ~3hr):** 5 parallel worktree subagents covering: bugfixes (SDK + Surface lockout + e2e), cold-start registry swap, Surface-internal contracts (5 new contracts), popup-fallback e2e, and biometric-helper-data API endpoints. **All 5 merged into main with zero conflicts.**

**Cycle 2 (Wave 2 Phase 1+2, ~2hr):** 4 parallel subagents covering: P1 1.2 contract bump + SDK plumbing, dev-rig orchestration extension, persistent-context Playwright fixture lift, and (in parallel by the parent) Surface-side BCH recovery on the sign path. **All 4 merged with zero conflicts.**

Each subagent:
- Worked in its own `git worktree add` directory
- Got a strict file allowlist as part of its prompt
- Committed to its own feature branch
- Did not push, did not switch branches, did not touch main worktree

Parent (orchestrator):
- Created the worktrees + branches
- Wrote prompts with explicit dependency-graph reasoning
- Merged each branch sequentially with `--no-ff` after all completed
- Did the post-merge sweep on shared docs (CHANGELOG, QUICKCONTEXT, METRICS, README) in a single commit

**Three subagents independently caught factual errors in my prompts** and made the right call to deviate (e.g., `compute-registry.sh` was hallucinated; BCH inner-code was unshipped speculation; Phase 2 RPCs don't have a fast-path-vs-popup decision). This means the pattern only works if subagents are explicitly invited to verify-then-flag rather than blindly follow.

---

## What Was Expected (vs. found)

REBAR templates today have `agents/subagent-guidelines.md` and an "Agent Coordination" section in `AGENTS.template.md`, but neither codifies a concrete fanout playbook. New project agents have to re-derive this pattern (or wing it and merge-conflict their way through).

Concretely missing:

1. **No "fanout pattern" artifact** documenting strict-allowlist + worktree + post-merge-sweep as the recommended shape.
2. **No counter-pattern** for "when NOT to fan out" — e.g., security-critical code paths warrant single-thread care; small tasks (<10 LOC) don't justify a 30k-token subagent prompt.
3. **No prompt template** for "verify source-of-truth before relying on this brief" — subagents that follow wrong prompts produce confidently-wrong work.
4. **No worktree-isolation fallback documented** — Claude Code's `Agent` tool's `isolation: "worktree"` flag fails when the harness doesn't detect the parent as a git workspace, even though raw `git worktree add` works fine. Today's pattern: parent creates worktrees via raw shell, then dispatches subagents with `cwd` set to the worktree path. This was figured out the hard way.

---

## Suggestion

### REBAR-A: Codify the fanout playbook as `agents/FANOUT_PATTERN.md`

Concrete artifact. Template ships with the playbook from this report (worktree-per-branch, strict allowlist, parent-owned post-merge sweep, dependency graph BEFORE dispatch, etc.). Adopting projects copy + customize for their package layout.

### REBAR-B: Add a "verify before relying" prompt clause to `agents/subagent-guidelines.md`

Mandatory line in any fanout prompt:

> "If you find that any specific claim in this brief doesn't match what's in source (file paths, line numbers, function names, config values), STOP, document the discrepancy, and choose the higher-fidelity path. Don't sugarcoat the parent's recall as fact — verify before relying."

This single sentence enabled three subagents on dapple-sdk to catch real prompt errors and produce correct work. Without it, they would have built on hallucinated foundations.

### REBAR-C: Codify "when NOT to fan out" decision rules

Add to `agents/subagent-guidelines.md` a section like:

```markdown
## Fanout — when not

- **Security-critical code paths.** Crypto touchpoints, key derivation, authn/authz boundaries. Parent does these with full attention (in a worktree if isolating from other parallel work).
- **Tasks where the prompt is longer than the expected output.** A 5-minute manual edit becomes a 30k-token subagent prompt with the briefing overhead. Just do it yourself.
- **Tasks that depend on the running rig and can't isolate state.** If two specs hit the same Postgres rows or browser cookies, sequence them.
```

### REBAR-D: Document the worktree-isolation fallback explicitly

`agents/subagent-guidelines.md` adds:

> "Claude Code's `Agent` tool supports an `isolation: 'worktree'` flag, but it fails when the harness doesn't detect the parent as a git workspace (observed on dapple-sdk despite git itself working). Workaround: the parent creates worktrees via raw `git worktree add ../proj-wt-<name> -b feature/<name> main`, then dispatches subagents with their `cwd` set to the worktree path. Each subagent commits to its own branch; parent merges sequentially after all complete."

### REBAR-E: Cross-link with the max-fidelity testing report

dapple-sdk's session also observed: **max-fidelity e2e surfaced 5 real production bugs that mock-tier surface-api unit tests had missed.** This is the same disease as the UAKS report from 2026-04-27 but at a different layer. The fanout playbook works particularly well for spinning up high-fidelity tests in parallel — each spec subagent runs against a shared rig with isolated test data.

Practical rule: when fanning out test-writing subagents, brief them with "MAX FIDELITY: do not gate the spec on `test.skip(!BACKEND_REACHABLE)` or any similar bailout. If the rig isn't running, fail loud." This was load-bearing on dapple-sdk — the cross-device-recovery spec caught the surface-api JSON limit + CORS bugs precisely BECAUSE it didn't skip.

---

## Why This Matters Beyond One Project

The fanout pattern is repeatedly useful — any non-trivial REBAR project will hit a moment where 4-6 chunks of work could land in parallel with strict file isolation. Today's dapple-sdk session shipped what would have been ~12-16 hours of sequential work in ~6-8 hours of orchestrated parallel work, with the same correctness bar. Those gains compound across projects.

The pattern also *encourages* better dependency-graph reasoning. Forcing yourself to write down "WT-A owns these files, WT-B owns these, no overlap" surfaces accidental couplings that you'd otherwise discover at merge time.

The "subagents catch prompt errors" observation is the *insurance*. Without it, fanout multiplies the cost of any factual error in the parent's planning. With it, fanout becomes self-correcting.

Together: codify the pattern, codify the "verify before relying" instruction, codify when NOT to fan out, and document the worktree-isolation fallback. Four small additions to `agents/subagent-guidelines.md` would save every future REBAR project the re-derivation cost.
