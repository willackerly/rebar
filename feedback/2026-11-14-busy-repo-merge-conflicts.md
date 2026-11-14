# Busy repos: rebar's generated files conflict on every PR

**Source:** virtru-analyze (rebar v2.0.0 pin), 2026-11-14
**Type:** bug + feature
**Status:** implemented in the same PR; open questions below

## Evidence [M]

`git log origin/main --since='30 days' --name-only` in virtru-analyze, ranked
by commit count:

| File | Commits | Typical diff |
|------|--------:|--------------|
| `.rebar/integrity.json` | 133 | only `generated_at` (4 of the last 4) |
| `architecture/CONTRACT-REGISTRY.md` | 84 | `Generated:` stamp + `Impl Files` counts |
| `METRICS` | 52 | `test_files`, `ci_workflows` +1 |

Every PR that ran `rebar commit` rewrote `generated_at`, so any two open PRs
conflicted. The adopter's `.gitattributes` has `.rebar/integrity.json
merge=ours`, which does nothing without a local `merge.ours.driver` and is
ignored by GitHub's merge button and merge queue. The adopter also forked
`compute-registry.sh` locally (C.UTF-8 locale, `git grep` over tracked files)
to stop the registry flip-flopping between machines.

## Design

1. **Manifest header written once.** `Save()` no longer restamps
   `generated_at`; `rebar commit` and `rebar sign` no longer rewrite
   `generated_by`. `rebar status` labels the timestamp "Created".
2. **Derived ratchet floors.** `total_assertions` = sum of per-file
   `assertion_count`; `contract_count` / `test_file_count` = tracked entry
   counts. `rebar commit` checks the disk against the floors from before the
   commit (the same guarantee as raising a stored min). Stored floors from old
   manifests stay as a frozen lower bound; `current` is no longer written.
3. **Deterministic registry.** No `Generated:` line (`--check` still ignores
   one in old files); `LC_ALL=C` so glob order is locale-independent.
4. **`registry_impl_counts`** in `.rebarrc` (env
   `REBAR_REGISTRY_IMPL_COUNTS`), default `true`. When `false`, the committed
   registry omits `Impl Files` and Contract Health; `--health` prints them.
   Read through a new `_rebar_config` helper in `_rebar-config.sh`.
5. **METRICS floors.** `key >= N` passes while the computed value is at least
   N. Closes the INVENTORY watchlist item "`compute-metrics.sh` … wait for real
   merge-conflict pain" with a smaller change: floors, not generation.
6. **`practices/busy-repos.md`** — the why, the knobs and adoption steps.

## Red team

- *Derived floors weaken the ratchet?* Not for `rebar commit`: it checks the
  disk against the pre-commit manifest, as the stored min did. Hand-editing a
  per-file `assertion_count` lowers the floor, just as hand-editing the stored
  `min` did before; both show up in review as an `integrity.json` change with
  no matching test change. Neither version compares a PR against its base
  branch in CI.
- *Identical-change merges hide drift.* That is exactly why impl counts must be
  off in busy repos: two PRs each taking a count from 13 to 14 merge cleanly
  and leave main at 14 instead of 15. `merge-friendly.test.sh` T2 pins this.
- *Removing the `Generated:` line makes every adopter's registry stale once.*
  Only in generate mode — `--check` ignores the line, so CI stays green until
  someone regenerates.
- *`LC_ALL=C` purpose truncation.* `head -c 80` was already byte-based; output
  is now identical on every machine. A cut can still split a multibyte
  character, as before.

## Open questions for maintainer

1. **Default for `registry_impl_counts`.** Lean: keep `true` for the 3.0 line
   (no output change for small repos), flip to `false` in 3.1 since the counts
   are never merge-safe and `--health` gives the same data.
2. **Tracked-files-only impl counting** (`git grep`, as virtru-analyze does) —
   worth upstreaming for repos that keep counts on? Lean: yes, separate PR.
3. **A local merge driver** (`rebar integrity merge`) for rebases. Lean: skip —
   GitHub ignores drivers, and with the header stable the remaining conflicts
   are real overlaps.

## Disposition

Questions answered 2026-11-14:

1. **Yes.** `registry_impl_counts` stays `true` on the 3.0 line and flips to
   `false` in 3.1. Queued in INVENTORY.
2. **Yes.** Tracked-files-only impl counting (`git grep`) ships as a separate
   follow-up PR.
3. **Skip.** No local merge driver.
