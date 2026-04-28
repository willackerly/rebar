# Feedback: Cross-Repo Contract Federation — Design Proposal

**Date:** 2026-04-28
**Source:** maintainer-direct design conversation post v2.1.0 UX pass
**Type:** missing-feature | proposal
**Status:** implemented (CHARTER + tooling landed across 5 commits 2026-04-28 eve)
**Template impact:** `CHARTER.md` (new §1.6), `templates/project-bootstrap/` (new `CONSUMES.md`), `scripts/`, `cli/cmd/contract.go`, `architecture/README.md`
**From:** maintainer-direct (assistant-conducted design exploration)

## What Happened

Post v2.1.0 UX pass shipped, maintainer asked for a strategy on cross-repo
contract management: how do owners + consumers coordinate when a contract
spans projects? Specifically: clear ownership, consumer self-registration,
notification on revision, fork-friendliness for consumer-needed extensions,
and a clean reconciliation flow.

## What Was Expected

A design that:
- Resolves the sovereignty-vs-alignment tension every federated protocol hits
- Reuses existing rebar primitives (CHARTER, INVENTORY, featurerequest, FR-*.md)
- Stays within CHARTER constraints (§2.4 not a knowledge graph, §2.8 not
  real-time, §2.9 bounded write surface)
- Makes forking the *default state* — consumers free to extend without
  asking permission — while making divergence *visible* via tooling
- Avoids new servers, daemons, central registries

## Suggestion — six load-bearing principles

Federation as a **discipline**, not infrastructure:

### §1. Every repo is sovereign
Owns its contracts, its velocity, its priorities, its CHARTER. No central
authority. Cross-repo coordination is opt-in, asynchronous, and fully
reversible (you can stop consuming at any time by removing the declaration).

### §2. `CONSUMES.md` is the only cross-repo declaration
A repo that depends on another repo's contract declares it explicitly:

```markdown
# CONSUMES — external contract dependencies

## rebar/C1-AGENTS.2.0
- **owner_repo:** rebar
- **contract_id:** C1-AGENTS
- **version_pinned:** 2.0.0
- **pin_date:** 2026-04-28
- **rationale:** orchestration layer for our worktree fan-out

## blindpipe/C2-ZK-VERIFY.1.3
- **owner_repo:** blindpipe
- **contract_id:** C2-ZK-VERIFY
- **version_pinned:** 1.3.0
- **pin_date:** 2026-04-15
- **rationale:** ZK proof verification for receipts
```

Or as YAML frontmatter on individual implementation files (less verbose
for repos with many cross-deps; either form is canonical).

**Mandatory semver** for cross-repo contracts (CHARTER §1.6 amendment).
Without semver, drift-check is intractable. Single-repo contracts can
still use any versioning scheme they like.

### §3. No `extends:` field — composition over inheritance
A consumer that needs more than the owner provides writes their *own*
contract (`architecture/CONTRACT-C2-MYTHING.1.0.md`) that documents in
prose how it relates to the upstream. Local additions are local
first-class contracts.

**Why no `extends:`:**
- *Inheritance form* implies behavioral substitutability (Liskov), which
  rebar contracts can't guarantee across repo boundaries — owner can't
  see consumer's extension, consumer can't see owner's future revision
- *Reference form* is unverifiable (consumer can claim `extends:` while
  silently changing base behavior) — decorative metadata, not a contract
- *Both forms* tempt tooling to "merge base + extension to synthesize the
  effective contract," which is code-generation by another name (CHARTER
  §2.5 violation)
- *Composition* is more honest: the local extension is its own contract,
  documented in prose, with full sovereignty

### §4. All cross-repo communication uses the featurerequest gate
No new server. No new protocol. No daemon. Notifications, upstream
proposals, breaking-change notices — all flow through the existing
`ask_<repo>_featurerequest` MCP intake. Reuses CHARTER §2.9's bounded
exception; doesn't create a new one.

### §5. Notifications are an async outbox, surfaced in `rebar status`
Owner controls flush timing. Consumers triage on their own schedule. No
real-time, no webhooks (CHARTER §2.8). Outbox lives at:

