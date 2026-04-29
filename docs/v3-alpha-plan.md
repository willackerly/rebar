# v3.0.0-alpha Plan

**Branch:** `v3.0.0-alpha` (off `main` @ `b08c98d`)
**Started:** 2026-04-29
**Driver:** Will Ackerly + Claude Opus 4.7 (1M)

## Why v3 (and why alpha)

v2.x landed contracts, the steward, ASK CLI, MCP wiring, federation
discipline. The methodology is real and dogfooded.

What v2.x didn't address: a project can stamp `Tier 2: ADOPTED` at the
top of its README while every contract underneath is a stub with no
product-driven scenarios. The compliance check confirms artifacts
*exist*, not that they're *mature*. **REBAR's badges currently lie by
default.** v3.0.0 introduces a maturity vocabulary so honesty is
declared per-artifact and the badge reflects reality.

That's the headline feature, but v3-alpha bundles five other concepts
that were ripe in `feedback/` and worth taking for a spin together.
Major bump because:

- New required `Status:` field on contracts (breaking for upgraders)
- Compliance score formula changes (some Tier-2 repos demote until
  artifacts are marked `active` or higher)
- New `SessionStart` hook expectation (adopters need to install it)

Alpha because we want real-world failure to refine, not pre-engineer.

## Five clusters (reduced from six 2026-04-29 — see note below)

| # | Cluster | Source |
|---|---------|--------|
| 1 | Maturity tagging + compliance honesty | conversation 2026-04-29 |
| 2 | SessionStart hook for cold-start enforcement | `feedback/2026-04-26-sessionstart-hook-cold-start-enforcement.md` |
| 3 | TEST_FIDELITY.md + UAKS tier + closed-loop demo gate | `feedback/2026-04-22-testing-rigor-six-moments.md`, `2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md`, `2026-04-27-user-at-keyboard-story-tier.md` |
| 4 | agents/FANOUT_PATTERN.md | `feedback/2026-04-28-multi-subagent-fanout-playbook.md` |
| 5 | Contract discipline followups | `feedback/2026-04-24-contract-discipline-and-jtbd-framing.md` |

**Note (2026-04-29):** Cluster 5 from the original draft (cold-start UX
completeness — C1/C3/C4/M10/L3/L4 from usability red team) was already
shipped on main in commits `b09f9fb` + `b8894f6` (2026-04-28) before the
v3-alpha branch was cut. Verified: `bin/ask featurerequest "test"`
works, `rebar new` auto-runs `ask init`, compliance score annotated.
Five-cluster scope below reflects this.

### Cluster 1 — Maturity tagging

Vocabulary (fixed, small, defined in `conventions.md`):

- **stub** — placeholder; structure exists, content is not real
- **draft** — real attempt, not yet reviewed/applied
- **in-progress** — actively being built; expect churn
- **active** — in use; defines current behavior
- **verified** — active + has passing tests/scenarios proving it

No auto-detection. People and agents apply markings honestly. We refine
definitions or add gates only when we see real-world failure.

Surfaces:

- `architecture/CONTRACT-TEMPLATE.md` — `Status:` line in frontmatter
- `practices/*.md` — optional `Status:` for new practice docs
- `scripts/check-compliance.sh` — read Status fields, weight badge:
  - <33% stub-or-draft among contracts → tier as declared
  - 33-66% → annotate "— IN PROGRESS"
  - >66% → demote one tier with reason
- README badge generator updated to show the annotation

### Cluster 2 — SessionStart hook

Concrete deliverables (per feedback REBAR-A through D):

- `templates/project-bootstrap/.claude/settings.json` with
  `SessionStart` hook block
- `scripts/cold-start-checks.sh` — runs the four enforcement scripts in
  sequence + Cluster 1's maturity counts; exits 0 even on failure
  (visible drift, not blocking)
- Output tagged `<rebar-cold-start>...</rebar-cold-start>` so agents
  can recognize it as harness-fact, not prose
- Reframe `CLAUDE.template.md` "Starting a Session" as documentation of
  what the hook does, not instructions to the agent
- Cross-cutting principle in `conventions.md`: "MUST run on event X" →
  hook for X, not prose

### Cluster 3 — TEST_FIDELITY.md + UAKS + closed-loop

