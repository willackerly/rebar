# Busy Repos — Keeping Generated Files Merge-Friendly

**Read when several PRs are open at once and rebar files conflict on every merge.**

---

## The problem

Rebar commits generated files: `.rebar/integrity.json`,
`architecture/CONTRACT-REGISTRY.md` and `METRICS`. A generated file causes a
merge conflict when every branch rewrites the same line, even if the branches
change unrelated things. In one busy adopter (virtru-analyze, 2026-11), over
30 days `integrity.json` changed in 133 commits, the registry in 84 and
`METRICS` in 52. Most of those diffs were one of these lines:

| File | Line every branch rewrote | Fixed by |
|------|---------------------------|----------|
| `.rebar/integrity.json` | `generated_at` restamped on every `rebar commit` | CLI: the header is written once by `rebar init` |
| `.rebar/integrity.json` | `ratchets` `min`/`current`, raised on every commit that adds a test | CLI: floors are derived from the per-file entries |
| `CONTRACT-REGISTRY.md` | `<!-- Generated: <timestamp> -->` | Script: no timestamp, C collation |
| `CONTRACT-REGISTRY.md` | `Impl Files` count, changed by any PR that adds a `CONTRACT:` header | Config: `registry_impl_counts = false` |
| `METRICS` | `test_files = 352`, changed by any PR that adds a test | Config: write `test_files >= 352` |

GitHub's merge button and merge queue ignore `.gitattributes` merge drivers,
so `merge=ours` or `merge=union` does not help on PRs. The files have to be
conflict-free by construction.

## What is automatic

With this version of the CLI and scripts, no setup is needed for these:

- **`integrity.json` header is stable.** `rebar commit` and `rebar sign` change
  only the entries of the files they touch. Two branches that touch different
  protected files merge cleanly, including neighboring entries.
- **Ratchet floors are derived.** The floor for `total_assertions` is the sum of
  the recorded per-file `assertion_count`; `contract_count` and
  `test_file_count` are the number of tracked entries. Floors stored by older
  versions stay as a frozen lower bound and are never raised. The first
  `rebar commit` after upgrading drops their `current` lines once; that edit is
  identical on every branch, so it merges cleanly.
- **The registry is deterministic.** No timestamp, and glob order does not
  depend on the caller's locale, so two machines regenerating the same
  contracts produce identical bytes.

## What to configure

Two derived counts can't merge correctly however they are written. If two PRs
each add an implementing file for the same contract, both change its count from
13 to 14. Git merges the identical change cleanly and main says 14 when the real
count is 15, so main's `compute-registry.sh --check` fails. Keep these counts
out of committed files:

```ini
# .rebarrc
registry_impl_counts = false
```

The registry then has no `Impl Files` column and no Contract Health section.
Read them on demand:

```bash
scripts/compute-registry.sh --health   # impl counts + zombie/shadow contracts
scripts/check-registry.sh              # orphan contracts, exits 1 on untracked ones
```

For `METRICS`, use floors for counts that most PRs change:

```ini
# METRICS
test_files >= 352       # passes while the real count is at least 352
ci_workflows >= 33
go_modules = 16         # exact: changes rarely, worth a deliberate edit
```

Raise a floor occasionally (for example at release time), not in every PR.

## What still conflicts

Changes that really overlap still conflict, as they should:

- two PRs editing the same contract's text, or the same protected file
- two PRs adding entries that sort next to each other at the end of an
  `integrity.json` category (the previous last entry gains a trailing comma)
- two PRs adding contracts whose registry rows are adjacent

To resolve an `integrity.json` conflict, take either side, then rehash the
protected files you changed with `rebar commit`. Don't hand-merge hashes.

## Adopting in an existing repo

1. Get this version's `compute-registry.sh` and ground-truth engine. With
   copied scripts, `rebar migrate --apply` moves your customizations into
   `.rebarrc` and deletes the copies it verified, so the install's current
   scripts run (with `registry_purpose_style = bytes` if your registry needs
   the old purpose cut). Otherwise update `scripts/compute-registry.sh`,
   `scripts/_rebar-config.sh` and the engine of `scripts/check-ground-truth.sh`
   (below the "do not modify" line) by hand, keeping local customizations.
2. Add `registry_impl_counts = false` to `.rebarrc` and regenerate the registry
   once: `scripts/compute-registry.sh`.
3. Change churning `METRICS` lines to `>=` floors.
4. Pin the CLI version that keeps the manifest header stable.
5. Remove `.rebar/integrity.json merge=ours` from `.gitattributes`. It needs a
   local `merge.ours.driver` setting to do anything, and it never applies to
   PR merges on GitHub.

Regression test: `scripts/tests/merge-friendly.test.sh`, and
`TestBranchesTouchingDifferentFilesMergeCleanly` in `cli/internal/integrity`.