```
architecture/.state/pending-notifications.md
```

Auto-populated by a post-commit hook (NOT pre-commit — non-blocking) when
a semver bump is detected on a contract that has known consumers. Owner
runs `scripts/flush-notifications.sh` at their convenience to send via
`ask_<consumer>_featurerequest`.

`rebar status` surfaces "N pending consumer notifications" so the queue
is *visible without being noisy*.

**Coalescence rule:** if owner bumps 1.0 → 2.0, queues, then bumps to 2.1
before flushing, the consumer gets *one* notification ("you're at 1.0,
current is 2.1") with diff against pinned version, not two.

### §6. Reconciliation is owner-pulled, never auto-merged
A consumer with a local extension that wants upstreaming files an FR via
the existing flow:

```bash
rebar contract upstream <my-extension-contract>
# → opens FR in owner repo: "consider this pattern for v3.0 of <upstream>"
```

Owner reviews + decides. Extensions that don't upstream live forever —
**that's fine**, and `rebar contract drift-check` makes the divergence
*visible* (deliberate state, not forgotten state).

---

## Proposed CHARTER §1.6 amendment

Append to `CHARTER.md` §1 (IS-positive section):

```markdown
### §1.6 Cross-Repo Contract Federation
A discipline (not infrastructure) for coordinating contracts across
multiple rebar-adopting repos: explicit consumer declaration via
`CONSUMES.md`, mandatory semver for cross-repo contracts, async
notification via the existing featurerequest gate (no daemons, no
central registry), and owner-pulled reconciliation of consumer
extensions. Forking is the default state; divergence is made visible
via `rebar contract drift-check`. Composition over inheritance — local
extensions are local first-class contracts, not `extends:` metadata.
```

Append to CHARTER.md §2 (IS-NOT section):

```markdown
### §2.10 Not a federation registry / package manager
rebar does not host a central index of contracts, consumers, or
versions. Cross-repo discovery is local-machine derivation from
`CONSUMES.md` declarations across known sibling repos. There is no
"rebar federation server," no auto-update mechanism, no central
authority. A consumer can declare and forget; an owner can publish and
forget. Reconciliation requires deliberate maintainer action on both
sides.
**Out of scope:** "add a contract registry server," "auto-update
consumers when owner revs," "centralized federation index," "automatic
extension upstreaming."
```

---

## Proposed deliverables

If approved, ~500 lines of scaffolding + docs:

| Artifact | Lines | Purpose |
|---|---|---|
| `CHARTER.md` §1.6 + §2.10 amendments | ~40 | Make federation pattern an explicit charter principle |
| `templates/project-bootstrap/CONSUMES.md` template + format spec | ~50 | Per-repo dependency declaration |
| `scripts/check-version-bump.sh` (post-commit hook, non-blocking) | ~70 | Detect semver bumps on contracts with known consumers; append to outbox |
| `scripts/scan-consumers.sh` | ~60 | Owner-side: derive consumer list from local-machine federation by greping CONSUMES.md across known repos |
| `scripts/flush-notifications.sh` | ~80 | Iterate outbox; file FRs via `ask_<consumer>_featurerequest`; mark sent |
| `architecture/.state/pending-notifications.md` schema doc | ~30 | Outbox format spec (one-entry-per-pending notification) |
| `cli/cmd/contract.go` — new `drift-check` subcommand | ~150 | Compare pinned versions in CONSUMES.md vs upstream's current version; flag deltas; CI-friendly exit codes |
| `cli/cmd/contract.go` — new `upstream` subcommand | ~80 | File an FR in owner repo proposing your local extension be absorbed |
| `architecture/README.md` updates | ~30 | Document cross-repo conventions + naming |
| Update `rebar status` to surface pending notifications | ~20 | Visibility without noise |
| Charter compliance updates to `scripts/check-compliance.sh` | ~30 | If repo has CONSUMES.md, validate format |

**Plus** an INVENTORY entry promoting the related Watchlist items:
- "Cross-repo `CONTRACT:namespace/ID` syntax" (Office180) → covered by §1.6
- "Contract impact DAG" (filedag) → partially covered (drift-check)
- "Contract catalog (git-repo-based)" (Office180 + scalability-deep-review) → **Rejected as superseded** by this design (was Tier 3-4 anyway; consumer-self-declaration replaces catalog-collection)

---

## Red-team survivors (failure modes that matter)

These are the concerns that survived the design conversation and need
explicit answers in the implementation. Not blockers — open questions
the implementation must address:

🟡 **Notification fatigue.** Popular owners get FRs from every adopter on
every revision. Mitigation: severity tagging at `flush-notifications.sh`
time (`breaking | additive | doc-only`), per-consumer batching, and
owner-side coalescence so multiple bumps queue as one delta.

🟡 **Pin-then-forget.** Consumers who don't run drift-check silently age
out. Mitigation: drift-check must run in CI by default for any repo with
a CONSUMES.md (template includes a GitHub Actions step). Without CI
integration, drift-check is decorative.

🟡 **Discovery doesn't scale beyond local machine.** Owner can't notify
consumers it doesn't know about. **This is an accepted constraint, not a
bug** — solving it requires a central registry (CHARTER §2.10 forbids).
Document explicitly: "rebar federation discovery is local-machine, by
design. For cross-machine federation, run `scan-consumers.sh` with an
explicit repos list."

🟡 **Trust model is wide open.** Consumer self-declarations are unsigned;
a malicious repo could spam an owner with bogus FRs. **Defer until abuse
appears.** Featurerequest already has CHARTER §3 gates that reject
out-of-scope asks, and the no-auto-commit rule means any spam lands as
untracked files for batch review. If actual abuse materializes, add an
allowlist later.

🟡 **Versioning tie-breakers in coalescence.** What counts as the
"effective notification" when a contract bumps 1.0 → 2.0 (breaking)
→ 2.0.1 (patch) before flushing? Probably: report owner's *current*
version with diff against consumer's pinned. The intermediate states
aren't load-bearing.

🟡 **What if owner unpublishes a contract?** Consumer's CONSUMES.md
references vanish silently. drift-check should distinguish "version
ahead" from "contract removed" and emit different exit codes. Implementation
detail.

🟡 **CHARTER §2.7 risk: "competing-doctrine hub."** This proposal makes a
strong opinion ("the rebar federation way is X"). Need to clearly state
this is THE rebar pattern, not "one option among many." Otherwise we get
factional forks of the federation pattern itself.

🟢 **Bootstrapping problem.** First adopter has no one to register
against. **Acceptable** — federation is an opt-in benefit; first adopter
just doesn't use the cross-repo features yet. The infrastructure exists
when needed.

🟢 **Owner attention is a bottleneck.** Reconciliation requires owner to
triage upstream-extension FRs. If they don't, extensions accumulate.
Same scaling problem rebar's own INVENTORY already has — *not new*.
Acceptable.

---

## Open questions for maintainer

1. **CHARTER amendment text — accept as drafted, or revise?** The §1.6
   addition is the most consequential — it becomes load-bearing for all
   future federation features. Worth careful read.

2. **Mandatory semver — is the constraint named correctly?** Should the
   charter say "semver MUST be used for cross-repo contracts" or "version
   schemes MUST be declared"? The former is stricter (more
   interoperable); the latter is more permissive (each owner picks).
   I lean strict.

