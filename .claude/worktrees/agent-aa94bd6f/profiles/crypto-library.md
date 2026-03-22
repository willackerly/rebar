# Profile: Crypto / Security-Critical Library

For encryption libraries, key management systems, auth frameworks, and
any project where a security bug is a catastrophic failure.

## Core Files — Copy All

| File | Priority | Notes |
|------|----------|-------|
| `README.template.md` | Required | Emphasize threat model, trust boundaries |
| `QUICKCONTEXT.template.md` | Required | Track audit status, CVE status |
| `TODO.template.md` | Required | Full two-tag system |
| `AGENTS.template.md` | Required | All sections, strengthen security guidance |
| `CLAUDE.template.md` | Required | Add strict crypto rules |
| `methodology.md` | Required | Reference — contracts are especially critical here |

## Architecture — Mission Critical

| Item | Relevance |
|------|-----------|
| Contract system | **Essential** — every crypto interface must have a contract |
| BDD features | **High** — encode security requirements as scenarios |
| Contract linking | **Essential** — every file must declare its contract |

**Suggested contract categories:**
- `S*` services: KMS, certificate authority, token service
- `C*` components: cipher implementations, key derivation, nonce generation
- `I*` interfaces: key exchange, session management, credential storage
- `P*` protocols: wire encryption, handshake, key rotation

**Additional CLAUDE.md crypto rules (add these):**
```markdown
## Crypto Rules (CRITICAL)
- Never roll custom crypto. Use stdlib or audited libraries only.
- AES-256-GCM only. No CBC, no ECB, no stream ciphers.
- Nonces: 12 bytes from crypto/rand. Never reuse. Never derive.
- Key derivation: Argon2 for passwords, HKDF for key expansion.
- Zeroize sensitive material after use.
- Constant-time comparison for all secret-dependent operations.
- No logging of key material, nonces, or plaintext.
```

## Subagent Templates — High Value

| Template | Relevance |
|----------|-----------|
| `security-surface-scan.md` | **Essential** — run on every package, every sprint |
| `contract-audit.md` | **Essential** — crypto interfaces must match contracts exactly |
| `code-review.md` | **Essential** — customize with crypto-specific dimensions |
| `feature-inventory.md` | **Essential** — never let an agent silently delete crypto logic |
| `doc-drift-detector.md` | **High** — security docs must be accurate |
| `test-shard-runner.md` | **High** — parallelise the test suite |
| `ux-review.md` | **Low** — unless there's a UI component |

**Recommended additional template:** Create a `crypto-audit.md` that checks:
- Algorithm usage against approved list
- Nonce generation and reuse patterns
- Key material lifecycle (generation → use → zeroization)
- Side-channel resistance (timing, power analysis)
- Interop test coverage across platforms/languages

## AGENTS.template.md Sections — What to Customize

| Section | Action |
|---------|--------|
| Core Tenets | Add: "No custom crypto", "Formal verification where possible", "Interop testing mandatory" |
| Requires Discussion | Add: ALL crypto algorithm changes, key format changes, protocol changes |
| Testing Cascade | Add: fuzz testing tier, interop test tier, benchmark regression tier |
| Agent Collaboration | Add: **cross-validation pattern** — two agents independently review security-critical code, third diffs findings |
| Quality Gates | Add: zero `security-surface-scan` high/critical findings before merge |

## CLAUDE.template.md Sections — What to Customize

| Section | Action |
|---------|--------|
| Crypto Rules | **Essential** — add the full crypto rules block above |
| Testing | Add fuzz tests, property tests, interop tests |
| Allowed Commands | Add: `go test -fuzz`, crypto benchmarks, interop test runners |
| Agent Autonomy → Requires Discussion | Expand with ALL crypto-adjacent changes |

## What You Can Skip

- E2E Test Server Management (no servers)
- Deployment Traps (library, not deployed service)
- UX review template (no UI)
- Most of the web-specific gotchas (CORS, MIME types, etc.)

## Retrofitting an Existing Project

1. **Algorithm implementations** — Each algorithm gets a CONTRACT: header. These are the core behavioral contracts.
2. **Public API surface** — Exported functions/classes that consumers use.
3. **Test vectors** — Tag test vector files with the contract they validate.

Security-critical code benefits most from contracts because the cost of misunderstanding intent is highest. Start with the crypto primitives, then work outward to the public API.

**Ground truth first step:** Set up `METRICS` with test counts (including fuzz/property tests) and algorithm coverage. In crypto code, "126 tests" vs "586 tests" changes your confidence in shipping.

## What to Add

- **Threat model document** — reference from QUICKCONTEXT and README
- **Interop test matrix** — which platforms/languages/versions are tested
- **Audit log** — track external security audits, dates, findings, resolutions
- **CVE response process** — how to handle vulnerability reports
