# rebar

Your AI agent just refactored a function that 12 other files depend on.
The tests pass. The types check. But the function's behavior now violates
an architectural decision you made last month — and nobody notices until
another agent builds on the broken assumption three sessions later.

**Rebar prevents this.** 30 minutes to set up. Zero infrastructure. Everything
is plain text — bash, markdown, grep, jq. No framework to install. Copy files
into your project, and your agents immediately understand what they can and
can't change.

---

## What You Get

| If you're... | Rebar gives you... | Setup |
|---|---|---|
| **Solo dev**, 1-3 repos | Contracts + grep-based discovery + persistent agent memory | 15 min |
| **Small team**, shared repos | + CI enforcement + automated health scans + zero-tolerance testing | 45 min |
| **Department**, cross-repo | + contract catalog + breaking change detection + shared agent knowledge | 2 hours |

Every tier is just git. No services until you outgrow 50 devs. [Details by team size](profiles/).

---

## The 5-Minute Picture

### 1. Every source file declares its contract

```go
// CONTRACT:C1-BLOBSTORE.2.1
package blobstore
```

```bash
# Find all code implementing a contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/

# Find what contract a file implements
head -5 internal/blobstore/store.go
```

### 2. Contracts define behavior, not just interfaces

```markdown
# architecture/CONTRACT-C1-BLOBSTORE.2.1.md

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| `Get` on missing key | Returns `ErrNotFound` (not generic error) |
| `Put` with empty data | Returns `ErrInvalidInput` |
| `Delete` on missing key | No-op (idempotent), returns nil |
| Concurrent safety | All methods safe for concurrent use |
```

An agent reading this knows *exactly* what edge cases to handle — and what
tests to write. No guessing. No assumptions.

### 3. Automated scans catch drift

```bash
$ scripts/steward.sh --summary
Steward: 12 contracts (0d/2a/3t/7v), 1 discovery, 6/6 enforcement passing
```

