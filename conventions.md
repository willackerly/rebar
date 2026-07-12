# Conventions

Standard conventions for contract-driven development. Adopt these in your
project for consistency across agents and humans.

---

## Minimum Viable (Tier 1 — Solo / Small Project)

Not every project needs the full convention set. At Tier 1, adopt only these:

1. **Contract headers** — Every source file gets a `CONTRACT:` or `Architecture:` header
2. **Two-tag TODOs** — `TODO:` in code = untracked (blocks commit), `TRACKED-TASK:` = tracked in TODO.md
3. **Commit references** — Reference contracts in commits when relevant (no strict format required)

Everything below is recommended for Tier 2+ (teams, shared repos, CI enforcement).

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

### Tier Classification Guidance

**When in doubt, Tier 2 under the parent service.** If you're unsure
whether a file deserves its own Tier 1 contract or should be Tier 2
under an existing service, default to Tier 2. Track the judgment call
in `CONTRACT-GAPS.md` for later review. Promoting from Tier 2 to
Tier 1 later is cheap; premature Tier 1 contracts create maintenance
overhead.

**Non-standard file locations:** Enforcement scripts search common
source extensions (`.ts`, `.tsx`, `.go`, `.py`, `.rs`, `.js`, `.jsx`,
`.mjs`, `.cjs`) but only in standard project directories. Files in
`reference-implementations/`, `examples/`, or other non-standard
locations may not be found by orphan detection. If you have contract
implementations in unusual directories, add those paths to the search
scope in `scripts/check-registry.sh`.

**Script exclusion patterns:** All enforcement scripts (contract refs,
headers, TODOs, ground truth) must exclude directories that contain
stale or third-party code:

- `.claude/worktrees/` — agent isolation directories with stale snapshots
- `node_modules/` — vendored third-party code (may contain CONTRACT-like strings)
- `vendor/` — Go vendored dependencies
- `.git/` — Git internals

Without these exclusions, worktree artifacts produce false-positive errors
(e.g., a stale agent worktree with older CONTRACT headers fails the header
check on clean main). Add `-not -path "./.claude/*"` to `find` commands
and `grep -v ".claude/worktrees"` to grep pipelines.

### Multiple Contracts

Rare, but some files bridge two contracts:

```go
// Package bridge connects the blob store to the relay.
//
// CONTRACT:C1-BLOBSTORE.2.1
// CONTRACT:C2-RELAY.1.0
package bridge
```

**Dual-tag enforcement:** A file tagged with two contracts will satisfy
orphan detection for both. However, enforcement scripts count each file
once per contract reference — a dual-tagged file is counted as one
implementing file for each contract, which is correct. If a file
"belongs to" a service (Tier 2) AND "implements" a protocol (Tier 1),
use both tags.

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

Full procedure, marker formats, and the retirement-deadline rubric:
`practices/contract-supersession.md`.

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

### Lifecycle Status Definitions (computed)

Contract lifecycle is computed by the Steward, never declared manually:

| Status | Criteria |
|--------|----------|
| **DRAFT** | Contract file exists but is missing required sections |
| **ACTIVE** | All required sections present, no implementing files found |
| **TESTING** | Has implementing files, but no test files found |
| **IMPL-PRESENT** | Has implementing files AND test files |

Required sections for spec gate: Interfaces, Behavioral Contracts, Error Contracts,
Test Requirements, Implementing Files.

> **v3 rename:** this top state was called `VERIFIED` through v2.x. It is
> computed from *file presence* — no test has necessarily executed — so the
> old name overclaimed (`feedback/2026-06-19-trustable-status-…`). The word
> `verified` now belongs exclusively to the *declared* maturity vocabulary
> below, where it means "proven by passing tests/scenarios." Anything parsing
> steward output must migrate `verified` → `impl-present`.

## Declared Maturity (`Status:` field)

Computed lifecycle answers "what files exist." Declared maturity answers
"how much should you trust this artifact" — and only a human or agent who
has looked can answer it. Every contract carries a `Status:` line in its
header block; practice docs may carry one optionally:

```
**Status:** active
```

The vocabulary is fixed, small, and applied honestly:

| Value | Meaning |
|-------|---------|
| **stub** | Placeholder; structure exists, content is not real |
| **draft** | Real attempt, not yet reviewed/applied |
| **in-progress** | Actively being built; expect churn |
| **active** | In use; defines current behavior |
| **verified** | Active + has passing tests/scenarios proving it |

