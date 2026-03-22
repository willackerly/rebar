# Conventions

Standard conventions for contract-driven development. Adopt these in your
project for consistency across agents and humans.

---

## Branch Naming

Branches reference the contract they're working on:

```
<type>/CONTRACT-<id>-<description>
```

| Type | When | Example |
|------|------|---------|
| `contract/` | Creating or modifying a contract | `contract/C1-BLOBSTORE-2.0` |
| `impl/` | Implementing a contract | `impl/C1-BLOBSTORE-retry-logic` |
| `fix/` | Fixing a bug within contract bounds | `fix/C2-RELAY-session-timeout` |
| `feat/` | New feature within existing contract | `feat/S4-STORAGE-list-pagination` |
| `refactor/` | Refactoring (no contract change) | `refactor/C1-BLOBSTORE-cleanup` |

For work that spans multiple contracts or isn't contract-specific:

```
docs/update-quickcontext
test/shard-runner-flaky
chore/upgrade-dependencies
```

## Commit Messages

Reference contracts in commit messages using conventional commit format:

```
<type>(<contract-id>): <description>

<body — optional, explain why not what>

CONTRACT: <full-id>
```

**Examples:**

```
feat(C1-BLOBSTORE): add retry logic for transient storage failures

Blob uploads occasionally fail with 503 during peak traffic.
Add exponential backoff with 3 retries.

CONTRACT: C1-BLOBSTORE.2.1
```

```
fix(S2-API-GATEWAY): validate auth token expiry before forwarding

Expired tokens were being forwarded to downstream services,
causing cascading 401 errors.

CONTRACT: S2-API-GATEWAY.1.0
```

```
contract(C3-CRYPTO-BRIDGE): bump to 2.0 — add key rotation interface

BREAKING: New required method `RotateKey()` on CryptoBridge interface.
All implementations must add this method.

CONTRACT: C3-CRYPTO-BRIDGE.2.0
SUPERSEDES: C3-CRYPTO-BRIDGE.1.0
```

### Commit Types

| Type | When | Contract Impact |
|------|------|----------------|
| `feat` | New feature | Within existing contract |
| `fix` | Bug fix | Within existing contract |
| `contract` | New or modified contract | **Creates/changes contract** |
| `refactor` | Code restructuring | No contract change |
| `test` | Test changes | No contract change |
| `docs` | Documentation | No contract change |
| `build` | Build/CI changes | No contract change |
| `chore` | Maintenance | No contract change |

## Source File Headers

Every source file declares its contract in the first 15 lines:

### Direct Implementation

The file directly implements the contract's interface:

```go
// Package blobstore implements encrypted blob storage.
//
// CONTRACT:C1-BLOBSTORE.2.1
package blobstore
```

```typescript
/**
 * CryptoBridge — client-side AES-256-GCM encryption at the gateway boundary.
 *
 * @contract CONTRACT:C3-CRYPTO-BRIDGE.1.0
 */
export class CryptoBridge {
```

```python
"""
Key exchange primitives for P2P session setup.

CONTRACT:I2-KEY-EXCHANGE.1.0
"""
```

### Belonging To (Helpers, Utils, Internal)

The file supports a service/component but doesn't directly implement its
interface:

```go
// Package httputil provides HTTP middleware for the API gateway.
//
// Architecture: CONTRACT:S2-API-GATEWAY.1.0
package httputil
```

### Multiple Contracts

Rare, but some files bridge two contracts:

```go
// Package bridge connects the blob store to the relay.
//
// CONTRACT:C1-BLOBSTORE.2.1
// CONTRACT:C2-RELAY.1.0
package bridge
```

## Contract Review Checklist

When reviewing a PR that touches contracts:

### New Contract (type: `contract`, creates file in `architecture/`)

- [ ] Has a BDD source reference (`product/features/` or equivalent)
- [ ] Defines interfaces with behavioral contracts, not just signatures
- [ ] Defines error contracts (which errors, when, with what codes)
- [ ] Has a "Test Requirements" section
- [ ] Registered in CONTRACT-REGISTRY.md
- [ ] ID follows naming convention (S/C/I/P prefix)
- [ ] Version starts at 1.0

### Modified Contract (type: `contract`, changes existing file)

- [ ] Version bumped appropriately (minor for additive, major for breaking)
- [ ] `grep -rn "CONTRACT:{old-id}"` run — all implementing code updated
- [ ] Old version marked `SUPERSEDED BY`
- [ ] Breaking changes documented in Change History
- [ ] Contract tests updated to cover new behavior

### Implementation PR (references contract, doesn't change it)

- [ ] File header declares correct `CONTRACT:` reference
- [ ] Behavior matches contract (not just "compiles")
- [ ] Error types match contract's error table
- [ ] Contract tests pass (not just unit tests)
- [ ] No behavior introduced that isn't covered by the contract

## Companion Files

Contracts may have companion files that hold tribal knowledge — implementation
notes, debugging tips, performance characteristics, migration guides. Companion
content supports the contract but doesn't define behavior.

### Naming Convention

```
CONTRACT-{ID}-{NAME}.impl.md
```

The companion filename has NO version number. One companion per contract ID,
regardless of how many versions exist. When a contract is bumped from 1.0 to
2.0, the same companion file serves both — update it to reflect the new version.

**Examples:**
- `architecture/CONTRACT-C1-BLOBSTORE.impl.md` — companion for C1-BLOBSTORE
- `architecture/CONTRACT-S2-API-GATEWAY.impl.md` — companion for S2-API-GATEWAY

### Rules

- Companion edits don't affect contract lifecycle (no version bump needed)
- Companions are optional — only create when there's tribal knowledge to capture
- Never put behavioral specifications in companions — those belong in the contract
- Companions should reference the contract they support: start with
  `Companion for: CONTRACT-{ID}-{NAME}`

## Discovery Tracking

Discoveries are findings about the gap between contracts and reality. They
live in the `## Discoveries` section of `TODO.md` and are parsed by the
Steward (`scripts/steward.sh`).

### Discovery Types

| Type | Meaning | Who Resolves |
|------|---------|-------------|
| **BUG** | Behavior contradicts a contract | Developer (fix code) |
| **DISCOVERY** | Behavior exists but no contract covers it | Architect (write contract) |
| **DRIFT** | Behavior matches contract literally but misses intent | Architect + Developer |
| **DISPUTE** | The contract itself is wrong or needs updating | Architect + Product |

### Format in TODO.md

```
- [ ] **TYPE** `CONTRACT:ID` — Description of the finding
```

Use `none` instead of a contract reference if no contract covers the behavior.

### Lifecycle Status Definitions

Contract lifecycle is computed by the Steward, never declared manually:

| Status | Criteria |
|--------|----------|
| **DRAFT** | Contract file exists but is missing required sections |
| **ACTIVE** | All required sections present, no implementing files found |
| **TESTING** | Has implementing files, but no test files found |
| **VERIFIED** | Has implementing files AND test files |

Required sections for spec gate: Interfaces, Behavioral Contracts, Error Contracts,
Test Requirements, Implementing Files.
