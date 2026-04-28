# Cross-Repo Contract Federation — Practice Guide

**Status:** load-bearing doctrine
**Charter anchor:** §1.6 (IS-positive) + §2.10 (IS-not)
**Source:** `feedback/2026-04-28-cross-repo-contract-federation.md`
**Implemented:** 5-commit landing, 2026-04-28 eve

This document codifies the **four principles** that govern every
federation feature in rebar. Future PRs that add federation tooling
must cite which principle they're advancing (or contesting). New
contributors should read this before proposing federation changes.

---

## Principle 1 — Federation as discipline, not infrastructure

Cross-repo coordination is achieved through *plain-text declarations*
(`CONSUMES.md`), *async outbox flushes* (`flush-notifications.sh`),
and the *existing featurerequest gate*. **There is no daemon, no
server, no central registry, no event bus.**

This is the asymmetric stance: federations that succeed (DNS, email,
git itself) are protocols + discipline. Federations that fail are
servers that need to be operated. Rebar's federation is the former.

### What this looks like in practice

- `CONSUMES.md` is a markdown file the consumer maintains by hand
  (or via `rebar contract` commands). Owner discovers consumers by
  greping registered sibling repos. No registration server.
- Notifications are append-only entries in
  `architecture/.state/pending-notifications.md`. Owner flushes when
  ready. No webhooks, no push notifications.
- Reconciliation goes through `ask_<owner>_featurerequest`. Same gate
  used for any other typed ask. No new RPC.

### Concrete forbidden things

- ❌ A `rebar federation` daemon
- ❌ A central index hosted at `federation.rebar.dev`
- ❌ Webhook callbacks on contract revision
- ❌ Real-time consumer→owner heartbeats
- ❌ A "federated steward" that auto-syncs across repos

### What CAN be added without violating this principle

- ✅ Better tooling on the *local* substrate (more CLI commands,
  smarter scan-consumers heuristics, prettier outbox formats)
- ✅ Optional GitHub Actions templates that wire the existing scripts
  into adopters' CI
- ✅ Conventions and best-practices docs

If the proposal requires a process running between repo events, it's
infrastructure, not discipline. CHARTER §2.10 forbids it.

---

## Principle 2 — Composition over inheritance

Local extensions of upstream contracts are **first-class local
contracts**, not metadata on the upstream consumption. There is no
`extends:` field in CONSUMES.md, and there will never be one.

### Why

- *Inheritance* implies behavioral substitutability (Liskov). Rebar
  contracts are behavioral — adding behavior usually breaks the
  parent contract's invariants. The relationship is "extension," not
  "subclass."
- *Reference-form `extends:`* is unverifiable — a consumer could
  claim it while silently changing base behavior. Decorative metadata.
- *Tooling temptation* — `extends:` invites "auto-merge base + extension
  to synthesize the effective contract," which is code generation
  (CHARTER §2.5 violation).
- *Composition is more honest* — a local contract documenting "we
  augment upstream X by adding Y" is fully owned by the consumer,
  fully forkable, fully reconcilable through normal upstreaming.

### What this looks like in practice

When you need more than the owner provides, write a *new* contract:

```markdown
# CONTRACT-C2-AGENTS-MYRBAC.1.0.md
> Augments rebar/C1-AGENTS.2.0 with role-based access control for
> our security model. The base contract stays unchanged; this contract
> defines the new behavior we layer on top.

## Relationship to upstream
This is an extension of `rebar/C1-AGENTS.2.0`. It adds:
- ACL evaluation before any agent action
- Audit log of denied operations
...
```

List the extension in your CONSUMES.md entry's `extension_contracts:`
field so owner can see patterns worth absorbing:

```markdown
## rebar/C1-AGENTS.2.0
- owner_repo: rebar
- contract_id: C1-AGENTS
- version_pinned: 2.0.0
- extension_contracts:
  - C2-AGENTS-MYRBAC.1.0
```

To propose upstreaming, run `rebar contract upstream <ext-path>` —
files an FR via the owner's featurerequest gate. **Never auto-merged.**

---

## Principle 3 — Self-selecting compliance

Adopters opt into the federation by adding a `CONSUMES.md`. Once they
add it (with at least one real entry), `scripts/check-compliance.sh`
requires `rebar contract drift-check` in CI. Single-repo adopters who
don't need federation see no change.

### Why

