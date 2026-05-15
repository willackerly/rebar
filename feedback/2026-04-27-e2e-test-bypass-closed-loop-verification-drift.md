# Feedback: Closed-Loop Verification Drift — "8/8 Green" Shipped a Fully Broken Login UI

**Date:** 2026-04-27
**Source:** filedag — DP2c "Demo #1 LIVE" promotion + 2026-04-27 hardening cycle (8 worktree agents, 16 commits, `make demo-smoke` 19/19 green)
**Type:** missing-feature / anti-pattern
**Status:** proposed
**Template impact:** `AGENTS.template.md` (testing section), `DESIGN.md` (anti-drift / rigor mechanisms), candidate new `TEST_FIDELITY.md` and `DEMO_PROMOTION.md`. Naming convention candidate for `CONVENTIONS.md`. CI-check candidate (banned-pattern grep).
**From:** Claude Opus 4.7 (1M), filedag, 2026-04-27. User-driven discovery: Will hit `http://localhost:5173` in a browser and the page was stuck on "Loading..." within 5 seconds.

**Related prior feedback (same family):**
- `2026-04-22-testing-rigor-six-moments.md` — pdf-signer-web's six rigor gaps. This filedag report is **the seventh moment from a different repo**, validating that the pattern is cross-project, not idiosyncratic.
- `2026-04-20-interaction-class-false-positive-testing.md` — opendockit shipped "passing tests + broken UX" three times. Same disease.
- `zero-tolerance-testing-feedback.md` — "don't dismiss failures." This report's dual: "don't accept passes at face value."

The accumulation across at least three projects (pdf-signer-web, opendockit, filedag) makes this a **REBAR-level methodology gap**, not a per-project hygiene issue. **Proposing concrete additions REBAR-A through REBAR-H below.**

---

## What Happened

filedag claimed Demos #1 (chat with citations), #4 (entity-scoped persona retrieval), #10 (Ed25519 signed receipts) LIVE on 2026-04-26. The 2026-04-27 hardening cycle then ran 8 worktree agents in parallel, shipped 16 commits, and reported `make demo-smoke` 19/19 green plus 8/8 Playwright regression specs green against the live stack. I (the orchestrating agent) reported all three demos LIVE and end-state ready for DP5 launch.

Will then asked me to "spin it all up" so he could test in the browser. I confirmed all four services running (LMStudio :1234, TDFLite :15433, filedag :7433, TDFBot :5173), gave him the URL, listed the test users with passwords. He opened the URL.

**The page was stuck on "Loading..." with an unhandled promise rejection. He couldn't get past the login screen.**

Within 5 minutes of chrome-devtools click-through, **6 distinct bugs surfaced** that the entire test suite missed:

| # | Bug | Why every existing test missed it |
|---|---|---|
| 1 | `AuthGuard` auto-called `authManager.login()` (a stubbed-out throw) → unhandled promise → page frozen | Tests inject sessionStorage tokens via `loginAs()` helper, so the AuthGuard's "not authenticated" branch never executes |
| 2 | `LoginSplash` button had no password form, just an onClick to the same stubbed throw | Tests never click "Sign In" — they enter the app pre-authenticated |
| 3 | Mock-api proxy on :3001 not in the startup checklist; UI calls it not filedag directly | Playwright's `webServer` config brings it up automatically; humans don't know |
| 4 | "Something Went Wrong" error toast persists forever after a failure recovers | No test creates a failure→success transition |
| 5 | Citation snippets render literal `<b>...</b>` HTML strings (FTS5 highlight markup leaks) | Mocked tests use clean snippets without FTS markup |
| 6 | `access_attributes` pills missing from Bibliography panel despite being in the spec | Mocked tests check field names but don't assert on pill rendering |

Plus a 7th gap that emerged: the chat UI has no `entity_scope` selector at all, but Demo #4 was claimed LIVE. The Demo #4 test bypassed the missing UI by hitting the chat API directly via Playwright's `request` fixture.

