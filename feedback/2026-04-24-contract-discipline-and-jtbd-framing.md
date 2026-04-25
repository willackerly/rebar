# Feedback: Contract discipline — what's working, what isn't, and the JTBD framing gap

**Date:** 2026-04-24
**Source:** `templates/CONTRACT-TEMPLATE.md`; `profiles/*/ci-check.sh`; `practices/contract-system.md`
**Type:** improvement
**Status:** proposed
**Template impact:**
- `templates/CONTRACT-TEMPLATE.md` — add Why/Who/Scenarios/Cross-references/Future-evolution/Retirement sections as required
- `practices/contract-system.md` — codify the "spike-first contracts" pattern proven in filedag's DP-A
- `practices/session-lifecycle.md` — note the parallel-agent contract-fanout pattern + worktree-isolation gotcha
- NEW: `practices/contract-supersession.md` — explicit migration plan + retirement deadline rubric
- `profiles/team/ci-check.sh` — propose new Tier 2/3 checks for cross-ref-to-user-stories presence + supersession-deadline tracking
**From:** filedag (`~/dev/filedag`), DP-A architectural spike retrospective 2026-04-24

## Context

filedag completed a 1-day **architectural spike phase** (DP-A) on 2026-04-24, drafting 8 new contracts (I5-RETRIEVER, P5-CHAT-ORCHESTRATION, D2-RECEIPT, I6-RECEIPT, I7-DELEGATION, T2-TDFBOT-API, P2-ABAC.3.1, O3-DEMO-READINESS) via 3 parallel worktree agents on disjoint files. Total wall-clock ~1 day; total contract LOC ~3,000.

Coming out of that spike + a same-day product/architect doc on TDFLite enhancement requests, I want to deposit honest feedback on REBAR's contract discipline as a methodology — what worked, where filedag (and likely other adopters) drift, and what the next refinement of the methodology could codify.

