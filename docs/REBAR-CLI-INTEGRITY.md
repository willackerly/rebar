# REBAR CLI & Integrity System

## Status: Implemented (Phases 1-5)

Design for the `rebar` CLI — a unified command surface that subsumes `ask`,
enforces structural invariants, and provides hash-based integrity verification
with optional digital signature authentication.

---

## Motivation

REBAR's enforcement today is advisory. Agents are *told* to follow rules via
prompt instructions and behavioral contracts. The enforcement scripts
(`steward.sh`, `ci-check.sh`, pre-commit hooks) run in the same environment
agents can modify. The result:

- Agents can `--no-verify` past pre-commit hooks
- Agents can weaken tests to make them pass
- Agents can modify contracts to match broken implementations
- Agents can edit the enforcement scripts themselves

None of these are malicious — they're mistakes of convenience. The agent takes
the path of least resistance to "task complete." The `rebar` CLI makes the
path of least resistance go through enforced checkpoints.

### Threat Model

We defend against **accidental bypass**, not sophisticated adversaries:

- An agent that modifies tests to make them pass (lazy, not malicious)
- An agent that edits a contract to match broken code (shortcut)
- An agent that skips enforcement (convenience)
- A human who forgets to run checks before pushing

We do NOT defend against an agent that reverse-engineers salts from disk,
exfiltrates signing keys, or deliberately subverts the CLI binary itself.
Those are supply-chain security problems, not methodology problems.

---

## 1. CLI Surface

`rebar` becomes the single entry point. `ask` becomes a subcommand.

### Command Map

```
rebar init                          # Bootstrap a REBAR repo (.rebar/, agents/, scripts/)
rebar status                        # Repo health at a glance
rebar verify                        # Hash integrity check (the core primitive)
rebar check                         # Run steward + CI checks
rebar commit [files]                # Enforced commit — no --no-verify, integrity updated
rebar push                          # Pre-push checks, then push
rebar diff                          # Show what changed since last verified state

rebar ask <agent> "<question>"      # Query an agent (read-only)
rebar ask -w <agent> "<question>"   # Query with write access
rebar ask who                       # List agents
rebar ask status <agent>            # Agent health
rebar ask serve [--port N]          # Enterprise multi-repo server

rebar agent start <task>            # Launch agent in sealed worktree
rebar agent finish                  # Audit + collect agent work
rebar agent list                    # Show active agent worktrees

rebar contract create <id>          # Create new contract (hashed + signed)
rebar contract edit <id>            # Edit existing contract (re-hashed + re-signed)
rebar contract verify               # Check implementations match contracts

rebar key init                      # Generate signing keypair for this identity
rebar key list                      # List known public keys / trusted identities
rebar key trust <pubkey-file>       # Add a public key to the trust store
rebar key revoke <key-id>           # Revoke a trusted key
```

### Backwards Compatibility

`ask` continues to work as a standalone command via alias:
```bash
alias ask='rebar ask'
```

The `bin/install` script sets this up. Existing `ask` usage is unaffected.

---

## 2. Integrity System (Hashes)

Every REBAR repo has `.rebar/integrity.json` — a manifest of cryptographic
hashes for all methodology-critical files. This is the **integrity layer**:
it answers "have these files been modified outside the expected workflow?"

### What Gets Hashed

Three categories of protected files:

| Category | Files | Who Should Modify |
|----------|-------|-------------------|
| **Enforcement** | `scripts/*.sh` | Humans only, via PR review |
| **Contracts** | `architecture/CONTRACT-*.md` | Architect role |
| **Tests** | `tests/**/*.test.*`, `tests/**/*.spec.*` | Tester role |

Source code (`src/`) is intentionally NOT hashed — it's expected to change
freely. The integrity system protects the *methodology*, not the product.

