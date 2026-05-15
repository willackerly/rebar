# Feedback: Human-Emulation Testing: Gaps Between Spec Coverage and Real-World Fidelity

**Date:** 2026-04-02
**Source:** REBAR testing practices, agent templates, conventions
**Type:** improvement
**Status:** proposed
**Template impact:** practices/testing-guardrails.md, templates/agent-templates/ (new cleanroom-audit template), practices/ (new notification-parity-testing), conventions.md
**From:** Dapple SafeSign UX scrub session (2026-04-01/02)

## What Happened

During a comprehensive UX review of Dapple SafeSign, we wrote detailed UX specs (AUTH_UX_SPEC.md, JOURNEY_UX_SPEC.md) with 5 personas, every screen state, exact copy text, and ran cleanroom red-team audits that found 14 real bugs. However, when assessing the Playwright test coverage against the spec, five systemic gaps emerged that the REBAR framework does not currently address.

### Gap 1: API Shortcuts Masquerade as Human Testing

Golden path E2E tests used `createAndSendEnvelope()` API calls instead of clicking through the 4-step compose wizard UI. The tests "passed" but never exercised the actual UI the user interacts with. The compose wizard could be completely broken and all tests would remain green.

This is a pervasive pattern. API helpers are faster, more reliable, and eliminate flake. They are the right choice for setting up preconditions. But when the API helper IS the thing being tested (creating an envelope via the compose wizard), the test is lying about what it covers. There is no guidance in the framework for when API shortcuts are acceptable versus when the test must drive the UI like a human would.

### Gap 2: Personas Exist in Spec But Not in Tests

We defined 5 personas (Alice the power user, Bob the first-time signer, Carol the upgrade path, Eve the impersonator, Alice-2 the cross-device recovery). Each persona represented a unique journey with unique failure modes. But in practice, most tests just used arbitrary email strings. Eve (impersonator) had zero tests. Carol (upgrade path) was gated behind API availability and never ran. The personas were documentation artifacts, not test drivers.

The spec did its job by identifying the personas. The test suite did not follow through. There is no enforcement mechanism that maps personas to test coverage.

### Gap 3: Email-Inbox Parity Was Critical But Unasserted

The spec stated that every email notification must have an equivalent in-app notification. The infrastructure existed (mock-resend captured all outbound email, the notifications API stored in-app items). But no test actually compared email content to inbox content. A notification could silently fail to appear in one channel and nobody would know.

This is a class of contract that the framework should recognize: when a system sends notifications through multiple channels, content parity across channels is a testable contract.

### Gap 4: Visual Assertions Were Absent

The spec goal was "screenshot aggressively" to establish visual baselines. Playwright's `toHaveScreenshot()` was never used. All screen state verification relied on text content assertions (`expect(page.getByText('Welcome'))`), which are fragile against layout regressions, styling changes, and visual bugs that don't affect text content.

The framework provides no guidance on when visual regression testing is worth the maintenance cost versus when text assertions are sufficient.

### Gap 5: Edge Cases Spec'd But Not Exercised

13 edge cases were defined in the spec: network drops during signing, popup blockers interfering with Surface, model preload timeouts, multi-signer sequencing race conditions, expired signing links, partially completed envelopes, and more. Approximately 4-5 were actually tested. The rest existed only as documentation.

The gap between "documented edge case" and "tested edge case" is invisible without an explicit mapping. There is no checklist or enforcement step that flags the delta.

### Bright Spot: Cleanroom Red-Team Audits Were the Highest-ROI Activity

Spawning a subagent with ONLY the spec (no implementation knowledge) to audit the code found 4 HIGH-severity bugs that the author missed. The pattern: give the auditor the spec as ground truth, point it at the implementation, and ask it to find every discrepancy. The auditor's lack of implementation context is a feature, not a bug -- it cannot rationalize away deviations because it does not know why they exist.

This pattern should be standardized as a reusable template.

## What Was Expected

The REBAR framework should provide guidance on:

- When API shortcuts are acceptable versus when UI-driven "human emulation" tests are required
- How to enforce persona coverage (not just persona documentation)
- A pattern for multi-channel notification parity testing
- When and how to add visual regression baselines
- A template for spec-driven red-team audits (the cleanroom pattern)

## Suggestion

### 1. Add a "Test Fidelity Ladder" to Testing Practices

Define explicit levels of test fidelity so projects can declare their target per journey:

| Level | Name | What It Tests | When to Use |
|-------|------|---------------|-------------|
| L1 | API contract | Request/response shapes, status codes | Every endpoint, always |
| L2 | UI happy path | Click through the primary flow via browser | Every user-facing journey |
| L3 | Human emulation | Every click, every screen state, every persona | Critical journeys (auth, signing, payment) |
| L4 | Visual baseline | Pixel-level screenshot comparison | Journeys where layout/styling is part of the contract |