The [Steward](#quality-infrastructure) scans your codebase and derives each
contract's lifecycle from what actually exists — draft, active, testing, or
verified. Status is computed from reality, never declared manually.

### 4. Agents query role-based experts

```bash
$ ask architect "should the blobstore handle encryption or delegate to the caller?"
# Persistent session — 10 questions cost 1x context, not 10x

$ ask steward summary
# Automated health check — which contracts need attention?

$ ask product "does offline sync need a new contract?"
# Product perspective — requirements before implementation
```

---

## The Core Idea

A **contract** is a versioned markdown document that defines what a component
does, who it serves, why it exists, and how it interfaces with other
components. Contracts live in `architecture/`.

What makes contracts different from docs:

1. **Doubly-linked to code.** Code references contracts (`CONTRACT:` headers).
   Contracts list their implementing files. Go from code to spec or spec to
   code with a single `grep`.

2. **Versioned.** Breaking changes bump the major version. `grep` finds every
   file that needs updating.

3. **Behavioral.** They specify edge cases, error conditions, and guarantees —
   not just function signatures. These are what tests verify.

4. **Computed lifecycle.** The Steward derives status (DRAFT → ACTIVE →
   TESTING → VERIFIED) from what exists in the codebase. No manual tracking.

5. **Unit of autonomy.** Agents have full authority within existing contracts.
   Creating or breaking a contract requires discussion. This makes autonomy
   safe — fast within boundaries, forced pause when boundaries change.

**The four rules:**

1. Don't implement without a contract
2. Don't modify code without checking its contract
3. Don't update a contract without searching all implementations
4. Contract changes that break interfaces → plan mode

See [DESIGN.md](DESIGN.md) for the full philosophy and
[architecture/CONTRACT-TEMPLATE.md](architecture/CONTRACT-TEMPLATE.md)
for the annotated template.

---

## The Cold Start Quad

Every agent session starts by reading four files in order:

1. **README.md** — what is this project?
2. **QUICKCONTEXT.md** — what's true right now? (branch, test counts, active work)
3. **TODO.md** — what needs doing? (tasks + known issues + blockers)
4. **AGENTS.md** — how do we work? (autonomy rules, testing cascade, contracts)

Plus **CLAUDE.md** for Claude Code-specific config.

This takes 5 minutes and prevents hours of wasted effort. An agent that reads
QUICKCONTEXT before diving into code knows what branch it's on, what's
in progress, and what's blocked — instead of guessing from `git log`.

---

## Quality Infrastructure

### The Steward

An automated quality scanner that produces per-contract health reports:

```bash
scripts/steward.sh           # Full scan → JSON + markdown report
scripts/steward.sh --summary # One-line health check
ask steward summary          # Same thing, via ASK CLI
```

The Steward checks every contract for completeness (required sections),
implementation (code references), and testing (test files exist). It routes
action items to roles: draft contracts → architect, testing gaps → eng lead,
discoveries → product.

### Enforcement Scripts

Six standalone checks, each runs in <5 seconds:

| Script | What It Catches |
|--------|----------------|
| `check-contract-headers.sh` | Source files missing `CONTRACT:` headers |
| `check-contract-refs.sh` | `CONTRACT:` refs pointing to nonexistent files |
| `check-todos.sh` | Untracked `TODO:` comments (two-tag system) |
| `check-freshness.sh` | Documentation with stale freshness dates |
| `compute-registry.sh` | Registry out of sync with contract files |
| `check-ground-truth.sh` | `METRICS` file doesn't match codebase reality |
| `check-compliance.sh` | Rebar version/tier/badge/AGENTS.md sections |

Run them all: `scripts/ci-check.sh`. As a pre-commit hook: `scripts/pre-commit.sh`.

### Tier-Aware Enforcement

Not every check applies to every project. Set your tier in `.rebarrc`:

| Tier | Name | What's Enforced |
|------|------|----------------|
| 1 | Partial | Contract refs + TODOs |
| 2 | Adopted | + headers, freshness, registry, compliance |
| 3 | Enforced | + ground truth, strict steward |

Scripts automatically skip checks above your tier.

---

## Real-World Results

Rebar has been adopted by four production projects. Measured results:

| Project | What Happened |
|---------|--------------|
| **Dapple SafeSign** | 17 contracts, 169 headers stamped across 168 files, 18 worktree agents, **0 merge conflicts**, 3 hours wall clock |
| **blindpipe** | Crypto-critical ZK suite. Selective adoption (kept existing docs, added enforcement). ASK sessions save **10x context** vs ephemeral subagents |
| **OpenDocKit** | 5,824 tests, 9 simultaneous agents, progressive-fidelity OOXML renderer. Source of the Cold Start Quad and testing cascade patterns |
| **Office 180** | Multi-repo product suite. Drove cross-repo namespacing (`CONTRACT:blindpipe/C1-BLOBSTORE.2.1`) and AI-native contract frontmatter |

See [feedback/](feedback/) for detailed adoption reports.

---

## Getting Started

### Quick Start (15 minutes)

```bash
git clone <rebar-repo> /path/to/rebar
cd /path/to/your-project

# Copy the essentials
cp /path/to/rebar/README.template.md    README.md
cp /path/to/rebar/QUICKCONTEXT.template.md QUICKCONTEXT.md
cp /path/to/rebar/TODO.template.md      TODO.md
cp /path/to/rebar/AGENTS.template.md    AGENTS.md
cp /path/to/rebar/CLAUDE.template.md    CLAUDE.md
cp /path/to/rebar/DESIGN.md        DESIGN.md
cp -r /path/to/rebar/architecture/      architecture/
cp -r /path/to/rebar/scripts/           scripts/
echo "v1.2.0" > .rebar-version
cp /path/to/rebar/.rebarrc.template     .rebarrc

# Fill in your project details, then verify
chmod +x scripts/*.sh
scripts/check-compliance.sh
```

### Full Setup

See [SETUP.md](SETUP.md) for the complete guide with customization steps.

### Pick Your Profile

**By project type:** [web-app](profiles/web-app.md) | [api-service](profiles/api-service.md) | [crypto-library](profiles/crypto-library.md) | [cli-tool](profiles/cli-tool.md)

**By team size:** [solo-dev](profiles/solo-dev.md) | [small-team](profiles/small-team.md) | [department](profiles/department.md)

### Compliance

Every rebar repo declares its version and tier at the top of README.md:

```markdown
> **rebar v1.2.0** | **Tier 2: ADOPTED**
```

This is validated by `scripts/check-compliance.sh` and the Steward. It tells
anyone looking at your repo: "this project speaks rebar, here's what's enforced."

---

## Project Structure

```
rebar/
├── DESIGN.md               # The philosophy (read first for depth)
├── conventions.md               # Branch naming, commits, headers, reviews
├── SETUP.md                     # Step-by-step adoption guide
├── CHANGELOG.md                 # Version history + migration guides
│
├── # Templates (copy into your project)
├── README.template.md           # Cold Start Quad #1
├── QUICKCONTEXT.template.md     # Cold Start Quad #2
├── TODO.template.md             # Cold Start Quad #3
├── AGENTS.template.md           # Cold Start Quad #4
├── CLAUDE.template.md           # Claude Code config
├── METRICS.template             # Ground truth metrics
├── .rebarrc.template            # Tier configuration
│
├── architecture/                # Contract system + templates
├── agents/                      # Role definitions + subagent templates
├── bin/                         # ASK CLI (persistent agent sessions)
├── scripts/                     # Enforcement + quality scanning
├── practices/                   # Reference guides (E2E, deployment, orchestration, worktrees)
├── profiles/                    # Adoption guides (by project type + team size)
├── feedback/                    # Real-world adoption reports
└── docs/                        # Proposals + design documents
```

---

## Learn More

- [DESIGN.md](DESIGN.md) — the full philosophy: contracts, BDD, lifecycle, autonomy, testing cascade
- [conventions.md](conventions.md) — branch naming, commit format, file headers, discovery taxonomy
- [architecture/README.md](architecture/README.md) — contract naming, linking, and lifecycle reference
- [bin/README.md](bin/README.md) — ASK CLI reference (commands, sessions, cross-project queries)
- [docs/learnings-from-opendockit.md](docs/learnings-from-opendockit.md) — war stories from 5,800+ tests and 9 simultaneous agents
- [feedback/](feedback/) — adoption reports from blindpipe, Dapple SafeSign, Office 180
- [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) — ASK CLI roadmap (bash v0 → Go v2)
