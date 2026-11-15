# Upgrading rebar

How to move an adopting project to rebar 3.1 from v2.0.0 or v3.0.x. Follow
the steps in order; each one says what to run and how to tell it worked.

**What changes in 3.1.** rebar's checks now run from the rebar install. A
project keeps only configuration (`.rebarrc`, `METRICS`, its contracts) and
two small entry points, `scripts/ci-check.sh` and `scripts/pre-commit.sh`,
that call `rebar check`. If your project copied rebar's scripts into
`scripts/` (every v2.0.0 and v3.0.x adopter did), those copies keep working
after the upgrade, with a deprecation notice. `rebar migrate` turns them into
`.rebarrc` settings and removes the ones the install verifiably reproduces.
Design notes: [self-contained install](feedback/2026-11-14-self-contained-install.md),
[rebar migrate](feedback/2026-11-14-rebar-migrate.md),
[versioned installer](feedback/2026-11-14-versioned-installer.md).

In the commands below, `v3.1.0-rc.1` is the release you are moving to. Use
the tag you are actually installing (for example `v3.1.0` once it ships).

| You are on | Steps |
|------------|-------|
| v2.0.0 (badge says v2.0.0, ~15 copied scripts, no `.claude/`) | all; see [Notes for v2.0.0](#notes-for-v200) |
| v3.0.x (badge says v3.0.0 or a v3.0.0 beta, ~27 copied scripts) | all; see [Notes for v3.0.x](#notes-for-v30x) |
| already thin (`scripts/` holds only `ci-check.sh` and `pre-commit.sh`) | 1, 4–11 |

## 1. Install the new version next to the old one

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.0-rc.1/setup-rebar.sh \
  | bash -s -- v3.1.0-rc.1 --no-switch
```

Fetch the installer from the tag you install: until 3.1 is on `main`,
`main`'s `setup-rebar.sh` is the old installer and has no version argument
(see [Testing a release candidate](SETUP.md#testing-a-release-candidate)).

The new version lands in `~/.rebar/versions/v3.1.0-rc.1/`, a complete rebar
tree. `--no-switch` leaves `~/.rebar/current` (the version on your PATH)
alone, so other projects are unaffected while you try this one. The
exception is a machine with **no versioned install yet**, including a
`~/.rebar` that is a plain git clone from the old installer: the first version
always becomes `current`, and `~/.rebar/current/bin` is added to your shell RC
after the old `~/.rebar/bin` line, so new shells run 3.1. The clone itself is
not touched; `.mcp.json` files and `REBAR_ROOT` values that name it keep
working.

To run a version that is not `current`, call it by path; each version runs
its own scripts:

```bash
~/.rebar/versions/v3.1.0-rc.1/bin/rebar version
```

Switch for good with `ln -sfn versions/v3.1.0-rc.1 ~/.rebar/current` (or
re-run the installer without `--no-switch`). The rest of this runbook writes
plain `rebar` for the 3.1 CLI.

If `REBAR_ROOT` is set in your shell, the CLI runs that tree's scripts rather
than the installed version's; unset it unless you mean it.

## 2. Migrate copied scripts: `rebar migrate`

Skip this step if `scripts/` holds only the two thin entry points.

Start from a clean work tree (commit or stash first): `--apply` refuses
uncommitted or untracked files, so that git can undo everything it does.
`--allow-dirty` overrides that, but then git cannot restore those files.

Dry run first. It changes nothing in the project (every probe runs in a
scratch copy):

```bash
rebar migrate
```

It takes seconds on a small project and a few minutes on a large one
(virtru-analyze: about 3 minutes). For every copy of a rebar script in
`scripts/` it runs the copy and the install's script and prints:

- **`.rebarrc` additions**, between `# --- rebar migrate … ---` markers and
  grouped by the script they came from: `metric.<key>.*` lines translated
  from your `check-ground-truth.sh`, registry settings from your
  `compute-registry.sh`, `source_dirs` / `source_extensions` from your
  scanners. Lines already in `.rebarrc` are never edited or duplicated.
- **delete / replace / wrap.** A copy goes when the install, with the derived
  settings, behaves the same: every metric has the same value,
  `compute-registry.sh --stdout` is byte-identical, other checks give the
  same exit status and findings (the source scanners also on a probe tree).
  An unmodified copy of an older release goes too, with the difference
  listed (for a scanner, the findings the install adds on your project, and
  the `source_exclude_dirs` remedy). Scripts that cannot be run safely
  (`steward.sh`, `inbox-watch.sh`, …) go when they are byte-identical to a
  released version; the CLI carries the list, so this works on any install.
  `ci-check.sh`, `pre-commit.sh` and `scripts/README.md` are replaced by the
  template's files. A verified copy that another project file calls is not
  deleted but **wrapped** (see step 3).
- **keep**, with the reason: a customized copy the install does not
  reproduce keeps running as before, and so does any copy a kept copy calls.
- **METRICS edits**: counts such as `shell_scripts` that the deletions change.
- **CONTRACT-REGISTRY.md**, when removing copies changes it (an old
  `steward.sh` carries a `CONTRACT:` header the registry lists): apply
  regenerates it and says why.
- **Notes**: callers of wrapped copies (file:line), CI files that now need the
  rebar CLI, the integrity manifest, and a proposed `checks_enable` when your
  copies ran checks below the install's tier gate (see step 7). migrate never
  changes `tier`.

When the dry run looks right, apply it:

```bash
rebar migrate --apply
```

`--apply` writes exactly what the dry run showed, reruns each check that had
a copy, prints `PASS`/`FAIL` with the result from before, and if a check that
passed before now fails, it rolls everything back and exits 1. A second
`rebar migrate` should say `Nothing to migrate.` (copies it kept stay
listed). Review with `git status` and `git diff`; don't commit yet.

Add `--regenerate-registry` to switch the registry to the 3.1 purpose format
in the same run (step 5) instead of keeping its old bytes.

## 3. Callers, wrappers and hooks

A copy that other files call (a project CI script, `.pre-commit-config.yaml`,
`.claude/settings.json`) becomes a thin wrapper: a few lines marked
`# rebar-thin-wrapper` that run `rebar run <name>` for this project, so the
callers keep working. Replace a caller with the `rebar run` form when
convenient, then delete the wrapper:

| Caller runs | Can run instead |
|-------------|-----------------|
| `scripts/check-todos.sh` (any check) | `rebar run check-todos` |
| `scripts/compute-registry.sh [--check]` | `rebar run compute-registry [--check]` |
| `scripts/cold-start-checks.sh` | `rebar check --cold-start` |
| `scripts/ci-check.sh`, `scripts/pre-commit.sh` | unchanged: they are the thin entry points |

`rebar run` with no arguments lists every script the install provides.

- **Git hook.** `.git/hooks/pre-commit → ../../scripts/pre-commit.sh` keeps
  working; the target is now the thin wrapper. If you never linked it:
  `ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit`.
- **Claude Code SessionStart hook** (v3.0.x projects). The v3.0.x
  `.claude/settings.json` runs `scripts/cold-start-checks.sh` (migrate wraps
  it for that reason). The 3.1 hook runs `rebar check --cold-start` instead:

  ```bash
  T=~/.rebar/versions/v3.1.0-rc.1/templates/project-bootstrap
  diff .claude/settings.json $T/.claude/settings.json
  # unmodified: copy it; customized: replace the SessionStart "command" by hand
  cp $T/.claude/settings.json .claude/settings.json
  rm scripts/cold-start-checks.sh           # the wrapper, once nothing calls it
  ```

  Deleting a wrapper changes counts that include it: if `METRICS` counts
  scripts (migrate set `shell_scripts` with the wrapper still there), lower
  the number, or ground truth reports DRIFT wherever it runs (tier 3, or
  `checks_enable`, step 7).

  The skills in `.claude/skills/` were rewritten to say `rebar run`; compare
  with `diff -r .claude/skills $T/.claude/skills` and copy the ones you have
  not changed. v2.0.0 projects have no `.claude/`: `rebar init` adds the hook
  and skills, `.mcp.json` and the role agents (it never overwrites existing
  files).
- **CI.** See [step 10](#10-ci).

## 4. Bump `.rebar-version` and the README badge

`check-compliance` fails when the two disagree, so change both together:

```bash
echo v3.1.0-rc.1 > .rebar-version
sed -i.bak 's/^> \*\*rebar v[^*]*\*\*/> **rebar v3.1.0-rc.1**/' README.md && rm README.md.bak
sed -n 3p README.md   # > **rebar v3.1.0-rc.1** | **Tier 1: PARTIAL**
```

The badge is the first line after the title. Keep its tier as it was; it must
match `tier` in `.rebarrc`.

## 5. The registry: regenerate it once, or keep the old bytes

3.1 cuts registry purposes differently (`registry_purpose_style = ellipsis`,
the default: `|` escaped, long purposes cut to 77 characters plus `...`). A
registry generated by an older script cuts at byte 80, so it reports STALE
once. migrate keeps your registry byte-identical by adding
`registry_purpose_style = bytes` when it needs to. Pick one:

- **Regenerate (recommended).** Delete the `registry_purpose_style = bytes`
  line from `.rebarrc` if migrate added it, then run
  `rebar run compute-registry` (or use `--regenerate-registry` in step 2).
- **Keep the old output.** Leave `registry_purpose_style = bytes` in
  `.rebarrc`.

Then check:

```bash
rebar run compute-registry --check     # "OK: Registry is current."
```

The registry counts implementing files that git tracks; after adding
contracts or source files, `git add` them before regenerating.

## 6. Busy repos: keep churn out of committed files

If several PRs are open at once (see [practices/busy-repos.md](practices/busy-repos.md)):

```ini
# .rebarrc — drop the Impl Files column and Contract Health section
registry_impl_counts = false
```

then `rebar run compute-registry` once. Read the counts on demand with
`rebar run compute-registry --health`.

In `METRICS`, turn counts that most PRs change into floors:

```ini
test_files >= 353       # passes while the real count is at least 353
go_modules = 17         # exact: changes rarely
```

## 7. `checks_enable`: checks your tier would skip

Tier gates in the install: contract headers, freshness, JTBD and prefix
uniqueness run from tier 2, ground truth only at tier 3; the registry check
runs at every tier. A copied script without a tier gate ran at any tier, so
after migrate it would quietly SKIP. migrate proposes the line that keeps it
running, for example:

```ini
tier = 1
checks_enable = check-ground-truth
```

Names are check script names without `.sh` (the same names as `rebar run`).
You can add checks yourself, for instance ground truth in a tier 1 or 2
project whose `METRICS` you want verified. Unknown names print one warning
and are ignored. `rebar status` shows the enabled checks.

## 8. Optional settings

All default to the old behavior; set them only if you need them. See the
commented examples in the template
[`.rebarrc`](templates/project-bootstrap/.rebarrc).

- `source_exclude_dirs` (default `node_modules vendor dist`; `.git` and
  `.claude` are always skipped): directory names the source scanners skip.
  3.1's scanners exclude by directory name rather than by substring, so they
  can report findings the old copies missed (for example a TODO in a
  `dist-esm/` build output); migrate lists those before `--apply`. Add such
  directories here.
- `contracts_dir = specs` if your contracts (and `CONTRACT-REGISTRY.md`, and
  the steward's `.state/`) live somewhere other than `architecture/`. Move the
  files with `git mv`, set the key, point any `metric.<key>.paths` that named
  `architecture` at the new directory, and run `rebar run compute-registry`.
- `contract_namespace = github.com/org/repo` lets source cite this repo's
  contracts as `CONTRACT:github.com/org/repo:<ID>.<version>` as well as the
  short form. Leave it unset until references travel across repos.

## 9. Commit, and the integrity manifest

Remove the old merge driver line if `.gitattributes` has it. It only works
with a local `merge.ours.driver` setting and never applies to merges on
GitHub:

```bash
if [ -f .gitattributes ]; then
  sed -i.bak '/integrity.json merge=ours/d' .gitattributes && rm .gitattributes.bak
  [ -s .gitattributes ] || rm .gitattributes
fi
```

Then commit. In a project with `.rebar/integrity.json`, use `rebar commit`:
it rehashes the protected files you changed and drops the entries of the
staged deletions (the removed copies).

```bash
git add -A
rebar commit -m "chore(rebar): upgrade to v3.1.0-rc.1"
rebar verify --strict      # RESULT: Integrity verified ✓
```

Without a manifest, `git commit` as usual. A manifest written by a v2 or
early v3 CLI also loses its ratchet `current` lines on this first commit;
that edit is the same on every branch and merges cleanly.

## 10. CI

Under 3.1 a project's checks run the install's `scripts/`, so CI needs a
whole rebar tree at the pinned version, not just a `rebar` binary. A CI step
that only builds the CLI from a pinned commit into a cache directory stops
working once the copies are gone: the binary finds no `scripts/` next to it,
`rebar check` fails with `rebar: ci-check.sh not found in the project or a
rebar install`, and `rebar check --pre-commit` skips every check and exits 0.
[SETUP.md § Run rebar in CI](SETUP.md#run-rebar-in-ci) has a GitHub Actions
step and a pinned wrapper that keeps the full tree.

## 11. Verify

```bash
rebar check --pre-commit   # what the git hook runs
rebar check                # the full suite, what scripts/ci-check.sh runs
rebar status               # Version, Tier, Enabled checks, Integrity
```

`rebar status` should show the new version and no integrity violations. A
project whose CI ran only a few checks (virtru-analyze ran five) may see
findings from checks it never ran, typically Doc References (links to
untracked files) and Rebar Compliance (contract maturity). Those are real
findings, not upgrade breakage; fix them, or run the checks you need
individually with `rebar run <check>`.

Then switch for good if you used `--no-switch` (step 1).

## Notes for v2.0.0

- `.rebar-version` and the badge say `v2.0.0`; both move in step 4.
- About 15 copied scripts. Unmodified v2 copies whose output changed since
  are deleted with the difference listed (for example `check-freshness.sh`,
  where the install reports a stale `last-synced:` marker the copy missed).
- **The v2 `compute-registry.sh` is kept** even when unmodified: its
  registry format differs from 3.1's, so no settings reproduce it (`no
  combination of settings reproduces the legacy registry`), and under macOS
  `/bin/bash` 3.2 it does not run at all (`local -n` needs bash 4.3;
  migrate says `the legacy copy's --stdout did not produce a registry`).
  If you never edited it, remove it and regenerate:

  ```bash
  git rm -q scripts/compute-registry.sh
  git add -A && rebar run compute-registry && rebar run compute-registry --check
  ```

  To check it is unmodified, compare it with
  `https://raw.githubusercontent.com/willackerly/rebar/v2.0.0/templates/project-bootstrap/scripts/compute-registry.sh`.
- There is no `.claude/` directory: run `rebar init` (step 3).
- The integrity manifest has ratchet `current` lines; the first 3.1
  `rebar commit` drops them (step 9).

## Notes for v3.0.x

- About 27 copied scripts, nearly all unmodified; a fresh v3.0.0 project
  migrates completely, with `cold-start-checks.sh` wrapped because
  `.claude/settings.json` calls it (step 3).
- Copied scripts carry their own tier gates, so migrate proposes no
  `checks_enable`. Ground truth stays tier 3 only unless you add it (step 7).
- The copied `steward.sh` adds a `S1-STEWARD` shadow reference to a registry
  with impl counts; migrate regenerates the registry when it removes it.

## Rolling back

Point `current` back at the old version, or call it by path, and revert the
upgrade commit:

```bash
ln -sfn versions/v3.0.0 ~/.rebar/current     # if v3.0.0 is a versioned install
git revert <upgrade commit>
```

If the old version is a pre-versioned `~/.rebar` clone, delete the
`~/.rebar/current/bin` lines the installer added to your shell RC; the
clone's own `~/.rebar/bin` line takes over again.
