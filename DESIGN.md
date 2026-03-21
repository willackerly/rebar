# Methodology: Contract-Driven Agent Development

**The central thesis:** Agent output quality is bounded by the quality of the
information environment agents operate in. Contracts are the operating system
of that environment. Everything else — autonomy, testing, documentation,
orchestration — runs on top of contracts.

---

## 1. Contracts Are The Operating System

A contract is a versioned, searchable document that defines **what** a component
does, **who** it serves, **why** it exists, and **how** it interfaces with
other components. Contracts live in `architecture/` and are the single source
of truth for system behavior.

**The rules are absolute:**

1. **Don't implement without a contract.** If there's no contract for what
   you're about to build, write the contract first. The contract is the spec,
   the test plan, and the architecture doc — all in one.

2. **Don't modify code without checking its contract.** Every source file
   declares which contract(s) it implements via a header comment. Before
   changing behavior, read the contract. If your change violates the contract,
   update the contract first (which triggers plan mode for review).

3. **Don't update a contract without searching all implementations.** Contracts
   are doubly-linked: code references contracts, and a grep finds all code that
   implements a contract. When you change a contract, you find and update every
   implementing file.

4. **Contracts are versioned.** Breaking changes bump the major version.
   Non-breaking additions bump the minor. Old versions are kept (marked
   superseded) so you can trace the evolution.

### Why Contracts, Not Just Code