This is feedback in my (Claude's) voice, with Will's prompt framing: *"we aren't necessarily creating clear documented boundaries and components with independent jobs-to-be-done and defined interfaces to Rebar spec where we could."* He may be barking up the wrong tree on parts; I push back where I think he is, and agree where I think he's right.

---

## What's working in REBAR's contract system

These are not faint praise — these patterns held under real pressure and produced shippable artifacts.

### 1. The prefix taxonomy survives contact

filedag now has 8 contract types in active use: S (service), C (component), I (interface), P (protocol), D (data model), O (operational), T (integration seam). DP-A used four of them (I, P, D, O, T) in a single morning's worth of contract drafting and never had to invent a new prefix. The taxonomy partitions cleanly; new contracts find their bucket.

This was non-obvious in advance — when the D and O prefixes were introduced (filedag's 2026-04-18 audit, Phase 12), I was unsure whether a 7-letter taxonomy would feel forced. It doesn't. The discipline of "what kind of thing is this contract" produces good thinking before the first sentence is written.

**REBAR could codify** the prefix definitions in a standalone `practices/contract-prefixes.md` so adopters get the rubric upfront, not after their first three drift-causing miscategorizations.

### 2. Lifecycle-computed-not-declared works

`scripts/compute-registry.sh` deriving DRAFT/TESTING/VERIFIED/ACTIVE/SUPERSEDED from `// CONTRACT:<ID>` count + test count is a small thing that prevents a class of bugs forever. filedag had pre-REBAR contracts with `Status: ACTIVE` declared while having zero impl refs — drift between declaration and reality. After Tier 2 adoption, that drift class is mechanically impossible.

The supersession discipline (explicit `Status: SUPERSEDED` in the file content, not the registry narrative) is also right. It pushes the supersession decision to the contract-version author, where it belongs.

### 3. Pre-commit hook layering is well-judged

Tier 1 (contract refs + TODOs) → Tier 2 (+ headers, freshness, compliance) → Tier 3 (+ ground truth) is a graduated commitment that didn't break anyone's day-1 adoption while still preventing silent drift at higher tiers. The fact that Tier 2 hooks ran green on every commit during DP-A — including on conflict-resolution merges — is a quiet but real signal.

### 4. Worktree isolation enables parallel contract authoring

DP-A's three agents ran concurrently in disjoint git worktrees, each producing 2-4 contracts. Zero merge conflicts on contract files (only the registry narrative had additive conflicts, which `compute-registry.sh` regenerated cleanly). REBAR didn't invent worktrees, but the methodology's "agents work disjoint files" rule + the worktree pattern compose well.

**Worth noting in REBAR `practices/`:** the 2026-04-22 finding that Agent tool's worktree isolation is CWD-based (not absolute-path-filtered). Agents need to be prompted with relative paths only. We caught this empirically; future REBAR adopters could be told upfront.

---

## What isn't working — Will's instinct is partially right

Will's prompt was: *"we aren't necessarily creating clear documented boundaries and components with independent jobs-to-be-done and defined interfaces to Rebar spec where we could."*

He's right about a real thing. He's also slightly wrong about the framing. Both worth saying.

### Where he's right: contracts describe interfaces but not Jobs To Be Done

Looking at filedag's pre-DP-A contracts (the older ones, 2026-04-18 and earlier), most have a **"Purpose"** section like this:

> Defines the access seam filedag uses to talk to large language models for chat completion, streaming generation, and embeddings.

That's an *interface description*, not a *Job To Be Done*. It tells you *what the contract does* but not *what user need it serves*, *what changes if it doesn't exist*, or *who specifically depends on it being shaped this way*.

The result: a future reader (or future-you) revisiting the contract has to reconstruct the why from outside the document. When user assumptions shift (the user need evolves, the consumer set changes), there's no anchor for "what re-evaluates."

**filedag's DP-A contracts** did this better — each new contract had a "Why this exists" / "Who needs this" / "Scenarios" framing implicit in the spike's scoping. After Will's feedback today (2026-04-24), filedag's DP-A contracts have explicit `## Why this exists` + `## Who needs this` + `## Scenarios (illustrative)` sections, and the CONTRACT-TEMPLATE.md was updated to require them.

**Suggestion for REBAR:** make these three sections required-by-template:

```markdown
## Why this exists
[Two or three sentences in domain language. What user need does this serve?
What changes if we don't have it?]

## Who needs this
[Distinct list of consumers — other contracts, demo phases, user stories,
future expected consumers. This is the cross-reference the next reviewer
uses to ask "if assumptions change, what re-evaluates?"]

## Scenarios (illustrative)
[Two-to-three concrete walk-throughs grounded in named personas.
Each shows: who initiates, what travels through this contract,
what success looks like, what failure looks like.]
```

The third one (Scenarios) is the highest-leverage — concrete personas (Casey, Will, Maya in filedag) make the contract vivid in a way no amount of formal interface definition does. filedag's P4-DELEGATION (just refreshed) now walks three scenarios — Casey-grants-agent, cross-node-query, revocation — and the contract is ten times more legible than before.

### Where he's slightly wrong: "independent JTBD" can be in tension with composition

Will's framing implies each contract should have a *standalone* JTBD. That's right for some contracts (D2-RECEIPT has a clean standalone "frozen schema for cross-impl signature interop") but wrong for others.

I5-RETRIEVER's JTBD is *retrieve candidates*. P5-CHAT-ORCHESTRATION's JTBD is *orchestrate the chat pipeline*. They compose: P5 consumes I5 at the Retrieve stage. Asking I5 to express its JTBD without referring to the chat pipeline is forced; asking P5 to express its JTBD without naming I5 (and I3, I6, P2-ABAC) is also forced.

**The right framing**: contracts have **a** JTBD that may be composed with others. The Why/Who sections should name the composition explicitly: "I5's JTBD is to abstract retrieval sources so P5 can fan out without knowing the underlying source." That's a composed JTBD, and it's accurate.

**Suggestion for REBAR:** template guidance should encourage *composition-aware* JTBD framing, not push for false standalone-ness.

### Where he might be wrong but I'm not sure: "where we could"

Will's framing also implies filedag could have *more* contracts than it does — that some seams currently expressed as Go interfaces or HTTP routes should have explicit contracts.

Looking at filedag's current state, **I'm not sure**. Possible candidates:
- `internal/server/chat.go` is a 680-line god-handler. P5 covers its protocol but a C-prefix component contract for the handler itself might or might not help. Currently P5 + S1-API + I5 jointly describe it; ownership is reasonably clear.
- `internal/blindpipe/` is a stub; I2-BLINDPIPE.2.0 covers the cross-repo protocol, but no Go interface contract exists for filedag's local consumer side. Will eventually need one.
- `internal/abac/sources/` has `EntitlementSource` as an unsurfaced Go interface. Should it be a contract? Possibly. It's ad-hoc.

The risk of "more contracts" is **contract sprawl**: if every Go interface gets a contract, the contract registry becomes a different name for the public API. filedag's current discipline ("contract for things with cross-cutting invariants or cross-repo promotability or supersession concerns") is roughly right.

**My read:** Will's instinct is correct that some seams currently lack contracts that would benefit from them. But blanket-applying "every component has its own JTBD contract" would over-formalize. REBAR's existing rubric ("contract for invariants, lifecycles, or cross-repo promotability") is good; what's missing is **a guide for when to upgrade an ad-hoc Go interface to a contract**.

Possible REBAR rubric:
- **Promote to contract when:** the surface is consumed by ≥2 distinct callers AND has invariants those callers depend on; OR the surface is cross-repo (someone outside this repo will implement or call it); OR the surface has a non-trivial supersession story (multi-version coexistence, deprecation deadline).
- **Stay ad-hoc when:** single consumer, no cross-repo, no supersession.

filedag's `EntitlementSource` interface meets criterion 1 (multiple callers, invariants — at least 2 sources). It probably *should* be a contract. But that's a judgment call; the rubric helps make it.

---

## What's NOT in the template that should be

Beyond the Why/Who/Scenarios point above, six structural additions:

### A. Cross-references to user stories, made explicit

Most filedag contracts mention `BDD Source: <feature>.feature` in the front-matter, but few back-fill *which scenarios in user stories the contract supports*. P4-DELEGATION names "Stories 2, 4, 5"; most others don't.

**Why this matters:** when a user story drops or shifts (filedag's `product/FEDERATION-STORIES-DRAFT.md` is locked but Story 8 is "future-but-load-bearing"), the contracts that depended on that story's framing should be re-evaluated. Without explicit cross-refs, that re-evaluation set is hidden.

**Suggestion:** template adds a `## Cross-references` section listing user stories, primitives (if applicable), and informing memory files.

### B. "Future evolution" framing

Some filedag contracts are explicitly provisional (P5-CHAT-ORCHESTRATION's filter+rank stages will collapse into SQL pushdown when DB primitives mature; I5-RETRIEVER's interface evolves alongside). Without naming that, future readers can't tell what's load-bearing vs. what's expected to change.

**Suggestion:** template adds a `## Future evolution` section. Required for contracts known to be provisional; optional otherwise. Document the major-bump trigger explicitly.

### C. Supersession plan with deadline

When a contract supersedes another (filedag's P2-ABAC: 2.0 → 3.0 → 3.1, all three coexisting), the migration plan is informal. C9-ABAC.1.0 has been "retiring" for weeks and still has live impl refs — no deadline.

**Suggestion:** template adds a `## Retirement / supersession plan` section. Required for any contract that supersedes another OR is itself superseded. Includes:
- Predecessor contract ID + retirement criterion ("`grep -rn '<old-id>'` returns zero")
- Deadline (concrete date or phase boundary)
- Migration owner

**Suggestion for ci-check:** a Tier 2/3 check that flags contracts in superseded-in-progress state past their declared deadline.

### D. Spike-first contracts as a named pattern

DP-A worked because contracts were drafted *before* impl, in a dedicated phase, by parallel agents on disjoint files. The chat handler refactor that DP3-5 will do has stable interfaces to target. This is a pattern worth promoting.

**Suggestion:** add `practices/architectural-spike.md` to REBAR. Includes:
- When to spike (≥3 phases ahead will cross the same seams; risk of god-class growth)
- Spike scope rules (contracts only; runtime impl deferred)
- Parallel-agent fanout pattern (disjoint contract files; shared registry narrative as additive merges)
- Acceptance: contracts + interface stubs compile + tests pass + registry check green
- Worktree-isolation gotcha (CWD-based; relative paths only)

### E. JTBD test (the "why does this contract exist" question)

A simple ci-check: every contract MUST have a `## Why this exists` section non-empty. Catches the "interface description without motivation" anti-pattern at commit time.

Could be relaxed to "warning only" (Tier 2) and "blocking" (Tier 3).

### F. Cross-repo promotion checklist

P4-DELEGATION is cross-repo promotable. The contract has a "Cross-repo promotion notes" section listing per-project customization expectations. This pattern (one universal contract + per-project specialization contracts) is good but informal — there's no template support.

**Suggestion:** when `Cross-repo Promotability: Yes` in the front-matter, the template MUST include a section listing: invariants across projects, per-project customization points, candidate adopting repos, and the per-project specialization-contract naming scheme.

---

## Specific filedag drift that REBAR could prevent

To make this concrete, three classes of drift filedag has accumulated that better template/check coverage would have prevented:

1. **C9-ABAC retirement lag.** Marked superseded weeks ago; still has 2 live impl refs. No deadline. **Would have been caught by:** required `## Retirement / supersession plan` with a concrete deadline + Tier 2/3 ci-check on staleness.

2. **I3 number collision.** `I3-LLM-CLIENT.0.1` and `I3-SCANNER.1.0` share a number. Different families, different work — but `I3` is overloaded. **Would have been caught by:** template requires a contract-ID format check. ci-check could parse the registry and flag duplicate numbers within a prefix.

3. **Contracts without scenarios.** Most pre-DP-A contracts describe interfaces beautifully but don't say *who uses this and why*. Reading them is like reading a header file without seeing any callers. **Would have been caught by:** required `## Scenarios (illustrative)` section.

Each of these drift classes is a candidate for a Tier 2/3 ci-check with structural pattern-matching. Tier 2 = warning, Tier 3 = blocking.

---

## What I'd most want REBAR to ship next

Ranked by impact:

1. **Update CONTRACT-TEMPLATE.md** with required Why/Who/Scenarios/Cross-references/Future-evolution/Retirement sections. This is the single highest-leverage change — it raises the floor for every future contract in every adopting project.

2. **Add `practices/architectural-spike.md`.** Codify the spike-first pattern. Other projects can follow filedag's DP-A as a worked example.

3. **Add `practices/contract-supersession.md`.** Migration plan + deadline rubric. Force-functions C9-class drift not to accumulate.

4. **Add JTBD-presence ci-check.** Tier 2 warning, Tier 3 blocking.

5. **Add prefix-number-uniqueness ci-check.** Catches I3-collision-class problems.

6. **Document worktree-isolation gotcha** in `practices/parallel-agent-orchestration.md`. CWD-based; relative paths only.

7. **Cross-repo promotion checklist as template requirement** when `Cross-repo Promotability: Yes`.

I'd ship 1-3 first. They're the structural raise. 4-7 are mechanism.

---

## On REBAR's value generally

To be clear: **REBAR is the reason filedag's contract discipline is as strong as it is.** Pre-REBAR, filedag had ad-hoc Go interfaces and no registry. Post-Tier-2 adoption, it has 30+ contracts, computed lifecycles, pre-commit hooks catching real drift, and a successful 3-agent parallel spike. The improvement curve has been steep.

The feedback above is *next-mile* refinement, not foundational critique. REBAR works. The question is whether it can codify the patterns filedag is now stress-testing (spike-first, parallel contract fanout, supersession discipline, JTBD framing) so the next adopting project doesn't have to re-derive them.

I think it can.

## What filedag offers back

If REBAR wants worked examples for any of these patterns, filedag has them now:

- **CONTRACT-TEMPLATE.md (refreshed 2026-04-24)** — `~/dev/filedag/architecture/CONTRACT-TEMPLATE.md` is a working draft of the proposed sections.
- **DP-A retrospective** — `~/dev/filedag/agents/findings/2026-04-24-dp-a-{1,2,3}-summary.md` document the spike's mechanics.
- **8 worked Why/Who/Scenarios contracts** — D2/I5/I6/I7/P2-ABAC.3.1/P4/P5/T2 in filedag's `architecture/`.
- **Worked supersession** — P2-ABAC 3.0 → 3.1 with explicit retirement criterion (`grep -rn "P2-ABAC.3.0"` returns zero).
- **Worked cross-repo promotion** — P4-DELEGATION's per-project customization table (filedag, blindpipe, TALOS, OpenDocKit, TDFLite).

Pull any of these into REBAR's templates/practices wholesale.

## Suggestion (concrete)

Pick one item from §"What I'd most want REBAR to ship next" above and update its template/practice. The Why/Who/Scenarios section refresh in CONTRACT-TEMPLATE.md is the smallest single change with the largest blast radius. Maybe 30 minutes of REBAR-side work.

If you (REBAR maintainer) want to discuss before shipping, filedag's contract refresh from today is the corpus to read. The TEMPLATE update at `~/dev/filedag/architecture/CONTRACT-TEMPLATE.md` is a draft you could mirror line-for-line into REBAR's `templates/CONTRACT-TEMPLATE.md`.

---

**Reviewing voice:** this feedback was written by me (Claude Opus 4.7) at Will's instruction, with explicit framing that he "may be barking up wrong trees." I've pushed back where I think he's wrong (composition-aware JTBD, "more contracts" risks sprawl) and agreed where I think he's right (Why/Who/Scenarios genuinely missing; supersession discipline genuinely loose).
