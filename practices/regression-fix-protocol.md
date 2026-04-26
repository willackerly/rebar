# Regression Fix Protocol — Six Gates

> **Source:** `feedback/processed/2026-04-24-process-gates-G-through-L.md` —
> Dapple SafeSign self-postmortem of a 6-hour session that "fixed" a phantom
> bug while ignoring the real one. The author's closing thesis:
> *"prose-form REBAR guidance does not bind agent behavior; what binds
> behavior is mechanical gates that fail closed."*

This protocol codifies six structural gates for regression-fix work. Three
(G, I, L) ship with mechanical enforcement scripts; the other three (H, J, K)
are doctrine because their mechanism is project-specific.

---

## Why this exists

A user reports a regression. An agent (or human) starts fixing. Over the next
few hours, the session grows: more files touched, more hypotheses tested,
more "while I'm in here" cleanups. By the time the session declares success,
nobody — not the agent, not the user — can answer:

- Which change actually fixed the user's report?
- Was the user's report ever reproduced before fixing began?
- Are there tests that broke from this change but were bypassed?
- Were the test environments controlled for state the user has but CI doesn't
  (trust stores, caches, partitioned storage)?

Each unanswered question is a future incident. The six gates below make
those questions un-skippable.

---

## Gate G — Reproduce-Before-Fix

**Rule:** Before any code change in response to a user-reported failure:

1. Document the exact symptom (user's words, screenshots, logs).
2. Reproduce it on **current prod** (or current main if no deployed equivalent), unchanged.
3. Capture the failure signature (error message, stack trace, screen state).
4. Only then propose fixes.

**What it prevents:** Chasing phantom bugs. The Dapple SafeSign session burned
~5 hours fixing AcroForm/DocMDP/cert-extension issues for a regression whose
root cause was browser storage partitioning — caught only because the user
later signed a PDF on prod and showed the actual verdict. Five minutes of
prod reproduction at session start would have saved the five hours.

**Mechanical enforcement:** [`scripts/check-fix-commit.sh`](../scripts/check-fix-commit.sh)
is a commit-msg hook that refuses `fix:` / `regression:`-prefixed commits
whose body lacks a `Reproduced on:` line referencing a SHA, deploy URL, log
excerpt, or screenshot link.

```
fix(signing): block popup write when iframe partition active

Reproduced on: prod 2026-04-23, recipient cert not in trust store
  → ./scripts/repro/quick-sign-face.sh
  → screenshot: docs/incidents/2026-04-23-quicksign-face.png
```

---

## Gate H — Single-Fix-Isolation

**Rule:** For each proposed fix in a regression session:

1. Apply ONE change (smallest reasonable unit).
2. Test the user-reported symptom — is it gone?
3. If still broken → that fix didn't solve it; revert or queue and try the next hypothesis.
4. If fixed → STOP, ship.

**What it prevents:** Spray-fixes. The most common agent failure mode is
"apply Fix A + Fix B + cert extensions + autonomy hooks all at once, declare
victory, never know which one mattered." When the test then fails on a
different machine, no one can isolate the cause.

**Mechanical enforcement (project-specific):** PR-size limit on `fix:` commits
— e.g., `fix:` must touch ≤2 files unless the commit body explains why
isolation isn't possible. REBAR doesn't ship a universal script for this
because every project's PR-size norms differ. Adopters wire it via
`pre-receive`/`pre-merge` hooks in their CI pipeline.

**Doctrine for agents:** Each `fix:` commit must be paired with a verify step
in the agent's reasoning: *"I applied X; the symptom is now Y; therefore X
was/wasn't the cause."* When this paragraph is missing, the fix is incomplete.

---

## Gate I — Skip-Stress is a Code-Red Event

**Rule:** Test-bypass flags (`--skip-stress`, `--no-verify`, `--force`,
`SKIP_TESTS=1`, etc.) require:

1. Explicit ticket reference per failing test.
2. Justification per ticket (why is bypass acceptable now?).
3. Reviewer sign-off — human, not the change author.
4. Auto-creates a follow-up ticket for each broken test.

**What it prevents:** Shipping regressions in regression-fix PRs. When the
author's own change breaks tests and they bypass the gate to ship, the next
person to touch that codepath inherits broken-baseline confusion plus the
original regression they were trying to fix.

**Mechanical enforcement:** [`scripts/check-bypass-flags.sh`](../scripts/check-bypass-flags.sh)
is a commit-msg hook that refuses commits whose body mentions any of the
bypass flag patterns unless the body also contains a `Bypass tickets:` line
listing the broken-test IDs.

```
build(deploy): skip stress for hotfix promote

Bypass tickets: WEB-1234 (popup-stays-alive contract changed), WEB-1235 (timing)
Justification: hotfix for prod outage; stress tests will be fixed in WEB-1236
within 24h. Ticket WEB-1236 opened.
```

For agents specifically: the agent **cannot self-authorize** a bypass. The
user must explicitly say *"I authorize skipping stress because <reason>"* —
the agent then includes that reason in the commit body. No reason → no
bypass.

---

## Gate J — Test-Diversity for External-Verifier Assertions

**Rule:** When verifying "external tool X accepts our output":

1. Test ≥3 distinct input fixtures spanning the input space dimensions.
2. All must pass for the claim to be "verified."
3. Document which fixtures were used in the test report.

**What it prevents:** Vacuous "verified" claims. The Dapple SafeSign session
tested Adobe acceptance on ONE PDF (an AcroForm one that triggered an
unrelated Adobe-specific bug). Five hours of "why does Adobe reject?"
followed. Testing a non-AcroForm PDF in the first hour would have isolated
AcroForm as the variable.

**Mechanical enforcement (project-specific):** Each project that interacts
with external verifiers (Adobe, Apple Notarization, browser PKI, etc.)
maintains a `tests/fixture-matrix.md` per category enumerating the
dimensions: AcroForm presence, page count, image-only vs text, encryption,
etc. CI rejects single-fixture verification PRs.

**Doctrine for agents:** When asserting "external tool accepts X," answer
*before* the test: which dimensions of the input space matter, and have I
sampled each one? If the diversity matrix is empty, the verification isn't
valid.

---

## Gate K — Trust-State-as-Variable

**Rule:** When testing systems involving local trust stores, caches, or
session-derived state (Adobe trust list, browser certificate store, OS
keychain, IndexedDB):

1. Run on a fresh state (no prior trust/cache).
2. AND run on a state mirroring a typical user (some prior trust/cache).
3. Compare verdicts. Difference reveals the variable being conflated.

**What it prevents:** Conflating "user has prior trust" with "tool accepts
this signature structure." Will's Adobe gave sigStatus=4 (green) because his
cert was in his Adobe trust store from prior interactions; the autonomous
verifier got sigStatus=0/2 (red) because each run used a fresh ephemeral
cert. Same code, opposite verdicts. Took the user signing in prod to
disambiguate.

**Mechanical enforcement (project-specific):** Verifier scripts report trust
state as a SEPARATE axis from the validity verdict — e.g., `verify --strict
--no-trust-cache`. Test framework refuses "verified" claims without isolating
both axes.

**Doctrine for agents:** When interpreting a "fail" verdict from an external
tool, the FIRST question is *"is this a trust-state issue or a structural
issue?"* Not the second.

---

## Gate L — Fix-Your-Own-Test-Drift

**Rule:** If your code change breaks N existing tests:

- The PR is INCOMPLETE until those N tests are updated to reflect the new
  (intended) contract OR the change is reverted.
- "Test contract drift" is not a free pass.
- "Future engineer will fix the tests" is not acceptable.

**What it prevents:** Test debt accumulation in regression-fix PRs. The
Dapple SafeSign session had two dapple tests broken by Fix B's contract
change (popup-stays-alive). The author bypassed via `--skip-stress` and
deferred. The actual fix took 30 minutes once they sat down for it — which
should have been the same session as Fix B.

**Mechanical enforcement (project-specific):** Pre-merge hook — if `git
diff` touches a function/symbol whose name appears in any failing test from
CI, refuse merge until those tests are updated. REBAR can't ship this
universally because the test-runner is project-specific.

**Doctrine for agents:** When a test fails after your change, the FIRST
hypothesis is *"I broke the test's contract assumption"* — investigate before
bypassing. If the contract change is intended, the test is part of the same
PR.

---

## Cross-cutting observation

**Gates G, H, J, K all share a structural property: they require explicit,
named control of the environment before claiming a verdict.**

The agent failure mode is to skip environmental setup ("I'll just sign a PDF
and see") and then misinterpret the result because the environment isn't
what was assumed (cert is trusted, fixture has an AcroForm, the storage is
partitioned, the state was seeded by a prior test). Every "miss the variable"
failure could be summarized as: **the agent did not enumerate the dimensions
of the test before running it.**

A unified self-check before any "verify" action:

1. What dimensions of the input space matter? (cert trust, PDF type,
   browser cache, etc.)
2. Have I controlled or sampled each one?
3. What's the expected outcome under each combination?

If any of those three is unanswered, the verify action isn't valid yet.

---

## Mechanical enforcement summary

| Gate | Universal mechanism | Project-specific mechanism |
|------|---------------------|----------------------------|
| G — reproduce-before-fix | `scripts/check-fix-commit.sh` (commit-msg hook) | PR template field |
| H — single-fix-isolation | doctrine in AGENTS.md | PR-size limit |
| I — skip-stress is code-red | `scripts/check-bypass-flags.sh` (commit-msg hook) | wrapper around bypass scripts requiring env var |
| J — test-diversity | doctrine in AGENTS.md | `tests/fixture-matrix.md` + CI gate |
| K — trust-state-as-variable | doctrine in AGENTS.md | verifier reports trust state as separate axis |
| L — fix-your-own-test-drift | doctrine in AGENTS.md | pre-merge hook on test-name overlap |

The two scripts ship in REBAR's `/scripts/` and are wired into
`ci-check.sh`. Adopters get them automatically via `cp -r
templates/project-bootstrap/*`. The doctrine entries live in
`AGENTS.template.md` (slim) and `templates/component-templates/AGENTS.template.md`
(full).

---

## When to apply the protocol

- **Always:** any commit prefixed `fix:` or `regression:` invokes Gates G + L.
- **Always:** any commit body mentioning bypass flags (`--skip-*`, `--no-verify`, `--force`, `SKIP_TESTS=1`) invokes Gate I.
- **Recommended for security-critical changes:** Gates H, J, K — verifying
  external-tool acceptance, trust-store-dependent behavior, or assertion-chain
  authenticity.
- **Optional for refactors:** Gates G/H/L still apply if the refactor is
  motivated by a regression report. Pure cleanup PRs are exempt.

The opening question of every regression-fix session should be:
*"Which gates apply, and have I satisfied them?"*
