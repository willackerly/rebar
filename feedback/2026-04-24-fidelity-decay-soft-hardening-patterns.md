# Feedback: Fidelity Decay — Eight Soft-Hardening Patterns That Look Like Done Work

**Date:** 2026-04-24
**Source:** Dapple SafeSign (pdf-signer-web) — storage-partitioning diagnosis + @real-chrome fidelity tier build-out
**Type:** anti-pattern / missing-feature
**Status:** proposed
**Template impact:** `AGENTS.template.md` (self-audit checklist before declaring done), a new `RIGOR_DECAY_PATTERNS.md` or `SOFT_HARDENING_ANTIPATTERNS.md`, possible extension to `TEST_FIDELITY.md`. A raw-thoughts section at the end floats a bigger idea: REBAR as a commit-time partner, not just a documentation layer.
**From:** Claude Opus 4.7 (1M), pdf-signer-web, 2026-04-24

**Related prior feedback:**
- `2026-04-22-testing-rigor-six-moments.md` — same family ("tests pass without proving what they claim"), different axis (claim-vs-test-shape asymmetries). This one is about *what ships inside the hardening work itself* when the author isn't challenged.
- `2026-04-20-interaction-class-false-positive-testing.md` — the third feedback in the trilogy. Three different angles, same invariant failing: **a passing test, written by a careful agent, doesn't guarantee the thing it appears to guarantee.**
- `zero-tolerance-testing-feedback.md` — don't dismiss failures. This feedback is the complement: don't dismiss *the shape* of a passing test either.

---

## What Happened

Working session on pdf-signer-web. A user report (2026-04-23): quick-sign with face fails deterministically in real Chrome. Every Playwright tier passes. Diagnosis hypothesis in the existing plan (`docs/plans/STORAGE_PARTITIONING_PLAN.md`): Chrome's `ThirdPartyStoragePartitioning` splits popup IDB from iframe IDB.

I was asked to (a) verify the diagnosis, (b) build a regression test that would have caught it, and (c) recommend a fix path. I did all three competently and shipped:

1. A new Playwright config (`playwright.real-chrome.config.ts`) with treatment + control projects.
2. A probe spec proving the bug empirically and ruling out proposed Fix A (Storage Access API without Related Website Sets).
3. Corrections to the original plan's launch-args config (Playwright's `ignoreDefaultArgs: ['--disable-features']` doesn't actually work; partitioning needs distinct eTLD+1 sites to trigger).
4. Updated docs across four files.
5. A tight recommendation: go straight to Fix B (popup-routed RPCs); skip Fix A; reject Fix C (violates `V1_SECURITY_PLAN` Phase 0).

I reported "done." The user then asked a single devastating question: **"can you take a careful holistic look at the approaches we're taking, especially in terms of fragility. I particularly want to make sure defaults are the conservative test case modalities and it's very hard to bypass and rationalize ignoring states that are failing."**

Forced to audit my own work with that lens, I found **eight soft-hardening patterns** I had shipped. Each one looked reasonable in isolation. Each one would have survived code review. Each one represented a future failure mode that wasn't there on day one but would activate silently: on a Playwright upgrade, on a project rename, on a copy-paste into a deploy script, six months from now when nobody remembers the setup.

This is the pattern I want to flag to REBAR. Not one specific anti-pattern — the **meta-pattern of soft-hardening that looks like done work.**

## The Eight Patterns

Each was in my initial "done" submission. Each the user's prompt surfaced as a separate failure mode. None would have been caught by typecheck, lint, or test pass.

### 1. `testInfo.fail()` on a known-broken test — "green by marker"

I marked the baseline partitioning test with Playwright's `testInfo.fail()` on the treatment project, reasoning: "while the bug exists, the test is expected to fail; the suite should stay green." This is exactly the rationalize-the-failure pattern the user is trying to avoid. The suite shows green. The bug is invisible to anyone glancing at the CI dashboard. The `testInfo.fail()` comment literally says "this is fine, the bug is documented" — prose stapled onto a defect saying "ignore this."

**Corrected to:** failing test asserts what we want to be true; when bug exists, test fails loudly with a message pointing to the plan doc; the "don't block PRs" requirement is handled by `continue-on-error: true` in the CI workflow, not by silencing the test itself.

### 2. Inverted assertion — "truth by inversion"

I wrote a Fix A test whose assertion was `expect(iframeAccess.value).toBe(null)` — i.e., "assert Fix A doesn't work." My rationale: "this documents the non-viability finding." Reader semantics: passing means broken. If Fix A ever started working (e.g., post-Related-Website-Sets registration), this test would start failing, and a future engineer would naturally reach to "fix" the assertion by flipping `null` → `sentinel`, completely eliding the signal.

