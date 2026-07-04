# TODO — rebar

**Last synced:** 2026-07-04

The canonical, vote-shaped backlog lives in [`feedback/INVENTORY.md`](feedback/INVENTORY.md).
This file holds short-horizon work that's actively in flight or imminent.

---

## P0 — In Flight

- [x] ~~Tag `v3.0.0-beta`~~ — **tagged + pushed 2026-07-04** after
      ci-check 15/15, audit 10.0/10, and the 71-agent adversarial
      review's findings were fixed. Plan + decision log:
      [`docs/v3-beta-plan.md`](docs/v3-beta-plan.md).

## P1 — Imminent

- [ ] Track adopter-trial responses — offer memos deposited 2026-07-04
      in TDFLite-main and tak-tdf inboxes; filedag/fontkit still need a
      channel (no inbox/ on disk).
- [ ] Triage `feedback/2026-07-02-reflexive-push-durability-rule.md`
      (steward unpushed-commit warning).
- [ ] **Next big push: opportunistic auto-federation** — see
      [`feedback/2026-04-28-auto-federation-experiment.md`](feedback/2026-04-28-auto-federation-experiment.md).
      7 maintainer-decision questions open. Pairs with
      `scripts/inbox-watch.sh` as the receiving-side ear.

## P2 — Maintainer Queue

See `feedback/INVENTORY.md` §🧰 Maintainer Queue, plus:
- Trustable-status items 2–4 (queryable ASK capability answers, PRODUCT
  traceability, semantic-consistency gate) —
  [`feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md`](feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md)
- ~~Ship `practices/` + `conventions.md` in the bootstrap template~~ —
  resolved differently 2026-07-04 (plan D10): shipped artifacts use
  abstract `rebar:` refs resolved by `scripts/rebar-doc.sh` / `rebar
  doc` (local → REBAR_ROOT → checkout → upstream URL). Vendoring is no
  longer needed for reference integrity; revisit only if offline
  adopters ask.

## Discoveries

<!-- The Steward parses this section. Each entry: BUG / DISCOVERY / DRIFT / DISPUTE
     scoped to a contract ID. Open issues only — close by removing the entry. -->

_None currently._

---

## Recently complete

(Move items here when shipped. Trim to last ~10 to stay concise.)

- ✓ 2026-07-04: `rebar:` abstract refs (D10) — resolvers + skills
  rewrite + feedback moved to processed/; outreach memos deposited
- ✓ 2026-07-04: v3.0.0-beta **tagged + pushed** after review fixes
  (`53f5f1c`…`4b7dcbd`)
- ✓ 2026-07-04: v3.0.0-beta clusters 1–7 + integration (9 commits,
  `6479b03`…`cf50022` + CLI script install `6e5b0c9`)
- ✓ 2026-07-04: Alpha→beta hard move — consolidation merges + plan
  rescope with decision log (`24d281c`, `9f7e7d5`)
- ✓ 2026-07-04: Feedback filings committed (trustable-status LOE,
  inbox-watch hygiene) — `6b699b1`
- ✓ 2026-04-29: v3.0.0-alpha branch cut (retired 2026-07-04)
- ✓ 2026-04-28: Federation Clusters 1–5 + 4 principles
- ✓ 2026-04-25: Max-compliance dogfooding (Tier 3, contracts, hook)
