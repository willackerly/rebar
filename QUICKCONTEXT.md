# QUICKCONTEXT — rebar (v3.0.0-beta)

last-synced: 2026-07-04
**Branch:** v3.0.0-beta
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology

---

## Current State

- **v3.0.0-beta built and verified 2026-07-04** — all seven clusters
  landed in one session (hard move from the never-tagged alpha line;
  decision log D1–D8 in `docs/v3-beta-plan.md`)
- The two-vocabulary honesty split is live: **declared maturity**
  (`Status:` on contracts, stub→verified) weighted into the compliance
  badge, and **computed lifecycle** renamed `verified` → `impl-present`
  (S1-STEWARD 2.0)
- **SessionStart hook** fires in this repo and in `rebar init/new/adopt`
  projects — `<rebar-cold-start>` block is harness fact, not prose
- **Peer-inbox paradigm** (field-verified in the tak cluster) shipped:
  convention entry, `scripts/inbox-watch.sh`, coordination-seat
  cold-start hygiene, four Claude Skills as cold-start nudges
- `ci-check.sh` runs 15 checks; `rebar audit` 9+/10 expected
- v2.x line remains at `main` (tag `v2.0.0`); alpha branches retired

## In Progress

- **v3.0.0-beta is tagged and pushed** (2026-07-04, all acceptance
  gates green incl. the adversarial review). Branch continues toward
  v3.0.0 final: adopter-trial feedback is the next input.
- **`rebar:` abstract refs live** — boundary-crossing artifacts
  (skills, templates, memos) reference doctrine by name; resolvers
  `scripts/rebar-doc.sh` + `rebar doc`.

## What's Next

1. ~~Tag~~ — **tagged + pushed 2026-07-04** (review-clean).
2. **Adopter trials in flight:** offer memos deposited 2026-07-04 in
   the TDFLite-main and tak-tdf inboxes; filedag/fontkit need a channel.
3. ~~Post-tag housekeeping~~ — done 2026-07-04: six implemented
   feedback files moved to `processed/`, links swept; boundary-crossing
   artifacts now use `rebar:` refs (conventions §Cross-Repo References,
   plan D10) resolved by `scripts/rebar-doc.sh` / `rebar doc`.
4. **v3.1 direction:** auto-federation experiments
   (`feedback/2026-04-28-auto-federation-experiment.md`, 7 maintainer
   questions open) pairs with the inbox watch as its receiving-side ear;
   trustable-status items 2–4 (`feedback/2026-06-19-…`).
5. Untriaged: `feedback/2026-07-02-reflexive-push-durability-rule.md`
   (steward unpushed-commit warning).

## Known issues / non-blockers

- v3 compliance weighting intentionally demotes adopters with mostly
  stub/draft contracts — that's the feature. Pre-v3 repos (no `Status:`
  fields) get an advisory only.
- `rebar new` compliance floor is ~6.9/10 (fresh projects lack tests
  and contracts by definition).

## Verification (run before trusting this file)

```bash
git log --oneline -15                 # the seven cluster commits + integration
scripts/ci-check.sh                   # 15 checks, all pass expected
bin/rebar audit                       # 9+/10 expected
scripts/cold-start-checks.sh          # the hook's block, on demand
cat docs/v3-beta-plan.md              # canonical plan + decision log
```

## Cross-device handoff

`v3.0.0-beta` is pushed to origin. Resume with:

```bash
git fetch origin && git checkout v3.0.0-beta
cat QUICKCONTEXT.md docs/v3-beta-plan.md
```

All load-bearing context lives in-repo per the in-repo persistence
preference; auto-memory does not travel.