Code tells you what exists right now. It does not tell you:
- What was **intended** (vs. accidentally implemented)
- What was **deliberately excluded** (vs. forgotten)
- What the **boundaries** are (vs. what just hasn't been tested yet)
- What the **dependencies** are (vs. what happens to work)
- **Who** this serves and **why** they need it

Contracts capture all of this. When an agent reads a contract, it understands
the design intent — not just the current state. This prevents the most
dangerous failure mode in agent-driven development: agents making changes that
are locally correct but globally wrong because they didn't understand the
architectural context.

### The Contract Lifecycle

```
1. BDD First    → Who needs this? Why? What does success look like?
2. Contract     → Formalize into a versioned architecture document
3. Implement    → Write code that references the contract
4. Verify       → Tests validate contract conformance
5. Evolve       → Update contract, search implementations, propagate changes
```

---

## 2. BDD First: Start With Who and Why

Before a contract is written, the **who** and **why** must be established.
This is non-negotiable. A contract without a user and a purpose is a
specification without a soul — technically correct but strategically aimless.

### The BDD Encoding

```gherkin
Feature: Encrypted document storage
  As a security-conscious analyst (persona: Sarah)
  I need to store documents so that the server never sees cleartext
  Because regulatory compliance requires zero-knowledge architecture

  Scenario: Upload encrypted document
    Given Sarah has classified a document as SECRET
    And Sarah has established a P2P session with Alex
    When Sarah saves the document
    Then the document is encrypted client-side with AES-256-GCM
    And the encrypted blob is stored in the blob store
    And the server never receives the plaintext
```

This BDD scenario tells you:
- **Who:** Sarah, a security-conscious analyst
- **Why:** Regulatory compliance, zero-knowledge requirement
- **What success looks like:** Server never sees cleartext
- **Constraints:** AES-256-GCM, client-side encryption

The contract that follows (`CONTRACT-S4-STORAGE.1.0`) formalizes the technical
interface, but the BDD scenario is the *reason the contract exists*. If the
scenario changes, the contract changes. If the contract changes, the code
changes. The chain is always: **who/why → contract → code**.

### Where BDD Lives

```
product/
  personas/           # who — the humans this serves
  epics/              # why — the goals at the highest level
  features/           # what — BDD .feature files with scenarios
  user-stories/       # bridge between personas and features
```

Contracts reference their source features: "Implements: `product/features/encrypted-storage.feature`"

---

## 3. The Contract System

### Naming Convention

```
CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}.md
```

| Component | Meaning | Example |
|-----------|---------|---------|
| `CONTRACT` | Prefix (searchable) | `CONTRACT` |
| `{ID}` | Unique identifier (short) | `S4`, `C1`, `I3` |
| `{NAME}` | Descriptive name | `STORAGE`, `AUTH`, `API-GATEWAY` |
| `{MAJOR}` | Breaking change version | `1`, `2` |
| `{MINOR}` | Non-breaking addition | `0`, `1` |

**ID prefixes (suggested):**
- `S` = Service (e.g., `S1-AUTH`, `S4-STORAGE`)
- `C` = Component (e.g., `C1-BLOBSTORE`, `C2-RELAY`)
- `I` = Interface (e.g., `I1-SESSION`, `I2-KEY-EXCHANGE`)
- `P` = Protocol (e.g., `P1-WIRE-FORMAT`, `P2-SIGNALING`)

**Examples:**
- `CONTRACT-S4-STORAGE.1.0.md`
- `CONTRACT-C1-BLOBSTORE.2.1.md`
- `CONTRACT-P1-WIRE-FORMAT.1.0.md`

### Contract Lifecycle Status

Lifecycle is **computed, never declared**. The Steward (`scripts/steward.sh`)
derives status from what exists in the codebase:

| Status | Criteria | What It Means |
|--------|----------|---------------|
| **DRAFT** | Missing required sections | Contract is incomplete — needs architect attention |
| **ACTIVE** | All sections present, no `CONTRACT:{id}` found in source | Spec is ready, awaiting implementation |
| **TESTING** | Implementing files exist, no test files found | Code exists, needs contract tests |
| **VERIFIED** | Implementing files AND test files exist | Contract is fully realized in code |

Required sections for spec gate: Interfaces, Behavioral Contracts, Error
Contracts, Test Requirements, Implementing Files.

The lifecycle is a quality signal, not a workflow gate. A contract in DRAFT
still has a valid ID and can be referenced in code. The status tells you
what's missing, not what's allowed.

### Discovery Taxonomy

Discoveries capture the gap between contracts and reality. They live in
`TODO.md` (Discoveries section) and are parsed by the Steward.

| Type | When | Resolution |
|------|------|-----------|
| **BUG** | Behavior contradicts a contract | Fix the code |
| **DISCOVERY** | Behavior exists but no contract covers it | Write a contract |
| **DRIFT** | Behavior matches contract literally but misses intent | Refine contract + code |
| **DISPUTE** | The contract itself is wrong | Update the contract |

Discoveries are the feedback loop that keeps contracts honest. Without them,
contracts become aspirational documents that describe what was intended, not
what exists. The Steward surfaces discoveries in per-role action items:
architects see DISPUTEs, developers see BUGs, product sees DISCOVERYs.

### Doubly-Linked References

**In code** (every source file header):
```go
// Package blobstore implements encrypted blob storage.
//
// CONTRACT:C1-BLOBSTORE.2.1
// See: architecture/CONTRACT-C1-BLOBSTORE.2.1.md
package blobstore
```

```typescript
/**
 * CryptoBridge — client-side encryption/decryption at the gateway boundary.
 *
 * @contract CONTRACT:C3-CRYPTO-BRIDGE.1.0
 * @see architecture/CONTRACT-C3-CRYPTO-BRIDGE.1.0.md
 */
```

**For helper/utility code** that doesn't implement a specific contract, reference
the parent service's contract:
```go
// Package httputil provides HTTP middleware for the API gateway.
//
// Architecture: CONTRACT:S2-API-GATEWAY.1.0
package httputil
```

**In architecture docs** (the contract itself):
```markdown
## Implementing Files
<!-- Updated by grep: `grep -rn "CONTRACT:C1-BLOBSTORE" src/ internal/` -->
- `internal/blobstore/file.go` — file-backed implementation
- `internal/blobstore/memory.go` — in-memory implementation (tests)
- `internal/blobstore/blobstore_test.go` — contract tests
```

### Searching (Zero Tooling Required)

```bash
# Find all code implementing a contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/ internal/ client/

# Find the contract doc for code you're editing
head -10 internal/blobstore/file.go  # read the CONTRACT: header

# Find all contracts
ls architecture/CONTRACT-*.md

# Find all code with ANY contract reference
grep -rn "CONTRACT:" --include="*.go" --include="*.ts" .
```

### Version Bumping

When a contract changes:

1. **Non-breaking addition** (new optional field, new method with default):
   - Bump minor: `CONTRACT-C1-BLOBSTORE.1.0.md` → `CONTRACT-C1-BLOBSTORE.1.1.md`
   - Update code refs that need the new capability
   - Old refs (`1.0`) remain valid

2. **Breaking change** (removed method, changed signature, new required field):
   - Bump major: `CONTRACT-C1-BLOBSTORE.1.1.md` → `CONTRACT-C1-BLOBSTORE.2.0.md`
   - Mark old version: add `<!-- SUPERSEDED BY: CONTRACT-C1-BLOBSTORE.2.0 -->` header
   - `grep -rn "CONTRACT:C1-BLOBSTORE.1"` → find and update ALL implementing code
   - This is a **plan mode** decision — breaking contract changes require discussion

---

## 4. Agent Autonomy Model

### Maximum Autonomy Within Contracts

Agents have full authority to write, edit, refactor, delete, test, commit, and
push — as long as they're working within existing contracts. The contracts
define the boundaries; within those boundaries, agents are unrestricted.

| Situation | Autonomy Level |
|-----------|---------------|
| Implementing an existing contract | **Full autonomy** — just do it |
| Fixing a bug within contract boundaries | **Full autonomy** — fix and ship |
| Refactoring without changing contract behavior | **Full autonomy** |
| Adding a feature that follows existing contracts | **Full autonomy** |
| Creating a new contract | **Plan mode** — discuss first |
| Modifying an existing contract (breaking) | **Plan mode** — discuss first |
| Modifying an existing contract (non-breaking) | **Full autonomy** — bump minor |
| Removing or deprecating a contract | **Plan mode** — discuss first |

### Why This Works

Contracts make autonomy safe. Without contracts, "full autonomy" means agents
can silently make architectural decisions that are hard to reverse. With
contracts, agents have clear boundaries — they can move fast within them, and
the system forces a pause when boundaries need to change.

### Trust But Verify

Agents trust the context they're given. Context drifts from reality at the
speed of code changes. Therefore:

1. **Freshness markers** on every status-bearing document. If the freshness
   date is >2 weeks old, treat all claims as suspect.

2. **Pre-Launch Audits** before fan-out campaigns. Grep for existing
   implementations before launching agents to build things that might already
   exist. (See the 50% waste incident in docs/learnings-from-opendockit.md §7.)

3. **Filesystem as source of truth.** When docs say one thing and `ls` +
   `grep` say another, the filesystem wins. Docs describe intent; the
   filesystem describes reality. Both matter, but reality takes precedence
   when they conflict.

4. **Cross-reference on cold start.** Every new agent session verifies
   QUICKCONTEXT.md claims against `git log` and the actual file tree before
   acting. This takes 2 minutes and prevents hours of wasted work.

---

## 5. The Information Environment

### The Cold Start Quad

Every agent session starts by reading four files in order:

| Order | File | Purpose | Freshness Risk |
|-------|------|---------|----------------|
| 1 | `README.md` | Universal orientation — what this project is, how to navigate | Low (changes rarely) |
| 2 | `QUICKCONTEXT.md` | Current state — branch, test counts, in-progress work | **High** (most volatile) |
| 3 | `TODO.md` | Tasks + known issues + blockers | Medium-High |
| 4 | `AGENTS.md` | Norms — how we work, testing cascade, contracts, collaboration | Low (changes with process) |

**Then:** `CLAUDE.md` for Claude-specific configuration (commands, autonomy,
allowed operations).

**Why this order:** Orientation first (README), then current state
(QUICKCONTEXT), then tasks (TODO), then norms (AGENTS). Agents must
understand the project before they understand the process.

### Anti-Drift Mechanisms

Documentation drifts from reality at the speed of code changes. Agents both
suffer from and contribute to drift. These mechanisms fight it:

| Mechanism | Where | Purpose |
|-----------|-------|---------|
| Freshness timestamps | Every status-bearing doc | Detect staleness |
| Two-tag TODO system | Code + TODO.md | Prevent invisible tech debt |
| Pre-commit TODO check | AGENTS.md | Enforce tracking before commit |
| Pre-launch audit | AGENTS.md | Verify before fan-out |
| Doc-drift-detector template | agents/subagent-prompts/ | Automated doc-vs-code audit |
| Contract version bumps | architecture/ | Force review of breaking changes |
| Feature inventories | agents/subagent-prompts/ | Prevent silent feature deletion |
| Doubly-linked contracts | Code headers + architecture/ | Bidirectional traceability |

### The Drift Chain (Why This Matters)

Observed failure pattern from real agent-driven development:

1. Agent A completes Feature X, updates QUICKCONTEXT but not TODO.md
2. Agent B reads TODO.md, sees Feature X as incomplete, wastes time investigating
3. AGENTS.md says "Feature X: in progress" — a third contradictory claim
4. Three documents disagree on the same fact
5. Trust in the documentation system collapses
6. Agents fall back to code-only, losing ability to understand intent and plans

**Cost:** Not just wasted time — lost strategic capability. Code tells you
what exists. Only trustworthy documentation tells you what was intended, what
was excluded, and what's planned.

### Numeric Claims: The Fastest Drift Vector

Quantitative documentation claims (test counts, endpoint counts, contract
counts, version numbers) drift faster than any other content. Every commit
that adds a test file, creates a contract, or bumps a version silently
invalidates numbers in multiple documents.

**The failure mode is silent success.** Tests pass. The app works. CI is
green. But "126 tests" became "586 tests" over 3 weeks, and five documents
still say 126. No mechanism detects this because the metric being wrong
doesn't break anything — it just makes documentation fictional.

**Defenses:**
1. **Structured metrics file** — A `METRICS` file with key=value pairs is
   the single source of truth for all quantitative claims. No prose matching,
   no fragile grep-against-docs. One file, one format, machine-verifiable.
2. **Ground truth script** — `scripts/check-ground-truth.sh` computes
   metrics from code and compares against the `METRICS` file. Fails on drift.
3. **Known locations** — By convention, all tests live in `tests/` and all
   contracts live in `architecture/`. Known locations make counting reliable.
4. **Cold start verification** — New sessions run the ground truth script
   before trusting QUICKCONTEXT.md claims.

---

## 6. Parallel Agent Orchestration

### Subagent Templates: Curated Context as Infrastructure

Reusable prompt templates in `agents/subagent-prompts/` encode how specific
tasks should be done. Templates work for both single invocations (one agent
does a UX review your way) and parallel fan-out (N agents each process a
shard).

**The critical insight:** Templates are just as valuable for single invocations
as for fan-out. If you've ever corrected an agent ("no, not like that — here's
how we do reviews"), that correction belongs in a template. Templates make
agents learn across sessions.

### Fan-Out Patterns

| Pattern | Description | Use When |
|---------|-------------|----------|
| **Shard** | Same task, different data slices | Tests, data processing, file-by-file analysis |
| **Map-Reduce** | Parallel map, single reduce | Per-package audits, cross-codebase searches |
| **Speculative** | Same task, different approaches | Bug diagnosis, algorithm comparison |
| **Progressive** | Wide first pass, narrow second | Security audit, code review of large codebase |

### Worktree Isolation

Parallel agents cannot coordinate in real time. Isolation is the only reliable
way to prevent conflicts:

- **Use worktrees for:** any task that modifies files
- **Use main-thread subagents for:** read-only research and validation
- **Never use worktrees for:** changes to a single shared file (merge will always conflict)

### Post-Merge Integration

Budget ~30% of agent time for post-merge fix-up:

| Agent task type | Success rate without correction |
|-----------------|-------------------------------|
| Create new files | High (no existing state to conflict) |
| Modify existing files | Medium (worktree may diverge from main) |
| Write tests for existing code | Low (~50% wrong assumptions) |

---

## 7. The Testing Cascade

**Fast inner loops, rigorous outer gates.** Never run the full suite when a
targeted test will do. Agents default to the most thorough validation they
know about unless you explicitly give them a faster option.

| Tier | Name | Speed | When |
|------|------|-------|------|
| T0 | Typecheck | <5s | Every edit |
| T1 | Targeted | <10s | Every change cycle |
| T2 | Package | <30s | Before commit |
| T3 | Cross-package | <60s | Before push |
| T4 | Visual/E2E | <2min | UI changes |
| T5 | Full suite | <10min | Release prep |

**Rules:** Iterate at T1. Promote on success. Background T3+. Never run T5
in the inner loop.

### The Scout Rule

Tests are sacred ground. A test suite tells you whether the system works. If
tests are skipped or failing, the suite is lying — and every decision based on
"tests pass" is built on that lie.

**The absolute rules:**
- No skipped tests. If a test can't run, fix it or remove it.
- No failing tests. A red test is a P0, not background noise.
- No flaky tests. Intermittent failure is still failure.

Every agent session that encounters a broken test and walks past it
contributes to the erosion. Every session that fixes one makes the codebase
stronger. This is the scout rule: leave the camp cleaner than you found it.

When a test is truly obsolete (the feature was removed, the behavior changed),
remove the test — but carefully. Verify the behavior it tested is either gone
or covered by other tests. Deleting a test without checking is as dangerous
as skipping one.

When a test only applies to certain platforms or conditions, use proper
conditional logic (`if runtime.GOOS == "linux"`, `test.skipIf(!supportsWebGL)`),
not a blanket `skip`. The test should run where it applies and be absent
where it doesn't.

---

## 8. Methodology in Practice

### New Project Setup

1. **Define personas** — who uses this and why? (`product/personas/`)
2. **Write BDD features** — what does success look like? (`product/features/`)
3. **Create contracts** — formalize the architecture (`architecture/`)
4. **Set up the Cold Start Quad** — README, QUICKCONTEXT, TODO, AGENTS
5. **Implement against contracts** — code references contracts in headers
6. **Verify with tests** — contract tests are king, the spine of the system

### Adding a Feature

1. Check: is there a contract for this? If not, write one first.
2. Update the BDD scenarios if the feature adds new user-facing behavior.
3. Implement with contract reference in the file header.
4. Add tests that verify contract conformance.
5. Update QUICKCONTEXT and TODO.

### Modifying Architecture

1. This is always plan mode. Discuss first.
2. Write the new/updated contract with version bump.
3. `grep -rn "CONTRACT:{old-id}"` to find all implementing code.
4. Update implementations to match new contract.
5. Update tests. Contract tests break first — that's by design.
6. Update cross-references in docs.

### Reviewing Changes

Use the code-review subagent template. The template checks: does this change
conform to its declared contract? Does it introduce behavior not covered by
a contract? Are contract references in file headers correct?

### Debugging with Cross-Representation Oracles

When a system has multiple implementations of the same specification, the
implementation closest to ground truth becomes an **oracle** for debugging
the others.

**The pattern:** Measure each implementation's distance from a known-correct
reference. The closest one is your oracle — use its internal state to debug
divergences in the others. Instead of guessing why Implementation B produces
wrong output, compare its intermediate state against the oracle's.

**Examples:**
- Two parsers for the same format → the more accurate one is oracle
- A reference implementation and a production implementation → reference is oracle
- Multiple rendering paths for the same input → the path closest to ground
  truth reveals what the others should be doing

This generalizes to any project where you can measure distance from ground
truth across multiple code paths.

---

## 9. The Steward: Automated Quality Scanning

The Steward is the project's technical program manager in code form. It scans
the contract system and produces per-role action items — facts, not opinions.

### What It Does

```bash
scripts/steward.sh             # full scan → JSON + markdown report
scripts/steward.sh --json      # aggregate JSON to stdout
scripts/steward.sh --summary   # one-line summary
scripts/steward.sh --check C1  # single contract
```

### What It Checks

Per contract:
- **Spec gate:** Are all required sections present? (Interfaces, Behavioral, Errors, Tests, Implementing)
- **Impl gate:** Do any source files reference this contract? Are there test files?
- **Lifecycle:** Derived from spec + impl gates (DRAFT → ACTIVE → TESTING → VERIFIED)
- **Discoveries:** Any BUG/DISCOVERY/DRIFT/DISPUTE entries in TODO.md for this contract?

Globally:
- **Enforcement:** Runs all `check-*.sh` scripts, captures pass/fail
- **Metrics:** Runs ground truth verification

### Output

All state is JSON in `architecture/.state/`:
- Per-contract: `<contract-id>.<version>.json`
- Aggregate: `steward-report.json`
- Human-readable: `STEWARD_REPORT.md`

The JSON schema is designed for a future single-file HTML dashboard that
`fetch()`es `steward-report.json` and renders it — no server, no build step.

### Who Uses It

| Role | Reads | Acts On |
|------|-------|---------|
| **Eng Lead** | Full report | Coordination, QA flow, fan-out |
| **Architect** | DRAFT contracts, DISPUTEs | Contract completion, dispute resolution |
| **Product** | DISCOVERYs, missing BDD refs | Requirements gaps |
| **Developer** | ACTIVE contracts, BUGs | Implementation, bug fixes |
| **CI** | Exit code | Blocks merge on enforcement failure |

### The QA Flow

QA is fully automated — no separate QA agent:

1. Eng Lead runs `scripts/steward.sh`
2. Reviews action items per role
3. Fans out work to developers (BUGs, implementations)
4. Escalates to architect (DISPUTEs, missing contracts)
5. Escalates to product (DISCOVERYs, missing BDD refs)
6. Re-runs steward to verify resolution

---

## Summary

| Principle | Implementation |
|-----------|---------------|
| Contracts are the operating system | `architecture/CONTRACT-*.md` with doubly-linked code refs |
| BDD first: who and why | `product/` with personas, epics, features before contracts |
| Max autonomy within contracts | AGENTS.md autonomy model |
| Trust but verify | Freshness markers, pre-launch audits, filesystem as truth |
| Information environment is infrastructure | Cold Start Quad, anti-drift mechanisms |
| Encode corrections as templates | `agents/subagent-prompts/` for repeatable tasks |
| Fast inner loops | Testing Cascade T0-T5 |
| Parallel by default | Worktree isolation, subagent templates, fan-out patterns |

---

## 10. Design Decisions

This repo was built iteratively from real agent-driven development. Each
decision emerged from a failure that the previous approach didn't prevent.

### Why contracts became the center, not docs or tests

We started with documentation templates (QUICKCONTEXT, AGENTS, TODO) and they
worked — agents oriented faster, drift decreased. But agents kept changing
code that *technically worked* but violated architectural intent. They'd
refactor a function in a way that broke an implicit contract with another
module, or add a feature that contradicted a design decision from weeks ago.

The problem: no document answered "what is this code *supposed to do*
according to the architecture?" Tests answer "does it work?" Code answers
"what does it do right now?" But neither answers "what was intended, and what
boundaries must be respected?" That's what contracts do.

### Why grep-based linking over tooling

`// CONTRACT:C1-BLOBSTORE.2.1` in code headers + `grep -rn` to find
implementations. No build plugins, no custom linters, no databases.

- **Zero adoption cost.** Any project can start using it today.
- **Tool-agnostic.** Works with any editor, any AI agent, any CI.
- **Transparent.** The linking mechanism is visible in the code.
- **Resilient.** No tool to break, update, or configure.

The value comes from the *practice* of writing and referencing contracts, not
from tooling. Start with grep. Add tooling later if scale demands it.

### Why subagent templates matter for single invocations

We built `agents/subagent-prompts/` for parallel fan-out. But the bigger
insight: **templates are just as valuable for single tasks.** When you ask an
agent to do a "UX review" without a template, it guesses what you mean. A
`ux-review.md` template encodes *your* definition — your criteria, heuristics,
and output format. If you've ever corrected an agent, that correction belongs
in a template. This is how agents learn across sessions.

### Why TODO absorbed KNOWN_ISSUES

We had QUICKCONTEXT, KNOWN_ISSUES, TODO, AGENTS, CLAUDE — five files. In
practice, KNOWN_ISSUES and TODO overlapped and agents had to maintain both.
Every additional file is a drift surface. Merging known issues into TODO as a
section reduced maintenance without losing information. Fewer files actually
maintained beats more files that drift.

### Why README is the universal first-read

Previously QUICKCONTEXT was first. But it answers "what's happening now," not
"what is this project?" An agent that dives into current state without
understanding the project's identity makes worse decisions. README provides
stable orientation; QUICKCONTEXT is volatile and tactical. The reading order
goes from stable/strategic to volatile/tactical.

### Why lifecycle is computed, not declared

We adopted lifecycle tracking from Purlin (see §11 Related Work), but with a
key difference: status is derived from what exists in the codebase (do
implementing files exist? do test files exist? are all spec sections present?)
rather than manually declared. Computed status can't drift — it's always
accurate because it's always recomputed from reality.

### Why agent commands are unquoted

`ask steward "what needs attention?"` asks a question. `ask steward` runs a
scan. The convention — quoted = question, unquoted = command — emerged from
wanting each role to own a slice of project health without requiring the full
persona invocation. Each agent has a `commands/` directory; drop a `.sh` file
in it and it's immediately available. Zero changes to `bin/ask` needed.

### How this repo was built

This repo emerged from a single conversation about Claude Code subagent
concurrency limits, evolved through prompt templates as version-controlled
infrastructure, and grew into a complete methodology for contract-driven
agent development. Each idea built on the last:

- Subagent fan-out → reusable prompt templates
- Templates → single-invocation value (not just fan-out)
- Templates need shared rules → behavioral contracts for agents
- Agent contracts → why not contracts for code?
- Code contracts → BDD first (who/why before what/how)
- Docs needed → Cold Start Quad
- Docs drifted → anti-drift mechanisms
- Different projects → profiles
- Quality gaps → Steward + enforcement scripts
- Agent roles → ASK CLI with commands

The [learnings document](docs/learnings-from-opendockit.md) captures the raw
failure analysis and war stories from 5,800+ tests and 9 simultaneous agents.

---

## 11. Related Work

### Purlin

[Purlin](https://github.com/purlin) is a spec-driven development framework
that influenced several concepts in rebar.

**Where we align:** specs before code, contract lifecycle, quality gates,
companion docs, discovery taxonomy.

**Where we diverge:**

| Aspect | Purlin | rebar | Why |
|--------|--------|-------|-----|
| Role rigidity | Fixed hierarchy, formal handoffs | Fluid roles | Small teams wear multiple hats |
| Scenarios | Gherkin-only, required | Gherkin optional | Infrastructure contracts don't benefit from Given/When/Then |
| Code philosophy | "Code is disposable, specs are permanent" | Code and contracts are co-equal | Contracts become stale when not grounded in implementation |
| Repo structure | Git submodules | Flat vendoring (proposed) | Submodules add complexity small teams can't absorb |
| Dashboard | Required web UI | JSON-first, dashboard optional | `jq` should answer what a dashboard can |
| Tooling | Purpose-built CLI | bash + jq + grep | Zero dependencies beyond a Unix shell |

**What we adopted and adapted:**
- Computed lifecycle status (derived from code, never declared)
- Discovery taxonomy (BUG/DISCOVERY/DRIFT/DISPUTE)
- Companion files (`.impl.md` for tribal knowledge)
- Role-based action items (Steward routes findings to the right role)
- Quality scanning as infrastructure (Steward = TPM in code form)

---

## 12. Future Concepts

Ideas we believe are directionally correct but haven't built yet.

### Contract Vendoring

When multiple repos implement the same contracts, each needs a local copy.
Copies drift. **Concept:** Vendor contracts like Go vendors dependencies —
`architecture/vendor/` contains read-only copies with a lock file pointing
to the upstream source. `scripts/vendor-contracts.sh` pulls updates.

### Cryptographic Contract Signing

Vendored contracts could be tampered with. **Concept:** Sign contracts when
they pass a trust checklist (BDD reviewed, architect approved, security
reviewed, tests passing, reference implementation exists). Child repos can
vendor but not modify without invalidating the signature. Matters for
high-assurance environments (financial, healthcare, defense).

### Expert Agent Hierarchy

At scale, a single orchestrating agent can't deeply understand product intent,
architecture, and all code simultaneously. **Concept:** A hierarchy of expert
agents — Product (reads BDD, answers "what and why"), Architect (reads
contracts, answers "how should it be structured"), Eng Lead (reads code,
answers "how to implement"), Engineers (subagents, do the work). Each level
reads summaries from above, detail at its own level. The Eng Lead is the
default persona you talk to.

This is partially realized through ASK roles and agent commands, but the full
hierarchy with structured inter-agent communication is future work.

See [docs/AGENT-RUNTIME.md](docs/AGENT-RUNTIME.md),
[docs/ASK-SHELL.md](docs/ASK-SHELL.md), and
[docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) for detailed proposals.
