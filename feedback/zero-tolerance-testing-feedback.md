# Feedback: Zero Tolerance for Test Failures and Skips

**Source project:** Dapple SafeSign (pdf-signer-web)
**Date:** 2026-03-18

---

## The Principle

**No test failures. No skips. No exceptions. No hand-waving.**

Every failure is a real signal. Every skip is a configuration bug. "Pre-existing" is not an excuse — it's a tracking failure. If a test is broken, fix it or remove it. If a test skips, fix the environment or gate it properly. Leaving failures in place compounds — the next agent trusts the baseline and doesn't notice when their change adds a second failure.

## What We Found

During our final verification, we initially reported "pre-existing failures" as acceptable:
- 1 unit test failure (stale assertion after a UX change 3 days ago)
- 14 E2E skips (wrong invocation method)
- 1 Railway failure (certificate consistency bug)

The user's response: **"can we ensure all 'pre-existing failures' are either fixed or determined n/a? I do not like allowing failures or skips anywhere."**

Investigation results:
- The unit test **was fixable in 30 seconds** — the assertion expected old dimensions (150×60) that had been changed to (300×80). A 3-day-old regression that every session ignored.
- The 14 E2E skips **were an invocation error** — running `pnpm test:e2e:envelope` from the wrong directory. With `test-stack.sh run envelope`, all 100 passed, 0 skipped.
- The Railway failure **is a real tracked bug** (TODO 4a) — certificate consistency across signing flows. Legitimate.

**The 30-second fix had been ignored for 3 days.** Multiple agent sessions saw "1 failed | 224 passed" and moved on. The test was signaling a real behavior change (signature box dimensions changed from 150×60 to 300×80) and nobody updated the assertion.

## Recommendation for Templates

### Add to AGENTS.template.md (Testing Expectations section)

```markdown
### HARD RULE: Zero Tolerance for Test Failures

**NEVER dismiss test failures as "pre-existing", "flaky", or "not caused by our changes."**

Every failure is a signal. Investigate every one.

| Situation | Action |
|-----------|--------|
| Test fails after your change | Fix the code or fix the test |
| Test was already failing before your change | Fix the test NOW — you found it, you own it |
| Test times out | The timeout is wrong OR the product is broken — fix one |
| Test skips unexpectedly | The environment/config is wrong — fix it |
| Test is genuinely environment-gated | Must have `test.skip()` with explicit env check AND be documented |

**Acceptable skips (rare):**
- Environment-gated tests (`test.skip(!process.env.DATABASE_URL)`) that only run in CI
- These MUST be documented in TODO.md or AGENTS.md with the reason

**Unacceptable:**
- "1 failed | 224 passed" treated as green
- "14 skipped" without investigating why
- "Pre-existing" as a reason to not investigate
- The word "flaky" in a commit message (describe the root cause instead)

**Why this matters:** A "pre-existing" failure is a test that was correct when written and
is now wrong because code changed without updating the test. Every session that ignores it
makes it harder to fix (context is lost) and normalizes broken baselines. The 30-second fix
you're avoiding today becomes a 30-minute archaeology project next month.
```

### Add to DESIGN.md (Anti-Drift Mechanisms)

Reference this as an anti-drift mechanism: test failures that are allowed to persist are a form of documentation drift — the tests describe a contract (expected behavior) and the code has drifted from it.

### Add to the Testing Cascade (T0-T5)

At every tier, the rule is: **0 failures, 0 unexpected skips.** A passing tier means ALL tests pass, not "most tests pass and the failures are known."

---

## The Deeper Insight

Test failures are the canary in the coal mine for contract drift. When `sign-for-download.test.ts` fails because the signature box changed from 150×60 to 300×80, that's the test system telling you: **"the contract between the signing flow and the signature appearance has changed, and one consumer wasn't updated."**

In contract-driven development, this maps directly to: "the implementation of CONTRACT:I4-SIGNATURE-TYPES changed, but not all implementing files were updated." The test caught the contract violation. Ignoring it defeats the purpose of having contracts at all.

**The zero-tolerance policy is the enforcement mechanism for contract conformance.** Without it, contracts are aspirational. With it, they're operational.
