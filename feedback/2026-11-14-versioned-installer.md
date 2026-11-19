# Versioned installer: side-by-side versions, prebuilt downloads, a release workflow

**Source:** willackerly/rebar#8 commit 708ba42 ("versioned shell installer
with binary download"), split out of #8 per
`feedback/2026-11-14-self-contained-install.md` Q6; built on #14
(`feat/thin-adoption-scripts`)
**Type:** feature (implemented in the same branch; open questions below)
**Status:** implemented on `feat/versioned-installer`

## Problem

`setup-rebar.sh` clones rebar into `~/.rebar` at one pinned tag and puts
`~/.rebar/bin` on PATH. That gives an adopter:

- **One version per machine.** Trying a newer rebar means moving the only
  install; going back means re-cloning.
- **A binary that only runs on Apple silicon.** `bin/rebar` is committed,
  built for darwin/arm64. The clone never builds it, so on Linux or an Intel
  Mac `rebar` fails to execute. [V: `file bin/rebar` → `Mach-O 64-bit
  executable arm64`; installer has no build step]
- **No releases to download.** The one GitHub Release (v3.0.0-beta) has no
  assets; v3.0.0-beta.2 through beta.4 have no Release at all; the repo has
  no CI. [M: `gh api repos/willackerly/rebar/releases` → one release, 0
  assets; `.github/` holds only the PR template]

Since #14 the install matters more: the CLI runs the install's own
`scripts/` against each project, so the install is no longer just a binary
but the engine every `rebar check` runs.

## What came from #8 (708ba42)

The intent survives: a version argument (`| bash -s -- v3.0.0-beta.4`),
`~/.rebar/versions/<version>/` plus a `current` symlink, prebuilt downloads
for tags with a source fallback, branches from source. The patch itself
could not be applied:

| # | Problem in 708ba42 | Evidence | Here |
|---|--------------------|----------|------|
| 1 | Downloads assets that don't exist, and nothing would ever publish them | [M] no assets, no CI (above) | release workflow + `scripts/package-release.sh` |
| 2 | "latest" from `/releases/latest`, which skips pre-releases — every rebar release was one when #8 was written | [M] at the time: `curl -s -o /dev/null -w '%{http_code}' https://api.github.com/repos/willackerly/rebar/releases/latest` → `404` | highest stable semver tag from `git ls-remote --tags` (tags API without git); `--pre` includes pre-releases |
| 3 | ldflags target `cli/cmd.Version`, which didn't exist: the version was a literal in the root command, so `-X` was silently ignored | [M] a beta.4 source build with `-X …Version=…` still prints `rebar v3.0.0-beta.4` | `cli/cmd.Version` variable, defaulting to `releaseVersion` |
| 4 | `"${install_args[@]}"` with an empty array — the bash 3.2 `set -u` crash fixed in #12 | [V] same line #12 fixed | `${arr[@]+…}` guard; tests run under /bin/bash 3.2 |
| 5 | `trap 'rm -rf "$tmp"' EXIT` inside a function with `local tmp`: at exit `$tmp` is out of scope, so under `set -u` a *successful* prebuilt install exits 1 and leaks its temp dir | [M] minimal repro under bash 3.2 and 5.3: `tmp: unbound variable`, `exit=1` | one script-level temp dir and trap |
| 6 | Runs `versions/<v>/bin/install`, so the RC gets a version-specific PATH; flipping `current` never changes which rebar runs | [V] `bin/install` records `$(cd "$(dirname "$0")" && pwd)` | `bin/install` runs through `current/`, whose logical path it records |
| 7 | Any download failure falls back to source; no checksum at all | [V] | SHA-256 verification; a mismatch is fatal |
| 8 | `mkdir -p versions/<v>` up front, clone into `versions/<v>/.src`, `cp -r .src/.` over it: a failed install leaves a half-populated version, a `.git`, and a stray `.src` | [V] | stage in `versions/.incoming.$$`, verify, then `mv` into place |
| 9 | `git clone --branch <sha>` cannot fetch a commit | [V] | `git fetch --depth 1 <repo> <ref>` handles tags, branches and full SHAs (short SHAs are refused with a clear error: a shallow fetch cannot resolve them) |
| 10 | Ignores the committed arm64 `bin/rebar` (a branch install keeps it if the build is skipped) and an existing `~/.rebar` clone (writes `versions/` into it, dirtying it) | [V] | every install builds or downloads `bin/rebar`; clone handled below |