### Manifest Structure

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-03-27T14:00:00Z",
  "generated_by": "rebar verify",
  "repo_id": "a1b2c3d4-...",
  "repo_salt": "hex-encoded-random-256-bit",
  "checksums": {
    "enforcement": {
      "scripts/steward.sh": {
        "sha256": "abcdef...",
        "role": "steward",
        "role_hmac": "123456...",
        "modified_at": "2026-03-27T14:00:00Z"
      }
    },
    "contracts": {
      "architecture/CONTRACT-S1-AUTH.2.1.md": {
        "sha256": "abcdef...",
        "role": "architect",
        "role_hmac": "789abc...",
        "modified_at": "2026-03-27T13:00:00Z"
      }
    },
    "tests": {
      "tests/auth.test.ts": {
        "sha256": "abcdef...",
        "role": "tester",
        "role_hmac": "def012...",
        "assertion_count": 47,
        "modified_at": "2026-03-27T12:00:00Z"
      }
    }
  },
  "ratchets": {
    "total_assertions": { "min": 70, "current": 70 },
    "contract_count": { "min": 2, "current": 2 },
    "test_file_count": { "min": 2, "current": 2 }
  }
}
```

### Role Salts

Each role gets a deterministic salt derived from the repo salt:

```
repo_salt      = random 256-bit value, created at `rebar init`
steward_salt   = HMAC-SHA256(repo_salt, "role:steward")
tester_salt    = HMAC-SHA256(repo_salt, "role:tester")
architect_salt = HMAC-SHA256(repo_salt, "role:architect")
developer_salt = HMAC-SHA256(repo_salt, "role:developer")
ci_salt        = HMAC-SHA256(repo_salt, "role:ci")
```

When a file is modified through `rebar` with a specific role:

```
file_hash = SHA256(file_contents)
role_hmac = HMAC-SHA256(role_salt, file_hash)
```

The `role_hmac` in the manifest proves the file was last written through a
`rebar` command operating under that role. If someone edits the file directly
(bypassing the CLI), the `role_hmac` won't match — they don't know the salt.

**Storage:** `repo_salt` lives in `.rebar/salt` (gitignored). When cloning,
`rebar init` generates a new salt and re-signs everything. The salt is
per-machine, not per-repo — it proves "went through the CLI on this machine,"
not "came from a specific origin."

### Ratchets

Certain metrics can only increase:

- **total_assertions**: sum of all assertion counts across test files
- **contract_count**: number of contracts in the registry
- **test_file_count**: number of test files

If a commit would decrease a ratcheted value, `rebar commit` blocks it.
Ratchets can be explicitly lowered with `rebar ratchet reset <metric> <value>`
which requires justification (logged in the manifest history).

### Verification

```bash
$ rebar verify

Integrity Check — 2026-03-27T14:32:00Z
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Enforcement scripts:
  ✓ scripts/steward.sh        hash OK, role: steward
  ✓ scripts/ci-check.sh       hash OK, role: ci
  ✓ scripts/pre-commit.sh     hash OK, role: ci

Contracts:
  ✓ CONTRACT-S1-AUTH.2.1.md   hash OK, role: architect
  ✗ CONTRACT-C1-BLOBSTORE.1.0.md  MODIFIED outside rebar CLI
    Last valid role: architect
    Current hash does not match any known role signature

Tests:
  ✓ tests/auth.test.ts        hash OK, assertions: 47 (was 47)
  ✗ tests/storage.test.ts     MODIFIED outside rebar CLI
    Assertions: 18 (was 23) — RATCHET VIOLATION: assertions decreased

Ratchets:
  ✗ total_assertions: 65 < min 70  VIOLATION

RESULT: 2 integrity violations detected
```

---

## 3. Agent Execution (Sealed Envelope)

`rebar agent` wraps agent execution with structural enforcement:

### Start

```bash
rebar agent start --role developer "implement S5-PAYMENTS"
```

1. **Creates worktree** — non-optional, always isolated
2. **Snapshots integrity** — copies `.rebar/integrity.json` as baseline
3. **Sets file permissions** — based on role:

| Role | Can Write | Read-Only |
|------|-----------|-----------|
| developer | `src/` | `tests/`, `scripts/`, `architecture/` |
| tester | `tests/` | `src/`, `scripts/`, `architecture/` |
| architect | `architecture/` | `src/`, `tests/`, `scripts/` |
| steward | (read-only) | everything |

4. **Launches agent** in the worktree with appropriate context

### Finish

```bash
rebar agent finish
```

1. **Diffs against snapshot** — which protected files changed?
2. **Checks permissions** — did the agent modify files outside its role?
3. **Computes new hashes** — signs modified files with the agent's role salt
4. **Runs ratchet check** — did assertion counts decrease?
5. **Runs steward** — full quality scan in the worktree
6. **Reports** — clean summary of what changed and any violations
7. **Cherry-picks or flags** — clean work gets picked to main; violations
   require human review

### Violation Handling

If `rebar agent finish` detects violations:

```
Agent Audit — developer task: "implement S5-PAYMENTS"
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