The key rule: **API helpers may set up preconditions at any level, but the journey under test must be driven at the declared fidelity level.** If the compose wizard is the journey, L2+ means clicking through all 4 steps. Using `createAndSendEnvelope()` is only acceptable when the compose wizard is a precondition for testing something else (e.g., the signing link flow).

Each journey in the spec should declare its target level:
```markdown
### Journey: Create and Send Envelope
**Test fidelity:** L3 (human emulation)
**Personas:** Alice (power user), Bob (first-time)
**Visual baseline:** L4 for compose wizard review step
```

### 2. Add a "Cleanroom Audit" Template to Agent Templates

A reusable prompt pattern for spawning a red-team subagent. The exact structure that worked:

**Inputs to the auditor:**
- The spec document (as ground truth)
- The implementation files (as the thing being audited)
- NO architecture docs, NO implementation rationale, NO known issues

**Auditor instructions:**
- Read the spec end-to-end first
- Read the implementation
- Report every discrepancy in a structured format: location, spec says, code does, severity (HIGH/MEDIUM/LOW), specific concern
- Do not rationalize deviations. If the spec says X and the code does Y, that is a discrepancy regardless of whether Y might be intentional

**Why the isolation matters:** An auditor with full context will unconsciously excuse deviations ("oh, they probably did it this way because..."). An auditor with only the spec has no choice but to flag everything. The false positive rate is low because specs are written with intent -- deviations are usually real bugs.

### 3. Add Notification Parity Testing as a Contract Pattern

When a system sends notifications through multiple channels (email + in-app, email + SMS, push + in-app), the contract should include a parity test:

```typescript
// Pattern: notification parity assertion
const email = await waitForEmail(recipient, 'Envelope Ready');
const inbox = await api.getNotifications(recipient);
const inAppNotif = inbox.find(n => n.type === 'envelope_sent');

expect(inAppNotif).toBeDefined();
expect(inAppNotif.envelopeId).toBe(email.metadata.envelopeId);
// Content parity: both channels reference the same envelope, sender, action
```

This is straightforward when mock transports exist (mock-resend, mock-sendgrid). The framework should call out this pattern and recommend it whenever multi-channel notification is part of the system design.

### 4. Add Visual Baseline Guidance to Testing Guardrails

When to use `toHaveScreenshot()`:
- **Use it** for screen states that are part of the product contract (login page, signing page, completed state). Layout regressions in these states are real bugs.
- **Skip it** for highly dynamic content (dashboards with live data, lists with variable items). The maintenance cost of updating baselines exceeds the bug-detection value.
- **Always** capture baselines on a fixed viewport size and with deterministic data (seeded test accounts, fixed timestamps).

Baseline management:
- Store baselines in the test directory alongside the spec files
- Update baselines explicitly (`--update-snapshots`) -- never auto-accept
- Review baseline diffs in PRs the same way you review code diffs

### 5. Persona Enforcement Checklist

Add a step to the test review process:

```markdown
### Pre-Merge Checklist: Persona Coverage
For each persona defined in the spec:
- [ ] Alice (power user): tested in [test file]
- [ ] Bob (first-time signer): tested in [test file]
- [ ] Carol (upgrade path): tested in [test file] OR explicitly deferred with reason
- [ ] Eve (impersonator): tested in [test file] OR explicitly deferred with reason
- [ ] Alice-2 (cross-device): tested in [test file] OR explicitly deferred with reason

Personas without tests must have a documented reason (e.g., "requires hardware not available in CI").
```

The key insight is that "explicitly deferred with reason" is acceptable; "silently absent" is not. The checklist forces the conversation about why a persona is untested.

---

## Priority Ranking

| # | Suggestion | Effort | Impact | Where It Goes |
|---|-----------|--------|--------|---------------|
| 1 | Test Fidelity Ladder | 2h | HIGH -- eliminates the API-shortcut blind spot | practices/testing-guardrails.md |
| 2 | Cleanroom Audit template | 1h | HIGH -- proven 4-bug ROI in one session | templates/agent-templates/ |
| 3 | Notification parity pattern | 30min | MEDIUM -- prevents silent channel failures | practices/ (new file) |
| 4 | Visual baseline guidance | 30min | MEDIUM -- prevents layout regression blind spot | conventions.md or testing-guardrails.md |
| 5 | Persona enforcement checklist | 15min | MEDIUM -- prevents spec-test coverage drift | testing-guardrails.md |
