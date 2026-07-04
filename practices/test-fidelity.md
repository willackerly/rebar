# Test Fidelity — Practice Guide

**Status:** draft
**Sources:**
`feedback/2026-04-22-testing-rigor-six-moments.md` (pdf-signer-web),
`feedback/processed/2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md`
(filedag),
`feedback/processed/2026-04-27-user-at-keyboard-story-tier.md` (pdf-signer-web)
**Enforced by:** `scripts/check-decay-patterns.sh` (P2, P8) + reviewer
discipline

Three repos — pdf-signer-web, filedag, opendockit — independently
produced the same incident: **every test green, the product broken for
a human.** A suite reported 8/8 green while the login page froze on
"Loading..."; 92 Playwright tests passed against a deployment whose PDF
render was CSP-blocked. The mechanism is always the same: the *shape of
the test* drifts away from the *shape of the claim* its name makes, and
nothing in the loop notices because the author of the test grades the
test.

This doc names the rungs of that drift, requires every claim-bearing
spec to declare its rung, and defines the two gates that keep the top
of the ladder honest.

---

## Fidelity is a third axis

Speed tiers (T0–T5) say how long a test takes. The journey-driving
ladder in [`practices/e2e-testing.md`](e2e-testing.md) (L1–L4) says at
what level a browser journey is driven. **Neither says what a green run
proves.** Fidelity is that statement: given this test passed, what do
we actually know?

A test can be fast AND high-fidelity; the axes are orthogonal. What is
forbidden is *claiming* high fidelity while *shipping* low — a
surrogate wearing a real-flow name, a bypass helper named after the
thing it bypasses.

---

## The ladder

Five rungs. Every rung is legitimate. Lying about the rung is not.

| Rung | What a pass proves | Standing alone, sufficient for |
|------|--------------------|-------------------------------|
| `tautology` | The detector doesn't fire on a clean environment | Nothing — requires a paired negative control |
| `surrogate` | A stand-in for the real path behaves | Secondary/regression coverage only |
| `real-flow` | The production code path works end-to-end | Claim closure for non-UI claims |
| `mutation-proof` | The detector provably fires on each enumerated violation | Security / differential claim closure |
| `uaks` | A human at a keyboard can complete the journey | "demo green" / "test env ready" on user-interactive repos |

### Rung 1 — `tautology`

Asserts *absence* on a clean environment: "the forbidden DB isn't
listed," "the canary doesn't appear in requests."

**Telltale signs:**

- Primary assertion is `toBeNull()`, `toEqual([])`, `not.toContain`
- Would still pass if the detection logic were replaced with
  `return null`
- Passes in environments where the subject under test never ran at all

**Standing rule:** a tautology test counts as coverage only when paired
with a **negative control** — a sibling test that stages the violation
and asserts the same detector fires. A sensor that always reads zero on
a quiet environment cannot be called functional.
`check-decay-patterns.sh` P2 flags bare absence-assertions in
security/audit specs.

### Rung 2 — `surrogate`

Exercises a stand-in for the real path: a fake token on the failure
branch, a mocked backend returning hand-built response shapes, a
canary through a 4xx instead of a successful round-trip.

**Telltale signs:**

- Asserts on the *error* path of a flow whose *success* path is the
  claim
- Exists because the real version needed a dedicated config or build
  flag the author deferred
- Mock fixtures are cleaner than anything production emits (e.g. no
  FTS highlight markup in snippets)

**Standing rule:** surrogates are fine — often the right cheap
regression guard — but **never the only coverage of a claim.** A
surrogate spec must carry a `claim:` id (see declaration format) shared
with a `real-flow` or higher test of the same claim. Mocked-spec counts
never roll into a demo or claim-closure headline.

### Rung 3 — `real-flow`

Drives the production code path end-to-end: real build, real backend,
real data shapes, no mocks anywhere on the claim-bearing path.

**Telltale signs it is NOT real-flow despite the name:**

- State injection: `sessionStorage`/`localStorage` writes, seeded
  cookies, `page.addInitScript`
- `page.route()` fulfillment anywhere on the claim-bearing path
- API-direct calls (`request.post`) standing in for user actions
- A helper named like the real thing (`loginAs`) that actually forges
  the state the real thing would produce