VIOLATIONS:
  ✗ tests/payments.test.ts was MODIFIED (role: developer, expected: tester)
  ✗ Assertion count decreased: 23 → 19

ALLOWED CHANGES:
  ✓ src/payments/handler.ts — new file, role: developer
  ✓ src/payments/types.ts — new file, role: developer

ACTION: Agent work quarantined. Run `rebar agent review <id>` to inspect.
```

The work isn't lost — it's in the worktree branch. But it won't be
auto-merged. A human (or a properly-roled agent) reviews it.

---

## 4. Enforced Git Operations

```bash
rebar commit [files...]     # Replaces `git commit`
rebar push                  # Replaces `git push`
```

### `rebar commit`

1. Runs pre-commit checks (same as `scripts/pre-commit.sh`)
2. Computes hashes for all modified protected files
3. Updates `.rebar/integrity.json` with new hashes + role signatures
4. Checks ratchets — blocks if any decrease
5. Commits (including the updated integrity manifest)

There is no `--no-verify` flag. The only bypass is `git commit` directly,
which is detectable via `rebar verify` (hashes won't have valid role HMACs).

### `rebar push`

1. Runs `rebar verify` — blocks if integrity violations exist
2. Runs `rebar check` — full steward + CI suite
3. Pushes if clean

---

## 5. Digital Signatures (Authenticity Layer)

> This section is independent of the hash integrity system above. Hashes
> verify **integrity** — "were these files modified outside the expected
> workflow?" Signatures verify **authenticity** — "which entity attests
> that they produced or approved this state?"
>
> Organizations can adopt hashes without signatures. Signatures build on
> top of hashes but never replace them. The two layers compose cleanly:
>
> ```
> Layer 0: File contents          "what is the data?"
> Layer 1: SHA-256 hashes         "has it changed?"        ← integrity
> Layer 2: Role HMACs             "did it go through CLI?" ← provenance
> Layer 3: Digital signatures     "who attests to this?"   ← authenticity
> ```

### 5.1 Purpose

The hash system answers: "this file went through the REBAR CLI under role X."
It does NOT answer: "who was sitting at the keyboard" or "which CI system
ran this check." Digital signatures close that gap.

Use cases that require signatures:

- **Audit compliance**: regulated industries need to know *which human*
  approved a contract change, not just that it went through the right CLI path
- **Multi-team trust**: Team A merges code that Team B depends on — Team B
  wants to verify that Team A's steward actually passed, signed by Team A's
  CI identity
- **Supply chain**: an open-source REBAR repo can include signatures so
  consumers can verify that releases were built from verified-integrity sources
- **Post-checkout verification**: after cloning or pulling, the CLI checks
  that every protected file has a valid signature chain — not just that
  hashes match, but that a trusted identity produced them

### 5.2 Key Management

Each identity (human, CI system, service account) has an Ed25519 keypair.

```bash
# Generate a keypair for this identity
rebar key init
  → Creates ~/.rebar/keys/private.ed25519  (never leaves this machine)
  → Creates ~/.rebar/keys/public.ed25519
  → Prints key ID (fingerprint of public key)

# Or init with a label
rebar key init --identity "alice@acme.com"
rebar key init --identity "ci-github-actions"

# Trust another identity's public key
rebar key trust ./alice-public.ed25519
rebar key trust --identity "alice@acme.com" --role architect,tester

# List trusted keys
rebar key list
  KEY_ID      IDENTITY              ROLES           TRUSTED_AT
  a1b2c3      alice@acme.com        architect       2026-03-20
  d4e5f6      ci-github-actions     ci,steward      2026-03-22
  g7h8i9      bob@acme.com          developer       2026-03-25