Rules:

Terminal statuses close a contract out: `superseded` (set together with
its `SUPERSEDED BY:` line), plus legacy `deprecated`/`retired`. Terminal
contracts are excluded from maturity weighting — a superseded contract
kept for its migration window neither drags nor inflates the badge.

The canonical machine-greppable form is one header line per contract:

```bash
grep -E '^\*\*Status:\*\*' architecture/CONTRACT-*.md
```

Parsers of record: `scripts/check-compliance.sh` (Check 9),
`scripts/compute-registry.sh`, `scripts/cold-start-checks.sh`.

- **No auto-detection.** Markings are declarations of honesty, not
  computations. Gates get added only when real-world failure shows the
  honor system breaking.
- **Marking down is free; marking up is a claim.** Move an artifact to
  `verified` only when you can point at the passing test or scenario.
- **Compliance weighting** (`scripts/check-compliance.sh`): <33% of
  live contracts stub-or-draft → tier badge as declared; 33–66% → badge
  annotated "— IN PROGRESS"; >66% → badge demoted one tier with the
  reason printed. Repos with no `Status:` fields at all are treated as
  pre-v3: no penalty, one advisory line. Once *any* contract declares,
  undeclared and out-of-vocabulary live contracts count as
  stub-or-draft — selective declaration cannot launder a badge.
- The two vocabularies never mix: computed lifecycle
  (draft/active/testing/impl-present) comes from the Steward; declared
  maturity (stub/draft/in-progress/active/verified) comes from people
  and agents. `verified` exists only on the declared side.

## Peer-Inbox Convention (repo-level)

Federated peer repos coordinate by depositing dated markdown memos into
each other's top-level `inbox/` directories. Field-proven in the tak
cluster (2026-07); mechanics and watcher in `practices/inbox-watch.md`.

- **Append-only.** Peers deposit new files; nobody edits or deletes an
  existing memo. This is what makes an inbox auditable and makes
  filename-diff watching a complete signal.
- **Naming:** `YYYY-MM-DD-<from>-<topic-slug>.md` — date, sending repo,
  then the topic.
- **Processed-on-read:** the receiving session reads, acts, and replies
  by depositing into the *peer's* inbox. Memos stay in place as the
  archive; state lives in the receiving repo's own docs, not in edits
  to the memo.
- **Who holds one:** repos actively exchanging cross-repo memos —
  typically the members of a coordination cluster. A repo whose intake
  is `ask` roles + feedback filings (rebar itself, for example) does not
  need one. Coordination **seats** sweep every inbox they hold at
  session start, then arm `scripts/inbox-watch.sh` as a persistent
  monitor (see `practices/session-lifecycle.md`).
- **A held inbox is a watched inbox** (federation Principle 5, ratified
  2026-07-11): every repo that holds a peer inbox arms the **same
  canonical watcher** — `scripts/inbox-watch.sh`, shipped in the
  project bootstrap — pointed at its **own** `inbox/`, at session
  start. Own inbox only (a peer's inbox self-echoes your outbound
  deposits — SOP 2026-07-06). The watcher drops a hidden
  `.inbox-watch.lock` PID file in the inbox and warns only on a genuine
  same-inbox double-watch; **gitignore that lock**
  (`**/.inbox-watch.lock`) on any repo whose `inbox/` is git-tracked —
  it is ephemeral per-watcher state, never coordination ledger, and a
  stray `git add -A` would otherwise stage a PID file (the bootstrap
  inbox ships the ignore). No repo writes its own watcher variant.
- **`reply-by:` (optional frontmatter):** a memo that needs a timely
  answer may carry `reply-by: <ISO datetime>` as a courtesy signal.
  Coordination seats may alarm on breach; absence means "human-ish
  timescales." This is coordinator policy, not doctrine — a memo
  without it is never wrong.
- **Namespace note:** this is unrelated to the ASK runtime's
  `agents/<role>/inbox/` message queues (gitignored, JSON, consumed by
  `bin/ask`). A repo-level `inbox/` holds durable peer mail; an agent
  inbox holds transient RPC. The overloaded word is historical — the
  qualifier "peer inbox" vs "agent inbox" disambiguates in prose.

## Reaching the Agent: Hook vs Skill vs Ask

Three channels put rebar in front of a working agent. Pick by binding
strength, not habit:

