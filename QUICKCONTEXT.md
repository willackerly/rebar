# QUICKCONTEXT — rebar (main, v3 line)

last-synced: 2026-08-02
<!-- freshness: 2026-08-02 -->
**Branch:** main — **now tracks v3** at `v3.0.0-beta.2` (plan D11)
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology

---

## Current State

- **main IS the v3 line** as of 2026-07-04: v3 built, tagged, adversarially
  reviewed, and merged here. **`v3.0.0-beta.2` tagged 2026-08-02** — the
  second annealing increment, *not* graduation. The version string carries
  the anneal state — `v3.0.0` final is gated on the graduation criteria
  in `docs/v3-beta-plan.md`. Tag `v2.0.0` remains the stable v2 point.
- **Contract prefix numbering is retired** (beta.2): the full descriptive
  name is the key — `CONTRACT-BLOBSTORE.2.1.md`, not `C1-BLOBSTORE`. A
  relaxation, not a migration: existing numbered contracts stay, never
  renumber, never mass-rename. Top-level README callout + conventions.md
  §Contract Naming.
- **Two new practices:** `inbox-protocol.md` (memo grammar, types,
  confidence labels) and `diagram-and-document-assets.md` (rebar's first
  diagram convention — `.pptv.svg` atoms, pinned to office180
  `0.1.0-alpha.4`).
- The two-vocabulary honesty split is live: **declared maturity**
  (`Status:` on contracts, stub→verified) weighted into the compliance
  badge, and **computed lifecycle** renamed `verified` → `impl-present`
  (S1-STEWARD 2.0)
- **SessionStart hook** fires here and in `rebar init/new/adopt`
  projects — the `<rebar-cold-start>` block is harness fact, not prose
- **Peer-inbox paradigm** (field-verified, tak cluster) + four Claude
  Skills shipped; **`rebar:` abstract refs** (conventions §Cross-Repo
  References) resolved by `scripts/rebar-doc.sh` / `rebar doc`
- **8 rebar-adopted repos** in the MCP swarm; **37 ASK agents** via the
  rebar-ask MCP server; federation tooling (CONSUMES.md, drift-check,
  outbox) live
- `ci-check.sh` runs 15 checks; `rebar audit` 10.0/10 on this repo

## Active Branches

| Branch | Purpose | Where to look |
|--------|---------|---------------|
| `main` | **the v3 line** (`v3.0.0-beta`, annealing toward v3.0.0) | this file + `docs/v3-beta-plan.md` |
| `v3.0.0-beta` | merged into main 2026-07-04; retained for provenance with the tag | history only |
| `v3.0.0-alpha` / `v3.0.1-alpha` | retired (never built; see plan D1) | history only |

## In Progress

- **Annealing toward v3.0.0 final.** Scored 2026-08-02 against
  `docs/v3-beta-plan.md` §Graduation — **1 of 4 met**:

  | # | Criterion | State |
  |---|---|---|
  | 1 | Two migrated repos, ≥2 wks clean | count met (`office180`, `go-tak-server`); **clean window reset 2026-08-02** |
  | 2 | Live seat on canonical watcher | **MET** — armed on rebar's own inbox, caught a real inbound |
  | 3 | All v3-feature feedback dispositioned | **NOT MET** — 18 open, 3 filed 2026-08-02 |
  | 4 | No downstream parser of computed `verified` | **UNVERIFIED** — needs a real read, not a grep |

- **Live peer coordination.** Four ANSWER/FINDING memos deposited
  2026-08-02 to `sch-repos`, `TDFLite`, `_atlas`, `office180`. Watcher is
  armed here (Principle-5 dogfood gap, closed).

## What's Next

1. **Criterion 3 is the graduation blocker.** Disposition the three
   v3-feature defects filed 2026-08-02: `check-freshness` parses one
   vocabulary (QUICKCONTEXT was invisible to it for 29 days),
   `check-compliance` maturity degenerates at one contract, canonical
   status grep diverges from parsers of record.
2. **Verify criterion 4 properly** — read the downstream scripts rather
   than grepping for the word `verified`.
3. Fix the watcher self-echo: banking an outbound memo in your own
   `inbox/` fires your own watcher
   (`feedback/2026-08-02-outbound-banking-self-echoes-…`). Sender-aware
   skip; the new `<sender>` filename segment makes it mechanical.
4. Ratify the accepted inbox doctrine into `conventions.md`:
   inbox-by-default + the `CONSUMES ⇒ upstream holds an inbox` rule
   (sch-repos), root `INBOX.md` (TDFLite), and Principle 5 restated as
   *hold ⇒ sweep; live ⇒ watch*.
5. Triage `feedback/2026-07-02-reflexive-push-durability-rule.md`
   (steward unpushed-commit warning).
3. **v3.1 direction:** auto-federation experiments
   (`feedback/2026-04-28-auto-federation-experiment.md`, 7 maintainer
   questions open) pairs with the inbox watch as its receiving-side ear;
   trustable-status items 2–4 (`feedback/2026-06-19-…`).
4. Older feedback still open: `2026-04-21-filedag-cross-ref…`,
   `2026-04-22-testing-rigor…` (remaining moments),
   `2026-04-24-fidelity-decay…` (self-audit-prompt addition).

## Known issues / non-blockers

- v3 compliance weighting intentionally demotes badges over mostly
  stub/draft (or undeclared-once-any-declares) contract sets — that's
  the feature. Pre-v3 repos (zero `Status:` fields) get one advisory.
- `rebar new` compliance floor is ~6.9/10 (fresh projects lack tests
  and contracts by definition).
- Auto-federation experiments stay research-only until the 7 open
  questions are answered.

## Verification (run before trusting this file)

```bash
git log --oneline -15                 # cluster commits + review fixes + merge
scripts/ci-check.sh                   # 15 checks, all pass expected
bin/rebar audit                       # 10/10 expected
scripts/cold-start-checks.sh          # the hook's block, on demand
bin/rebar doc rebar:practice/inbox-watch   # abstract-ref resolution
cat docs/v3-beta-plan.md              # plan + decision log D1-D11 + graduation
```

## Cross-device handoff (2026-07-04)

Everything lives on `main` at origin (plus tags `v2.0.0`,
`v3.0.0-beta`). Resume with:

```bash
git fetch origin && git checkout main
cat QUICKCONTEXT.md docs/v3-beta-plan.md
```

All load-bearing context lives in-repo per the in-repo persistence
preference; auto-memory does not travel.