`practices/test-fidelity.md` (new). Codifies the fidelity ladder with
machine-checkable declarations:

- **tautology** — proves nothing real; flagged
- **surrogate** — proves something but not the user-facing claim
- **real-flow** — exercises real code paths against real data
- **mutation-proof** — survives mutation testing
- **UAKS** (User-At-Keyboard Story) — hand on keyboard, eyes on screen,
  full deployed surface

UAKS tier definition for user-interactive repos. Gate for "test env
ready" claim: must include UAKS or explicit "no UAKS layer for this
repo."

Closed-loop demo gate (per 2026-04-27 e2e feedback): a repo cannot
claim "demo green" or merge a `demo:` change without browser-hit
evidence captured. Banned-pattern grep extends `check-decay-patterns.sh`
to flag silenced-failure patterns in demo specs.

### Cluster 4 — FANOUT_PATTERN.md

`agents/FANOUT_PATTERN.md` (new). Lifts the dapple-sdk pattern that
shipped 9 zero-conflict fanouts in one session:

- Worktree-per-branch, strict file allowlist, parent-owned post-merge
  sweep
- Dependency-graph reasoning *before* dispatch
- "Verify before relying" mandatory prompt clause (added to
  `agents/subagent-guidelines.md`)
- "When NOT to fan out" decision rules: security-critical paths,
  prompt-longer-than-output, shared mutable state
- Worktree-isolation fallback documented (raw `git worktree add` when
  Agent tool's `isolation: 'worktree'` flag fails)

### Cluster 5 — Contract discipline followups

Cleans up the 2026-04-24 feedback. Most parts already shipped (Why/
Who/Scenarios in CONTRACT-TEMPLATE landed 2026-04-25). Remaining:

- `practices/spike-first-contracts.md` (the filedag DP-A pattern)
- `practices/contract-supersession.md`
- `scripts/check-jtbd-presence.sh` — fail if a contract lacks Why/Who/
  Scenarios sections
- `scripts/check-prefix-uniqueness.sh` — fail on duplicate prefix
  numbers across the registry

## Sequence

1. **Cluster 1** — maturity tagging [establishes vocabulary]
2. **Cluster 2** — SessionStart hook [surfaces Cluster 1]
3. **Cluster 3** — TEST_FIDELITY.md / UAKS / closed-loop gate
4. **Cluster 4** — FANOUT_PATTERN.md
5. **Tag `v3.0.0-alpha`**
6. **Cluster 5** — contract discipline followups (alpha refinement)

Each cluster commits independently. After Cluster 4, the alpha is
tag-able and externally testable. Cluster 5 is post-tag refinement.

## What's deferred (with rationale)

| Item | Source | Why deferred |
|------|--------|--------------|
| Auto-federation experiments | `feedback/2026-04-28-auto-federation-experiment.md` | 7 open questions need maintainer answers (test partner, bot identity, fatigue thresholds). v3.0.x or v3.1 once experiments inform design. |
| Interaction-class fix protocol | `feedback/2026-04-20-interaction-class-false-positive-testing.md` | Doesn't share v3-alpha narrative. Watchlist. |
| Usability RT Cluster E | `feedback/2026-04-28-usability-red-team.md` | Polish (tab completion, log formatting). Post-tag follow-up. |

## Acceptance for tag

`v3.0.0-alpha` ships when:

- Clusters 1, 2, 3, 4 land (Cluster 5 is post-tag refinement)
- `rebar audit` passes 9-10/10 on rebar itself
- Cold-start hook fires + emits the maturity-aware status block
- A clean `rebar new` produces a working `ask architect` state
- CHANGELOG entry + migration notes for upgraders

External adopters who want to take the alpha for a spin: TDFLite,
filedag, fontkit are the lowest-risk first targets (already federated
into the swarm).

## Provenance

- Conversation 2026-04-29 — Will authorized scope: "lets branch to a
  major version bump alpha and fold in the very best concepts we have
  throughout"
- Maturity-tagging proposal: Will, same conversation — "basic
  descriptions of the valid states and we trust agents/people to only
  use the markings that apply. i don't think we need to fuss too much
  on that unless we see in world failure"
- Plan doc filed in-repo per Will's persistence preference (assistant
  memory `feedback-prefer-in-repo-persistence`)
