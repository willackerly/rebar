# Feedback: Cold-Start Health Checks Should Be a `SessionStart` Hook, Not Prose in CLAUDE.md

**Date:** 2026-04-26
**Source:** dapple-sdk — cold-start review session. CLAUDE.md "Starting a Session" section instructs the agent to run a battery of `scripts/check-*.sh` (contract refs, todos, freshness, ground-truth) on every session start.
**Type:** anti-pattern / missing-feature
**Status:** proposed
**Template impact:** `CLAUDE.template.md` (Starting a Session section), `AGENTS.template.md`, REBAR profile templates that ship `scripts/check-*.sh`. Probable new artifact: `settings.template.json` with a `SessionStart` hook block.
**From:** Claude Opus 4.7 (1M), dapple-sdk, 2026-04-26. User-driven discovery: Will asked the agent to verify the cold-start quad against repo state. The agent caught real staleness (toy-app→hello-human rename residue across 9+ files, MIGRATION.md sync commands pointing at pre-monorepo paths, "this session" markers in TODO.md that no longer resolve) — but skipped the four enforcement scripts CLAUDE.md explicitly tells the agent to run on every cold start. When asked why, the agent admitted it had narrowed the task to "read the docs and check assumptions" and treated the script list as advisory rather than load-bearing.

**Related prior feedback (same family):**
- `2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md` — the dual: tests claimed green that didn't exercise the user-facing surface. This report's dual: enforcement scripts that exist but didn't run because the runner (the agent) interpreted the contract as text.
- `2026-04-22-testing-rigor-six-moments.md` §"intent-vs-actual divergence" — same disease: an instruction that *describes* a check is not the same as a system that *runs* one.

The pattern: REBAR ships an enforcement script, CLAUDE.md says "run it on cold start," and then it doesn't run because it's a sentence in a document, not a hook. **An instruction the harness can execute is qualitatively stronger than an instruction the agent has to remember.**

---

## What Happened

dapple-sdk's `CLAUDE.md` "Starting a Session" section reads:

```bash
git status
git worktree list
pnpm typecheck
pnpm test
scripts/check-contract-refs.sh
```

Plus the four health scripts referenced elsewhere in the file (`check-todos.sh`, `check-freshness.sh`, `check-ground-truth.sh`, `ci-check.sh`).

The agent (me) was asked to audit the cold-start quad. It read the four docs, verified test counts (140/140 across two packages, ran the suites), cross-referenced QUICKCONTEXT against `git log`, and surfaced ~25 staleness items spanning 9+ files. **It did not run any of the `scripts/check-*.sh`.** Reasons given when asked:

1. "I narrowed the task." The user said "look at the cold start quad," and the agent treated the quad (4 docs) as the scope, not the full cold-start protocol that the docs describe.
2. The script list reads as advisory in the prose: it's bulleted under "Check project health," indistinguishable from `git status` and `pnpm typecheck`, both of which the agent ran. The discriminator wasn't visible.
3. There's no system-side feedback when the scripts don't run. The agent doesn't see a "you skipped X" reminder; the user has to notice and ask.

The user's reaction — "why didn't you run those, how do we enforce that?" — is the correct one. Prose instructions to an agent are advisory by construction. Hooks are not.

---

## What Was Expected

REBAR's enforcement scripts (`check-contract-refs`, `check-todos`, `check-freshness`, `check-ground-truth`) exist precisely to detect drift. Their failure mode when *not* run is the failure mode they were built to prevent — staleness goes undetected, the agent works from a stale mental model, and downstream changes compound the drift. In this session the staleness was caught only because the user asked for an explicit audit. In the normal case (start a session, fix a bug, commit) it would have shipped with the agent operating against incorrect assumptions.

The cold-start health check should run **before the agent's first user-facing turn**, deterministically, with output landing in the agent's context the same way `<system-reminder>` blocks do. This is what `SessionStart` hooks are for in Claude Code's settings.json.

---

## Suggestion

### REBAR-A: Ship a `SessionStart` hook block in the REBAR profile templates

Add to `CLAUDE.template.md` (or a new `settings.template.json` co-located with the other profile artifacts) a hook configuration like:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "command": "scripts/cold-start-checks.sh",
        "description": "REBAR cold-start health check — runs check-contract-refs, check-todos, check-freshness, check-ground-truth. Output is injected into the agent's first turn."
      }
    ]
  }
}
```

And a thin `scripts/cold-start-checks.sh` that invokes the four enforcement scripts in sequence, prints a single summary block, and exits 0 even on failures (the agent should *see* drift, not be blocked by it — blocking on session start is hostile to interactive work).

### REBAR-B: Reframe CLAUDE.md "Starting a Session" as documentation of what the hook does, not as instructions to the agent

Today the prose tells the agent what to run. The agent skips it. After REBAR-A, the prose should say "the SessionStart hook runs the following on every cold start; the output appears at the top of your first turn. If the output is missing, the hook isn't installed — install it via `scripts/install-hooks.sh`." This eliminates the failure mode where the agent forgets, and adds a self-check (missing output = missing hook).

### REBAR-C: Generalize the principle in `DESIGN.md` or `CONVENTIONS.md`

Cross-cutting rule: **"Any 'MUST run on event X' instruction in a REBAR-managed CLAUDE.md belongs in a hook for event X, not in prose."** Concrete events:

| Event | Hook | Use case |
|---|---|---|
| `SessionStart` | health checks, freshness audit, drift detection | the case in this report |
| `PreToolUse` (Bash) | bypass-flag detection (already exists per INVENTORY W3-3) | enforce on attempt, not retroactively |
| `commit-msg` | claim/fidelity headers, fix-commit reproducer line (W3-2) | already shipped |
| `Stop` / `SubagentStop` | TODO/contract-ref re-check before declaring done | catch the "I forgot to update the contract" failure mode |
| `UserPromptSubmit` | inject task-relevant context (e.g. CONTRACT-GAPS.md, TODO.md priorities) | reduce the "agent re-reads the same files every turn" tax |

The pattern makes REBAR's enforcement durable across agent identities, model versions, and prompt-engineering drift. The current model — write a sentence, hope the agent reads it the same way next time — is exactly the brittleness the testing-rigor feedback already identified.

### REBAR-D: Make the hook output structurally distinct from agent-generated content

The hook output should be tagged (e.g. `<rebar-cold-start>...</rebar-cold-start>`) so the agent can recognize "this is ground truth from the harness, not prose I have to interpret." Untagged output gets melted into the rest of the system prompt and loses its load-bearing status.

---

## Why This Matters Beyond One Session

The dapple-sdk session caught ~25 doc staleness items by reading carefully. The four scripts probably catch a strict superset (since they verify against the registry, the contract refs, the todo conventions, and the freshness markers — content the agent's manual review skipped). The script suite is the higher-fidelity check; the agent's prose audit is the lower-fidelity backup. The current REBAR architecture inverts that: the high-fidelity check is opt-in (via agent memory + prose discipline), the low-fidelity check is the default. **A SessionStart hook flips the polarity.**

This is the same shape of fix as commit-msg hooks for `fix:` commits (W3-2 in INVENTORY): turn an "agent should remember" rule into a "harness enforces" rule. Two votes, two repos, same lesson.
