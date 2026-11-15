# Changelog

All notable changes to rebar. Versioned with [semver](https://semver.org/).

**Versioning policy:**
- **Major** — breaking changes to contract format, agent role structure, CLI interface, or enforcement script API
- **Minor** — new agents, templates, practices, profiles, scripts
- **Patch** — doc fixes, script bug fixes, template clarifications
- **Pre-release suffixes (`-alpha`, `-beta`) are the annealing channel:**
  work we believe in but that needs field time ships **on `main`,
  honestly labeled** — the version string and per-artifact `Status:`
  markers carry the anneal state, not branch topology (parallel lines
  rot). Each pre-release label is earned by an explicit checklist and
  graduates by explicit criteria recorded in its release plan doc —
  never by the passage of time alone.

---

## Unreleased

_Nothing yet._

---

## v3.1.0-rc.1 — 2026-11-15

First release candidate for 3.1.0, a **pre-release**: the installer's default
"latest" stays on `v3.0.0`; install it with an explicit version
(`curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.0-rc.1/setup-rebar.sh | bash -s -- v3.1.0-rc.1`)
or `--pre`. The theme is **adopters carry configuration, not rebar's
scripts**: the checks run from a versioned, self-contained install, everything
repo-specific moves into `.rebarrc`, and `rebar migrate` converts existing
copied (even customized) scripts, verified by running them. Upgrade steps:
[UPGRADING.md](UPGRADING.md).

- **Implementing-file counts are off by default** (`.rebarrc`
  `registry_impl_counts`, now `false`): the committed registry has no
  `Impl Files` column and no Contract Health section, because any PR that adds
  a `CONTRACT:` header changes them and two such PRs merge into a wrong count.
  `rebar run compute-registry --health` prints them. **Upgrade note:** a
  registry with counts reports STALE once — regenerate it (UPGRADING.md step
  5), or set `registry_impl_counts = true` to keep them. `rebar migrate`
  follows the new default and regenerates unless the project opted in.
- **The registry's regenerate hint names the CLI:** `<!-- Regenerate: rebar run
  compute-registry -->` (it said `./scripts/compute-registry.sh`, which adopters
  no longer have). `compute-registry.sh --check` ignores that comment, as it
  ignores the old timestamp, so a registry with the old hint stays current and
  picks up the new one when next regenerated; `rebar migrate` compares
  registries the same way.
- **The source scanners skip binary files** (`grep -I`). A matching binary was
  reported as "Binary file … matches" and read as a reference or a TODO — a
  built `bin/rebar` that embeds `CONTRACT:` strings made the registry STALE.

- **Upgrading:** [UPGRADING.md](UPGRADING.md) is the runbook for moving an
  adopting project from v2.0.0 or v3.0.x (install side by side, bump
  `.rebar-version` and the badge, `rebar migrate`, registry, `checks_enable`,
  integrity manifest, CI, verify). Install a release candidate with its own
  tag's installer:
  `curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.0-rc.1/setup-rebar.sh | bash -s -- v3.1.0-rc.1`
  ([SETUP.md § Testing a release candidate](SETUP.md#testing-a-release-candidate)).
  CI must install a full rebar tree, not just the binary
  ([SETUP.md § Run rebar in CI](SETUP.md#run-rebar-in-ci)).
- **`rebar migrate`** turns an adopter's legacy, possibly customized,
  script copies into `.rebarrc` settings, decided by running them: each
  `compute_metrics()` line of a custom `check-ground-truth.sh` becomes
  `metric.<key>.*` only if the install computes the same value, on the
  project and on a probe tree built from the metric; registry settings are
  kept only if `compute-registry.sh --stdout` stays byte-identical
  (`registry_purpose_style = bytes` when the old cut is needed); other checks
  must give the same exit status and findings, and the source scanners also
  on a probe tree with exclusion bait. All probes run in a scratch copy of
  the project, and a copy that writes files is kept. Dry run by default;
  `--apply` refuses a dirty work tree (`--allow-dirty`), writes `.rebarrc`
  atomically, deletes only verified copies (a copy other files call becomes
  a `# rebar-thin-wrapper` instead, so callers keep working), adjusts METRICS
  counts the deletions change, reruns the checks and rolls everything back on
  a regression. It refuses symlinked `scripts/` outside the project and any
  rebar checkout, ignores `REBAR_*` in the environment, never changes `tier`,
  and keeps checks whose copies ran ungated below the install's tier gate
  running with a proposed `checks_enable`. Unmodified copies are recognized
  from a released-blobs list embedded in the CLI
  (`scripts/gen-released-blobs.sh`, a release step), so installs without git
  history work. A scanner copy that is a released version plus settings
  (its directories, extensions) is migrated even though the install now
  excludes directories by name (#19), with the newly reported findings
  listed before `--apply`; old substring filters map to
  `source_exclude_dirs`. The CLI now honors `REBAR_PROJECT_ROOT` for repo
  detection, as the scripts do.
  `check-ground-truth.sh --print` and `REBAR_RCFILE` support it. Design:
  `feedback/2026-11-14-rebar-migrate.md`.
- **CLI errors are printed.** A failing command printed nothing and exited 1
  (cobra's printing was off and `main` didn't print), so hints like
  "rebar install not found (set REBAR_ROOT …)" never reached the user. They now
  go to stderr as `rebar: <error>`.
- **`rebar commit` drops manifest entries for staged deletions** (a rename
  counts as delete + add). They used to stay, so `rebar verify --strict`
  reported the files as Missing — e.g. after `rebar migrate --apply`.
- The registry check's hint says `rebar run compute-registry` instead of the
  project-local `scripts/compute-registry.sh`.

- **Versioned installer.** `setup-rebar.sh [<version>|latest|<branch>]`
  installs side by side under `~/.rebar/versions/<version>/`, with
  `~/.rebar/current` pointing at the active one and `~/.rebar/current/bin`
  on PATH. "latest" (the default) is the highest stable tag (the highest
  pre-release only while no stable tag exists); `--pre` includes
  pre-releases. Tags download a
  prebuilt tarball verified against the release's SHA-256 checksums, and
  build from source (git + Go) when the release has no assets; branches and
  full commit SHAs always build from source, replacing the committed darwin/arm64
  `bin/rebar`. `--pre`, `--no-switch`, `--source`. An existing `~/.rebar` clone is
  left untouched; versions are added inside it. Design:
  `feedback/2026-11-14-versioned-installer.md` (from #8).
- **Release workflow.** Pushing a `v*` tag builds darwin/linux ×
  amd64/arm64 with `scripts/package-release.sh` and publishes a GitHub
  Release (a pre-release for suffixed tags) with the tarballs and
  `rebar_<ver>_checksums.txt`.
- **`rebar version` can be stamped:** `-ldflags "-X
  github.com/willackerly/rebar/cli/cmd.Version=<ver>"`; branch builds from
  the installer report `<branch>@<sha7>`. README badges written by `rebar
  new` / `rebar adopt` still use the release string.
- **Adopters carry configuration, not rebar's scripts.** `rebar check`,
  `rebar check --pre-commit|--cold-start` and the new `rebar run <script>`
  run the rebar install's scripts against the project
  (`REBAR_PROJECT_ROOT`). The bootstrap template's `scripts/` is down to
  two `# rebar-thin-wrapper` entry points. Legacy project copies keep
  running, with a deprecation notice. Design and migration:
  `feedback/2026-11-14-self-contained-install.md` (from #8).
- **Repo-specific settings in `.rebarrc`:** declarative METRICS
  (`metric.<key>.*`, same `=` / `>=` semantics), registry classification
  and purpose settings, `source_dirs`, `source_extensions`,
  `freshness_max_days`.
- **`contracts_dir`** (default `architecture`) moves the contracts,
  registry and steward state anywhere in the repo; honored by every script
  and the CLI.
- **Registry purposes are cut cleanly by default** (`registry_purpose_style
  = ellipsis`): `|` is escaped so it can't break the table, and long
  purposes are cut to 77 characters plus `...` instead of at byte 80, which
  could split a multibyte character. **Upgrade note:** a registry generated
  with the old cut reports STALE once; regenerate it, or set
  `registry_purpose_style = bytes` to keep the old output.
- **`checks_enable`** (`.rebarrc`, env `REBAR_CHECKS_ENABLE`) runs named
  checks regardless of the tier, each as it would run at tier 3:
  `tier = 1` + `checks_enable = check-ground-truth` runs ground truth.
  Names are check script basenames without `.sh`; an unknown name warns
  once per run and is ignored. `rebar status` and `rebar audit --all` show
  the enabled checks. Unset, nothing changes. The registry check was never
  tier-gated (docs said tier 2+); they now say it runs at every tier.
- **`contract_namespace`** (optional, e.g. `github.com/org/repo`): source
  may cite this repo's contracts as `CONTRACT:<namespace>:<ID>.<v>` as well
  as the short form. Steward, compute-registry, check-contract-refs,
  check-registry and `rebar context developer` treat both alike
  (check-contract-headers only shows the qualified form in its hint); references
  qualified with another repo's namespace are counted, never treated as
  ours. Filenames stay unnamespaced; composes with `contracts_dir`; unset
  behaves exactly as before. From #8; see
  `feedback/2026-11-14-contract-namespace.md`.
- **Fix: the source scanners skipped too much.** check-todos, check-contract-refs,
  check-registry and compute-registry's non-git fallback dropped any match whose
  path or line text contained `dist`, `vendor`, `node_modules` or `.git`
  (a broken reference under `.github/` or in `distance.go`, a TODO mentioning
  `TODO.md`, every match when the project's own path contained `dist`). They now
  skip directories by name: `.rebarrc` `source_exclude_dirs` (default
  `node_modules vendor dist`) plus `.git` and `.claude` (agent worktrees, which
  were scanned before). `contracts_dir` is normalised (`./docs/specs/` →
  `docs/specs`); check-registry also scans `*.sh` and `bin/` and skips
  `CONTRACT-SEAM-TEMPLATE.md`. **Upgrade note:** checks can report findings
  they used to miss — e.g. TODOs in compiled `dist-esm/` output now block
  pre-commit; add such directories to `source_exclude_dirs`.
- **Fix:** `check-registry.sh` stopped silently (exit 1, no ORPHAN line)
  at the first contract with no implementing file.
- `sync-bootstrap.sh --check` also keeps the bootstrap template's
  `CONTRACT-TEMPLATE.md` identical to `architecture/`.

---

## v3.0.0 — 2026-11-14

Graduation of the v3 line. The four annealing betas (`v3.0.0-beta`,
`-beta.2`, `-beta.3`, `-beta.4`) shipped on `main`, honestly labeled; this
release drops the pre-release suffix. One fix since `v3.0.0-beta.4`; otherwise
only version strings, the rebuilt `bin/rebar`, and the recorded decision.

- **`rebar new` projects passed their own compliance check again.** `rebar new`,
  `rebar init` and `rebar audit --fix` wrote a hardcoded `v3.0.0-beta` into
  `.rebar-version` while the README badge used the CLI's version, so a fresh
  project failed `check-compliance.sh` ("Badge says v3.0.0 but .rebar-version
  says v3.0.0-beta"). All three now write the CLI's version, and the bootstrap
  template's `.rebar-version` and badge say `v3.0.0`. Broken since beta.3.

**What v3 is:** badges that stop lying by default. Contracts declare their
maturity (`Status:` stub → verified) and the compliance badge is weighted by
it; the steward's computed `verified` is renamed `impl-present` so the two
vocabularies never share a word; a SessionStart hook makes the cold start a
harness fact; and the field-proven peer-inbox paradigm, Claude Skills, and
abstract `rebar:` refs ship alongside. Details and upgrade notes are in the
beta entries below, starting with `v3.0.0-beta`.

**Graduation decision.** Decided by the project owner on 2026-11-14, not
measured to completion. Of the four criteria in `docs/v3-beta-plan.md`
§Graduation, criteria 2, 3 and 4 were met on 2026-08-02. Criterion 1 (two
migrated repos clean for ≥2 weeks) had its repo count met and a clean window
expected to close 2026-08-16, but it was never re-verified, and beta.3 and
beta.4 shipped changes after that date. It was accepted by the owner's
decision, recorded as D12 in the plan's decision log.

---

## v3.0.0-beta.4 — 2026-11-14

Installer fix. **Not** graduation; the §Graduation scoreboard was not
rescored.

- **`setup-rebar.sh`:** the one-line `curl … | bash` install failed on a
  stock Mac. `/bin/bash` 3.2 treats an empty `"${install_args[@]}"` as
  unbound under `set -u`, so the installer died after cloning unless
  `--server` was passed. Present since at least beta.2 (#12).
- `bin/rebar` rebuilt so `rebar version` reports `v3.0.0-beta.4`.

---

## v3.0.0-beta.3 — 2026-11-14

Third annealing increment on the v3 line, driven by an adopter
(virtru-analyze) whose parallel PRs conflicted on rebar's generated files.
**Not** graduation: the `docs/v3-beta-plan.md` §Graduation scoreboard was not
rescored for this release.

### Merge-friendly generated files for busy repos

Parallel PRs no longer conflict on rebar's generated files when they change
unrelated things. See `practices/busy-repos.md`.

- **`rebar commit` / `rebar sign`** no longer restamp `generated_at` /
  `generated_by` in `.rebar/integrity.json`; the header is written once by
  `rebar init`. `rebar status` shows it as "Created".
- **Ratchet floors are derived** from per-file entries instead of a stored,
  constantly raised `min`. Legacy stored floors stay as a frozen lower bound;
  `current` is no longer written.
- **`compute-registry.sh`** drops the `Generated:` timestamp and pins
  `LC_ALL=C`, so output is identical on every machine. New `.rebarrc` key
  `registry_impl_counts = false` (env `REBAR_REGISTRY_IMPL_COUNTS`) keeps the
  code-derived `Impl Files` column and Contract Health section out of the
  committed registry; `--health` prints them.
- **`compute-registry.sh`** counts implementing files over tracked files only
  (`git grep`) in a git work tree, so untracked scratch files and gitignored
  build output no longer change the registry; outside git it falls back to
  `grep -r`. The exclusions now match path components instead of substrings:
  only the top-level `architecture/` is skipped (a nested
  `website/src/pages/architecture/` file now counts), as are `node_modules/`,
  `vendor/` and `dist/` directories. Counts can shift once on regeneration.
- **`check-ground-truth.sh`** accepts `key >= N` floors in `METRICS`.
- **`_rebar-config.sh`** gains `_rebar_config <key> <ENV> <default>`.
- Regression tests: `scripts/tests/merge-friendly.test.sh`,
  `cli/internal/integrity/manifest_test.go`.

### Release housekeeping

- `rebar version` now reports the release (`v3.0.0-beta.3`). It had stayed
  at `v3.0.0-beta` through beta.2. `rebar new` and `rebar adopt` stamp the
  README badge from the same constant.
- `bin/rebar` rebuilt from this source (darwin/arm64).

---

## v3.0.0-beta.2 — 2026-08-02

Second annealing increment on the v3 line. **Not** graduation to `v3.0.0` —
scored against the four criteria in `docs/v3-beta-plan.md` §Graduation on
2026-08-02: criterion 2 met (a live seat armed the canonical watcher in real
cross-repo traffic and caught a genuine inbound); criterion 1 met on repo count
(`office180`, `go-tak-server`) but its clean window reset the same day; criterion
3 not met (three v3-feature defects filed 2026-08-02); criterion 4 unverified. No
criterion was amended or skipped — per D1, the tag is earned, not declared.

### Changed

- **Contract prefix numbering retired — the full descriptive name is the key.**
  `CONTRACT-BLOBSTORE.2.1.md`, not `CONTRACT-C1-BLOBSTORE.2.1.md`. The number was
  bookkeeping that collided on merge and broke every `CONTRACT:` reference when
  resolved; the type letter duplicated the contract's own `Type:` field. Verified
  by probe that the tooling never required either — `compute-registry.sh`,
  `check-contract-refs.sh`, `check-prefix-uniqueness.sh`, and `check-registry.sh`
  all accept unnumbered IDs unchanged. **Relaxation, not migration:** existing
  numbered contracts stay, never renumber, never mass-rename. Promoted to a
  top-level README callout. (`README.md`, `conventions.md` §Contract Naming,
  `architecture/README.md`, `CONTRACT-TEMPLATE.md`, `DESIGN.md`,
  `practices/contract-supersession.md`, bootstrap copies.)
- **Peer-inbox memo grammar reconciled** to the TYPE-bearing form
  (`YYYY-MM-DD-<sender>-<TYPE>-<description>.md`) that
  `practices/inbox-protocol.md` and the user-level skill already taught;
  `conventions.md` had diverged. The date is now normatively the sender's
  **local** deposit date — agents read UTC out of tooling, which west of
  Greenwich silently dates a memo tomorrow and breaks the arrival-order sort.
  Deposited memos are never renamed.
- `check-prefix-uniqueness.sh` marked **LEGACY-ONLY** in both copies. Behavior
  unchanged — a collision between two still-numbered contracts is still a real
  defect — but a passing result must not be read as "numbering is required."

### Added

- **`practices/inbox-protocol.md`** — the memo grammar itself: naming, the ten
  memo types, `[M]/[V]/[S]/[R]/[U]` confidence labels, crossed-memo handling, and
  the memo-vs-synchronous-ask rule. Distilled from the TDFLite ↔ nanotdf2
  exchange; complements `inbox-watch.md` (how you learn a memo arrived) and
  `federation.md` (the principles). Recommends a short root `INBOX.md` per repo,
  since coordination is load-bearing from a session's first minute.
- **`practices/diagram-and-document-assets.md`** — rebar's first diagram
  convention. A standalone `.pptv.svg` atom is the default when a figure outgrows
  an ASCII fence; markdown + `md2docx` when a document must open in Word. Adopted
  from `office180-md-office-converter` and pinned to `0.1.0-alpha.4`; the alpha
  pin is bounded because an atom degrades to a plain renderable SVG if the tooling
  disappears. Verified end to end before adoption.
- **Federation Principle 5 — "a held inbox is a watched inbox"**
  (`practices/federation.md`; owner-ratified 2026-07-11). Every repo
  holding a peer `inbox/` arms the **same canonical watcher**
  (`scripts/inbox-watch.sh`) on its **own** inbox at session start; no
  per-repo watcher variants. Generalizes the 2026-07-06 field SOP
  (own-inbox-only, stale-watcher check) from
  `feedback/2026-07-06-inbox-watch-self-echo-scope-to-own-inbox.md`.
- `scripts/inbox-watch.sh`: arm-time Principle-5 checks — a **per-inbox PID
  lock** (`.inbox-watch.lock`, a hidden dotfile invisible to the watch) that
  warns only when another **live** watcher already holds the **same** inbox
  (real double-coverage), and reclaims a crashed watcher's stale lock silently;
  plus an own-inbox scope warning when a watched dir resolves outside the
  current repo. Warn-only; never blocks. The lock replaced an earlier
  process-global `pgrep inbox-watch` check that false-positived on every
  legitimate sibling seat in a single-machine, multi-seat setup (go-tak-server
  / _atlas, 2026-07-11) — cry-wolf that defeats the warning's purpose. Lock is
  git-ignored (bootstrap + rebar). Template copy synced byte-identical
  (`templates/project-bootstrap/scripts/inbox-watch.sh`).
- Project bootstrap now ships a top-level `inbox/` (with a one-line
  README) so adopter repos are born holding a peer inbox.
- Peer-Inbox Convention (`conventions.md`): Principle-5 bullet and an
  optional `reply-by:` frontmatter field (courtesy deadline signal for
  coordination seats; absence means human-ish timescales).
- `practices/inbox-watch.md`: Scope SOP section (own inbox only, stale
  watchers); `practices/session-lifecycle.md` step 4 aligned.

### Fixed

- `scripts/inbox-watch.sh`: **full-backlog re-emit on inbox git operations.**
  The per-poll baseline update replaced the seen-set with the current listing
  (`mv cur snap`), so a git op (merge/checkout/stash) that transiently
  removes-and-restores tracked inbox files reset the baseline to the empty/
  partial mid-operation state — and the whole backlog then re-diffed as new on
  the next poll. Now the seen-set is an **append-only ledger** (`sort -u` union
  each poll): a filename, once reported, stays reported for the watcher's life,
  so transient disappearance-and-return cannot re-notify. Field-reported by the
  go-tak-server seat (2026-07-11, one 8-file burst); reproduced and regression-
  tested (old logic emits the backlog, new logic emits only genuine deposits).
  Matters under Principle 5: a fleet-wide re-emit would train every seat to skim
  NEW-deposit events — the exact reflex the principle exists to prevent.
  Trade-off (correct for the inbox convention): a deleted-then-recreated
  same-name file will not re-notify; inbox names are dated, unique, append-only.
  Template copy synced byte-identical.
- `scripts/inbox-watch.sh`: **collation follow-up to the ledger fix — phantom
  re-emit every poll under a non-C locale.** The first ledger fix unioned with a
  bare `sort -u`, which re-collates under the ambient locale, while `list_dir`
  emits `LC_ALL=C`-sorted listings. With mixed-case filenames (the memo norm),
  the ledger and the current listing then sorted differently and `comm` merge-
  walked two mis-ordered files, reporting a stable subset as new **every poll,
  permanently** — worse than the git-op bug it replaced. Now `LC_ALL=C` is
  pinned on both the union sort and the `comm` walk, so every sort/merge over
  these files shares the one C collation. Field-reported same day by the
  go-tak-server stress rig; the earlier regression test missed it because its
  fixtures were lowercase-only (identical under C and locale collation).

### Added

- `scripts/tests/inbox-watch.test.sh`: regression guard for both inbox-watch
  bugs above. Uses mixed-case, digit-heavy fixtures and forces a divergent
  UTF-8 locale (skips loudly if none is installed, rather than passing falsely
  under C), then asserts a full lifecycle — armed baseline silent, one emit per
  genuine deposit, silent through a transient remove/restore — emits **exactly**
  the genuine deposits. Confirmed to fail against both pre-fix revisions.
- `scripts/tests/inbox-watch-lock.test.sh`: regression guard for the per-inbox
  lock — asserts the lock is written with the watcher's PID and hidden from
  `ls -1`, a second watcher on the same inbox warns and does not steal it, a
  clean exit releases it, and a stale (dead-PID) lock is reclaimed silently.

## v3.0.0-beta (2026-07-04)

Seven clusters consolidated onto one trunk — the alpha→beta hard move.
Decision log: `docs/v3-beta-plan.md`. The `v3.0.0-alpha`/`v3.0.1-alpha`
branches are **retired** (kept on origin for provenance; all their
content is on this line).

### Breaking (migration notes for upgraders)

1. **Steward's computed top lifecycle state renamed `verified` →
   `impl-present`** (S1-STEWARD 2.0). Anything parsing steward output
   must migrate: JSON `summary.contracts.verified` →
   `summary.contracts.impl_present`; report row and `--summary` suffix
   (`Xv` → `Xip`) likewise. Rationale: computed-from-file-presence
   "verified" overclaimed (`feedback/2026-06-19-trustable-status-…`).
2. **Contracts carry a declared-maturity `Status:` header** —
   `stub | draft | in-progress | active | verified` (canonical
   definitions: `conventions.md` §Declared Maturity). The old
   `draft | active | deprecated` template vocabulary is replaced;
   `superseded`/`deprecated`/`retired` are terminal and excluded from
   weighting. `check-compliance.sh` Check 9 weights the tier badge:
   <33% stub-or-draft → as declared; 33–66% → "— IN PROGRESS"
   annotation; >66% → demoted one tier (exit 1). **Repos with no
   `Status:` fields are treated as pre-v3: advisory only, no penalty** —
   upgrade by adding one line per contract, honestly.
3. **SessionStart hook expectation.** `.claude/settings.json` runs
   `scripts/cold-start-checks.sh` on session start (guarded — prints an
   install pointer if the script is absent). `rebar init/new/adopt` now
   install the hook, the four skills, and the core scripts.
   Adopting by copy: use `cp -r templates/project-bootstrap/. <target>/`
   — the old `/*` glob silently dropped `.claude/`.
4. **`ci-check.sh` gained two unconditionally-registered checks**
   (`check-jtbd-presence.sh`, `check-prefix-uniqueness.sh`) and
   `check-decay-patterns.sh` gained the P8 demo-bypass pattern — Tier-3
   repos that were green on v2 can newly fail. Escape hatches while you
   migrate: `SKIP_JTBD=1`, `SKIP_PREFIX_UNIQUENESS=1`; P8 findings mean
   a demo/UAKS spec is faking user actions — fix the spec.

### Added

- **Peer-inbox paradigm** (field-verified, tak cluster 2026-07): repo-level
  `inbox/` convention in `conventions.md`, `scripts/inbox-watch.sh`
  persistent watcher, coordination-seat cold-start hygiene
  (`practices/session-lifecycle.md` step 4, `practices/inbox-watch.md`)
- **Claude Skills packaging**: `rebar-coldstart`, `rebar-feedback`,
  `rebar-audit`, `rebar-inbox-watch` in `.claude/skills/` + bootstrap
  template — skills are pointers to practices, never copies
- **`scripts/cold-start-checks.sh`**: enforcement quad + maturity counts,
  `<rebar-cold-start>` wrapped, always exits 0
- **`practices/test-fidelity.md`**: five-rung fidelity ladder, UAKS tier,
  closed-loop demo gate; `check-decay-patterns.sh` P8-demo-bypass
- **`agents/FANOUT_PATTERN.md`** + `subagent-guidelines.md` §Verify
  Before Relying
- **`practices/spike-first-contracts.md`**, **`practices/contract-supersession.md`**,
  **`scripts/check-jtbd-presence.sh`**, **`scripts/check-prefix-uniqueness.sh`**
  (both checks wired into `ci-check.sh` — now 15 checks)
- **conventions.md**: Declared Maturity, Peer-Inbox Convention, and
  "Reaching the Agent: Hook vs Skill vs Ask" doctrine

### Changed

- `conventions.md` computed-lifecycle table: `VERIFIED` → `IMPL-PRESENT`
  with rename note
- `installer`: thin curl|bash shim over the canonical CLI (carried from
  the alpha line, #2)

---

## v2.0.1 (2026-04-26)

### Fixed
- **`bin/ask` resilient session resume** — when `claude --resume <id>` returns "No conversation found with session ID …" (vanished `~/.claude` store, interrupted first call, server-side retention boundary), `_ask_direct` now wipes the stale `.session-id`, regenerates a fresh UUID, and replays the question once with full agent context. Prints `[<agent>] session expired, restarting fresh` to stderr so the recovery is visible. Retry is capped at one attempt and only fires on the specific stale-session signature — auth/network/rate-limit errors still surface immediately.

---

## v2.0.0 (2026-04-01)

### Added
- **Session lifecycle protocol** — `practices/session-lifecycle.md`: start/checkpoint/end framework with wrapup template, marathon session guidance, architect review checkpoints
- **Red team protocol** — `practices/red-team-protocol.md`: 5-persona adversarial review strategy (adversarial user, performance, security, fidelity, API/contract)
- **Visual fidelity methodology** — `practices/visual-fidelity.md`: ground truth, RMSE measurement, oracle pattern, human emulator tests, regression prevention
- **Red team subagent template** — `agents/subagent-prompts/red-team.md`: multi-persona template with structured JSON output and fix DAG
- **Product review subagent template** — `agents/subagent-prompts/product-review.md`: BDD alignment, persona fit, flow completeness, scope assessment
- **Seam contracts** — new contract type for integration points across language/protocol boundaries; `architecture/CONTRACT-SEAM-TEMPLATE.md`
- **`rebar context` CLI command** — context shepherd that cats role-relevant files in reading order (`rebar context`, `rebar context architect`, `rebar context session-start`, etc.)
- **Context refresh script** — `templates/project-bootstrap/scripts/refresh-context.sh`: automated QUICKCONTEXT staleness checker
- **Feedback status tracking** — `feedback/README.md` now includes `Status:` and `Template impact:` fields

### Changed
- **Cold Start Quad enhanced** — staleness verification step added between QUICKCONTEXT and TODO reads
- **AGENTS.template.md** — session lifecycle reference, priority tracking rule (QUICKCONTEXT is single source of truth), issue dedup rule
- **CLAUDE.template.md** — session-end protocol added, `refresh-context.sh` in health check
- **TODO.template.md** — forward-looking only (<50 lines open items), completed items in collapsed `<details>` section
- **QUICKCONTEXT.template.md** — "What's Next" section as canonical priority list
- **Worktree collaboration** — fan-out merge ordering strategy (HOT/WARM/COLD), shared mock consolidation rule, worktree lifecycle checklist, cherry-pick best practices
- **Multi-agent orchestration** — file-level conflict matrix and interface-change sequencing in pre-launch audit
- **UX review template** — interaction stability (human emulator) dimension added
- **Security surface scan template** — red team mode (adversarial mindset) section added
- **DESIGN.md** — seam contracts in §3 (The Contract System), session lifecycle in §5 (The Information Environment)
- **README.md** — battle-tested results updated with OpenDocKit (15+ agents, 8K tests) and filedag (40+ agents, 62 commits in 48hrs)

### Why v2.0

Three field reports from production deployments (Dapple SafeSign, OpenDocKit, filedag) converged on the same meta-insight: rebar's structural protocols work; its behavioral protocols don't. v2.0 addresses the session lifecycle gap, adds adversarial review and visual fidelity practices, introduces seam contracts for integration points, and adds the `rebar context` CLI command. The core contract system is unchanged — this is a methodology evolution, not a rewrite.

### Migration from v1.2.0
1. Copy new practice files: `practices/session-lifecycle.md`, `practices/red-team-protocol.md`, `practices/visual-fidelity.md`
2. Copy new subagent templates: `agents/subagent-prompts/red-team.md`, `agents/subagent-prompts/product-review.md`
3. Copy `architecture/CONTRACT-SEAM-TEMPLATE.md`
4. Copy `templates/project-bootstrap/scripts/refresh-context.sh` to your project's `scripts/`
5. Re-diff your AGENTS.md against the new template — session lifecycle and priority tracking sections are new
6. Re-diff your CLAUDE.md — session-end protocol and staleness verification are new
7. Consider shortening your TODO.md — move completed items to a collapsed section
8. Add a "What's Next" section to QUICKCONTEXT.md as your canonical priority list
9. Rebuild rebar CLI: `cd cli && go build -o ../bin/rebar .`
10. Update `.rebar-version` to `v2.0.0`

---

## v1.2.0 (2026-03-20)

### Added
- **Scalability overhaul** — AGENTS.template.md slimmed from 917 to 382 lines
- **`practices/` directory** — multi-agent orchestration, E2E testing, deployment patterns, worktree collaboration extracted as reference guides
- **Computed registry** — `scripts/compute-registry.sh` generates CONTRACT-REGISTRY.md from contract files on disk (replaces manual maintenance)
- **Memory compaction** — `ask compact <agent>` summarizes old memory entries, auto-triggers at 50KB threshold
- **Script versioning** — all scripts have `# rebar-scripts: YYYY.MM.DD` headers; steward detects stale copies
- **Tier-aware enforcement** — `.rebarrc` configures tier (1=partial, 2=adopted, 3=enforced); scripts skip inapplicable checks
- **Team-size profiles** — `profiles/solo-dev.md`, `profiles/small-team.md`, `profiles/department.md`
- **Conventions minimum viable** — Tier 1 section at top of conventions.md
- **Rebar version tracking** — `.rebar-version` file + README badge for adopting repos
- **Adoption level validation** — steward checks README badge against actual compliance
- **CHANGELOG.md** — you're reading it

### Changed
- `AGENTS.template.md` — mandatory foundations only; advanced practices moved to `practices/`
- `architecture/CONTRACT-REGISTRY.template.md` — now documents the computed format
- `scripts/check-registry.sh` — deprecated in favor of `compute-registry.sh`
- `scripts/steward.sh` — uses `compute-registry.sh --check`, adds compliance validation
- `scripts/ci-check.sh` — uses `compute-registry.sh --check`
- `conventions.md` — added minimum viable section for Tier 1
- `SETUP.md` — added tier selection, team-size profiles, practices/ copy step
- All profile files — updated section references for practices/

### Migration from v1.1.0
1. Copy `practices/` directory into your project
2. Copy `scripts/compute-registry.sh` and `scripts/_rebar-config.sh`
3. Create `.rebarrc` from `.rebarrc.template` (set your tier)
4. Create `.rebar-version` with `v1.2.0`
5. Add rebar badge to top of your README.md (see README.template.md)
6. Re-diff your AGENTS.md against the new AGENTS.template.md — moved sections are now in `practices/`
7. Run `scripts/compute-registry.sh` to generate your registry
8. Optional: update all scripts from rebar (check `# rebar-scripts:` dates)

---

## v1.1.0 (2026-03-19)

### Added
- Merger agent — branch integration + conflict resolution (actor agent)
- Subagent LOE levels in prompt index
- Scout Rule — zero tolerance for skipped or failing tests
- Adoption badges — PARTIAL / ADOPTED / ENFORCED tiers
- AI-native contracts feedback — cross-repo namespacing (`CONTRACT:namespace/ID`)
- Blindpipe adoption feedback — ASK as context preservation, role discipline pattern

### Changed
- `ask init` now creates merger agent directory
- `AGENTS.template.md` — added Scout Rule section, merge coordinator flow

### Migration from v1.0.0
1. Copy `agents/merger/` directory
2. Merge Scout Rule section into your AGENTS.md (§Testing Expectations)
3. Optional: add adoption badge to README.md

---

## v1.0.0 (2026-03-17)

### Added
- Cold Start Quad templates — README, QUICKCONTEXT, TODO, AGENTS
- CLAUDE.md template with full Claude Code configuration
- Contract system — CONTRACT-TEMPLATE, CONTRACT-REGISTRY, naming conventions
- Methodology.md — full philosophy document
- Conventions.md — branch naming, commits, headers, discovery taxonomy
- 5 enforcement scripts — contract-headers, contract-refs, TODOs, freshness, ground-truth
- Steward — automated project health scanner with JSON + markdown output
- CI check runner and pre-commit hook
- ASK CLI — role-based agent queries with persistent sessions
- 8 subagent templates — code review, contract audit, security scan, UX review, doc drift, feature inventory, test shard, merge coordinator
- 4 project profiles — web-app, api-service, crypto-library, cli-tool
- SETUP.md — step-by-step adoption guide
- METRICS template with ground truth verification
- Learnings from OpenDocKit — 37KB of battle-tested patterns

### Migration
This is the initial release. Follow SETUP.md.
