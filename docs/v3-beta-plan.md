# v3.0.0-beta Plan

**Branch:** `v3.0.0-beta` (consolidation of `v3.0.1-alpha` + `main` +
`practice/inbox-watch` + `feedback/reflexive-push-durability`)
**Started:** 2026-07-04 (supersedes `docs/v3-alpha-plan.md`, started 2026-04-29)
**Driver:** Will Ackerly + Claude (Fable 5)
**Status of this plan:** active

## Why v3 (unchanged from the alpha plan)

v2.x landed contracts, the steward, ASK CLI, MCP wiring, federation
discipline. What it didn't address: a project can stamp `Tier 2:
ADOPTED` while every contract underneath is a stub. The compliance
check confirms artifacts *exist*, not that they're *mature*. **REBAR's
badges currently lie by default.** v3.0.0 introduces a maturity
vocabulary so honesty is declared per-artifact and the badge reflects
reality.

Major bump because:

- New `Status:` field on contracts (breaking for upgraders)
- Compliance score formula changes (some Tier-2 repos demote until
  artifacts are marked `active` or higher)
- Steward's computed `verified` lifecycle state renamed `impl-present`
  (breaking for anything parsing steward output)
- New `SessionStart` hook expectation (adopters need to install it)

## Decision log — alpha → beta hard move (2026-07-04)

Recorded per Will's directive to "document decisions." Each entry is a
judgment call made this session.

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Hard move to a `v3.0.0-beta` trunk; the beta tag is earned, not declared.** Branch cut now; tag lands only when the acceptance checklist below passes. | The alpha's clusters were never built (plan + installer shim only). Tagging "beta" on draft work would commit the exact "badges lie" sin v3 exists to kill. Field feedback validated the *concepts*, so we commit to the line now and let the label be the checklist's verdict. |
| D2 | **Collapse the v2.1-vs-v3.1 fork: everything lands on one v3.0.0 trunk.** The field-proven inbox paradigm ships as Cluster 6 rather than a v2.1 minor. | One consolidated line beats three unreconciled ones. The swarm is eight repos under one maintainer; nothing external pins rebar semver. |
| D3 | **Clusters 6 (peer-inbox) and 7 (skills) added to the original five.** | Cluster 6 is the only field-*verified* v3-era material (proven in the tak cluster 2026-07-02→04). Cluster 7 (Will, this session): skill descriptions load into every session's context at start, making the skills list a cold-start nudge by construction — the missing middle between hooks (enforce) and ask (query). |
| D4 | **Steward's computed `verified` → `impl-present`** folded into Cluster 1, from `feedback/2026-06-19-trustable-status-…` ("the one rename removes the lie at ~zero cost"). `draft`/`active`/`testing` keep their names — they don't overclaim. The *declared* maturity vocabulary owns the word `verified`, with its honest meaning. | Two vocabularies must not share a lying word. Computed-from-file-presence "verified" was the counterfeit the June feedback documented; the declared vocab needs `verified` to mean "proven by passing tests/scenarios." S1-STEWARD contract bumps accordingly. |
| D5 | **Alpha branches retired, not deleted.** `v3.0.0-alpha` / `v3.0.1-alpha` stay on origin; CHANGELOG records their retirement. | Deleting pushed branches is destructive and erases provenance; retirement in docs is sufficient. |
| D6 | **QUICKCONTEXT merge conflict (main vs alpha) resolved by taking the alpha side.** | The file is fully rewritten for the beta before tag; resolving content by hand twice would be waste. |
| D7 | **Skills are pointers, not copies.** Every skill references its `practices/*.md`; paradigm content lives in exactly one place. | A second prose surface that drifts is the SessionStart-feedback disease with a new organ. |
| D8 | **rebar does not grow its own `inbox/` in this release.** The peer-inbox convention entry documents *when* a repo should hold one (multi-repo coordination seats); rebar's intake remains `ask featurerequest` + feedback branches. | Scope discipline: rebar is not currently a coordination seat in a memo-exchanging cluster. Adding an empty inbox would be ceremony. |
| D9 | **(post-review) Badge weighting counts undeclared live contracts as stub-or-draft once any contract declares.** Supersedes the build-time decision to weight only declared contracts. Same review pass also unified the two `Status:` parsers (bold/bare, case-folded) and moved thresholds to product comparisons. | Adversarial review reproduced badge-laundering by selective declaration, by unbolded/capitalized values, and an integer-floor gap at the >66% threshold — all defeating Cluster 1's purpose. Pre-v3 repos (zero declarations) keep the no-penalty advisory. |
| D11 | **(Will, 2026-07-04) The beta merges to `main`; the version string, not the branch, carries the anneal state.** `main` tracks the v3 line at `v3.0.0-beta`; the `v2.0.0` tag remains the stable v2 point (a `v2.x` maintenance branch can be cut from it if ever needed). `v3.0.0` final is **earned by the graduation criteria below**, exactly as the beta tag was earned by its checklist. | Adoption and feedback are the goal, and main is where discovery happens. Parallel lines rot (the alpha sat unbuilt for two months while main moved); the resolvers' upstream URLs point at `blob/main/`; every consumer is a swarm repo with escape hatches. "Beta on main" is honest as long as the badge says beta. |
| D10 | **(post-tag, Will 2026-07-04) Boundary-crossing artifacts reference rebar doctrine by abstract `rebar:<kind>/<name>` refs, not literal paths.** Convention in `conventions.md` §Cross-Repo References; resolvers of record `scripts/rebar-doc.sh` + `rebar doc` (resolution: local → `$REBAR_ROOT` → discovered checkout → upstream URL + ask-hint). Replaces the "(rebar checkout if not vendored)" prose qualifiers in the skills. | Literal paths in shipped artifacts dangle in adopter repos; prose qualifiers were a band-aid. An abstract name plus one documented resolution chain survives vendoring, checkouts, and upstream-only layouts. Literal paths stay correct for files that travel with the adopter set (`scripts/`). |

## Seven clusters

Maturity markers use the Cluster-1 vocabulary, applied to the cluster's
*inputs* at beta start — dogfooding the vocabulary on the release itself.

| # | Cluster | Source | Maturity at beta start |
|---|---------|--------|------------------------|
| 1 | Maturity tagging + compliance honesty + steward rename | conversation 2026-04-29; `feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md` §1 | draft |
| 2 | SessionStart hook for cold-start enforcement | `feedback/processed/2026-04-26-sessionstart-hook-cold-start-enforcement.md` | draft |
| 3 | TEST_FIDELITY + UAKS tier + closed-loop demo gate | `feedback/2026-04-22-testing-rigor-six-moments.md`, `feedback/processed/2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md`, `feedback/processed/2026-04-27-user-at-keyboard-story-tier.md` | draft |
| 4 | agents/FANOUT_PATTERN.md | `feedback/processed/2026-04-28-multi-subagent-fanout-playbook.md` | draft |
| 5 | Contract discipline followups | `feedback/processed/2026-04-24-contract-discipline-and-jtbd-framing.md` | draft |
| 6 | Peer-inbox paradigm + watch | `practices/inbox-watch.md` (field-proven), `feedback/processed/2026-07-04-inbox-watch-cold-start-hygiene-coordination-seats.md` | **verified** (tak cluster, 2026-07-02→04) |
| 7 | Claude Skills packaging | conversation 2026-07-04 (Will) | stub |

### Cluster 1 — Maturity tagging + compliance honesty

Vocabulary (fixed, small; canonical definition lands in `conventions.md`):

- **stub** — placeholder; structure exists, content is not real
- **draft** — real attempt, not yet reviewed/applied
- **in-progress** — actively being built; expect churn
- **active** — in use; defines current behavior
- **verified** — active + has passing tests/scenarios proving it

No auto-detection. People and agents apply markings honestly; gates are
added only on real-world failure.

Deliverables:

- `architecture/CONTRACT-TEMPLATE.md` — `Status:` line in header block
- `scripts/check-compliance.sh` — read `Status:` fields, weight badge:
  <33% stub-or-draft among contracts → tier as declared; 33–66% →
  annotate "— IN PROGRESS"; >66% → demote one tier with reason. Repos
  with zero `Status:` fields are treated as pre-v3 (no penalty, one
  advisory line).
- **Steward rename (D4):** `scripts/steward.sh` computed lifecycle
  `verified` → `impl-present` (JSON field, report table, one-liner).
  Grep for downstream consumers of the label and update them.
  `architecture/CONTRACT-S1-STEWARD` bumps (breaking output change →
  2.0, SUPERSEDES 1.0, registry updated, implementing-file headers
  updated). `conventions.md` lifecycle table update is owned by
  integration.

### Cluster 2 — SessionStart hook

- `templates/project-bootstrap/.claude/settings.json` with a
  `SessionStart` hook invoking `scripts/cold-start-checks.sh`
- `scripts/cold-start-checks.sh` — runs the enforcement quad
  (contract-refs, todos, freshness, ground-truth) + Cluster 1 maturity
  counts; **always exits 0** (visible drift, not blocking); output
  wrapped in `<rebar-cold-start>…</rebar-cold-start>`
- rebar's own `.claude/settings.json` installs the same hook (dogfood;
  none exists today — greenfield)
- `templates/project-bootstrap/CLAUDE.md` "Starting a Session" reframed
  as documentation of what the hook does, not instructions to the agent
- Cross-cutting principle (integration-owned, `conventions.md`): "MUST
  run on event X" → hook for X, not prose

### Cluster 3 — Test fidelity + UAKS + closed-loop

- `practices/test-fidelity.md` — the fidelity ladder (tautology /
  surrogate / real-flow / mutation-proof / UAKS), declarations
  machine-greppable; UAKS tier definition for user-interactive repos;
  "test env ready" claims must include UAKS or an explicit "no UAKS
  layer for this repo"
- Closed-loop demo gate: no "demo green" claim or `demo:` merge without
  captured browser-hit evidence
- `scripts/check-decay-patterns.sh` extended with silenced-failure
  banned patterns in demo/test specs

### Cluster 4 — FANOUT_PATTERN.md

- `agents/FANOUT_PATTERN.md` — worktree-per-branch, strict file
  allowlist, parent-owned post-merge sweep; dependency-graph reasoning
  before dispatch; when-NOT-to-fan-out rules (security-critical paths,
  prompt-longer-than-output, shared mutable state); raw
  `git worktree add` fallback
- `agents/subagent-guidelines.md` gains the mandatory "verify before
  relying" prompt clause

### Cluster 5 — Contract discipline followups

- `practices/spike-first-contracts.md` (the filedag DP-A pattern)
- `practices/contract-supersession.md`
- `scripts/check-jtbd-presence.sh` — fail if a contract lacks Why /
  Who / Scenarios sections; rebar's own S1–S3 contracts must pass
  (backfill sections if missing — Tier 3 means green on our own gates)
- `scripts/check-prefix-uniqueness.sh` — fail on duplicate prefix
  numbers across the registry

### Cluster 6 — Peer-inbox paradigm (field-verified)

- `scripts/inbox-watch.sh` — executable extraction of the loop embedded
  in `practices/inbox-watch.md`: multi-inbox capable, 30s default poll,
  one `NEW INBOX DEPOSIT: <path>` line per new file, zero-dep bash 3.2
- `practices/inbox-watch.md` updated to point at the script (loop stays
  as illustration)
- `practices/session-lifecycle.md` gains coordination-seat cold-start
  hygiene: sweep held inboxes, then arm the watch; manual fallback
  (`ls -lat inbox/ | head`) stays in the ritual
- Peer-inbox convention entry (integration-owned, `conventions.md`):
  repo-level `inbox/` peer mail — append-only, dated
  `YYYY-MM-DD-<from>-<slug>.md`, processed-on-read; explicitly
  disambiguated from the ASK runtime's `agents/<role>/inbox/` queues
- `feedback/processed/2026-07-04-inbox-watch-cold-start-hygiene-coordination-seats.md`
  dispositioned: accepted → implemented here

### Cluster 7 — Claude Skills packaging

- `templates/project-bootstrap/.claude/skills/<name>/SKILL.md` — thin
  skills with YAML frontmatter (name, description); each references its
  practice/script, duplicating nothing (D7):
  - `rebar-coldstart` — the session-start ritual; on coordination
    seats, arm the inbox watch
  - `rebar-feedback` — file a feedback item correctly (template, naming,
    provenance)
  - `rebar-audit` — run the audit + steward surfaces and read them
  - `rebar-inbox-watch` — arm `scripts/inbox-watch.sh` as a persistent
    monitor
- rebar's own `.claude/skills/` gets the same four (dogfood)
- Doctrine line (integration-owned, `conventions.md`): **MUST-run →
  hook; workflow/paradigm → skill; question → ask.**

## Ownership boundaries (build discipline)

Cluster builders own only their listed files. Shared surfaces —
`conventions.md`, `scripts/ci-check.sh` registration, `practices/README.md`
index, `feedback/INVENTORY.md` + dispositions, `README.md`, `CHANGELOG.md`,
`QUICKCONTEXT.md`, `TODO.md`, this plan — are integration-owned.
`templates/project-bootstrap/scripts/` is never edited by hand; run
`scripts/sync-bootstrap.sh` after scripts change.

## Sequence

1. Consolidation merges (done — `24d281c`)
2. This plan (rescope + decision log)
3. Clusters 1–7 built in parallel, disjoint file ownership
4. Integration: conventions spine, ci-check registration, dispositions,
   bootstrap sync
5. Verification: ci-check green, `rebar audit` 9–10/10, hook fires,
   scripts functionally tested, `rebar new` smoke test
6. Adversarial multi-agent review over the full beta diff; fix confirmed
   findings
7. CHANGELOG + migration notes; QUICKCONTEXT/TODO rewrite; version bump
   (`cli/cmd/root.go`, README badge, `setup-rebar.sh`) + CLI rebuild
8. **Tag `v3.0.0-beta`** — only if step 5–6 verdicts are green

## Acceptance for the tag

- Clusters 1–7 landed
- `rebar audit` passes 9–10/10 on rebar itself
- Cold-start hook fires and emits the maturity-aware status block
- A clean `rebar new` produces a working `ask architect` state
- New scripts pass functional tests (not just `bash -n`)
- CHANGELOG entry + migration notes for upgraders

## Graduation to v3.0.0 (annealing criteria)

The beta anneals **in use, labeled** — on `main`, version string
`v3.0.0-beta`, per-artifact `Status:` markers. It graduates to
`v3.0.0` final when ALL of the following are observable (not vibes):

1. **Two external swarm repos complete the migration** — `Status:`
   declared on all live contracts, SessionStart hook installed and
   emitting — and run **≥2 weeks** with no P0/P1 feedback filed against
   a v3 feature.
2. **A live coordination seat adopts the canonical
   `scripts/inbox-watch.sh`** in real cross-repo traffic (tak cluster
   is the natural first).
3. **Every v3-feature feedback item filed during the window is
   dispositioned** — implemented, watchlisted, or rejected; none
   pending.
4. **No swarm tooling still parses computed `verified`** — the steward
   rename is verified downstream, not assumed.

Then: version strings → `v3.0.0`, CHANGELOG entry, tag, announce via
the outbox/inbox channels. Criteria may be amended here (with a
decision-log entry), never silently skipped.

## What's deferred (unchanged rationale)

| Item | Source | Why deferred |
|------|--------|--------------|
| Auto-federation experiments | `feedback/2026-04-28-auto-federation-experiment.md` | 7 open maintainer questions. v3.0.x or v3.1 — pairs naturally with the now-shipped inbox watch (receiving-side ear). |
| Interaction-class fix protocol | `feedback/2026-04-20-interaction-class-false-positive-testing.md` | Doesn't share the v3 narrative. Watchlist. |
| Usability RT Cluster E remainder | `feedback/2026-04-28-usability-red-team.md` | Polish. Post-tag. |
| Trustable-status items 2–4 (queryable ASK capability answers, PRODUCT traceability, semantic-consistency gate) | `feedback/2026-06-19-trustable-status-…` | Item 1's cheapest step (the rename) ships in Cluster 1; the rest is v3.1-scale work. |

## Provenance

- Conversation 2026-04-29 — Will authorized the alpha scope
- Conversation 2026-07-04 — Will authorized the hard move to beta, the
  inbox paradigm as headline addition, and Skills packaging: "ok amazing
  lets do it. agree on all. execute with autonomy, document decisions"
- Plan doc filed in-repo per the in-repo persistence preference