# Revoke
rebar key revoke a1b2c3
```

**Trust store location:** `.rebar/trusted-keys/` (committed to the repo).
This is the list of public keys the repo recognizes. Private keys never
enter the repo.

**Key-role binding:** When trusting a key, you specify which roles that
identity is authorized for. `alice@acme.com` can sign as `architect` but
not as `ci`. This is enforced at verification time, not signing time —
anyone can produce a signature, but `rebar verify --signatures` will
reject a signature from an identity not authorized for that role.

### 5.3 Signature Format

Signatures live alongside hashes in the integrity manifest. Each file
entry gains an optional `signatures` array:

```json
{
  "checksums": {
    "contracts": {
      "architecture/CONTRACT-S1-AUTH.2.1.md": {
        "sha256": "abcdef...",
        "role": "architect",
        "role_hmac": "789abc...",
        "modified_at": "2026-03-27T13:00:00Z",
        "signatures": [
          {
            "key_id": "a1b2c3",
            "identity": "alice@acme.com",
            "role": "architect",
            "timestamp": "2026-03-27T13:05:00Z",
            "hash_signed": "abcdef...",
            "signature": "base64-encoded-ed25519-signature..."
          }
        ]
      }
    }
  }
}
```

**What is signed:** The signature covers `SHA256(file_hash || role || timestamp || repo_id)`.
Including `repo_id` prevents replay attacks across repos. Including `role`
binds the signature to the role claim. The timestamp provides ordering.

**Countersignatures:** A file can have multiple signatures. This enables
workflows like:

```
1. alice@acme.com signs CONTRACT-S1-AUTH as architect (authored it)
2. bob@acme.com countersigns as architect (reviewed it)
3. ci-github-actions signs as steward (automated checks passed)
```

### 5.4 Signing Workflows

Signatures are produced at natural checkpoints — not on every file save.

**Manual signing (human attests):**

```bash
# Sign all files you modified in the current role
rebar sign --role architect

# Sign a specific file
rebar sign architecture/CONTRACT-S1-AUTH.2.1.md --role architect

# Countersign (review approval)
rebar sign --role architect --countersign
```

**Automatic signing (CLI attests):**

When using `rebar commit` or `rebar agent finish`, the CLI automatically
signs modified files with the current identity's key, IF a private key is
configured. This is opt-in — repos without keys configured skip signing
silently.

```bash
rebar commit
  → Computes hashes (Layer 1)
  → Computes role HMACs (Layer 2)
  → Signs with current identity's Ed25519 key (Layer 3, if available)
  → Commits
```

**CI signing:**

CI systems get their own keypair. The CI identity signs after checks pass:

```yaml
# .github/workflows/rebar-ci.yml
- name: Verify integrity
  run: rebar verify --strict

- name: Run checks
  run: rebar check --strict

- name: Sign verified state
  run: rebar sign --role ci --all-verified
  env:
    REBAR_PRIVATE_KEY: ${{ secrets.REBAR_CI_SIGNING_KEY }}
```

### 5.5 Verification with Signatures

```bash
# Integrity only (hashes + role HMACs)
rebar verify

# Integrity + authenticity (hashes + signatures)
rebar verify --signatures

# Strict: require signatures from specific roles
rebar verify --signatures --require-roles architect,ci

# Verify a specific file's full chain
rebar verify --signatures architecture/CONTRACT-S1-AUTH.2.1.md
```

Example output:

```
Integrity + Authenticity Check — 2026-03-27T14:32:00Z
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Enforcement scripts:
  ✓ scripts/steward.sh        hash OK, role: steward
    ✓ sig: ci-github-actions (ci) — 2026-03-26T10:00:00Z

Contracts:
  ✓ CONTRACT-S1-AUTH.2.1.md   hash OK, role: architect
    ✓ sig: alice@acme.com (architect) — 2026-03-27T13:05:00Z
    ✓ sig: bob@acme.com (architect) — 2026-03-27T13:20:00Z [countersign]
    ✓ sig: ci-github-actions (ci) — 2026-03-27T13:30:00Z
  ✗ CONTRACT-C1-BLOBSTORE.1.0.md   hash OK, role: architect
    ✗ NO SIGNATURE — unsigned by any trusted identity

Tests:
  ✓ tests/auth.test.ts        hash OK, assertions: 47
    ✓ sig: charlie@acme.com (tester) — 2026-03-27T12:00:00Z
    ✓ sig: ci-github-actions (ci) — 2026-03-27T12:05:00Z
  ✓ tests/storage.test.ts     hash OK, assertions: 23
    ✗ sig: dave@acme.com — ROLE MISMATCH (signed as developer, file requires tester)

Required roles: architect ✓, ci ✗ (CONTRACT-C1-BLOBSTORE unsigned)