Bypass helpers are allowed *below* this rung — but they must announce
themselves: name them `STUB_*`, `INJECT_*`, or `BYPASS_*` so the
compromise is visible at every callsite. Naming a bypass after what it
pretends to do is a meta-bug.

### Rung 4 — `mutation-proof`

Real-flow **plus** negative controls **plus** enumerated drift modes.
For any comparison-style test ("X and Y produce equivalent output"),
"structural comparison" is not a category — the author lists exactly
which drift modes the fingerprint catches, which it doesn't, and proves
each covered mode by injecting it and watching the detector fire.

Declaration includes a `DriftModes:` block:

```
// fidelity: mutation-proof
// claim: SECURITY_PLAN.G4
// DriftModes covered: text content (Tj operands), image XObject count
// DriftModes NOT covered: positional drift (x,y), hex-encoded strings,
//   TJ array form, drawing order
```

The `NOT covered` list is the honest half. A reviewer challenges an
empty one.

### Rung 5 — `uaks` (User-At-Keyboard Story)

The apex tier for user-interactive repos, defined by what it
**forbids**: every interaction with the system under test must be
something a real user with keyboard, mouse, and webcam could physically
do. No state injection. No backend mocks. No browser internals. The
test opens the page, acts as the user, and fails when the user can't
proceed.

The API rule is an **allowlist, not a blacklist** (blacklists rot as
frameworks add APIs):

- **Allowed (hands):** `click`, `fill`, `press`, `hover`, file picker
  via `setInputFiles`, native dialog accept/dismiss
- **Allowed (eyes — observation only):** `locator`, `expect`,
  `waitForSelector`
- **Allowed (browser permission grants):**
  `context.grantPermissions([...])` — the automation equivalent of the
  user clicking "Allow" on the native popup
- **Forbidden:** `page.evaluate`, `page.addInitScript`, `page.route`,
  `addCookies`, storage writes, fetch stubs, API-direct calls for
  user-equivalent actions, CDP commands beyond documented exceptions

**Canonical exception — webcam:** CI has no human at the camera. Feed a
real fixture image through the browser's own capture path via launch
flags (`--use-fake-device-for-media-stream`,
`--use-file-for-fake-video-capture=<fixture>.mjpeg`). No test code
touches `getUserMedia`. Any further exception requires a documented
entry with reasoning — the exception list is append-only and reviewed.

**Position and cost:** UAKS sits between deployed-env E2E and manual
user verification. It is the slowest automated tier, by design — run it
nightly and before any "ready" claim, not on every commit. It is the
only automated tier whose *definition* makes "passes while the user is
broken" structurally impossible.

**When to build it:** any repo where humans use a UI to accomplish
tasks. **When to skip it:** pure CLI / library / server-side repos with
no human-facing surface — but skipping must be declared, not silent
(see Gate 1).

---

## Declaration format (machine-greppable)

Every claim-bearing spec file declares its rung in a header comment,
comment style per language (`//`, `#`, `--`):

```
// fidelity: real-flow
// claim: SECURITY_PLAN.A2
```

Rules:

- The token is exactly `fidelity:` followed by one rung id:
  `tautology` | `surrogate` | `real-flow` | `mutation-proof` | `uaks`
- One declaration per file, at the top. A file that would need two
  rungs should be two files.
- `claim:` is **required** at `surrogate` (it is the link to the
  real-flow counterpart via shared claim id) and recommended
  everywhere.
- `mutation-proof` additionally requires the `DriftModes:` block
  (covered + NOT covered).

Grep surface (what tooling and reviewers key on):

```bash
grep -rEn '(//|#|--)[[:space:]]*fidelity:[[:space:]]*(tautology|surrogate|real-flow|mutation-proof|uaks)' \
  --include='*.spec.*' --include='*.test.*' . \
  | grep -v -e node_modules -e vendor -e '\.claude/worktrees'
```

A `fidelity: uaks` declaration opts the file into the P8 bypass ban in
`check-decay-patterns.sh` wherever it lives — the declaration *is* the
enforcement hook.

---

