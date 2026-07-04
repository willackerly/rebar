# Spike-First Contracts — draft the seams before the implementation

**Status:** field-proven pattern (filedag DP-A, 2026-04-24 — 8 contracts,
3 parallel agents, ~1 day wall-clock, zero merge conflicts)
**Source:** [`feedback/2026-04-24-contract-discipline-and-jtbd-framing.md`](../feedback/2026-04-24-contract-discipline-and-jtbd-framing.md) §D,
[`feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md`](../feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md)

When several upcoming phases of work will cross the same architectural
seams, run a dedicated **contract-drafting spike** before writing any
runtime code: a short, bounded phase whose only deliverable is contracts.
Implementation phases then target stable, reviewed interfaces instead of
discovering the seams mid-refactor.

---

## The problem

The default failure mode without a spike: seams get discovered *during*
implementation. The first phase that touches an undrawn boundary improvises
an interface under delivery pressure; the second phase inherits it; by the
third phase there is a god-class nobody planned (filedag's 680-line chat
handler grew exactly this way). Retrofitting contracts onto grown code is
strictly more expensive than drafting them first — the improvised interface
already has callers.

## The pattern

1. **Declare the spike.** A named, time-boxed phase (filedag's was one day)
   whose scope is contracts only. Say out loud which seams it covers.
2. **Draft contracts, not code.** Each contract gets the full
   [CONTRACT-TEMPLATE.md](../architecture/CONTRACT-TEMPLATE.md) treatment —
   including the Why / Who / Scenarios sections. Drafting time is the
   cheapest moment to write the JTBD framing: the "why" is still in working
   memory, and no code exists to bias the description toward "what it does."
3. **Fan out in parallel when contracts are disjoint.** Contracts have no
   compile-time coupling, so multiple agents can draft concurrently on
   disjoint files — see mechanics below.
4. **Close with the acceptance gate.** The spike is done when the gate
   passes, not when the clock runs out.

## When to spike

Reach for a spike when **any** of these hold:

- **≥3 upcoming phases will cross the same seams.** The spike cost is
  amortized across every phase that targets the drafted interfaces.
- **A god-class is forming.** A handler/module is accreting
  responsibilities faster than its interface is being defined. Draft the
  decomposition contracts before the refactor, so the refactor has stable
  targets.
- **Parallel workstreams are about to share a boundary.** Two teams/agents
  building against an undefined seam will each invent half of it.

Skip the spike when the work is single-phase, single-seam, or the interface
is already covered by an existing contract — a spike that drafts one
contract is just "writing a contract" with extra ceremony.

## Spike scope rules

- **Contracts only; runtime implementation deferred.** The deliverable is
  `architecture/CONTRACT-*.md` files. Building the implementation inside
  the spike defeats the purpose (the contract stops being a target and
  becomes documentation of whatever got built).
- **Interface stubs are allowed** (empty Go interfaces, TypeScript types)
  when the language can verify them — "stubs compile" is a cheap
  correctness check on the contract's Interfaces section.
- **JTBD sections are written at draft time, not backfilled.** Every
  spiked contract ships with non-empty Why this exists / Who needs this /
  Scenarios sections (`scripts/check-jtbd-presence.sh` enforces this).
  Prefer *composition-aware* framing over forced standalone-ness: "I5
  abstracts retrieval sources **so P5 can fan out** without knowing the
  underlying source" is an accurate composed JTBD; pretending I5 has a
  user-facing job of its own is not.
- **New prefix numbers are claimed once, up front.** Before fanning out,
  assign each planned contract its ID from the next free numbers in the
  registry — parallel agents inventing IDs independently is how prefix
  collisions happen (`scripts/check-prefix-uniqueness.sh` catches the
  damage, but assignment-up-front prevents it).

## Parallel fanout mechanics

- **Disjoint contract files per agent.** Each agent owns an explicit list
  of `CONTRACT-*.md` files it may create; no two agents share a file. This
  is the same strict-allowlist discipline as any fanout (see
  [Multi-Agent Orchestration](multi-agent-orchestration.md) and
  [Worktree Collaboration](worktree-collaboration.md); the general fanout
  playbook lives in `agents/FANOUT_PATTERN.md`).
- **The registry is regenerated, never hand-merged.** Registry conflicts
  after merging agent branches are resolved by running
  `scripts/compute-registry.sh`, which re-derives the registry from the
  contract files on disk. filedag's DP-A had additive registry conflicts
  from all three agents and regenerated them away cleanly.
- **Personas and user stories are shared inputs.** Give every agent the
  same persona set and story corpus so the Scenarios sections stay
  consistent across concurrently-drafted contracts.

### The worktree-isolation gotcha

Found empirically (filedag, 2026-04-22): the Agent tool's worktree
isolation is **CWD-based, not absolute-path-filtered**. A subagent given an
absolute path like `/Users/you/dev/project/architecture/CONTRACT-X.md` will
happily write to the *main checkout* even though it was launched inside a
worktree. **Prompt spike agents with relative paths only** — the worktree
CWD then resolves them inside the isolation boundary.

## Acceptance gate

The spike closes when all of these are green:

- [ ] Every planned contract file exists with all required template
      sections (including Why / Who / Scenarios —
      `scripts/check-jtbd-presence.sh` passes)
- [ ] Interface stubs compile (where the language supports it)
- [ ] Existing tests still pass (the spike must not have touched runtime
      behavior — if tests broke, scope was violated)
- [ ] Registry regenerated and current (`scripts/compute-registry.sh --check`)
- [ ] No prefix-number collisions (`scripts/check-prefix-uniqueness.sh`)

## Worked example — filedag DP-A (2026-04-24)

One day, three parallel worktree agents on disjoint files, eight contracts
drafted (~3,000 LOC of contract text): I5-RETRIEVER, P5-CHAT-ORCHESTRATION,
D2-RECEIPT, I6-RECEIPT, I7-DELEGATION, T2-TDFBOT-API, P2-ABAC.3.1,
O3-DEMO-READINESS. Zero merge conflicts on contract files; only the
registry narrative conflicted (additively), and `compute-registry.sh`
regenerated it. The chat-handler refactor scheduled for the following
phases had stable interfaces to target before its first line was written.
Tier 2 pre-commit hooks ran green on every commit during the spike,
including conflict-resolution merges.

## See also

- [`architecture/CONTRACT-TEMPLATE.md`](../architecture/CONTRACT-TEMPLATE.md) — required sections, JTBD framing guidance
- [`architecture/README.md`](../architecture/README.md) — prefix taxonomy, versioning, registry
- [Contract Supersession](contract-supersession.md) — when a spike produces a new major version of an existing contract
- [Multi-Agent Orchestration](multi-agent-orchestration.md) — pre-launch audit for fanouts
- [Worktree Collaboration](worktree-collaboration.md) — merge strategy for parallel agents