3. **`CONSUMES.md` location/format — one file with sections, or per-dep
   files in `consumes/` directory?** One file is simpler for small
   adopters; per-file scales better and tracks change history per-dep.
   I lean: one file for now, per-file as a future evolution if N>10.

4. **`drift-check` exit codes — what should CI do?** Suggest:
   - exit 0 if all consumed contracts pinned to current upstream
   - exit 1 if any consumed contract is N+ minor versions behind
   - exit 2 if any consumed contract is upstream-removed
   - exit 0 with warning if N=0 minor versions behind (current)

5. **Notification severity classification — manual or scripted?** Owner
   classifies severity per-revision (manual, more accurate), or
   `flush-notifications.sh` diffs the contract and infers (scripted, less
   accurate but lower friction). I lean scripted with manual override.

6. **INVENTORY treatment — promote related Watchlist items, reject
   contract-catalog?** The Office180 cross-repo syntax + filedag impact
   DAG are precursors to this design; the contract-catalog proposal is
   superseded. Worth explicit closure entries.

7. **Adoption path for existing rebar adopters.** TDFLite, blindpipe,
   filedag, etc. don't have CONSUMES.md today. Should they retroactively
   declare their cross-repo deps, or only do so when they hit a real
   notification need? I lean: opportunistic — declare when needed, not as
   a Tier-promotion gate.