- *Mandatory federation tooling* would be hostile to single-repo
  adopters (most of rebar's user base today)
- *Optional federation tooling* without compliance gating leads to
  pin-then-forget: consumer pins v1.0, forgets, owner ships v3.0,
  consumer ages out silently
- *Self-selection* threads the needle: federation is opt-in, but the
  moment you opt in, the discipline that makes it work is required

### What this looks like in practice

- Adopter without cross-repo deps: `CONSUMES.md` doesn't exist;
  compliance Check 8 is silent. No federation tooling required.
- Adopter creates `CONSUMES.md` from template but leaves it empty
  (no `## owner/contract` sections): Check 8 reports "no entries —
  federation not yet active." Compliance still passes.
- Adopter adds first real entry: Check 8 fires; requires
  `rebar contract drift-check` to be wired into `ci-check.sh`,
  `pre-commit.sh`, `Makefile`, or `.github/workflows/`. If absent,
  compliance fails with actionable error.

### Phased adoption (deferred future work)

Phase 2 (warn on cross-repo `CONTRACT:` refs without `CONSUMES.md`)
and Phase 3 (require) are deferred until 2+ adopters use the
federation in production. Don't pre-build pressure that's not
warranted.

---

## Principle 4 — Automation tries, doesn't require

Anything we automate is **best-effort**. Failure modes degrade
gracefully to manual fallback. **No automation creates false
confidence** that something happened when it didn't.

### Why

- *Required automation* couples repos. If owner's auto-notify breaks,
  consumers age out. If consumer's auto-update breaks, they drift.
  Coupling is what we pay CHARTER §1.6 + §2.10 to avoid.
- *Best-effort automation* lets the federation lean toward the happy
  path without making it load-bearing. The 80% case gets free
  coordination; the 20% case falls back to manual.
- *False confidence is the failure mode that ends federations*.
  Adopter believes auto-notify fired but it didn't; bug propagates
  silently. The discipline is **visibility-is-non-negotiable** — if
  automation fails, the user MUST know.

### What this looks like in practice

- ✅ Post-commit hook auto-detects version bumps → falls back: owner
  can run `scripts/check-version-bump.sh` manually, or just maintain
  outbox by hand
- ✅ `rebar contract drift-check` runs in CI → falls back: maintainer
  runs it locally, or accepts pin staleness with eyes open
- ✅ `flush-notifications.sh` dispatches via featurerequest gate →
  falls back: owner emails consumers, files issues, or sends nothing
  (their call)

### Concrete forbidden things

- ❌ Auto-merge of upstream extensions into owner's contract
- ❌ Auto-bump of consumer's `version_pinned` without maintainer
  approval
- ❌ Silent failure of any automation step (must surface in `rebar
  status` or stderr)

### What this enables (future work)

The "wonderful world" loop where opportunistic automation closes the
federation circuit:

```
   owner bumps contract
         ↓
   post-commit hook queues notification (auto)
         ↓
   owner runs flush-notifications.sh (manual or scheduled)
         ↓
   ask_<consumer>_featurerequest fires (auto)
         ↓
   consumer's next CI run sees drift-check warning (auto)
         ↓
   consumer's maintainer triages FR, decides upgrade plan (manual)
         ↓
   consumer bumps version_pinned in CONSUMES.md (manual or PR-bot)
         ↓
   drift-check returns to clean (auto)
```

Each arrow is best-effort. If any step doesn't fire, the next manual
fallback exists. Future work explores making more arrows auto-fire
(see `feedback/2026-04-28-auto-federation-experiment.md` for the
proposal). All future automations must satisfy this principle:
**try, don't require; surface failure.**

### The visibility corollary

Every automated action surfaces somewhere a human (or another agent)
will see:

| Automation | Surface |
|------------|---------|
| version-bump detection | stderr message + outbox file growth |
| outbox state | `rebar status` "Federation:" section |
| drift-check failures | CI exit code + actionable text |
| FR dispatch | normal featurerequest landing in consumer's `feedback/` |
| extension upstreaming | normal FR in owner's `feedback/` |

If a future automation lands and you can't trace its effects through
this kind of surface, the implementation is wrong.

---

## How to use this doc when proposing federation features

When you (or an agent) propose adding to the federation surface, walk
through these checks:

1. **Does it create infrastructure?** (Principle 1) — if yes, it
   needs charter amendment, not just feature work
2. **Does it add `extends:`-like coupling?** (Principle 2) — find a
   composition shape instead
3. **Does it require something of single-repo adopters?**
   (Principle 3) — make the requirement self-selecting via
   CONSUMES.md presence
4. **Does it automate without surfacing?** (Principle 4) — add the
   visibility surface before shipping the automation

If the proposal survives all four, it fits the doctrine. If not, the
right path is usually a smaller, more honest version of the proposal.

---

## See also

- [`CHARTER.md`](../CHARTER.md) §1.6 + §2.10 — the load-bearing rules
- [`architecture/README.md`](../architecture/README.md) — operational
  reference (file paths, scripts, commands)
- [`feedback/2026-04-28-cross-repo-contract-federation.md`](../feedback/2026-04-28-cross-repo-contract-federation.md) — design source + 7 open-question resolutions
- [`feedback/2026-04-28-auto-federation-experiment.md`](../feedback/2026-04-28-auto-federation-experiment.md) — proposal for next push (opportunistic auto-federation)
- [`templates/project-bootstrap/CONSUMES.md`](../templates/project-bootstrap/CONSUMES.md) — format spec
- [`practices/federation-outbox.md`](federation-outbox.md) — outbox file schema
