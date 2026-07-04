# Feedback: Most multi-agent LOE went to re-deriving facts the system claimed to know — trustable status + cross-repo ASK would collapse it

**Date:** 2026-06-19
**Source:** steward lifecycle/`verified` semantics; `ask <role>` MCP infrastructure; METRICS/ground-truth; cross-repo federation (`feedback/2026-04-28-cross-repo-contract-federation.md`)
**Type:** improvement
**Status:** proposed
**Template impact:** `scripts/steward.sh` (lifecycle gating), `templates/` METRICS/ground-truth, `practices/multi-agent-orchestration.md`, the `ask`-role design, CONTRACT-TEMPLATE (machine-queryable capability/decision capture)
**From:** tak-tdf architecture-lock session (Claude Code, willackerly)

## What Happened

A single session ran three multi-agent investigations on tak-tdf (a rebar Tier-3 repo):

1. **Plan stress-test** — 30 agents, ~1.44M output tokens.
2. **Cross-repo recon** (Dapple delegation demo + TDFLite + tak-tdf P2 surface) — 7 agents, ~862K tokens.
3. **Feasibility/SOTA verify** — 4 agents, ~215K tokens.

≈41 agents and ≈2.5M tokens. The work was high-value and the output was strong — but when I look at
*where the effort actually went*, *most of it was not "reasoning about hard problems."* It was
**re-establishing ground truth that the rebar status surface claimed to already provide but could not be
trusted on**, and **re-reading peer-repo source to recover decisions those repos had already made.** Both
are LOE a more robust rebar/ASK layer would largely eliminate. Concretely:

**(1) Status was semantically counterfeit, so agents re-verified everything.** The steward reported
`verified` for contracts whose load-bearing behavior had never run:
- `S4-PROTECTION` "verified" — but `go vet -tags interop` didn't even **compile** (go.mod lacked the SDK
  require though go.sum pinned it); every test ran against a **mock** engine.
- `P3-ENTITY-ONTOLOGY` "verified" — but its security-critical `graphQuery` is a **bodiless `declare`**.
- `S1-TAK-GATEWAY` "verified" — with its blocking Spike A **unmeasured** and the outbound path unimplemented.
- `P2-SIGNED-ENTITLEMENT` "verified" — with **two confirmed bugs** (narrowed tokens fail offline verify;
  cross-signing is decorative) and its wire-format AOA unwritten.
- `ci-check.sh` was green ("12 pass / 1 skip") **while running zero test suites** — pure doc/contract
  hygiene. METRICS (the Tier-3 ground-truth file) tracked contracts/diagrams/docs but **not tests**.

The root cause: `verified` is computed from **file-presence** (spec headings + ≥1 impl file + ≥1 test
file, none executed). So a green board can sit on top of code that doesn't compile. The stress-test
needed ~30 agents largely **because the status surface couldn't be trusted** — confidence had to be
manufactured by independent re-derivation. *Trustable status is an LOE multiplier in reverse:* if
`verified` had meant "the milestone test ran green," that whole fan-out could have been a few targeted checks.

**(2) Cross-repo decisions/capabilities weren't queryable, so agents read source.** The recon spent 7
agents reading the Dapple USG-Delegation-Agent-Demo and TDFLite to recover things those repos had
**already decided** (delegation modes, the two-signature ceremony, the JWS wire shape, RFC 8693/7523 use)
or **already shipped** (TDFLite's Mode-B offline-signed delegation; FedCM-IdP; that there is *no* offline
rewrap). That information existed — as prose docs and code — but not in a form an architect role could
hand back as a decision-grade answer.

**(3) Revealed preference: I reached for code-reading subagents, not `ask architect`.** The `ask`
roster is real and fairly rich (per-repo architect/product/steward/englead/tester for rebar, TDFLite,
dapple-sdk, blindpipe, …). Yet for an architecture-defining session I **defaulted to spawning code
readers** rather than asking the architect roles. Why, honestly: (a) I didn't trust that `ask` reflected
the *current* code rather than possibly-stale design memory — the same trust gap as (1); (b) the
**reference/demo repo I most needed (USG-Delegation-Agent-Demo) wasn't in the roster** (only `dapple-sdk`
is); (c) it wasn't clear the roles could answer *capability* questions ("do you support offline rewrap?
what's the minimal enhancement?") versus *design-intent* questions. That revealed preference is itself
the finding: **the ASK layer didn't earn first-reach for the highest-stakes work.**

