# Feedback: User-at-Keyboard Story Tier — a missing top of the test pyramid for user-interactive repos

**Date:** 2026-04-27
**Source:** Dapple SafeSign (pdf-signer-web) — post-FedCM-Wave-5 deploy + manual smoke that found a CSP-blocking-PDF-render bug NO automated tier caught
**Type:** missing-feature / template-impact
**Status:** proposed
**Template impact:** `TEST_FIDELITY.md` (new tier definition + slot in pyramid), `AGENTS.template.md` (gate before declaring "test env ready"), possibly a new `USER_AT_KEYBOARD_STORY.md` template stub for user-interactive repos to drop in.
**From:** Claude Opus 4.7 (1M), pdf-signer-web, 2026-04-27
**Driving incident:** the user opened the deployed test env in a browser, tried to sign a document, the PDF stuck on "Loading PDF..." forever. Console: CSP `script-src` directive blocked pdfjs's blob: worker. **92 Playwright tests had passed against that exact deployment.**

**Related prior feedback (same axis, different angles):**
- `2026-04-22-testing-rigor-six-moments.md` — claim-vs-test asymmetries.
- `2026-04-24-fidelity-decay-soft-hardening-patterns.md` — eight ways tests look passing while the tested thing is broken.
- `2026-04-20-interaction-class-false-positive-testing.md` — passing tests + real bug.
- `2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md` — same theme, different wording: bypassed tests count as not-tested.

This proposal is the constructive complement to that body of feedback. Those say "watch out for these failure modes." This one says "here is a tier whose DEFINITION makes those failure modes structurally impossible."

---

## The pattern

User-interactive products (anything where humans use a browser/app to accomplish tasks: SaaS, signing tools, identity providers, dashboards) accumulate test tiers over time:

- Unit
- Integration
- API/contract
- E2E with mocks
- E2E against deployed env
- Visual regression

Every tier ABOVE unit ends up taking shortcuts to be fast and reliable: state injection (DB seeds), test debug hooks (`window.__lastFooResult`), network mocks (`page.route().fulfill()`), CDP-driven dialog acceptance, browser flag relaxations. Each shortcut is reasonable in isolation. **Together, they let the suite drift away from what a real user actually experiences.**

Symptom: tests pass + the user reports a broken experience. The team adds another mock, another flag, another assertion. Drift compounds.

The missing tier — call it **`@user-at-keyboard-story`** — is defined by what it *forbids*. A test belongs to this tier if and only if every interaction with the system under test is something a real user with a keyboard, mouse, and webcam could physically do. The tag is the rule.

**No state injection. No backend mocks. No browser internals. Real user, real journey, end to end.**

This isn't novel — it's "true E2E" stripped of all the shortcuts the industry has gradually accepted. What's novel is making it a *defined tier with a tag and an enforced API allowlist*, not just a vague aspiration.

---

## Why "user at keyboard" is the right framing

Other names for the same idea exist in the wild:

- "True E2E" — preachy (implies others are fake), still ambiguous
- "User journey tests" — Playwright-community vocabulary, but doesn't convey the constraint
- "Black-box tests" — engineering jargon, doesn't communicate the user POV
- "Acceptance tests" — overloaded; means BDD/Cucumber to many

**"User at keyboard"** captures the rule in the name. A reviewer asks "is this something a user at a keyboard could do?" and the answer is binary. The tag becomes a reading guide for both author and reviewer.

The "story" suffix anchors to BDD heritage without requiring the BDD machinery — each test reads as a sentence: "a returning user opens the app, clicks Sign with Face, completes face capture, and downloads the signed document."

---

## The structural rule: API allowlist, not blacklist

Other test tiers can use the full Playwright/Cypress/Selenium API. `@user-at-keyboard-story` uses a strict whitelist:

- **Allowed:** anything a user can do with hands + eyes (`click`, `fill`, `press`, `hover`, file picker via `setInputFiles`, native dialog accept/dismiss)
- **Allowed (observation only):** finding elements, assertions, waiting for UI (`locator`, `expect`, `waitForSelector`)
- **Allowed (browser permission grants):** `context.grantPermissions(['camera'])` — equivalent to user clicking "Allow" on browser native popup
- **Forbidden:** `page.evaluate`, `page.addInitScript`, `page.route`, `addCookies`, direct DB writes, fetch stubs, CDP commands beyond documented exceptions

A blacklist is fragile (new APIs slip through). A whitelist is robust: anything not on the list is forbidden by default. New additions require an entry in the spec doc with reasoning.

### The one inevitable exception: webcam

In CI, no human is in front of the camera. The closest analog is a fixture image fed to the camera input layer. The pattern that preserves the "user at keyboard" property:

- Chrome launch flags consume a fixture image directly: `--use-fake-device-for-media-stream --use-file-for-fake-video-capture=<fixture>.mjpeg`
- NO test code touches `getUserMedia`. NO `page.evaluate` to fake the camera stream.
- The fixture is a real face image (corpus we already have for biometric testing). Same data the real user's webcam would produce.

This carve-out is documented as the canonical exception, with a "reserved for future exceptions" slot for anything else (each addition requires reasoning).

---

## Where it sits in the pyramid

| Layer | Cost | Catches | Bypass tolerance |
|---|---|---|---|
| Unit | <1s | Logic regressions in functions | None — pure |
| Integration | <5s | Module-composition regressions | Limited (test fixtures OK) |
| Contract | seconds | API shape regressions | Mocks at one layer OK |
| E2E with mocks | 1-2 min | Flow regressions assuming mocks are accurate | High — by design |
| E2E against deployed env | minutes | Deploy + integration regressions | Medium — state injection common |
| **`@user-at-keyboard-story`** | **5-15 min** | **User-facing regressions** | **ZERO** |
| Manual user verification | indefinite | The truth | N/A |