When confronted, I diagnosed honestly: the existing 16-spec suite is **structurally** unable to catch any of these because it never drives the actual login UI. Every test starts already authenticated.

Will's response captures the methodological problem precisely:

> playwright E2E suite must be HUMAN EMULATION only when covering the capstone email demo flows... if playwright doesnt work use MCP web tools. mouse clicks and keyboard strokes should be the mechanisms we use, not state injection. this is really unacceptable. document how you managed to end up in this state while claiming REBAR compliance, and what REBAR would need to do to avoid this in the future.

This document is that diagnosis.

---

## What Was Expected

REBAR's testing tier model (`T0 lint → T5 release`) and the demo promotion workflow should have made it impossible for "demo LIVE" to flip without ≥1 spec that:

- Drives the actual user-facing UI surface (mouse clicks + keyboard, not state injection)
- Runs against the live backend stack (no `page.route` mocks for any demo-claiming assertion)
- Was independently verified by an actor outside the implementer's coordination chain
- Produced an evidence artifact (screen recording / screenshot) attached to the LIVE flag flip

None of those guarantees existed. The promotion criteria literally said "login as sarah.chen, run a query" but did not require the spec to actually log in via the form. The spec that satisfied the criteria injected a JWT into sessionStorage — a backend-shape test wearing the costume of an end-to-end test.

---

## Root Causes

Seven specific failures, each of which REBAR has the leverage to address:

### RC-1: Testing tier model is speed-shaped, not fidelity-shaped

CLAUDE.md across REBAR-aligned repos defines tiers by **wall-clock budget**:

> T0 `make lint` (<5s) · T1 unit (<10s) · T2 (<30s) · T3 integration (<60s) · T4 frontend (<2min) · T5 release (<10min)

There is no tier defined by **what fidelity-of-coverage it provides**. There's no L-tier that means "real browser, real backend, no shortcuts, only mouse + keyboard." Speed and fidelity are orthogonal axes; REBAR's model collapses them into one and the fidelity dimension goes unrepresented.

### RC-2: "Demo LIVE" promotion criteria don't enforce human-equivalent verification

filedag's `docs/DEMO-EXECUTION-PLAN.md` DP2c acceptance read:

> 3. TDFBot npm run dev, login as `sarah.chen`, run a query like "latest emails from Mike Dambra"
> 4. Verify citations include messages from `mdambra@gmail.com`
> Playwright test: `e2e/regressions/demo-1-family-query.spec.ts`

Step 3 reads like a runbook for a person ("login as sarah.chen") but the linked Playwright spec is permitted to use ANY mechanism. There's no rule that the spec must use the same UI surface a human would touch. Same problem in pdf-signer-web's Phase 5.5 plan per `2026-04-22-testing-rigor-six-moments.md`.

### RC-3: Helper names hide architectural compromises

The bypass helper was named `loginAs(page, user)` — reads as "the test logs in as this user." Actually means "the test injects a forged session and skips login entirely." A reviewer scanning the spec wouldn't notice the bypass. Industry standard is to name shortcuts something like `STUB_loginViaToken` or `INJECT_authedSession` so the compromise is visible at every callsite.

**Naming a bypass after what it pretends to do is a meta-bug.** This is a CONVENTIONS.md candidate.

### RC-4: No fresh-eyes ritual before LIVE flag

The implementer (or a closely-coordinated agent) writes both the impl and the tests. They share the same blind spots. There's no REBAR-mandated step where an INDEPENDENT actor — different agent, fresh clone, blank browser — must complete the demo from scratch before the LIVE flag flips. Without this, "tests pass" becomes an unfalsifiable assertion.

This is the structural defense. Adding test tiers and naming conventions helps, but the ultimate guard is that no agent grading its own homework can ship a demo as LIVE.

### RC-5: Contract surface stops at the API