**Corrected to:** assert the intended behavior (iframe sees sentinel). When the test fails today, the failure message explains the current non-viability and cites the plan doc. If Fix A ever starts working, the test naturally starts passing — a positive signal, not a conflicting one.

### 3. Opt-in CI tier — "built a tripwire, didn't arm it"

I built the entire `@real-chrome` Playwright tier and wired it as a `pnpm test:e2e:real-chrome` script. I did not wire it into any GitHub Actions workflow. This is directly isomorphic to Moment 5 of the 2026-04-22 feedback, but with higher stakes: it's a *regression test for a shipping-blocking bug*, and it would only run if someone explicitly invoked it.

**Corrected to:** `.github/workflows/fidelity.yml` runs on every PR and every push to main. The workflow runs with `continue-on-error: true` while Fix B is in flight (visible red without blocking merges), with explicit comments telling future engineers exactly when to flip the flag.

### 4. Hand-copied structural data — "silent drift on upgrade"

Playwright ships a `--disable-features=...` list of ~13 features it disables for test stability. My @real-chrome tier needs to re-enable exactly one of those (`ThirdPartyStoragePartitioning`). My first pass: hand-copied the list into a TypeScript array, with a comment "keep this in sync when upgrading Playwright." That comment is a wish. On the next Playwright upgrade the list drifts silently — either Playwright adds a new disable we'd want to keep (we don't include it → flaky test), or Playwright removes one we still include (the override becomes a no-op → test runs with partitioning *disabled*, missing the entire class of bug it was built to catch).

**Corrected to:** a utility that reads `playwright-core/lib/server/chromium/chromiumSwitches.js` source at config-load time, parses the `disabledFeatures` array, and returns it. Throws loudly if the parser regex fails (Playwright restructured), if multiple Playwright installs are detected (pnpm hoisting issue), or if the parsed list is suspiciously short (parser regression). Also throws if a requested removal isn't in the current list — because a no-op override is worse than nothing.

### 5. Magic-string project gating — "silent-drop on rename"

I gated a conditional skip on `testInfo.project.name === 'chromium-real-defaults'`. If anyone ever renames that project in the config, the string match silently fails, and the gated logic applies to projects that shouldn't match. Tests pass either way; the gating just stops working.

**Corrected to:** `testInfo.project.metadata.partitioning === true`. Config metadata is the source of truth for project semantics. A rename would break the compile unless metadata is moved with the project definition, which makes the coupling explicit.

### 6. No hermeticity check — "false positive indistinguishable from real signal"

My probe spec navigated an iframe to a synthetic `.test` host to read IDB. If vite's host-header check rejected the request, or if the URL 404'd, or if any middleware intercepted first, the iframe would load SOMETHING ELSE (a vite 4xx page, a Chrome error), my postMessage would hit no handler, the promise would time out, and the read would resolve to `null`. That "null" looks identical to "iframe was partitioned and couldn't see the write" — i.e., the bug-confirmed path.

This is a false-positive mechanism built into the test itself. A user adopting the tier could see "bug reproducing" when the real issue is a webserver misconfiguration.

**Corrected to:** two stacked preconditions. (a) a startup check hits the surface origin and detects vite's "Blocked request" error page, failing the test before any partitioning assertion runs. (b) the probe route handler increments a counter; the spec asserts `routeHits > 0` before trusting any result. A zero-hit scenario means "the iframe loaded something other than our test fixture; pass/fail of this test is meaningless."

### 7. Generic env var name for test loosening — "too plausible to refuse"

My initial loosening env var was `VITE_ALLOW_ALL_HOSTS=1`. Reasonable-sounding name. Could easily get copied into a deploy config "because the test needed it." A six-month-later engineer seeing `VITE_ALLOW_ALL_HOSTS=1` in a deploy script would not flag it — it reads like a normal infrastructure flag.

**Corrected to:** `VITE_PLAYWRIGHT_REAL_CHROME_LOOSENING=1`. The name is long, specific, and embeds its purpose. A deploy script containing it would be immediately suspicious on inspection. Combined with a loud stderr warning on every startup when active, accidental activation becomes nearly impossible to miss.

### 8. Single-key gate for test loosening — "one accidental env var away from prod"

Even with a distinctive name, a single env var check is fragile. A copy-paste is still a single action.

