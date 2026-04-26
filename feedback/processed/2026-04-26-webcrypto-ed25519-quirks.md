# WebCrypto Ed25519 quirks — surfaced by DP3c (TDFBot ReceiptPanel)

> **Date:** 2026-04-26
> **Source:** filedag DP3c — TDFBot WebCrypto verification of D2-RECEIPT envelopes
> **Audience:** Anyone shipping browser-side asymmetric verification before ~2027

## What we hit

Building a WebCrypto Ed25519 verify path against `crypto.subtle.verify` for filedag's signed receipt envelope (D2-RECEIPT.0.1) surfaced four discrete quirks worth tracking. None block DP3c — the verifier ships with feature-detection and fall-through error states — but they shape the realistic deployment matrix.

## Quirk 1 — Browser support is recent and asymmetric

| Browser | Ed25519 in `subtle.verify` |
|---|---|
| Chromium | 113+ (May 2023) |
| Safari | 17+ (Sep 2023) |
| Firefox | 130+ (Sep 2024) |
| jsdom (Vitest) | NOT IMPLEMENTED |
| node:crypto | yes (since 14.x) but NOT via WebCrypto subtle in test runtimes |

The TC39 "Secure Curves in WebCrypto" draft remained Editor's Draft for years. Implementation landed at different times across vendors. **Practical floor today: assume Ed25519-via-subtle works in Chrome ≥113, Safari ≥17, Firefox ≥130.** Anything older needs a JS-only fallback (e.g., `@noble/ed25519`) or a server-verify endpoint.

## Quirk 2 — TypeScript types lag the runtime

The standard lib `AlgorithmIdentifier` does not include `"Ed25519"` as a string literal; `subtle.importKey({name: "Ed25519"}, ...)` and `subtle.verify({name: "Ed25519"}, ...)` work at runtime but require a cast. We used:

```ts
{ name: 'Ed25519' } as unknown as AlgorithmIdentifier
```

`@types/web` and DOM lib have not picked up the secure curves yet (as of TS 5.9). Filing a deposit-internal note so future TDFBot work upgrading to PQ-hybrid or X25519-ECDH knows to expect the same.

## Quirk 3 — jsdom does not implement `subtle.verify` for Ed25519

This is the biggest practical headache. Vitest defaults to jsdom; jsdom's Web Crypto polyfill (`@peculiar/webcrypto` or built-in) does not surface Ed25519. Two options:

1. **Mock subtle.verify** in the unit test — what we did. Install a fake `subtle` returning a chosen boolean per test. Lets us assert the badge UI for verified/tampered/untrusted-issuer paths without depending on a real Ed25519.
2. **Run those specific tests in `node` or `happy-dom` environment** — heavier; node:crypto exposes Ed25519 but not via the `crypto.subtle` API surface.

The mock approach is fine for asserting UI state machines and the input/output of the verify wrapper. **But** it does NOT exercise the actual canonical-bytes → Ed25519 path. That coverage requires either:
- A live Playwright browser test (DP3c has one — `e2e/regressions/demo-10-receipt.spec.ts`).
- A node-runtime test using `crypto.sign('ed25519', ...)` instead of `subtle.verify`.

We chose Playwright; the canonical-bytes layer is independently verified via the cross-impl fixture against DP3a's Go signer.

## Quirk 4 — TextEncoder Uint8Array realm mismatch in jsdom

`expect(bytes).toBeInstanceOf(Uint8Array)` fails in jsdom even when `bytes` IS a Uint8Array — the `TextEncoder` polyfill returns a Uint8Array from a different realm than the test's globalThis. `instanceof` is realm-sensitive. Workarounds:
- Duck-type: check `byteLength` + indexed access.
- Use `Object.prototype.toString.call(bytes) === '[object Uint8Array]'`.

This is a known jsdom irritation, not specific to crypto, but we tripped on it inside the canonicalization tests where the encoded bytes flow through TextEncoder.

## What this means for blindpipe + future TALOS

If `blindpipe` (peer repo) decides to do browser-side TDF unwrap that involves WebCrypto-native crypto (HKDF, ECDH-ES), the same support floor applies. Upcoming PQ-hybrid (ML-KEM + Ed25519) is even more recent — Chromium has it behind a flag as of 2026-Q1, no other browser shipped yet. **Plan for native and JS-only fallback paths in any browser-side crypto consumer for at least the next 18-24 months.**

## Concrete recommendations

1. **Always feature-detect** before calling `subtle.importKey` with a curve name — wrap in try/catch returning a typed `unsupported` outcome rather than throwing.
2. **Document the version floor** in user-visible tooltips (we put "Chrome 113+, Safari 17+, Firefox 130+" in the TamperDemo's unsupported-state tooltip).
3. **Have a server-verify fallback** for old browsers — filedag's `verify-receipt` CLI (DP3b) is the natural off-browser verifier; a `?verify=server` query param on the receipt endpoint is a v1.1 candidate per T2-TDFBOT-API.0.1.
4. **Mock subtle.verify in unit tests; rely on Playwright for the real path.** Do not try to make jsdom verify Ed25519.
5. **Cross-impl byte equivalence is the only real defense against canonicalization drift.** D2-RECEIPT's shared fixture pattern (`canonical-reference.json` / `.bin`) is the right shape; promote it to a REBAR template for any future cross-language signed-envelope work.

## Generalization for REBAR

The pattern "freeze a canonicalization spec + ship a fixture that implementations on each side test against" is the load-bearing piece, not the choice of Ed25519 or WebCrypto specifically. Whenever two repos in the TALOS ecosystem need to agree on signed bytes (assertion chains, audit-log entries, federated query receipts), the same fixture-as-oracle pattern should apply. Recommend documenting this as a REBAR Tier 2 pattern under `templates/canonical-fixture-pattern.md` once DP3 retro lands.