The user-at-keyboard tier exists between "deployed E2E" and "manual user verification." It's the most expensive automated tier, but it's the only automated tier whose definition makes "passes while the user is broken" structurally impossible.

---

## What this catches that other tiers don't

Concrete examples from SafeSign:

1. **CSP `script-src blob:` missing → pdfjs worker blocked → PDF stuck on "Loading PDF..."** — Caught only when the user actually tried to sign a doc. State-injection-heavy E2E tiers passed because they navigated to the page (HTTP 200) without rendering the PDF, OR they mocked the PDF data fetch.
2. **FedCM Storage Access auto-grant** — proposal hinged on "post-FedCM, the iframe gets silent storage access." CDP-accepted FedCM in automation does NOT trigger Chrome's user-trust path; only real user clicks do. State-injection tier had to skip; user-at-keyboard would have validated it.
3. **SPA fallback** — `/sign/<token>` returned 404 on deployed test env (Railway auto-detected static server didn't honor `serve.json`). Tests that visit `/` or `/auth` passed; the signing-link route a real user would hit broke silently.
4. **VITE_FEDCM_TEST_HOOKS gating** — debug hook gated on a build var that test env didn't set. Tests worked around with page-injected hooks; bundled hook was never exercised.

Every one of these would have been caught by `@user-at-keyboard-story` because that tier opens the page, acts as the user, and fails when the user can't proceed.

---

## Proposed REBAR template additions

### `TEST_FIDELITY.md` — add a new section

```
## Tier 6: User-at-Keyboard Story (the apex)

For user-interactive products, the highest automated tier should be a
"user-at-keyboard story" tier defined by a strict allowlist of permitted
test framework APIs. The rule: every interaction must be something a real
user with keyboard, mouse, and webcam could physically do. No state
injection, no backend mocks, no browser internals.

This tier is slow and brittle by design. It's the only automated layer
whose DEFINITION makes "test passes while user is broken" structurally
impossible.

See: USER_AT_KEYBOARD_STORY.md template (new) for spec format,
allowed/forbidden API enumeration, and the canonical webcam exception.

When to add this tier: any product where humans use a UI to accomplish
tasks. When to skip: pure CLI/library/server-side products with no
human-facing surface.
```

### `AGENTS.template.md` — add to "before declaring done" checklist

```
## Before declaring "test env ready" or "ready for prod promote"

For user-interactive products, the LAST automated check before either
of those declarations MUST be a passing run of the user-at-keyboard
story tier. If the tier doesn't exist yet, declaring "test env ready"
is premature — the gate the user actually cares about isn't wired.

A green deployed-env-E2E tier is necessary but not sufficient. State-
injection-heavy tiers can pass while the actual user journey is broken;
the canonical incident is documented in pdf-signer-web's 2026-04-26
session feedback.
```

### NEW `USER_AT_KEYBOARD_STORY.md` template stub

A drop-in template for user-interactive repos. Includes:
- The whitelist + blacklist of test framework APIs (Playwright-flavored, with notes for Cypress/WebdriverIO equivalents)
- The webcam carve-out spec
- A "reserved for documented exceptions" slot
- An initial-stories template (asks the user to fill in 3–6 user stories specific to their product)
- Schedule guidance ("run as final pre-deploy gate, run nightly")
- Enforcement guidance ("documentation enforcement v1; ESLint rule v2")

The template would link to a worked example: pdf-signer-web's `docs/plans/USER_AT_KEYBOARD_STORY_TIER.md`.

---

## Why this matters beyond SafeSign

The fidelity feedback loop in user-interactive products is brutal:
- Bug ships → user complains → developer adds a test → test passes → next bug ships → user complains again
- Each test the developer adds tends to be in their existing tier, with their existing shortcuts
- The shortcuts are why the bug got missed in the first place
- Tests grow in count without growing in fidelity

`@user-at-keyboard-story` interrupts that loop. When a user reports a bug, the question is: "is this a user-at-keyboard story we don't have yet, or one that's currently failing?" Either has a clear next step.

---

## Pushback I expect

**"Won't this be slow and brittle?"** Yes — by design. It's the slowest automated tier. That's why it sits at the top of the pyramid, not the middle. Run it nightly + before deploys, not on every commit.

**"Real webcams are a CI nightmare."** True. The fixture-image-via-Chrome-launch-flag pattern is the documented canonical exception. NOT mocking `getUserMedia` in test code — that's state injection. Feed the real camera-acquisition path with a real-looking image via the same flag a CI runner would use.

**"We already have E2E tests."** That's a different tier. The user-at-keyboard tier doesn't replace existing E2E tests; it sits above them. They catch different things.

**"Why 'user-at-keyboard'? 'User journey' or 'true E2E' would be fine."** The name encodes the rule. A reviewer reading the spec asks "could a user at a keyboard do this?" and the answer is unambiguous. "User journey" loses the constraint; "true E2E" implies others are fake. The proposed name was Will's; rationale held up.

---

## Asks for the REBAR maintainers

1. Land a `TEST_FIDELITY.md` section + an `AGENTS.template.md` checklist item per the proposed text above.
2. Land the `USER_AT_KEYBOARD_STORY.md` template stub. Reference the SafeSign worked example.
3. (Bonus) Land a starter ESLint rule for projects to enforce the API allowlist via lint, not just docs.

The SafeSign worked example will be at:
`pdf-signer-web/docs/plans/USER_AT_KEYBOARD_STORY_TIER.md` (committed 2026-04-27)
`pdf-signer-web/packages/web/tests/e2e/user-at-keyboard-story/*.spec.ts` (initial specs to land within the week)

Happy to draft the template + ESLint rule when there's appetite.