| Channel | Binding | Use for |
|---------|---------|---------|
| **Hook** (`SessionStart` etc., settings.json) | Deterministic — harness executes it | Anything phrased "MUST run on event X." An instruction the harness executes is qualitatively stronger than one the agent must remember. |
| **Skill** (`.claude/skills/*/SKILL.md`) | Discoverable — description loads into every session's context | Workflows and paradigms the agent should *reach for*: the cold-start ritual, filing feedback, arming the inbox watch. Skills are pointers to `practices/` — never a second copy of the doctrine. |
| **Ask** (`bin/ask`, MCP roles) | On-demand — pull, synchronous, stateless | Questions with answers: design intent, capability queries, cross-repo decisions. |

The failure mode this table prevents: writing "always do X on session
start" into CLAUDE.md prose (advisory by construction, skipped in
practice — `feedback/processed/2026-04-26-sessionstart-hook-cold-start-enforcement.md`)
when a hook could execute it, or duplicating a practice into a skill
body where it drifts.

## Cross-Repo References (`rebar:` refs)

A literal repo-relative path (`practices/inbox-watch.md`) is correct
**inside the rebar repo** — `check-doc-refs.sh` validates it. But
artifacts that **ship across repo boundaries** (skills, bootstrap
templates, peer-inbox memos, CONSUMES notes) must not carry literal
paths to files that only exist upstream: in an adopter repo they
dangle. Those artifacts use the abstract form instead:

```
rebar:<kind>/<name>
```

| Kind | Resolves to |
|------|-------------|
| `rebar:practice/<name>` | `practices/<name>.md` |
| `rebar:script/<name>` | `scripts/<name>.sh` |
| `rebar:agents/<name>` | `agents/<name>.md` |
| `rebar:convention[/<section>]` | `conventions.md` (section is informational) |
| `rebar:charter` | `CHARTER.md` |
| `rebar:doc/<name>` | `<name>.md` at repo root, else `docs/<name>.md` |
| `rebar:feedback/<name>` | `feedback/<name>.md`, else `feedback/processed/<name>.md` |

**Resolution order** (implemented identically by both resolvers of
record — `scripts/rebar-doc.sh` and `rebar doc`):

1. the current repo (a vendored/synced copy)
2. `$REBAR_ROOT`
3. a discovered checkout: `~/.rebar`, `~/dev/rebar`, `~/src/rebar`,
   `~/code/rebar`
4. otherwise: print the canonical upstream URL
   (`https://github.com/willackerly/rebar/blob/main/<path>`) and the
   matching `ask rebar <role>` hint, exit 4

**Rule of thumb for shipping artifacts:** literal path if the file
travels with the adopter set (everything under `scripts/`); `rebar:`
ref if it lives only upstream (`practices/`, `conventions.md`,
`CHARTER.md`, `feedback/`, `agents/*.md` doctrine).

## Testing Conventions

What a test *proves* (its fidelity rung), the UAKS tier, and the
closed-loop demo gate live in `practices/test-fidelity.md` — including
the `fidelity: <rung>` declaration format enforced by
`scripts/check-decay-patterns.sh`.

### The Scout Rule

Leave the test suite cleaner than you found it. See `DESIGN.md` §7 and
`AGENTS.template.md` "The Scout Rule" for the full philosophy.

### Prohibited Patterns

These patterns are **never acceptable** in committed code:

```
test.skip(...)           # No. Fix or remove.
xit(...)                 # No. Fix or remove.
xdescribe(...)           # No. Fix or remove.
@pytest.mark.skip        # No. Fix or remove.
t.Skip(...)              # No. Fix or remove.
#[ignore]                # No. Fix or remove.
```

### Accepted Conditional Patterns

When a test legitimately only applies in certain conditions:

```go
// Go: conditional skip with reason
if runtime.GOOS != "linux" {
    t.Skip("cgroup tests require Linux")
}
```

```typescript
// TypeScript: conditional describe
const describeIf = (condition: boolean) =>
  condition ? describe : describe.skip;

describeIf(process.env.HAS_GPU)('GPU rendering', () => { ... });
```

```python
# Python: conditional skip with reason
@pytest.mark.skipif(
    sys.platform != "linux",
    reason="cgroup tests require Linux"
)
```

The key difference: **unconditional skip = banned**, **conditional skip with
a reason that evaluates at runtime = acceptable**. The test runs everywhere
it can and is skipped only where it provably cannot.