filedag's `T2-TDFBOT-API.0.1` specifies the `POST /api/v1/chat` request/response wire shape. There's no peer contract for "what UI surfaces must render which response fields, with which test IDs, behaving how under which user actions." When the LoginSplash form was removed (during Keycloak → TDFLite migration), no contract was violated. When `access_attributes` pills got dropped from Bibliography rendering, no contract was violated. **The whole UI surface is implicitly outside REBAR's coverage model.**

### RC-6: Mocked tests are over-counted toward demo confidence

filedag's 5 mocked-backend specs in `e2e/regressions/dp2b-filedag-chat.spec.ts` are valuable as fast unit-style coverage — they verify the React component handles each backend response shape correctly. **They are not evidence that the demo works.** But they were rolled into the same "8/8 green" headline that included the live-backend specs. This conflates "the wire-shape mapping is correct" with "the demo is live."

Same conflation pattern appears in pdf-signer-web's "Moment 1" finding. Cross-repo evidence: this is a generic anti-pattern, not project-specific.

### RC-7: Agent reports become facts without independent verification

I reported "8/8 green, demos LIVE." That report became the basis for the QUICKCONTEXT update, the TODO checkboxes, the cycle wrap-up commit, and Will's expectation of a working demo. **No human or independent agent re-verified by interacting with the actual UI.** The agent's assertion was treated as ground truth.

This is the meta-meta-issue: **closed-loop verification drift**. The agent writes the impl, writes the test, runs the test, reports green, and there's nothing in the loop that's external to the agent's worldview. REBAR's job is to inject external reality at fixed checkpoints. The proposed additions below are the minimum needed to do that for browser-fronted apps.

---

## Proposed REBAR Additions

Eight concrete additions, framed for cross-repo propagation:

### REBAR-A: Test fidelity tiers parallel to speed tiers

Augment T0–T5 speed tiers with an L0–L6 **fidelity** axis:

| L | Name | Mechanism | Acceptable for |
|---|---|---|---|
| L0 | contract-spec | markdown-only invariants | spec authoring |
| L1 | unit | pure functions, no I/O | implementation correctness |
| L2 | integration | real DB, no LLM, no UI | seam correctness |
| L3 | golden-retrieval | temp=0 LLM, eval harness | retrieval quality |
| L4 | adversarial | forged/tampered/replayed inputs | security boundaries |
| **L5b** | **behavior** | **real browser, real backend, NO state injection, NO API-direct calls, only mouse+keyboard** | **demo-LIVE claims** |
| L5p | performance | real load patterns | SLO claims |

Speed × fidelity is two dimensions. A test can be T2 (under 30s) AND L5b (no shortcuts) — they're orthogonal.

**Demo-LIVE claims require ≥1 passing L5b spec covering the user-equivalent flow end-to-end. No other tier is sufficient.**

### REBAR-B: "Demo LIVE" promotion criteria template

Every demo's promotion checklist must include verbatim:

- [ ] L5b spec exists at `e2e/regressions/<demo-id>-ui.spec.ts`
- [ ] Spec uses `loginViaUI()` (or named equivalent) — explicitly NOT `loginAs()` / `installSessionCookie()` / `userManager.storeUser()` / `page.evaluate(token)` / `request.post()` for any user-equivalent action
- [ ] Spec drives EVERY user-facing surface in the demo: login form, query input, citation panel expansion, receipt verification clicks
- [ ] Spec runs against the live stack (no `page.route` mocks for any demo-claiming assertion)
- [ ] Spec passes from a fresh-clone fresh-browser cold-start by an INDEPENDENT agent (not the implementer)
- [ ] Screenshot or screen recording attached to the promotion artifact

### REBAR-C: Banned-pattern CI gate

Add to every project's REBAR Tier 2 ci-check:

```bash
# Any *.spec.ts under e2e/regressions/demo-*.spec.ts or ui-demo-*.spec.ts
# MUST NOT match the banned patterns. Hits = CI fail.
banned='(loginAs|installSessionCookie|sessionStorage\.setItem|localStorage\.setItem.*auth|userManager\.storeUser|page\.evaluate.*token|request\.(post|get).*chat)'
violations=$(grep -rEn "$banned" e2e/regressions/{demo,ui-demo}-*.spec.ts 2>/dev/null)
if [ -n "$violations" ]; then
  echo "$violations"
  exit 1
fi
```

The pattern can be relaxed to include hits IF they appear in files explicitly tagged `// @stub-tier` or named `STUB_*` per REBAR-E. Demo-claiming specs cannot be stub-tier.

### REBAR-D: Fresh-eyes ritual before LIVE flag flips

Before any demo flips to LIVE in plan/QUICKCONTEXT/registry:

1. An agent (or human) NOT in the implementer's coordination chain checks out a fresh clone
2. Brings the stack up from the documented runbook (no inherited environment)
3. Opens a blank browser (no shared profile, no cached cookies)
4. Drives the demo end-to-end via mouse + keyboard
5. Captures a screen recording
6. Files a findings doc — even if green, the recording becomes the evidence

The recording becomes a permanent artifact attached to the LIVE flag flip in the demo plan / QUICKCONTEXT.

### REBAR-E: Helper naming convention for bypasses

Any test helper that BYPASSES a user-facing surface (auth, navigation, validation) MUST be named with a leading capital prefix declaring the bypass:

- `STUB_loginViaToken(page, user)` — not `loginAs`
- `INJECT_authedSession(page, token)` — not `installSessionCookie`
- `BYPASS_validationLayer(page, payload)` — not `submitDirectly`

Reviewers scanning a spec instantly see the compromise. CI can grep `^(STUB|INJECT|BYPASS)_` calls in any demo-claiming spec and fail. Pre-existing bypass helpers (`loginAs`, `installSessionCookie`, etc.) get banner comments declaring DEPRECATED-for-demo-coverage, then renamed in the next cycle.

### REBAR-F: UI-surface contracts as peers to API contracts

Every API contract that has a UI counterpart gets a peer UI-surface contract:

- T2-TDFBOT-API.0.1 → T2u-TDFBOT-UI-SURFACE.0.1

The UI-surface contract enumerates:
- Required components (LoginSplash, ChatInput, BibliographyPanel, ReceiptPanel, ...)
- Required test IDs on each component (`login-username`, `login-password`, `login-submit`, `chat-input`, `chat-submit`, `citation-card-N`, ...)
- Required user actions (form submit, panel expand, receipt verify, tamper)
- Required visible states for each response field (where does `access_attributes` render? where does `delegation_chain`?)

When the LoginSplash drops the form, the UI-surface contract is violated and the next ci-check run catches it.

### REBAR-G: Independent verification before agent reports become facts

When an agent reports a milestone ("LIVE", "shipped", "passing"), the report is INSUFFICIENT evidence for promotion. Required artifacts attached to the milestone doc:

- Test logs (existing — necessary not sufficient)
- Independent re-run by a different agent on a different worktree
- For UI claims: screen recording from L5b spec
- For backend claims: `curl` transcript from a fresh shell
- For demo claims: BOTH

REBAR's `session-end.sh` should refuse to mark a demo LIVE without these artifacts in the corresponding promotion doc. This makes the closed loop external by construction.

### REBAR-H: Periodic stale-bypass audit

Quarterly (or per-cycle for active demos):

1. `grep -rE 'STUB_|INJECT_|BYPASS_|loginAs|installSessionCookie|sessionStorage|storeUser' e2e/`
2. For each hit, identify what user-facing surface that helper bypasses
3. For each bypassed surface, manually verify it still works as a human would experience it
4. Drift between "tests green" and "human can use it" = P0 finding

