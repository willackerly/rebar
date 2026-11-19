# Upgrading rebar

How to move an adopting project to rebar 3.1 from v2.0.0 or v3.0.x. Follow
the steps in order; each one says what to run and how to tell it worked.
Moving to 4.0 (release candidate `v4.0.0-rc.1`)? Read
[Upgrading from 3.x to 4.0](#upgrading-from-3x-to-40): it lists 4.0's
breaking changes and the fix for each.

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

**What changes in 4.0.** The `rebar` binary is self-contained: the checks,
the suites, ASK, the MCP server and the bootstrap templates are built in. A
release is one binary per platform (`rebar_<ver>_<os>_<arch>`, verified by
the installer), not a tarball of the rebar tree, and the checks never read a
rebar tree. The bash implementations are gone: rebar no longer ships
`scripts/*.sh`, `scripts/_rebar-config.sh`, the bash `ask` or the python MCP
and ASK servers (`bin/ask`, `bin/ask-agent-loop`, `bin/ask-mcp-server` and
`bin/ask-server` remain as shims that run `rebar`, so `.mcp.json` files and
PATH entries keep working), and `REBAR_CHECK_IMPL` is ignored. Your
project's own copies of rebar's old scripts still run, with the deprecation
notice: they are your code, and rebar never runs them except as `rebar run`
or the suites find them. `rebar migrate` compares your copies with the
built-in checks, and also turns a `ci-check.sh` / `pre-commit.sh` copy that
only adds calls to your own scripts into `checks_extra` /
`checks_extra_pre_commit` lines (only for plain calls whose failure fails
the suite: a copy that runs a script with `|| true`, only in CI, or only to
warn is kept for you to review). The steps below apply unchanged; step 10
(CI) gets simpler.

**Installer changes in 3.1.2 and 4.0.** `bin/install` (the PATH setup beside the
binary) is now `rebar-install`: on PATH, a file named `install` shadowed
install(1) and broke `install -d` / `make install`. `rebar install-shims`
removes rebar's old `install` from the directory it writes; a pre-versioned
`~/.rebar` clone still has one in `~/.rebar/bin`, so drop that directory's
old PATH line from your shell RC. "latest" now means the newest published
GitHub Release, and a release tag is no longer built from source when its
download is missing or fails: the installer stops, and `--source` builds it
on purpose. Use the installer from the tag you install (step 1). The
installer of a 3.1.x tag still works for 4.0.x releases, which also ship a
small `rebar_<ver>_<os>_<arch>.tar.gz` for it (the binary and its
companions; dropped in 4.1). It warns that the version "lacks
templates/project-bootstrap/ or scripts/steward.sh" (harmless: they are
built in) and, finding no `bin/install`, does not touch your shell RC,
which already has `~/.rebar/current/bin` if you installed 3.1 with it.

In the commands below, `v3.1.2` is the release you are moving to (the
latest stable release until v4.0.0 is final). Use the tag you are actually
installing, such as `v4.0.0-rc.1`.

