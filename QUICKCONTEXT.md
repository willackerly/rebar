# QUICKCONTEXT — rebar (main, v3 line)

last-synced: 2026-11-14
<!-- freshness: 2026-11-14 -->
**Branch:** main — **now tracks v3** at `v3.0.0-beta.4` (plan D11)
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology

---

## Current State

- **main IS the v3 line** as of 2026-07-04: v3 built, tagged, adversarially
  reviewed, and merged here. **`v3.0.0-beta.2` tagged 2026-08-02** — the
  second annealing increment, *not* graduation. The version string carries
  the anneal state — `v3.0.0` final is gated on the graduation criteria
  in `docs/v3-beta-plan.md`. Tag `v2.0.0` remains the stable v2 point.
- **`v3.0.0-beta.3` released 2026-11-14** — merge-friendly generated files
  for busy repos (#9, #10): the `integrity.json` header is no longer
  restamped, ratchet floors are derived, the registry has no timestamp and
  can leave out code-derived impl counts (`registry_impl_counts`), and
  `METRICS` accepts `>=` floors. See `practices/busy-repos.md`. Also not
  graduation.
- **`v3.0.0-beta.4` released 2026-11-14** — installer fix: the one-line
  `curl … | bash` install now works under macOS `/bin/bash` 3.2 (#12).
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
- `ci-check.sh` runs 15 checks; `rebar audit` 10.0/10 when this file
  is fresh (audit's QUICKCONTEXT threshold is 7 days, tighter than
  `check-freshness.sh`'s 14 — a repo can read fresh on one surface and
  stale on the other; see Known issues)

## Active Branches

| Branch | Purpose | Where to look |
|--------|---------|---------------|
| `main` | **the v3 line** (`v3.0.0-beta.4`, annealing toward v3.0.0) | this file + `docs/v3-beta-plan.md` |
| `v3.0.0-beta` | merged into main 2026-07-04; retained for provenance with the tag | history only |
| `v3.0.0-alpha` / `v3.0.1-alpha` | retired (never built; see plan D1) | history only |

## In Progress

- **Graduation rescore is overdue.** The scoreboard below is from
  2026-08-12; the 2026-08-16 date for criterion 1 passed with no recorded
  rescore, and beta.3 did not rescore. Rescore §Graduation before
  graduating — including whether beta.3's integrity/registry changes count
  as v3-feature changes for the clean clock.

- **Annealing toward v3.0.0 final.** Rescored 2026-08-12 against
  `docs/v3-beta-plan.md` §Graduation — **3 of 4 met**:

  | # | Criterion | State |
  |---|---|---|
  | 1 | Two migrated repos, ≥2 wks clean | **the only open gate.** Count met (`office180`, `go-tak-server` — both `v3.0.0-beta`, `Status:` declared, hook emitting). Clean clock started **2026-08-02**; earliest satisfaction **2026-08-16**. |
  | 2 | Live seat on canonical watcher | **MET 2026-08-02** — armed on rebar's own inbox in real traffic; delivered a genuine inbound and surfaced a live defect. |
  | 3 | All v3-feature feedback dispositioned | **MET 2026-08-02** — all five items filed in the window implemented and moved to `feedback/processed/`. |
  | 4 | No downstream parser of computed `verified` | **MET 2026-08-02, verified by reading** — every v3 repo emits `impl-present`, zero `lifecycle == "verified"` comparisons. |

- **Quiet 2026-08-12 → 2026-11-14**, then beta.3 (#9, #10) from
  virtru-analyze's busy-repo feedback
  (`feedback/2026-11-14-busy-repo-merge-conflicts.md`). Queued from it:
  flip the `registry_impl_counts` default to `false` in 3.1.

- **Watcher re-armed 2026-08-12** with own-sender suppression
  (`--self`, default = repo dir name). Two stale pre-fix watchers on
  this inbox were cleared first — see Outstanding actions.

## Outstanding actions

Ordered by what blocks graduation, then by cost.

| # | Action | Owner | Notes |
|---|---|---|---|
| 1 | Hold the clean window to **2026-08-16** | rebar | Verify `office180` + `go-tak-server` still green, then graduate. Nothing to build — the work is *not breaking anything*. |
| 2 | Leaked test watcher | rebar | A test-spawned watcher ran **10 days** against a deleted temp dir (found 2026-08-12, killed). Both watcher tests have EXIT traps, so the leak path is not obvious — needs a real look before it recurs. |
| 3 | `TDFLite` runs **two** live watchers on one inbox | TDFLite seat | PIDs armed 2026-08-02 and 2026-08-04, both on `TDFLite/inbox` — the split-provenance case Principle 5 warns about. Report; not rebar's to kill. |
| 4 | `pdf-signer-web` watches `vendor/dapple/inbox` | pdf-signer-web seat | Watching a vendored peer's inbox is the self-echo / peer-surveillance case (SOP 2026-07-06). Report. |
| 5 | office180 `text-fit` unusable for consumers | office180 | Font fixtures are not in the published `files`; the one command defending the no-reflow constraint cannot run out of the box. Open in their inbox since 2026-08-02. |
| 6 | `sch-repos` is **not a git repo** | Will | Two rebar memos sit there with no provenance and no durability. Sharpened by their own reachability argument. |
| 7 | Emit the canonical `Status:` regex from tooling | rebar | sch-repos' option 2 — the durable fix for prose/parser drift. Currently documented, not mechanized. |
| 8 | `rebar inbox` vs `ask inbox` verb naming | **Will** | rebar seat ruled `rebar inbox` (the peer/agent inbox overload is already flagged in conventions.md). This contradicts Will's stated instinct, so it needs his sign-off before `_atlas` builds slice 2. |
| 9 | rebar's own `CONSUMES.md` | Will | Declaring the office180 PPTV pin would be rebar's first consumed contract, and inverts owner→consumer direction. Open federation-posture question. |
| 10 | 14 pre-window feedback items | rebar | Out of scope for criterion 3 (which reads "filed during the window"), but the real backlog. See `feedback/INVENTORY.md`. |

## What's Next — proposed go-forward

**Now → 2026-08-16 — hold still and graduate.** Criterion 1 needs no
work, only an absence of breakage. Do not land v3 features in this
window; a new v3-feature defect restarts the clock (that is exactly
what happened on 2026-08-02). Non-v3 work, adopter support, and
answering peer mail are all safe.

**At 2026-08-16 — graduate to `v3.0.0`.** Re-verify all four criteria,
confirm `office180` and `go-tak-server` filed no P0/P1 against a v3
feature, then version strings → `v3.0.0`, CHANGELOG entry, tag,
announce through the outbox/inbox channels. Per D1 the tag is earned:
if the clock broke, say so and reset rather than amend.

**After graduation — v3.1.** Three threads are already staged:
1. **`rebar watch` as a shipped command** (`_atlas` slice 1) — kills the
   copy-distribution hazard (11 copies, 2 versions, measured 2026-07-30).
   Pure lift of a proven script; the vendored copy becomes a shim.
2. **`rebar inbox` pull-with-memory + `rebar_inbox_check` MCP tool**
   (`_atlas` slices 2–3) — gated on action 8 above.
3. **Auto-federation experiments**
   (`feedback/2026-04-28-auto-federation-experiment.md`) — 7 maintainer
   questions still open; research-only until answered.

**Not planned, deliberately:** mass-renaming numbered contracts (the
relaxation is explicitly not a migration), a blocking pre-commit inbox
gate (advisory only — blocking trades durability for coordination), and
an events ledger in the peer lane (the inbox *is* the trail).

## Known issues / non-blockers

- v3 compliance weighting intentionally demotes badges over mostly
  stub/draft (or undeclared-once-any-declares) contract sets — that's
  the feature. Pre-v3 repos (zero `Status:` fields) get one advisory.
- `rebar new` compliance floor is ~6.9/10 (fresh projects lack tests
  and contracts by definition).
- Auto-federation experiments stay research-only until the 7 open
  questions are answered.
- **Two freshness thresholds disagree by design-accident:**
  `check-freshness.sh` warns at 14 days, `rebar audit` marks
  QUICKCONTEXT stale at 7. On 2026-08-12 this file was simultaneously
  PASS on cold-start and 5/10 on audit. Not wrong, but a repo reading
  fresh on one surface and stale on another is the shape of drift this
  methodology exists to catch.

## Verification (run before trusting this file)

```bash
git log --oneline -15                 # cluster commits + review fixes + merge
scripts/ci-check.sh                   # 15 checks, all pass expected
bin/rebar audit                       # 10/10 expected (9/10 if this file is >7d old)
scripts/cold-start-checks.sh          # the hook's block, on demand
bin/rebar doc rebar:practice/inbox-watch   # abstract-ref resolution
cat docs/v3-beta-plan.md              # plan + decision log D1-D11 + graduation
```

## Cross-device handoff (2026-08-12)

Everything lives on `main` at origin (plus tags `v2.0.0`,
`v3.0.0-beta`, `v3.0.0-beta.2`, `v3.0.0-beta.3`, `v3.0.0-beta.4`). Resume with:

```bash
git fetch origin && git checkout main
cat QUICKCONTEXT.md docs/v3-beta-plan.md
```

All load-bearing context lives in-repo per the in-repo persistence
preference; auto-memory does not travel. Durable working-agreement
context is checked in at `.claude-memory-commits/` — read its
`README.md` first.