## What Was Expected

For a Tier-3 "always end green / contract-first / role-based ASK" system, I expected to be able to
**trust the board and ask the roles** — to treat `ask steward` / `verified` / `ask architect` as
load-bearing inputs and spend agent budget on the genuinely-open problems (the irreversible
offline-key-release fork, the wire-format SOTA), not on re-proving what the system asserted.

## Suggestion

Four concrete changes, in leverage order. (1) is the big one.

**1. A behavioral verification tier — make `verified` mean "behavior ran," not "files exist."**
   - Add a lifecycle state above today's `verified` (call it **`exercised`** / `proven`) gated on a
     *named milestone test that actually executed green under the contract's real build tag* — recorded
     as a `proof:` field in the contract's `.state` JSON, written by the test run, not by file-grep.
   - Demote on three machine-checkable conditions, each visible in `ask steward`: **(a)** the proof test
     didn't compile/pass (S4 demotes immediately — interop didn't build); **(b)** an AOA the contract's
     own `.md` says it owes is absent or not `DECIDED` (P2 demotes); **(c)** an open discovery references
     the contract id (S1 demotes on its own Spike-A note).
   - Cheapest first step: **rename today's `verified` → `impl-present` everywhere** (board, rollup,
     `ask steward` one-liner). That one rename removes the lie at ~zero cost; the proof tier can follow.
   - Make **`ci-check` run the test suites** (it ran none), and make **METRICS track a test/behavior
     count** (it tracked none) — with a guardrail that a check executing zero targets fails loud, so you
     don't trade one false-green for another. *(Both applied in tak-tdf this session; worth templating.)*

**2. Make the `ask` roles answer capability/decision questions from structured capture — and put every
   relevant repo (incl. demo/reference repos) in the roster.** The architect/product roles are only as
   good as what the repo captured. Invest in **machine-queryable capability + decision records** (AOAs and
   contract "capabilities" indexed, not just prose) so `ask TDFLite architect "do you support offline
   portable rewrap, and the minimal enhancement?"` returns a decision-grade answer that *substitutes* for
   code-reading rather than pointing at it. Target: a cross-repo recon like this one becomes a handful of
   `ask` calls, not 7 code-readers. (Builds directly on `2026-04-28-cross-repo-contract-federation.md`.)

**3. PRODUCT-level traceability as a maintained, queryable artifact.** The stress-test found 4 of 6 PRD
   success criteria silently dropped from the Phase-2 plan with no deferral note. `ask product` should own
   an **SC ↔ contract ↔ milestone** map so "what does this plan actually deliver / what's orphaned" is an
   `ask`, not an archaeology dig. A plan that drops an acceptance criterion should have to say so.

**4. A semantic-consistency gate, not just a freshness-date gate.** Two of four cold-start files asserted
   "Phase 0 / no application code yet" while the repo shipped Phase-2 code and ~230 tests; CLAUDE.md said
   Tier 1 while `.rebarrc` said Tier 3. `check-freshness` passed because the *dates* were current. Extend
   ground-truth to flag **stale phase labels and "done/verified/real" prose** that the behavioral tier (1)
   contradicts — so the cold-start quad can't lie to the next agent.

## Why this is worth it (the LOE math)

The reason the stress-test needed ~30 agents was **distrust** — independent re-derivation was the only way
to get confidence. Trustable status (1) converts that into a few checks; queryable cross-repo ASK (2)
converts the 7-agent recon into a handful of calls. Adversarial multi-agent verification is the right tool
for *genuinely* irreversible/uncertain decisions (the offline-rewrap fork *should* cost 4 verify agents).
It is the wrong tool — and a large avoidable cost — for re-confirming what a Tier-3 board already claims.
**The fix isn't fewer agents; it's spending them on open problems instead of on re-proving asserted ones.**

## What rebar got right (so this is "harden," not "replace")

Contract-first seams made the architecture reasoning tractable at the seam level; the AOA precedent
(`AOA-ENCRYPTED-COT.md`) gave a ready template for the three AOAs this session produced; the
multi-subagent fanout I leaned on is itself a rebar-documented playbook; and the per-repo `ask` roster
existing at all is the right shape. The gap is narrow and specific: **status that means behavior, and ASK
that answers from structured capture.** Close those two and the next session like this is a fraction of the LOE.