## Design

### Layout

```
~/.rebar/
  versions/
    v3.0.0-beta.4/            complete rebar tree: scripts/, templates/,
    feat-thin-adoption-scripts/   agents/, bin/rebar, bin/ask*, practices/, …
  current -> versions/v3.0.0-beta.4
```

Each version directory satisfies #14's install marker
(`templates/project-bootstrap/` + `scripts/steward.sh`). The CLI finds it
through its own `../` — `~/.rebar/current/bin/rebar` resolves to
`versions/<ver>/bin/rebar` — so every version runs **its own** scripts:
switching `current` switches the engine and the binary together, and there is
no clone/binary version skew to explain. `findRebarRoot` also tries
`~/.rebar/current` before `~/.rebar` for binaries that live elsewhere (a
`go install`ed rebar).

Each version also gets a `.rebar-install` record (version, kind, commit,
method, repo, time), for "which rebar is this, really?".

### What gets installed

| Request | Path | Needs |
|---------|------|-------|
| tag, release has assets | download `rebar_<ver>_<os>_<arch>.tar.gz`, check it against `rebar_<ver>_checksums.txt`, unpack | curl/wget, tar, shasum/sha256sum |
| tag, no assets (every release today) | source build | git, Go |
| branch or full commit SHA | source build | git, Go |
| `--source` | source build | git, Go |

A source build is `git archive` of the ref plus `go build` with the same
flags the workflow uses (`CGO_ENABLED=0 -trimpath -s -w -X
…cli/cmd.Version=<ver>`). It produces the same tree a release tarball holds,
so the two paths differ only in who ran the compiler. Branch builds stamp
`<branch>@<sha7>`; refs older than the `Version` variable ignore `-X` and
report their hard-coded string (fine: for tags that string is correct).

Failure policy: no assets / unsupported platform / no hash tool → source
build (logged). **Checksum mismatch, or assets listed but undownloadable →
abort**, never a quiet source build: a mismatch means corruption or
tampering, and falling back would hide it.

### What the release tarball contains, and why

**The full tree at the tag (`git archive`) with `bin/rebar` built for the
platform**, under a top-level `rebar_<ver>_<os>_<arch>/`. About 2.5 MB
compressed per platform.

The alternative — a binary-only tarball plus a `git clone` of the same tag
for scripts/templates — was rejected:

1. #14 made the install's `scripts/`, `templates/project-bootstrap/`,
   `agents/` and `bin/ask*` load-bearing for every `rebar check`. A
   binary-only artifact is not an install.
2. The point of a prebuilt is "no git, no Go". Binary + clone still needs
   git, and the clone is outside the checksum: one verified artifact beats a
   verified binary next to an unverified tree.
3. One shape. Prebuilt and source installs are both "git archive + binary",
   so root detection, `rebar run`, and support questions have one answer.

### "latest"