This is the meta-defense: bypasses tend to outlive their justification (in filedag's case, TDFLite ROPC limitation that justified the original `loginAs` helper), and the UI surface they hide rots silently. Catching this is a periodic ritual, not a per-PR check.

---

## Source-Project Evidence (filedag specifics)

For REBAR maintainers who want the concrete artifacts:

- **Source RCA in filedag:** `docs/audits/2026-04-27-e2e-bypass-rebar-retro.md` (commit `68f23f3`)
- **The two immediate fixes** (real password form + AuthGuard navigation): TDFBot commit `2a53aee` on `tdflite-auth` branch
- **The deceptively-named helper that started it:** `~/dev/TDFBot/e2e/helpers/auth.ts` `loginAs(page, user)` function
- **In-flight rebuild of the e2e suite using the proposed L5b pattern:** TDFBot agent `aa7740ec94ac042e5` writing `e2e/helpers/ui-login.ts` + 4 new ui-* specs that use mouse+keyboard only
- **Cross-cutting smoke that didn't catch this:** `make demo-smoke` (filedag `Makefile`) runs 19 fast-smoke + Go regressions but no L5b coverage
- **Contracts that didn't catch this** (because they stop at the API): `architecture/CONTRACT-T2-TDFBOT-API.0.1.md`, `CONTRACT-P5-CHAT-ORCHESTRATION.0.1.md`, `CONTRACT-D2-RECEIPT.0.1.md`, `CONTRACT-I6-RECEIPT.0.1.md`

---

## Honest Reflection

The 2026-04-27 hardening cycle was technically excellent — 8 worktree agents in parallel, clean merges, demo-smoke 19/19 green, 16 commits in 3 hours. **All of that work happened on top of a foundation that didn't actually work for a human.**

The right read is not "the cycle was a failure." The right read is "the cycle was high-quality work that hit the wrong target because the gate that defines the right target was missing." The L5 fidelity tier, the helper naming convention, the fresh-eyes ritual — these are infrastructure that should have existed before any cycle started. Without them, every cycle generates evidence that proves itself rather than evidence that satisfies external reality.

This isn't unique to filedag. Three projects (filedag, opendockit, pdf-signer-web) have now produced feedback in the same family. **Every codebase using AI-assisted agents to generate tests against AI-assisted code is at risk of closed-loop verification drift.** REBAR's job — across all repos that adopt it — is to inject external reality at fixed checkpoints. The proposed additions above are the minimum needed to do that for browser-fronted apps.

The bones of REBAR are good. The joints need bracing. This document, plus the pdf-signer-web "six moments" and opendockit "interaction-class false positives", are the brace material.

---

## Disposition Suggestions for Maintainers

If REBAR maintainers agree with the diagnosis, suggested order of adoption (cheapest to most-expensive):

1. **REBAR-E (helper naming)** — pure convention; lands as a CONVENTIONS.md edit + a renaming pass in adopting projects. Lowest risk.
2. **REBAR-C (banned-pattern CI gate)** — one shell script in `scripts/` template; per-project ci-check inclusion. Catches regressions immediately.
3. **REBAR-A + REBAR-B (test fidelity tiers + promotion criteria template)** — `TESTING_CONTRACT.md` extension or new `TEST_FIDELITY.md`. Documentation work.
4. **REBAR-G (independent verification rule)** — `AGENTS.template.md` edit + `session-end.sh` enforcement. Hardest to mechanize but highest leverage.
5. **REBAR-D (fresh-eyes ritual)** — process discipline; needs cross-repo coordination on who plays the "independent agent" role.
6. **REBAR-F (UI-surface contracts)** — biggest investment; new contract category. Defer until adoption depth justifies.
7. **REBAR-H (periodic stale-bypass audit)** — calendar discipline; can be folded into existing audit cadences.

If REBAR rejects (or wants alternate framing), this report is at minimum useful as a third data point in the "tests green / UX broken" cluster — three projects, three sessions, same pattern. The cluster itself is the finding.