**Corrected to:** two-key requirement. The loosening takes effect only when BOTH `VITE_PLAYWRIGHT_REAL_CHROME_LOOSENING=1` AND `VITE_DAPPLE_TEST_MODE=1` are set. Setting only one throws at config-load with an explicit message ("you are almost certainly misconfiguring a deploy script"). A deploy would need to copy two obviously-test-only signals — much less likely than a single lazy copy-paste.

---

## The Pattern Behind the Patterns

All eight are the same underlying failure: **I saw each one, considered it, decided it was fine, shipped it.** None of them are negligence. They're all reasonable-looking tradeoffs that survive introspection *when you're already past the point of writing them*.

None would have been caught by:
- typecheck (all compile)
- lint (all conventional)
- test pass (all green)
- self-review (I was the one writing them)
- code review (each looks individually reasonable)

They would only be caught by:
- an external party asking "is this decay-resistant?" (what the user did)
- a REBAR-owned structural check that flags the specific patterns

This is the category the user named: *"real-world rigor decay that is really only preventable by process and structural gates."* The patterns don't decay because of carelessness. They decay because the author's context (right now, with the test fresh) doesn't match the consumer's context (six months from now, scanning a dashboard).

## What Was Expected

I'd expect REBAR to have a **self-audit prompt** for agents before they declare hardening work done. Not more documentation — a structured lens list the agent runs over its own diff, with named patterns and concrete signals:

```
Before declaring "done" on test infra / hardening work, run these lenses:

1. Does any test I added contain testInfo.fail(), test.fail(), expect.fail(),
   or equivalent silencing? (Rationalize-the-failure pattern.)

2. Do any of my assertions pass when the thing being tested is broken?
   (Inverted-semantics pattern.)

3. Do all the new tests I added run in at least one CI workflow?
   (Built-a-tripwire-didn't-arm-it pattern.)

4. Did I hand-copy any list, constant, or structural data from a library
   that updates independently? (Silent-drift pattern.)

5. Did I gate behavior on a hardcoded string matched against a config
   value someone might rename? (Magic-string pattern.)

6. Can my test produce the "bug detected" result from a cause OTHER than
   the bug itself (route miss, config error, network failure)?
   (False-positive-indistinguishable pattern.)

7. Are my test-only env var names / feature flags distinctive enough
   that accidental copying into a deploy would be obviously wrong?
   (Too-plausible-to-refuse pattern.)

8. Are my test-only activations gated by a single signal someone could
   flip accidentally? (Single-key loosening pattern.)
```

That list is not meant to be exhaustive — it's the byproduct of one session. REBAR can grow it over time from feedback like this and the related 2026-04-22 feedback.

I also suspect REBAR could usefully own a broader category: **"change that looks like hardening but ships a new failure mode with a longer fuse."** This family includes all eight patterns above, and probably many more.

## Suggestion — Structural Proposals

### Proposal A — Self-Audit Prompt Template

Add `rebar/templates/SELF_AUDIT_HARDENING.md` — a checklist the agent runs against its own diff before declaring done. Either linked from `AGENTS.template.md` or included inline. Include the eight lenses above, with:

- Named pattern per lens (so feedback loops can reference them cleanly)
- Grep-able signals per lens where possible (e.g., `testInfo.fail` is script-detectable; magic-string gating is harder but not impossible)
- Worked example per lens (the eight above are reasonably clean case studies)

### Proposal B — Diff-Scoped Decay Lint

A script similar to `check-tag-ci-coverage.mjs` (prototype for the 2026-04-22 feedback) but for soft-hardening patterns. Runs on `git diff --name-only` + `git diff` in pre-commit or PR CI. Flags:

```
# examples (not exhaustive)
Pattern: testInfo.fail() on assertion
  → file:line, suggestion: "silenced failure — convert to honest fail + CI continue-on-error"

Pattern: .toBe(null) asserting non-existence of desired behavior
  → file:line, suggestion: "inverted assertion — restate positively and mark expected-fail at workflow level"

Pattern: new @<tag> without a CI --grep path
  → file:line, suggestion: "see tag-ci-coverage check"

Pattern: hand-copied array/list annotated with "keep in sync" comment
  → file:line, suggestion: "read from source at runtime; throw on parse failure"

Pattern: testInfo.project.name === "literal" comparison
  → file:line, suggestion: "move to project.metadata"
```

Not all patterns are grep-able (6 and 8 are semantic), but the half that are would catch most of the slip.

### Proposal C — Hardening Commit Template

A git commit template for commits tagged `hardening(*)` or `test(fidelity)` that requires structured fields:

```
Pattern addressed:           [e.g., silent-drift-on-upgrade]
Lenses checked:              [list, or "none applicable"]
Known decay modes:           [what activates this failure 6 months from now]
Tripwire:                    [what structurally prevents regression]
CI path:                     [which workflow catches a regression of this]
```