RESULT: Integrity OK, 2 authenticity issues
```

### 5.6 Signature Policies

Organizations configure signature requirements in `.rebar/policy.json`:

```json
{
  "signature_policy": {
    "enabled": true,
    "require_on_commit": false,
    "require_on_push": true,
    "require_on_merge_to_main": true,

    "rules": [
      {
        "category": "contracts",
        "require_signatures_from_roles": ["architect"],
        "min_signatures": 1,
        "require_ci_signature": true
      },
      {
        "category": "enforcement",
        "require_signatures_from_roles": ["ci"],
        "min_signatures": 1,
        "require_ci_signature": true
      },
      {
        "category": "tests",
        "require_signatures_from_roles": ["tester"],
        "min_signatures": 1,
        "require_ci_signature": false
      }
    ],

    "post_checkout": {
      "auto_verify": true,
      "block_on_failure": false,
      "warn_on_unsigned": true
    }
  }
}
```

### 5.7 Post-Checkout Verification

When an organization enables `post_checkout.auto_verify`, cloning or
pulling a REBAR repo triggers automatic verification:

```bash
git clone git@github.com:acme/payments-service.git
cd payments-service

# Triggered automatically by rebar's git hook, or manually:
rebar verify --signatures

Verifying repository integrity and authenticity...

  Checking 3 enforcement scripts...  ✓ all signed by ci
  Checking 12 contracts...           ✓ all signed by architect + ci
  Checking 47 test files...          ✓ all signed by tester
  Checking ratchets...               ✓ all satisfied

  Trust chain: 3 identities, 2 roles, 62 signatures
  Last CI verification: 2026-03-27T10:00:00Z (ci-github-actions)

RESULT: Repository integrity and authenticity verified ✓
```

This is the "future iteration" the user described — an organization can
enforce that after checkout, all protected files must have valid signatures
from authorized identities. If someone pushed a commit that bypassed the
CLI, the signatures will be missing or invalid, and the next developer to
pull will see the warning.

### 5.8 Trust Model Summary

```
┌─────────────────────────────────────────────────────────────┐
│                    WHAT EACH LAYER PROVES                    │
├───────────────┬─────────────────────────────────────────────┤
│ SHA-256 hash  │ File has not been modified since last check │
│ Role HMAC     │ Modification went through rebar CLI         │
│ Ed25519 sig   │ A specific identity attests to this state   │
│ Policy check  │ Required identities have all signed         │
├───────────────┼─────────────────────────────────────────────┤
│               │           WHAT EACH LAYER COSTS             │
├───────────────┼─────────────────────────────────────────────┤
│ SHA-256 hash  │ Nothing — computed automatically            │
│ Role HMAC     │ repo_salt in .rebar/ (gitignored)           │
│ Ed25519 sig   │ Keypair per identity, trust store in repo   │
│ Policy check  │ Policy file authored + maintained by humans │
└───────────────┴─────────────────────────────────────────────┘
```

**Adoption is incremental:**

1. `rebar verify` — hashes only (zero setup beyond `rebar init`)
2. `rebar verify` with role HMACs — automatic once CLI is used
3. `rebar verify --signatures` — requires `rebar key init` per identity
4. `rebar verify --signatures --require-roles` — requires policy file

Each layer adds assurance without invalidating the layers below.
Organizations start at layer 1 and move up as their compliance
requirements demand it.

---

## 6. Implementation Roadmap

**Target: Go binary** (aligns with ASK v2 roadmap in IMPLEMENTATION.md)

### Phase 1: Foundation (days)

- `rebar` CLI skeleton (cobra or similar)
- `rebar init` — create `.rebar/`, generate repo_id + salt
- `rebar verify` — compute hashes, compare against manifest
- `.rebar/integrity.json` read/write

### Phase 2: Enforcement (days)

- `rebar commit` — hash computation + ratchet checking + commit
- `rebar agent start/finish` — worktree creation, role-based permissions,
  sealed envelope pattern
- Role HMAC computation

### Phase 3: ASK Migration (week)

- `rebar ask` — port current bash `ask` as subcommand
- Backwards-compatible `ask` alias
- `rebar ask serve` — enterprise server mode

### Phase 4: Digital Signatures (week)

- `rebar key init/trust/list/revoke` — Ed25519 key management
- `rebar sign` — produce signatures
- `rebar verify --signatures` — validate signature chains
- `.rebar/policy.json` — configurable signature requirements
- CI integration examples (GitHub Actions, GitLab CI)

### Phase 5: Ecosystem (ongoing)

- `rebar verify --signatures` as GitHub Action
- VS Code extension showing integrity status
- Dashboard for multi-repo signature coverage
- Key rotation workflows