## Gate 1 — "test env ready"

A **"test env ready"** (or "ready for prod promote") claim MUST cite
one of:

1. **A passing UAKS run against that environment** — spec paths plus
   the run evidence; or
2. **The repo's standing declaration that no UAKS layer applies**, one
   line in the repo's `AGENTS.md` or testing contract:

   ```
   UAKS: none — pure CLI/library; no human-facing surface
   ```

   Greppable as `grep -rn '^UAKS:'` over `AGENTS.md` and
   `architecture/`. The reason clause is mandatory — "none" without a
   reason is a stub, not a declaration.

Silence is not a third option. No UAKS spec **and** no `UAKS: none`
line means the "test env ready" claim is premature: the gate the user
actually cares about isn't wired. A green deployed-env E2E tier is
necessary, not sufficient — state-injection-heavy tiers pass while the
actual user journey is broken (the canonical incident: CSP-blocked PDF
worker behind 92 green Playwright tests).

---

## Gate 2 — closed-loop demo gate

No **"demo green"** / **"demo LIVE"** claim, and no `demo:`-prefixed
commit or branch merge, without **captured browser-hit evidence**:

- A screenshot or screen recording of the deployed surface being
  driven at UAKS mechanics — mouse and keyboard, from login onward, no
  injection
- Captured at (or after) the SHA being promoted, and referenced from
  the promotion artifact (plan entry, QUICKCONTEXT line, PR
  description)
- Produced by an actor outside the implementer's coordination chain
  where feasible: fresh clone, stack up from the documented runbook,
  blank browser profile

The reason this gate exists is **closed-loop verification drift**: the
agent writes the impl, writes the test, runs the test, reports green —
and nothing in the loop is external to the agent's worldview. An
agent's "8/8 green" report is a claim, not evidence. The browser hit is
the injection of external reality; the capture is what makes it an
artifact instead of an assertion.

Corollaries:

- A demo-claiming spec must itself be `uaks`-rung. A spec that
  satisfies a "login as sarah.chen" runbook step by injecting a JWT is
  a backend-shape test wearing the costume of an end-to-end test.
- Mocked specs are valuable and are **counted separately** — they never
  contribute to a demo headline number.
- `check-decay-patterns.sh` P8 mechanically fails any demo-claiming or
  `uaks`-declared spec containing bypass patterns (`loginAs`,
  `sessionStorage.setItem`, `page.route`, `page.evaluate`,
  `request.post`, ...). Reviewed exceptions go through the script's
  allowlist file with a reason.

---

## What this doc deliberately does not restate

- **Prohibited skip patterns** (`test.skip`, `xit`, `xdescribe`,
  unconditional skips) — owned by `conventions.md` → Testing
  Conventions. The fidelity ladder assumes those are already banned;
  a skipped test has no rung because it produces no evidence at all.
- **Speed tiers and E2E infrastructure** (managed stacks, port ranges,
  Playwright gotchas, the L1–L4 journey-driving ladder) —
  [`practices/e2e-testing.md`](e2e-testing.md).
- **Soft-hardening decay patterns** (silenced failures, magic-string
  gating, plausible env names) —
  `feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md`,
  enforced by `scripts/check-decay-patterns.sh`.

---

## See also

- [`conventions.md`](../conventions.md) — Testing Conventions
  (prohibited skip patterns, accepted conditional patterns)
- [`practices/e2e-testing.md`](e2e-testing.md) — journey-driving
  ladder (L1–L4), managed test stacks
- `scripts/check-decay-patterns.sh` — mechanical enforcement (P2
  inverted assertions, P8 demo/UAKS bypass ban)
- [`feedback/2026-04-22-testing-rigor-six-moments.md`](../feedback/2026-04-22-testing-rigor-six-moments.md)
  — the six claim-vs-test asymmetries
- [`feedback/processed/2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md`](../feedback/processed/2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md)
  — the "8/8 green, login broken" incident + REBAR-A…H proposals
- [`feedback/processed/2026-04-27-user-at-keyboard-story-tier.md`](../feedback/processed/2026-04-27-user-at-keyboard-story-tier.md)
  — the UAKS tier origin + allowlist rationale