---

## Provenance notes

Design conversation ran ~2 hours post v2.1.0 UX pass. Key decisions
along the way:

- **Mandatory semver** — accepted (maintainer's call)
- **Kill `extends:`** — assistant red-teamed both inheritance and reference
  forms; both fail (substitutability or unverifiability). Composition
  via local first-class contracts wins.
- **Notification model** — maintainer proposed: scripted surface, async
  satisfy, non-blocking. Refined: outbox lives at
  `architecture/.state/pending-notifications.md` (not TODO.md, which has
  user-attention semantics). Coalescence on multiple bumps before flush.
- **Reconciliation** — never auto-merge; always owner-pulled via FR.
  Permanent extension divergence is an *acceptable end state*, not a
  failure mode.
- **No new infrastructure** — no daemon, no server, no central registry.
  Federation as discipline + a few scripts + the existing
  featurerequest gate.

This file lands as a research artifact. No code in this commit. Per the
propose-then-ship cycle that worked well for the featurerequest landing
(2026-04-28), maintainer reviews, decides which open questions to
resolve which way, and the implementation follows in subsequent commits.

---

## Disposition (maintainer-filled)

- [x] **Accepted with modifications** — proceed with deliverables list
- [ ] Accepted as designed
- [ ] Deferred
- [ ] Rejected

**Triaged:** 2026-04-28 by maintainer (Will)
**Implemented across 5 commits the same day:**
- e79c56b — Cluster 1: CHARTER §1.6 + §2.10 + INVENTORY closures
- 614b353 — Cluster 2: CONSUMES.md template + format spec
- (TBD)   — Cluster 3: Owner-side scripts + outbox
- (TBD)   — Cluster 4: Consumer-side rebar commands
- (TBD)   — Cluster 5: Compliance gating + docs (this commit)

**Resolutions on the 7 open questions:**

1. **CHARTER amendment text** — accepted as drafted ("looks EXCELLENT"); shipped verbatim.
2. **Mandatory semver** — strict (per-CHARTER text says "mandatory semver for cross-repo contracts").
3. **CONSUMES.md format** — single file with H2 sections per dep (deferred per-dep dir to future need).
4. **drift-check exit codes** — final mapping: 0 (current or minor-behind), 1 (major-behind), 2 (removed), 3 (no CONSUMES.md), 4 (parse/registry error).
5. **Notification severity** — scripted-infer with `--severity` manual override (per maintainer).
6. **INVENTORY treatment** — predecessor Watchlist items struck through with explicit closure entries; contract-catalog REJECTED.
7. **Adoption path** — opportunistic (CONSUMES.md is opt-in; no Tier-promotion gate). Compliance check fires only when CONSUMES.md is present and has entries — adopters self-select when ready. Phase 2 (warn on cross-repo CONTRACT: refs without CONSUMES.md) and Phase 3 (require) deferred until 2+ adopters use the federation in production.

**Bonus addition (maintainer-introduced):** Optional `notify_on_change` field in CONSUMES.md per-dep section — consumer-side hint that owner may use to filter dispatch list. Default behavior when absent: notify (overridable via `REBAR_NOTIFY_DEFAULT=skip`).

**Notes:** Federation lands as a discipline, not infrastructure. No daemons, no central registry, no real-time. Consumer self-declaration + owner-pulled reconciliation via the existing featurerequest gate. CHARTER §2.10 explicitly preempts the central-registry temptation. All 5 clusters delivered ~600 net lines (charter + template + 3 scripts + 2 Go commands + status integration + compliance gate + docs). No new dependencies. ci-check 13/13 green throughout.
