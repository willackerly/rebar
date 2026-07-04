# TODO — rebar

**Last synced:** 2026-07-04

The canonical, vote-shaped backlog lives in [`feedback/INVENTORY.md`](feedback/INVENTORY.md).
This file holds short-horizon work that's actively in flight or imminent.

---

## P0 — In Flight

- [ ] **Tag `v3.0.0-beta`** — all seven clusters + integration landed
      2026-07-04; adversarial review is the last acceptance gate.
      Plan + decision log: [`docs/v3-beta-plan.md`](docs/v3-beta-plan.md).

## P1 — Imminent

- [ ] Offer the beta to the first federated adopters (TDFLite, filedag,
      fontkit) — maturity tagging + SessionStart hook trial.
- [ ] Post-tag: move implemented feedback files to `feedback/processed/`
      and update the practice-doc links that point at them.
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
- Ship `practices/` + `conventions.md` (or pointer stubs) in
  `templates/project-bootstrap/` so skill references resolve in adopter
  repos without the upstream checkout.

## Discoveries

<!-- The Steward parses this section. Each entry: BUG / DISCOVERY / DRIFT / DISPUTE
     scoped to a contract ID. Open issues only — close by removing the entry. -->

_None currently._

---

## Recently complete

(Move items here when shipped. Trim to last ~10 to stay concise.)

- ✓ 2026-07-04: v3.0.0-beta clusters 1–7 + integration (9 commits,
  `6479b03`…`cf50022` + CLI script install `6e5b0c9`)
- ✓ 2026-07-04: Alpha→beta hard move — consolidation merges + plan
  rescope with decision log (`24d281c`, `9f7e7d5`)
- ✓ 2026-07-04: Feedback filings committed (trustable-status LOE,
  inbox-watch hygiene) — `6b699b1`
- ✓ 2026-04-29: v3.0.0-alpha branch cut (retired 2026-07-04)
- ✓ 2026-04-28: Federation Clusters 1–5 + 4 principles
- ✓ 2026-04-25: Max-compliance dogfooding (Tier 3, contracts, hook)