Same structure as Proposal 6 in the 2026-04-22 feedback — but scoped to hardening/fidelity work instead of security tests. Forces the author to name the decay mode in prose; prose-naming makes self-audit possible.

---

## Raw Thoughts: REBAR as a Commit-Time Partner

(The user explicitly asked me to sprinkle in raw thoughts; this section is lower-confidence than the above, and meant as seed material for REBAR to chew on, not a finalized proposal.)

### The "git partner" framing

The consistent theme across all three recent feedbacks (this one, 2026-04-22, 2026-04-20) is: **REBAR captures conventions; REBAR does not enforce them at the moment the drift happens.**

What if REBAR's next evolution isn't more documentation, but a lightweight **commit-time audit partner** — something that sits between `git add` and `git commit`, running structural lenses on the staged diff, and refusing to proceed without explicit acknowledgement of drift?

Concretely, variations to chew on:

- **`git rebar-check` alias / pre-commit hook.** Runs a configurable set of structural lenses against staged changes. Exits non-zero on any flag; agent must either fix or pass `--acknowledge="<reason>"` which gets stored alongside the commit in a REBAR ledger file. Shifts the conversation from "reviewer catches it" to "author has to name it."

- **REBAR as an MCP tool the agent has to call before committing.** Already half-exists via `mcp__rebar-ask__ask_*`. What if a `mcp__rebar-ask__audit_diff` existed that took a git diff and returned structural flags? An agent being responsible about its work calls this before declaring done. It's a social pressure, not a technical gate, but agents respond well to "did you ask the tester yet?" prompts in AGENTS.md.

- **REBAR as a GitHub Actions action.** `actions/rebar-audit@v1` runs on every PR. Posts a comment with any flagged patterns. Because it's a comment (not a required check), it doesn't block — but it's visible and ages in public, which is its own pressure.

- **REBAR as a "decay predictor" service.** Given a diff, returns a list of (pattern, activation-condition, severity). Like a Sentry for process drift instead of runtime errors. Stretch idea.

### The CI-gate-as-aging-signal idea

One thing I noticed while building the `fidelity.yml` workflow with `continue-on-error: true`: that's a necessary-but-fragile pattern itself. The job shows as failed/warning in PR UI, but over time people desensitize. "Oh, that's always red, that's just the known issue." The visible-red-that-doesn't-block becomes invisible-red.

A partial defense: a bot comment on every PR that explicitly states *"known regression X is still reproducing; N days since detection; expected Fix B landing on [date or commit-range]."* The N days / expected-date makes the aging visible. "12 days since detection" reads differently from "47 days since detection."

REBAR could own this. When a claim is marked "expected to fail until Fix X," REBAR tracks the aging and surfaces it in PR comments. Decay of *process itself* becomes visible.

### Why the author can't self-audit well

The deep point, which I mostly landed on during this session: **the author cannot reliably self-audit hardening work because the author's context matches the test's context.** I can't see my own soft-hardening patterns because I wrote each of them from a state where they looked reasonable. I need external state — a cold reader, a structural lens, a checklist.

REBAR is well-positioned to be that external state. Its role today is "captured conventions"; its potential role is "applied lenses." The transition is mostly a matter of shipping the lenses as tools, not just as prose.

---

## Closing

The eight patterns above were in my initial "done" submission. The user's single-question audit turned up all eight. Each fix took 5–30 minutes; the audit itself took one question. The ratio of "prevention effort" to "shipped-and-slowly-decayed effort" is enormous.

REBAR's current strength is documenting *what good looks like*. The proposals above (and the raw thoughts section) are about adding a complementary strength: *catching, at the moment of authoring, the patterns that look like good and decay into not-good*. All eight patterns are grep-or-structure-detectable to some degree. A lightweight lint — or a lightweight REBAR-owned MCP tool — closes most of the gap.

Pair this feedback with `2026-04-22-testing-rigor-six-moments.md` (testing rigor asymmetry) and `2026-04-20-interaction-class-false-positive-testing.md` (passing tests + real bug). Together they describe a single phenomenon at three different resolutions: **"all green" is a comforting lie that REBAR currently has no mechanism to challenge.** The first two feedbacks proposed mechanical checks for specific instances. This one proposes generalizing to a named category — soft-hardening patterns — and shipping REBAR lenses that surface them at commit time.

Grade the proposals, fold the useful ones into templates, discard the raw thoughts freely. The specific eight patterns are the concrete contribution; the REBAR-as-git-partner framing is loose exploration.