| You are on | Steps |
|------------|-------|
| v2.0.0 (badge says v2.0.0, ~15 copied scripts, no `.claude/`) | all; see [Notes for v2.0.0](#notes-for-v200) |
| v3.0.x (badge says v3.0.0 or a v3.0.0 beta, ~27 copied scripts) | all; see [Notes for v3.0.x](#notes-for-v30x) |
| already thin (`scripts/` holds only `ci-check.sh` and `pre-commit.sh`) | 1, 4–11 |

## 1. Install the new version next to the old one

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.2/setup-rebar.sh \
  | bash -s -- v3.1.2 --no-switch
```

Fetch the installer from the tag you install: until 3.1 is on `main`,
`main`'s `setup-rebar.sh` is the old installer and has no version argument
(see [Testing a release candidate](SETUP.md#testing-a-release-candidate)).

The new version lands in `~/.rebar/versions/v3.1.2/` (a complete rebar
tree for 3.1; from 4.0, the binary with its `ask` shims and completions). `--no-switch` leaves `~/.rebar/current` (the version on your PATH)
alone, so other projects are unaffected while you try this one. The
exception is a machine with **no versioned install yet**, including a
`~/.rebar` that is a plain git clone from the old installer: the first version
always becomes `current`, and `~/.rebar/current/bin` is added to your shell RC
after the old `~/.rebar/bin` line, so new shells run 3.1. The clone itself is
not touched; `.mcp.json` files and `REBAR_ROOT` values that name it keep
working.

To run a version that is not `current`, call it by path; each version runs
its own checks:

```bash
~/.rebar/versions/v3.1.2/bin/rebar version
```

Switch for good with `ln -sfn versions/v3.1.2 ~/.rebar/current` (or
re-run the installer without `--no-switch`). The rest of this runbook writes
plain `rebar` for the 3.1 CLI.

If `REBAR_ROOT` is set in your shell, a 3.1 CLI runs that tree's scripts
rather than the installed version's; unset it unless you mean it. From 4.0 the
checks ignore it.

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
(virtru-analyze: about 3 minutes with 3.1; from 4.0 the reference side runs
in process, so the time is mostly your copies' own). For every copy of a
rebar script in `scripts/` it runs the copy (with bash) and rebar's check (the
install's script in 3.1, the built-in check from 4.0) and prints:

- **`.rebarrc` additions**, between `# --- rebar migrate … ---` markers and
  grouped by the script they came from: `metric.<key>.*` lines translated
  from your `check-ground-truth.sh`, registry settings from your
  `compute-registry.sh`, `source_dirs` / `source_extensions` from your
  scanners. Lines already in `.rebarrc` are never edited or duplicated.
- **delete / replace / wrap.** A copy goes when the install, with the derived
  settings, behaves the same: every metric has the same value (and
  ground truth fails where the copy did without a METRICS file or with a key
  missing from it, adding `metrics_strict = true` if needed),
  `compute-registry.sh --stdout` is byte-identical, other checks give the
  same exit status and findings (the source scanners also on a probe tree).
  An unmodified copy of an older release goes too, with the difference
  listed (for a scanner, the findings the install adds on your project, and
  the `source_exclude_dirs` remedy). Scripts that cannot be run safely
  (`steward.sh`, `inbox-watch.sh`, …) go when they are byte-identical to a
  released version; the CLI carries the list, so this works on any install.
  `ci-check.sh`, `pre-commit.sh` and `scripts/README.md` are replaced by the
  template's files. From 4.0, a `ci-check.sh` or `pre-commit.sh` copy that is
  a released version plus calls to your own scripts is replaced too, those
  scripts moving to `checks_extra` / `checks_extra_pre_commit` (rebar's
  suite runs them after its own checks), when rebar's suite with them exits
  as the copy does on the project; otherwise it is kept with the reason. A verified copy that another project file calls is not
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

`rebar run` with no arguments lists the built-in checks and the project's own scripts.

A file that *sources* a copy for its functions keeps the copy: the wrapper
cannot provide them. The one exception is `check-ground-truth.sh` sourced for
`compute_metrics()`, which its wrapper defines (it calls `rebar run
check-ground-truth --print`). If the sourcing file also calls other functions
of the copy (`count_*` helpers), migrate keeps the copy and names them; have
the caller read the values from `rebar run check-ground-truth --print`
(`key=value` lines) instead, then run `rebar migrate` again.

- **Git hook.** `.git/hooks/pre-commit → ../../scripts/pre-commit.sh` keeps
  working; the target is now the thin wrapper. If you never linked it:
  `ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit`.
- **Claude Code SessionStart hook** (v3.0.x projects). The v3.0.x
  `.claude/settings.json` runs `scripts/cold-start-checks.sh` (migrate wraps
  it for that reason). The 3.1 hook runs `rebar check --cold-start` instead:

  ```bash
  T=~/.rebar/versions/v3.1.2/templates/project-bootstrap
  # 4.0 installs have no template tree: T="$(mktemp -d)/template" && rebar new "$T" >/dev/null
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
echo v3.1.2 > .rebar-version
sed -i.bak 's/^> \*\*rebar v[^*]*\*\*/> **rebar v3.1.2**/' README.md && rm README.md.bak
sed -n 3p README.md   # > **rebar v3.1.2** | **Tier 1: PARTIAL**
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

Since 3.1 the committed registry leaves out implementing-file counts and the
Contract Health section by default (they change in every PR that adds a
`CONTRACT:` header); `rebar migrate` follows that default and regenerates. To
keep them in the file, set `registry_impl_counts = true` in `.rebarrc` before
migrating. Read them any time with `rebar run compute-registry --health`; it
counts implementing files that git tracks, so `git add` new files first.

## 6. Busy repos: keep churn out of committed files

If several PRs are open at once (see [practices/busy-repos.md](practices/busy-repos.md)),
make sure the registry stays free of implementing-file counts: that is the
default since 3.1, so only check you haven't set `registry_impl_counts = true`.

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

All default to the old behavior; set them only if you need them. Every key
is described in [docs/CONFIGURATION.md](docs/CONFIGURATION.md), with
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
rebar commit -m "chore(rebar): upgrade to v3.1.2"
rebar verify --strict      # RESULT: Integrity verified ✓
```

Without a manifest, `git commit` as usual. A manifest written by a v2 or
early v3 CLI also loses its ratchet `current` lines on this first commit;
that edit is the same on every branch and merges cleanly.

## 10. CI

From 4.0, CI needs only the `rebar` binary of the version you pin: install it
with `setup-rebar.sh`, or download the release binary in a pinned wrapper
([SETUP.md § Run rebar in CI](SETUP.md#run-rebar-in-ci) has both). No rebar
tree, no `REBAR_ROOT`.

Under 3.1 a project's checks ran the install's `scripts/`, so CI needed a
whole rebar tree at the pinned version: a step that only built the CLI from a
pinned commit into a cache directory stopped working once the copies were
gone (`rebar check` failed with `rebar: ci-check.sh not found in the project
or a rebar install`, and `rebar check --pre-commit` skipped every check and
exited 0). Such a wrapper keeps working with 4.0; it no longer needs the
tree. A 3.1 wrapper that builds the whole tree and refuses one without
`scripts/steward.sh` (3.x's test for a complete tree) must drop that test:
a 4.0 tree and a 4.0 install have no `scripts/steward.sh`.

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

## Upgrading from v3.1.0 or v3.1.2

Install the new version with the installer from its own tag, as in step 1;
that is all a project already on 3.1 needs. Changes to know about:

- **The PATH helper is `bin/rebar-install`** (it was `bin/install`, which,
  first on PATH, shadowed the system `install(1)`: `install -d dir` failed
  with "Unknown option: -d"). Once `current` points at the new version,
  `~/.rebar/current/bin` has no `install`; the old name stays only inside
  `versions/v3.1.0/` or `versions/v3.1.2/`, which are not on PATH. Switching
  `current` back to one of those brings it back (re-run the new installer for
  that version instead: it renames the helper). An older `setup-rebar.sh`
  looks for `bin/install`, so it does not wire PATH or `--server` for a
  version that has only `bin/rebar-install`; use the installer from the tag
  you install. If your RC still puts a pre-versioned clone's `~/.rebar/bin`
  on PATH, its `bin/install` shadows `install(1)` too; remove that line once
  nothing uses the clone.
- **The installer no longer builds release tags from source on its own.**
  "latest" is the newest published GitHub Release (not the newest tag), and
  a tag without downloadable prebuilt assets is an error; pass `--source` if
  you mean to build it with your own Go (CI that relied on the fallback needs
  `--source` and `actions/setup-go`). Tags older than v3.1.0-rc.1 still build
  from source without it.

## Upgrading from 3.x to 4.0

4.0 is at its first release candidate, `v4.0.0-rc.1`. Until v4.0.0 is
final, the stable examples in this file, README.md and SETUP.md stay on
v3.1.2. Try the candidate beside your current version:

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v4.0.0-rc.1/setup-rebar.sh \
  | bash -s -- v4.0.0-rc.1 --no-switch
~/.rebar/versions/v4.0.0-rc.1/bin/rebar version     # rebar v4.0.0-rc.1
```

- **From v2.0.0 or v3.0.x:** follow steps 1–11 above, with `v4.0.0-rc.1` in
  place of `v3.1.2`, then read the list below.
- **From 3.1.x:** install the candidate, read the list below, run
  `rebar migrate` (step 2: it also points `.mcp.json` at `rebar mcp
  serve`), bump `.rebar-version` and the README badge together (step 4),
  update the version your CI or pinned wrapper installs (step 10), and run
  `rebar check` on the project with the new binary (step 11).

Each item below is one of 4.0's breaking changes (CHANGELOG, `v4.0.0-rc.1`
"Breaking changes"), with who it affects and how to fix it. A project that
uses only `rebar check`, the thin `scripts/ci-check.sh` / `pre-commit.sh`,
and an `.mcp.json` that runs `ask-mcp-server` or `rebar mcp serve --stdio`
needs no changes. To find the callers in a project:

```bash
grep -rnE '\.rebar[^ ]*/scripts/|REBAR_ROOT[^ ]*/scripts/|_rebar-config\.sh|REBAR_CHECK_IMPL|test-mcp-client|bin/install\b|--ask-bin|/mcp\b' \
  --exclude-dir=.git --exclude-dir=node_modules .
ls -l "$(git rev-parse --git-path hooks)"; git config core.hooksPath
```

1. **rebar's scripts are gone from the install.** A 4.0 install has no
   `scripts/` directory, so anything that ran an install's script by path
   fails, for example `~/.rebar/current/scripts/check-todos.sh` or
   `$REBAR_ROOT/scripts/ci-check.sh` in CI, a Makefile or a hook. Run the
   built-in version instead. It takes the same arguments and gives the same
   output and exit codes:

   | Was | Now |
   |-----|-----|
   | `<install>/scripts/check-<name>.sh [args]` (any check or utility) | `rebar run check-<name> [args]` |
   | `<install>/scripts/ci-check.sh` | `rebar check` |
   | `<install>/scripts/pre-commit.sh` | `rebar check --pre-commit` |
   | `<install>/scripts/cold-start-checks.sh` | `rebar check --cold-start` |
   | `<install>/scripts/compute-registry.sh [--check]` | `rebar run compute-registry [--check]` |

   `rebar run` with no arguments lists them. Your project's own copies in
   `scripts/` are your code and still run, with a deprecation notice; `rebar
   migrate` (step 2) removes the ones the built-in checks reproduce.
2. **Custom scripts that source `_rebar-config.sh`.** A script of yours that
   sources the install's `scripts/_rebar-config.sh` (for `_rebar_config` or
   `_rebar_source_exclude_dirs`) fails, because the file is gone. Keep a copy
   in the project, which makes it your code:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.2/scripts/_rebar-config.sh \
     -o scripts/_rebar-config.sh
   ```

   Or read `.rebarrc` directly (`key = value` lines). To have `rebar check`
   run the script, list it in `checks_extra` (or `checks_extra_pre_commit`)
   in `.rebarrc` ([docs/CONFIGURATION.md](docs/CONFIGURATION.md)).
3. **The bash and python ASK tools are gone.** `bin/ask`, `bin/ask-agent-loop`,
   `bin/ask-mcp-server` and `bin/ask-server` are now sh shims that run
   `rebar ask`, `rebar mcp serve` and `rebar ask serve`, so `.mcp.json` files
   and PATH entries that name them keep working (`rebar migrate` offers to
   point `.mcp.json` at `rebar mcp serve` directly). `bin/test-mcp-client`
   is gone: use `rebar mcp client http://127.0.0.1:7232`. A shim that finds
   only a rebar older than 4.0 says "install rebar 4.0 or later". Run
   `rebar install-shims` to write the shims next to a binary you installed
   by hand.
4. **`REBAR_CHECK_IMPL` is ignored.** There is no `shell` fallback. Remove
   the variable from CI and shell RCs. If a check behaves differently from
   3.1 on your project, report it.
5. **Suite summaries count differently.** Outside the rebar repository,
   `rebar check` lists Bootstrap Sync and Script Tests as `SKIP … (rebar
   source repo only)` and counts them as skipped (3.1 ran them, they did
   nothing, and they counted as passed). `16 passed, 0 failed, 0 skipped`
   becomes, for example, `14 passed, 0 failed, 2 skipped`. Hints say `rebar
   run <name>` instead of naming scripts, and a new registry's header says
   "AUTO-GENERATED by rebar run compute-registry" (`--check` accepts the old
   header). If a script or dashboard parses the summary, match the exit
   code or the `failed` count instead.
6. **Git hooks linked to deleted copies.** A hook that is a symlink to a
   copy you deleted, such as `.git/hooks/commit-msg →
   ../../scripts/check-fix-commit.sh`, is skipped by git without a word, so
   the gate silently stops. `rebar migrate` counts hooks (the git
   directory's `hooks/` and `core.hooksPath`) as callers and leaves a thin
   wrapper, and `rebar check` warns about a dangling hook, a hook that calls
   a deleted check, and a hook that runs a suite copy that would skip
   checks. If you already deleted the copies, rewrite the hook to call the
   binary:

   ```bash
   H="$(git rev-parse --git-path hooks)"
   printf '#!/bin/sh\nexec rebar check --pre-commit\n' > "$H/pre-commit" && chmod +x "$H/pre-commit"
   printf '#!/bin/sh\nexec rebar run check-fix-commit "$1"\n' > "$H/commit-msg" && chmod +x "$H/commit-msg"
   ```

   Check the `ln -sf` form of step 3 as well: `.git/hooks/pre-commit →
   ../../scripts/pre-commit.sh` still works, because that file is the thin
   entry point.
7. **MCP over HTTP is Streamable HTTP.** `rebar mcp serve --port` (and the
   `ask-mcp-server` shim) serves `/mcp` with MCP's Streamable HTTP transport.
   A client sends `Accept: application/json, text/event-stream`, takes the
   `Mcp-Session-Id` header from the `initialize` response, and sends it on
   every later request; `DELETE /mcp` ends the session. A script that POSTed
   single JSON-RPC requests to `/mcp` and read one JSON reply breaks; use an
   MCP client library, `rebar mcp client`, or the classic `/v1/*` API, which
   is unchanged. Claude Code `"type": "http"` servers work. stdio is
   unaffected for Claude Code.
8. **MCP error shapes.** A failing ask, a timeout, an empty `question` and
   arguments that break the schema now return a result with `isError: true`
   and the reason as text (they were JSON-RPC errors -32603 / -32602). An
   unknown tool is error -32602, an unknown resource -32602 "Resource not
   found", and a request before `initialize` is -32600 (was -32002). The
   protocol version is negotiated rather than always 2024-11-05, and
   `serverInfo.version` is rebar's version. A client that matched error
   codes or messages should check `result.isError`.
9. **HTTP server defaults** (`rebar mcp serve --port`, `rebar ask serve`
   and the `bin/ask-mcp-server` / `bin/ask-server` shims). The servers bind
   127.0.0.1. Another `--host` needs `--api-key` (or `ASK_API_KEY`); without
   one they exit 2, unless `--insecure-no-auth` is given. On loopback,
   requests need a loopback `Host` header (403 otherwise), and browser
   cross-origin requests are refused. POST bodies must be
   `application/json` and at most 1 MiB, `/v1/ask` needs a `Content-Length`,
   and at most 8 asks run at once (`--max-asks N`; 0 means no limit).
   These are v3.1.2's rules, so a 3.1.2 setup carries over. From 3.1.0 or
   3.1.1, a server meant for other machines needs `--host 0.0.0.0 --api-key
   "$KEY"`, and its clients need to send the key.
10. **A custom `--ask-bin`.** The servers run `<ask-bin> [-v] --question --
    <role> <question>`, so the question is never taken for a command. A
    wrapper passed as `--ask-bin` must accept that form; `bin/ask` from
    v3.1.2 and `rebar ask` do.
11. **Releases are binaries.** A release publishes `rebar_<ver>_<os>_<arch>`
    plus `rebar_<ver>_checksums.txt`, and the installer lays out
    `versions/<ver>/bin/` with the binary, its shims, `rebar-install` and the
    completions. There is no rebar tree in the install: no `scripts/`,
    `templates/`, `agents/` or `practices/`. For v4.0.x only, a small
    `rebar_<ver>_<os>_<arch>.tar.gz` with that `bin/` lets 3.1.x installers
    install 4.0; it is dropped in 4.1. If you downloaded the tarball for its
    files, download the binary (SETUP.md's pinned wrapper does), or use the
    git tag for rebar's documents. The bootstrap template is built in: to
    compare your files with it (step 3), write a scratch project instead of
    reading `~/.rebar/versions/<ver>/templates/project-bootstrap`:

    ```bash
    T="$(mktemp -d)/template" && rebar new "$T" >/dev/null
    diff .claude/settings.json "$T/.claude/settings.json"
    ```

    `REBAR_ROOT` is no longer needed by the checks; `rebar doc` and `rebar
    migrate` still use a rebar checkout when they find one.
12. **`bin/install` is `bin/rebar-install`**, as in v3.1.2 (on PATH,
    `install` shadowed install(1)). Replace calls to
    `~/.rebar/current/bin/install`, and see
    [Upgrading from v3.1.0 or v3.1.2](#upgrading-from-v310-or-v312) for the
    old PATH line.
13. **rebar contributors: `bin/rebar` is not tracked any more.** A rebar
    checkout has no prebuilt binary. Build it with `(cd cli && go build -o
    ../bin/rebar .)`; it is git-ignored. `scripts/tests/run.sh` and CI build
    their own, and the `ask` shims and agent commands fall back to `rebar` on
    PATH. Adopting projects are unaffected.

Rolling back works as for 3.1 (see [Rolling back](#rolling-back)): point
`current` back at the 3.1 version. The 4.0 changes need no edit to
`.rebarrc` or `METRICS`.

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