The highest **stable** semver tag from `git ls-remote --tags --refs <repo>
'v*'` (without git: the GitHub tags API, first 100); while no stable tag
exists, the highest pre-release. `--pre` takes the highest tag of any kind.
Pre-releases rank below their release (`v3.0.0-beta.10` > `v3.0.0-beta.2`,
`v3.0.0` > `v3.0.0-rc.1`); non-semver tags are ignored. The first version of
this installer included pre-releases by default, which was right while every
rebar release was one; with v3.0.0 out, a `v3.1.0-rc.1` would have become
every default install (#17 review). Neither releases endpoint is used:
`/releases/latest` excludes tags without a Release, and `/releases` misses
tags that never got one (v3.0.0-beta.2 to beta.4).

### PATH and switching

The installer runs the active version's `bin/install` **through the
`current` symlink**. `bin/install` records its logical directory, so the RC
line is `export PATH="$HOME/.rebar/current/bin:$PATH"` and never needs
editing again. `--no-switch` installs without moving `current` (and without
re-running `bin/install`); the first install always sets `current`. Manual
switch: `ln -sfn versions/<v> ~/.rebar/current`.

### Existing installs (`~/.rebar` is a git clone)

**Coexist; don't migrate.** The clone is not moved, updated, or modified:

- `versions/` and `current` are created inside it and added to the clone's
  `.git/info/exclude` (local, untracked), so `git status` stays clean.
- `bin/install` appends the `current/bin` line after the old `~/.rebar/bin`
  line, so the new install wins in new shells.
- Everything that names the clone keeps working: `.mcp.json` files written by
  `rebar init` (absolute paths to `~/.rebar/bin/ask-mcp-server`), a
  `REBAR_ROOT=~/.rebar`, scripts calling `~/.rebar/bin/ask`.

Moving the clone into `versions/` was rejected for exactly that last point:
it would break every project's `.mcp.json` silently, for tidiness
(working agreement 06, relaxation not migration). The cost is that the old
files stay until the user deletes them. The installer says so, and warns
when `REBAR_ROOT` is set (it overrides the installed version's scripts).

A non-empty `~/.rebar` that is neither a clone nor a versioned install, or a
`current` that is not a symlink, is refused untouched.

### Version stamping

`cli/cmd/root.go`:

- `const releaseVersion = "v3.0.0-beta.4"` — bumped per release; the
  workflow refuses a tag that differs.
- `var Version = releaseVersion` — what `rebar version` prints; `-X` sets it.
- `badgeVersion()` — what `rebar new` / `rebar adopt` write into a README
  badge: `Version` if it is a release tag, else `releaseVersion`. Without it
  a branch build would write `rebar feat/x@abc1234` into an adopter's badge,
  which `check-compliance.sh` rejects as malformed.

### Release workflow

`.github/workflows/release.yml`, on `v*` tag push, one ubuntu job:
checkout → setup-go (`cli/go.mod`) → tag equals `releaseVersion` → `go test
./...` → `scripts/package-release.sh "$TAG" --out dist` (4 targets +
checksums) → `gh release create "$TAG" dist/* --generate-notes --verify-tag
[--prerelease]` (pre-release when the tag contains `-`). `actions/checkout`
v7.0.1 and `actions/setup-go` v7.0.0 are pinned by commit SHA; publishing
uses the preinstalled `gh`, so no third-party release action holds the
token. Workflow token: `contents: write` on the job only.

### Pinned versions (`.rebar-version`) — proposed, not implemented

The idea: `rebar` reads the project's pin and re-execs
`~/.rebar/versions/<pin>/bin/rebar` when installed, else prints an install
hint, with an env opt-out. Small in code, but not clean given what
`.rebar-version` means today:

- It is the project's **declared methodology version**, checked by
  `check-compliance.sh` against the README badge — not a CLI pin.
- `rebar new` / `init` / `audit --fix` write `v3.0.0-beta` (not the running
  version); older adopters carry `v2.0.0`. Auto-switching would send most
  projects to a version nobody installed, printing a hint on every command,
  or re-exec into v2 behavior.

So the switch needs its own, opt-in key. Sketch: `.rebarrc: rebar_version =
v3.0.0-beta.5` → re-exec `$REBAR_DIR/versions/<v>/bin/rebar` if present
(`REBAR_NO_SWITCH=1` opts out, a `REBAR_SWITCHED=1` guard stops loops),
else run as-is with one stderr line: `this project pins rebar <v>; install
it: setup-rebar.sh <v> --no-switch`. ~40 lines of Go plus tests. See Q3.

## Red team

- *Latest = highest tag, including a stray one.* A mistaken `v9.0.0` tag
  would become everyone's default; a local clone with an unpushed tag
  would too (measured: this machine's rebar clone has a local-only
  `v3.0.1-alpha`, which `latest` against that clone picks). Against GitHub,
  only pushed tags count, and tags need write access. Acceptable; Q4 covers
  "only tags with a Release".
- *Checksums come from the same place as the tarball.* SHA-256 here guards
  against corruption and partial downloads, not a compromised release
  (whoever can replace an asset can replace `checksums.txt`). Signing
  (cosign / GitHub artifact attestations) is the real fix; see Q5.
- *`curl | bash` under `env -i` / C locale.* The bash 5 run in the test
  suite caught `"$REBAR_REPO…"`: in the C locale bash 5.3 read the UTF-8
  ellipsis bytes as part of the variable name and died with `unbound
  variable`. Fixed with braces; the suite runs every scenario under
  `env -i` with both bashes so this class stays covered.
- *`set -e` inside an `if`.* `install_prebuilt` runs as an `if` condition,
  where `set -e` is off; each step there ends in `|| die`.
- *Truncated download.* `curl | bash` cut short used to execute a prefix:
  cut after "Replacing the existing <ver>", it moved the installed version
  aside and the EXIT trap deleted it (#17 review). Everything now runs in
  `main`, called on the last line with an end-of-script marker main
  requires, so a cut anywhere runs nothing (T11 replays 60+ cuts).
- *Interrupted install.* Work happens in `versions/.incoming.$$` and moves
  into place; the EXIT trap removes it. A re-install moves the old copy to
  `.replaced.$$` first; if the new copy never lands (mv fails, HUP/INT/TERM
  between the two moves) the trap moves the old copy back (T12). `current`
  is switched by renaming a new symlink over it. One install per version
  at a time (`versions/.lock-<name>`). Tests assert nothing starting with
  `.` is left in `versions/`.
- *Name collisions.* Branch names are made path-safe, so `feat/x` and
  `feat-x` share `versions/feat-x`; the second install is refused, naming
  the ref already there (T15). An injective encoding (`feat%2Fx`) was
  rejected for readability; collisions need two near-identical branch
  names.
- *Foreign directories.* Only a git checkout with rebar's install markers
  counts as a pre-versioned clone; any other non-empty directory, git or
  not, is refused untouched (T8, T14).
- *Transport.* Release downloads are HTTPS-only (redirects too), TLS 1.2+,
  with a connect timeout; a 404 means "no assets", any other failure is
  named before the source fallback (T19). Credentials in `REBAR_REPO` are
  redacted from output and `.rebar-install` (T18).
- *Linux `.mcp.json` paths.* `rebar init` records the ask-mcp-server path
  from `os.Executable()`. On macOS that is the invoked path
  (`~/.rebar/current/bin/…`), which survives switches [M: `rebar new` from
  a versioned install wrote `"command": "$HOME/.rebar/current/bin/ask-mcp-server"`]. On Linux it is the
  resolved `/proc/self/exe` (`versions/<v>/bin/…`), which breaks when that
  version is removed. Fixed: `stableBinDir` maps `<base>/versions/<v>/bin`
  to `<base>/current/bin` when `current` points there
  (`TestMCPServerPathUsesCurrent` feeds it the resolved Linux form).
- *Disk.* ~7 MB per version unpacked (measured: 6.7M); nothing prunes old versions.
  `rm -rf ~/.rebar/versions/<v>` is documented; a prune flag can come later.
- *Smallest input (agreement 06).* Zero tags → "found no release tags";
  one tag → it; a checksums file without our platform → source build; the
  first install with `--no-switch` still sets `current` (warned), else
  nothing would be on PATH.

## Open questions (lean in brackets)

1. **Stop committing `bin/rebar`?** It is darwin/arm64 only, 6 MB per
   rebuild in history, and stale between releases. Once a tagged release
   publishes assets, installs no longer need it; `scripts/test-e2e-live.sh`
   and docs that call `bin/rebar` from a checkout would build it first
   (`cd cli && go build -o ../bin/rebar .`) and `.gitignore` it.
   [Lean: yes, in the release after the first one with assets — relaxation:
   keep it until then so existing clone users don't lose a working binary.]
   **Resolved (owner, 2026-11-18):** untracked from v4.0.0 (`.gitignore`d;
   contributors build it, CI and the script tests build their own).
2. **Strip more dev-only files from the tarball?** `.gitattributes` now
   export-ignores `/.mcp.json` (a maintainer's absolute paths), `/inbox/`
   and `/.claude/`. `feedback/` and `docs/` stay (`rebar doc
   rebar:feedback/...` resolves into the install); `.claude-memory-commits/`
   and rebar's own `.rebar/` could go. [Lean: leave the rest until someone
   measures what installs actually read.]
3. **Pinned-version switching** as above. [Lean: yes, as a separate PR,
   keyed on a new `.rebarrc: rebar_version`, never on `.rebar-version`;
   no automatic install.]
4. **Should "latest" require a published Release** (skip bare tags)? Not
   until releases with assets are routine (v3.0.0-beta.2 to beta.4 never got
   one).
   [Lean: no; revisit if a stray tag ever ships.]
5. **Sign release assets** (GitHub artifact attestations or cosign
   keyless)? [Lean: add `actions/attest-build-provenance` in a follow-up;
   verification stays optional in the installer until `gh attestation
   verify` can be assumed.]
6. ~~`.mcp.json` should point through `current`~~ — done in this branch
   (#17 review).
7. **A `rebar versions` / `rebar use <v>` command** instead of `ln -sfn`?
   [Lean: not until someone asks; the symlink is the interface and
   `setup-rebar.sh` already lists versions.]
8. **Reproducible tarballs?** Today each tarball is re-tarred from an
   extracted `git archive` with the host tar, so bytes vary with mtime,
   owner and tar flavor (GNU vs bsdtar spell sorting/owner/mtime flags
   differently); the binary itself is `-trimpath`. [Lean: worth doing when
   signing lands (Q5): append the binary to `git archive` output with fixed
   mtime/owner and `gzip -n` rather than re-tarring.]
9. **Split the release job** into build (read-only token) and publish
   (`contents: write`)? [Lean: yes when signing lands; one job is fine
   while the only privileged step is `gh release create`. setup-go's cache
   is already off in this job.]

## What still needs a human

- **Push a real tag** (the next one after v3.0.0, after bumping `releaseVersion`)
  to exercise the workflow on GitHub; until then the prebuilt path is proven
  only against locally built assets served over `file://`.
- **Repository settings:** Actions enabled, and workflow permissions allow
  `contents: write` for the job (Settings → Actions → General). Optionally a
  tag protection rule for `v*`.
- The docs' one-liner points at `main/setup-rebar.sh`; it is right once this
  merges.

## Evidence [M]

- **Hermetic suite** `scripts/tests/setup-rebar.test.sh` (in ci-check's
  Script Tests, ~10 s): a local upstream with tags v9.0.0, v9.0.10,
  v9.1.0-beta.2, annotated v9.1.0-beta.10, non-semver `vnext`, branch
  `feat/x`, and a dependency-free `cli/` that prints its stamped Version;
  `file://` releases built by `package-release.sh`. Under /bin/bash 3.2.57
  and bash 5.3.15, all in `env -i` throwaway HOMEs: latest = beta.10 via
  prebuilt + checksum; no-assets tag → source, stamped; `9.0.10` → v9.0.10
  with `--no-switch`; branch → `feat/x@<sha7>`; `--source` re-install in
  place; tampered tarball refused with nothing left behind; legacy clone
  untouched and clean with the new PATH line after the old one; foreign
  `~/.rebar` refused; unknown ref refused; without Go, prebuilt works and
  source explains Go is needed.
- **Against GitHub** (throwaway HOMEs, both bashes, 12 installs, all exit
  0): `curl | bash -s -- v3.0.0-beta.4` → no assets → source → `rebar
  v3.0.0-beta.4`; `latest` → v3.0.0-beta.4, replaced in place;
  `feat/thin-adoption-scripts` → current switches; a
  `feat/versioned-installer` build reports `feat/versioned-installer@<sha>`;
  re-installing beta.4 switches back; a `--depth 1` beta.4 clone wired by its
  own `bin/install` keeps HEAD and a clean status while `command -v rebar`
  moves from `~/.rebar/bin` to `~/.rebar/current/bin`. In each, `rebar new`
  + `rebar check` in a throwaway project runs the install's suite (16
  checks for #14+ installs, 15 for beta.4) and fails only the two findings
  every fresh project has on #14 (missing registry; badge vs
  `.rebar-version`).
- **Prebuilt path with real artifacts:** `package-release.sh` on a throwaway
  clone tagged `v3.0.0-beta.5-test.1` (4 targets: Mach-O x86_64/arm64,
  static ELF x86-64/aarch64, ~2.5 MB each), served as
  `REBAR_RELEASE_URL=file://…`, installed with `PATH=/usr/bin:/bin` (no
  Go): "Checksum OK", `rebar v3.0.0-beta.5-test.1`, `rebar check` runs from
  the unpacked install. One appended byte: "checksum mismatch … Refusing to
  install", exit 1, empty `versions/`, no RC edits.

## Disposition

Implemented on `feat/versioned-installer`, stacked on #14.

#17 review ("merge after fixes"), addressed in follow-up commits: truncated
downloads run nothing (blocker); a failed swap restores the replaced copy;
"latest" is stable-first with `--pre`; full SHAs only; only rebar clones
count as legacy installs; `.mcp.json` goes through `current`; transport,
redaction, locking, atomic `current`, collision, HOME and `--server` nits.
Declined for now: injective directory names (collisions are refused
instead), reproducible tarballs (Q8), a split release job (Q9). The
evidence below predates v3.0.0, when `latest` meant v3.0.0-beta.4. Open questions
await maintainer triage; close into `feedback/processed/` once answered.
