# TODO — rebar

**Last synced:** 2026-11-14

The canonical, vote-shaped backlog lives in [`feedback/INVENTORY.md`](feedback/INVENTORY.md).
This file holds short-horizon work that's actively in flight or imminent.

---

## P0 — In Flight

- [ ] **Assemble v3.1.0 on `release/v3.1.0-rc`.** Collects #14
      (self-contained install) and #8 (`contract_namespace`), plus
      `rebar migrate` and a versioned installer, both in progress.

## P1 — Imminent

- [ ] **`rebar new` / `init` / `adopt` write a stale `.rebar-version`.**
      `cli/cmd/new.go`, `init.go` and `audit.go` hardcode `v3.0.0-beta`
      while the README badge comes from `rootCmd.Version`, so a fresh
      `rebar new` project fails its own `check-compliance.sh` ("Badge says
      v3.0.0 but .rebar-version says v3.0.0-beta"). Present since beta.3,
      when the badge moved to the constant; measured 2026-11-14 against
      the v3.0.0 build. Fix: write `rootCmd.Version` in all three, and
      refresh `templates/project-bootstrap/.rebar-version` /
      `README.md` badge (still `v3.0.0-beta` / `-beta.2`).
- [ ] **Leaked test watcher.** A test-spawned `inbox-watch.sh` ran for
      **10 days** against a deleted `/var/folders/...` temp dir (found and
      killed 2026-08-12). Both `scripts/tests/inbox-watch*.test.sh` have
      EXIT traps and explicit kills, so the leak path is not obvious —
      needs a real look, not a guess. A leaked poller is cheap to ignore
      and exactly the kind of thing that erodes trust in the watcher.
- [ ] **A harness-backed watcher does not outlive its task.** Observed
      2026-08-12: `scripts/inbox-watch.sh` armed as a background task was
      reaped when the harness marked the task complete — it released its
      lock and exited 0, cleanly and **silently**. Between the arm and the
      notification, the inbox was unwatched while everything reported
      healthy. Workaround in use: `nohup … & disown`. This is the
      strongest field evidence yet for `_atlas`'s 3.1 argument that
      **pull-with-memory beats push** — `rebar inbox --sweep` at session
      start cannot die quietly, whereas a background monitor can, and
      silence from a dead monitor is indistinguishable from silence from a
      quiet inbox. Feeds `practices/inbox-watch.md` as a named limitation.
- [ ] **Report two watcher hygiene findings to peers** (both verified by
      `lsof` on 2026-08-12, neither rebar's to fix):
      - `TDFLite` has **two** live watchers on one inbox (armed 08-02 and
        08-04) — the split-provenance case Principle 5 warns about.
      - `pdf-signer-web` watches `vendor/dapple/inbox` — watching a
        vendored peer's inbox is the self-echo case (SOP 2026-07-06).
- [ ] Triage `feedback/2026-07-02-reflexive-push-durability-rule.md`
      (steward unpushed-commit warning).

## P2 — Maintainer Queue

**Needs Will specifically** (agent cannot decide these):
- [ ] **`rebar inbox` vs `ask inbox`** — the rebar seat ruled `rebar
      inbox`, because `conventions.md` already flags the peer-inbox /
      agent-inbox overload as historical and `ask inbox` reintroduces it
      at the CLI. This **contradicts Will's stated instinct**, so it needs
      his sign-off before `_atlas` builds slice 2.
- [ ] **rebar's own `CONSUMES.md`** — declaring the office180 PPTV pin
      would be rebar's first consumed contract and inverts the usual
      owner→consumer direction. Also note: an entry trips compliance
      Check 8 unless `drift-check` is wired into CI first.
- [ ] **`sch-repos` is not a git repository** — two rebar memos sit there
      with no provenance or durability.

**v3.1 candidates** (staged, not started):
- [ ] `rebar watch` as a shipped command (`_atlas` slice 1) — kills the
      copy-distribution hazard measured 2026-07-30 (11 copies of
      `inbox-watch.sh` on one machine, 2 versions, one carrying a fixed
      bug). Pure lift; the vendored script becomes a shim.
- [ ] `rebar inbox` pull-with-memory + `rebar_inbox_check` MCP tool
      (`_atlas` slices 2–3) — gated on the naming decision above.
- [ ] Emit the canonical `Status:` regex from tooling (sch-repos option 2)
      so prose cannot drift from the parser again. Documented today,
      not mechanized.
- [ ] Auto-federation experiments — 7 maintainer questions open,
      research-only until answered.

See `feedback/INVENTORY.md` §🧰 Maintainer Queue, plus:
- Trustable-status items 2–4 (queryable ASK capability answers, PRODUCT
  traceability, semantic-consistency gate) —
  [`feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md`](feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md)
- ~~Ship `practices/` + `conventions.md` in the bootstrap template~~ —
  resolved differently 2026-07-04 (plan D10): shipped artifacts use
  abstract `rebar:` refs resolved by `scripts/rebar-doc.sh` / `rebar
  doc`. Vendoring is no longer needed for reference integrity.

## Discoveries

<!-- The Steward parses this section. Each entry: BUG / DISCOVERY / DRIFT / DISPUTE
     scoped to a contract ID. Open issues only — close by removing the entry. -->

_None currently._

---

## Recently complete

(Move items here when shipped. Trim to last ~10 to stay concise.)

- ✓ 2026-11-14: **Graduated to `v3.0.0`** by project-owner decision —
  3 of 4 criteria met; criterion 1's clean window was never re-verified
  and was accepted by the decision (`docs/v3-beta-plan.md` D12). No code
  change since `v3.0.0-beta.4`.
- ✓ 2026-08-02: **`v3.0.0-beta.2` tagged + pushed** — contract prefix
  numbering retired (full name is the key), `practices/inbox-protocol.md`
  and `practices/diagram-and-document-assets.md` added
- ✓ 2026-08-02: **Graduation criteria 3 + 4 cleared** — five v3-feature
  defects fixed in one pass (freshness two-vocabulary, Check-9 degenerate
  population, `Status:` grep divergence, watcher self-echo, CONSUMES
  fenced-example miscount). A fresh bootstrap project failed compliance
  **twice** before that commit; it now reports OK.
- ✓ 2026-08-02: Inbox doctrine ratified — inbox-by-default,
  `CONSUMES ⇒ upstream holds an inbox`, root `INBOX.md`, and Principle 5
  restated as *hold ⇒ sweep; live traffic ⇒ watch*
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
