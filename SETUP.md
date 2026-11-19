# Setup Guide

How to adopt the rebar kit in your project.

**Start by reading [DESIGN.md](DESIGN.md)** to understand the
philosophy. Then pick your [project profile](profiles/) to know which
parts to adopt.

## Prerequisites

- A git repository
- Claude Code (or another AI coding agent that reads markdown context files)
- 30 minutes for initial setup
- The `rebar` binary — see [Install rebar](#install-rebar)

## Install rebar

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v4.0.0/setup-rebar.sh | bash -s -- v4.0.0
```

That installs v4.0.0, the latest stable release, and puts `~/.rebar/current/bin` (`rebar`, `ask`,
`ask-mcp-server`) on your PATH via your shell RC; open a new terminal (or
source the RC) and check with `rebar version`. Fetch `setup-rebar.sh` from the
tag you install, as above: the copy on `main` is whatever `main` holds. With no
version argument the installer takes the latest stable release: the highest
published GitHub Release that is not a pre-release such as `v4.1.0-rc.1`,
looked up through GitHub's API (unauthenticated, 60 requests/hour per IP; past
that it says so; name the version instead). A tag that is pushed but not yet
published as a Release, and draft Releases, never count. Variants (`…` is the
`curl` part):

| Command | Installs |
|---------|----------|
| `… \| bash -s -- v4.0.0` | that release (`v3.1.2` for the 3.1 line) |
| `… \| bash -s -- --pre` | the newest published release, pre-releases included |
| `… \| bash -s -- main` | a branch, built from source; `rebar version` says `main@<sha>` |
| `… \| bash -s -- v4.0.0 --no-switch` | side by side, without changing the active version |
| `… \| bash -s -- v4.0.0 --source` | from source with your Go (needed for a tag without prebuilt assets) |
| `… \| bash -s -- v4.0.0 --dir /opt/rebar` | under `/opt/rebar` instead of `~/.rebar` (CI); your shell RC is left alone |
| `… \| bash -s -- v4.0.0 --no-modify-rc` | in `~/.rebar` without editing your shell RC |

**Your shell RC.** The installer runs the version's `rebar-install`, which
adds `~/.rebar/current/bin` (and the shell completions) to the RC file your
`$SHELL` reads: `~/.zshrc`, `~/.bashrc`, else `~/.profile`. From 4.0 the
installer:

- leaves the RC alone when you pass `--dir` (or set `REBAR_DIR`), and prints
  the `export PATH=…` line to add instead; `--modify-rc` adds it anyway;
- leaves it alone with `--no-modify-rc` for the default `~/.rebar` too;
- never adds a second PATH block: re-running it, or an RC that already has
  the directory as `$HOME/.rebar/current/bin` or `~/.rebar/current/bin`,
  leaves the RC as it is.

The 3.1.x installers wrote the RC on every switched install, also with
`--dir`; remove stale `rebar CLI tools` blocks naming old directories by
hand.

Each version lives in `~/.rebar/versions/<version>/`; `~/.rebar/current` is
a symlink to the active one, so switching never edits your RC again
(`ln -sfn versions/<version> ~/.rebar/current` switches by hand;
`rm -rf ~/.rebar/versions/<version>` removes one). From 4.0 a release is one
self-contained binary per platform, `rebar_<ver>_<os>_<arch>`: the installer
verifies it against the release's SHA-256 checksums (a mismatch aborts), puts
it at `versions/<version>/bin/rebar` and runs `rebar install-shims`, which
writes the `ask*` shims, `rebar-install` (the PATH setup; `bin/install`
before 3.1.2, renamed because a file named `install` on PATH shadows
install(1)) and the shell completions beside it. Releases v3.1.0-rc.1 to
v3.1.x published a tarball of the whole tree instead, which the installer
still unpacks. "latest" is the newest published GitHub Release (not a tag
whose release is missing or still building). A release tag is never built
from source behind your back: if its assets are missing or the download
fails, the installer stops; `--source` builds it with your Go (tags before
v3.1.0-rc.1, which have no assets, build from source by themselves).
Branches and commits (by full 40-character SHA) build from source, which
needs `git` and Go 1.25 or
newer (use a [supported release](https://go.dev/doc/devel/release#policy):
the binary includes its standard library; see [SECURITY.md](SECURITY.md)) and
keeps the whole tree of that ref. Branch
names are made path-safe for the directory (`feat/x` → `versions/feat-x`); if
two refs map to the same name, the second install is refused rather than
replacing the first. `./setup-rebar.sh --help` lists every option.

The checks are built into the binary, so `rebar check` always uses the
checks of the version you run. Downloaded the binary by hand (or built it
with `go build`)? It works on its own; run `rebar install-shims` once if you
want the `ask` command and the completions next to it. Upgrading a project
that already uses rebar: [UPGRADING.md](UPGRADING.md).

### Testing a release candidate

Release candidates are tags like `v4.1.0-rc.1` (4.0's were `v4.0.0-rc.1`
and `-rc.2`), published as GitHub pre-releases; the default "latest" install
skips them. There is no open candidate now: v4.0.0 is the current release.
For a future one, use the installer from the candidate's own tag (or `--pre`
for the newest pre-release), for example:

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v4.1.0-rc.1/setup-rebar.sh \
  | bash -s -- v4.1.0-rc.1 --no-switch
~/.rebar/versions/v4.1.0-rc.1/bin/rebar version     # rebar v4.1.0-rc.1
```

`--no-switch` keeps your current version active (except on a first versioned
install, which always becomes current), so you can try the candidate by path
on one project, then switch with
`ln -sfn versions/v4.1.0-rc.1 ~/.rebar/current`. Then follow
[QUICKSTART.md](QUICKSTART.md) for a new project or
[UPGRADING.md](UPGRADING.md) for an existing one, and report problems as
issues on the repository.

Name the candidate explicitly: without a version the installer takes the latest
*stable* release, so a candidate is only picked by `--pre` (the newest
published release, pre-releases included).

**Upgrading from a `~/.rebar` clone** (what the installer made before versioned installs): the
installer leaves the clone as it is — not moved, not updated — and adds
`versions/` and `current` inside it (git-excluded). The new PATH entry comes
later in your RC, so it wins; `.mcp.json` files and `REBAR_ROOT` that point at
the clone keep working until you re-run `rebar init` or change them.
`REBAR_ROOT` is no longer needed (see "Where a rebar tree still matters"
below).

## Run rebar in CI

From 4.0 the `rebar` binary is self-contained: the checks, the suites, ASK,
the MCP server and the bootstrap templates are built in, so CI needs only the
binary of the version you pin, no rebar tree and no `REBAR_ROOT`.

**GitHub Actions, with the installer:**

```yaml
      - uses: actions/checkout@v4
      - name: Install rebar
        env:
          REBAR_VERSION: v4.0.0   # the release this repository pins
        run: |
          curl -fsSL "https://raw.githubusercontent.com/willackerly/rebar/${REBAR_VERSION}/setup-rebar.sh" \
            | bash -s -- "$REBAR_VERSION" --dir "$RUNNER_TEMP/rebar"
          echo "$RUNNER_TEMP/rebar/current/bin" >> "$GITHUB_PATH"
      - name: rebar checks
        run: rebar check            # or ./scripts/ci-check.sh
```

The release binary needs only `curl` and `shasum` or `sha256sum`; the
installer verifies it against the release's checksums and writes the `ask`
shims and `rebar-install` beside it. A tag without prebuilt assets is an
error unless you pass `--source` (needs `git` and Go: add
`actions/setup-go`). The checks
themselves need `git`. With `--dir` the 4.0 installer does not touch the
runner's shell RC (it prints the PATH line instead), so the `$GITHUB_PATH`
line is what puts rebar on PATH; the 3.1.x installers also appended a line to
the RC, which is harmless on a runner.

**Pinned wrapper.** To run the version a repository pins without an install
step, download the release binary for the platform once, verify it against a
SHA-256 *committed in the repository*, cache it and run it; nothing else is
needed. Pinning the hash (not just checking the release's own checksums file)
means a release asset replaced later cannot reach your CI.

```bash
#!/usr/bin/env bash
# scripts/rebar.sh — run the rebar version this repository pins.
# Usage: scripts/rebar.sh check | run <check> | ...
set -euo pipefail

REBAR_PIN="v4.0.0"   # a release from 4.0 on (self-contained binary)
# The pinned release's rebar_<ver>_checksums.txt (at least this platform's
# binary line); a download that does not match is refused.
REBAR_SHA256="
paste-the-lines-of-rebar_4.0.0_checksums.txt-here  rebar_4.0.0_linux_amd64
"
REBAR_RELEASES="${REBAR_RELEASES:-https://github.com/willackerly/rebar/releases/download}"
ROOT="${REBAR_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/rebar}/$REBAR_PIN"

if [ ! -x "$ROOT/rebar" ]; then
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) echo "rebar: no release binary for $(uname -m)" >&2; exit 1 ;;
  esac
  asset="rebar_${REBAR_PIN#v}_${os}_${arch}"
  want="$(printf '%s\n' "$REBAR_SHA256" | awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }')"
  case "$want" in
    [0-9a-f][0-9a-f][0-9a-f][0-9a-f]*) ;;
    *) echo "rebar $REBAR_PIN: no SHA-256 pinned for $asset in REBAR_SHA256" >&2; exit 1 ;;
  esac
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/rebar-pin.XXXXXX")"
  curl -fsSL -o "$tmp/$asset" "$REBAR_RELEASES/$REBAR_PIN/$asset"
  got="$( (sha256sum "$tmp/$asset" 2>/dev/null || shasum -a 256 "$tmp/$asset") | awk '{ print $1 }')"
  if [ "$want" != "$got" ]; then
    rm -rf "$tmp"
    echo "rebar $REBAR_PIN: checksum mismatch for $asset (pinned $want, got $got)" >&2
    exit 1
  fi
  mkdir "$tmp/bin" && mv "$tmp/$asset" "$tmp/bin/rebar" && chmod 755 "$tmp/bin/rebar"
  mkdir -p "$(dirname "$ROOT")" && rm -rf "$ROOT" && mv "$tmp/bin" "$ROOT"
  rm -rf "$tmp"
fi

export PATH="$ROOT:$PATH"   # nested `rebar` calls get the same version
exec "$ROOT/rebar" "$@"
```

When you change `REBAR_PIN`, paste that release's
`rebar_<ver>_checksums.txt` into `REBAR_SHA256`, after checking the file's
build provenance:

```bash
gh release download v4.0.0 -R willackerly/rebar -p 'rebar_4.0.0_checksums.txt'
gh attestation verify rebar_4.0.0_checksums.txt --repo willackerly/rebar
```

(`gh attestation verify <binary> --repo willackerly/rebar` checks a
downloaded binary the same way: it was built by rebar's release workflow
from the tagged commit.)

The project's `scripts/ci-check.sh` and `scripts/pre-commit.sh`, and the
wrappers `rebar migrate` leaves for copies that other files call, run
whatever `rebar` is on PATH, so a project script that CI runs directly should
call the pinned wrapper (`scripts/rebar.sh run check-todos`), unless the CI
job puts `$ROOT` on PATH. Cache `$REBAR_CACHE_DIR` between runs to skip the
download. (Before 4.0 the binary ran the scripts of the rebar tree beside it,
so a wrapper had to keep the whole tree of the pinned commit. Such a wrapper
works with 4.0 once its pin, commit and `.rebar-version` name a 4.0 release,
except for one common guard: 3.x called a tree complete when it held
`scripts/steward.sh` and `templates/project-bootstrap/`, and a 4.0 tree has
no `scripts/steward.sh` (a 4.0 install, as in `REBAR_BIN=~/.rebar/current/bin/rebar`,
has neither), so drop such a check. Its `REBAR_ROOT` export is harmless; the
tree itself is no longer needed.)

**Where a rebar tree still matters.** Almost nowhere: rebar is a single
binary, and its checks never read a rebar tree. `rebar doc` resolves
`rebar:<kind>/<name>` references from a checkout when it finds one, and
`rebar migrate` reads a checkout's release history to recognize old copies
(the binary carries the list of released scripts, so it does not need one).
They look in `$REBAR_ROOT`, else the directory above the binary, the current
directory, then `~/.rebar/current`, `~/.rebar`, `~/dev/rebar`, `~/src/rebar`,
`~/code/rebar` (`findRebarRoot` in `cli/cmd/init.go`).

## Step 1: Pick Your Profiles

Profiles have two dimensions — pick one from each:

**By project type** ([profiles/](profiles/)):
- [web-app.md](profiles/web-app.md) — SPA, SSR, frontend + API
- [api-service.md](profiles/api-service.md) — Backend API, microservice
- [crypto-library.md](profiles/crypto-library.md) — Security-critical library
- [cli-tool.md](profiles/cli-tool.md) — Command-line tool

**By team size** ([profiles/](profiles/)):
- [solo-dev.md](profiles/solo-dev.md) — 1 dev, Tier 1 (15 min setup)
- [small-team.md](profiles/small-team.md) — 2-10 devs, Tier 2 (45 min setup)
- [department.md](profiles/department.md) — 10+ devs, Tier 3 (2 hour setup)

Your profiles tell you which files to copy, which sections to customize,
and which enforcement tier to configure.

## Step 2: Scaffold the Project

With rebar installed, let the CLI lay down the files:

```bash
cd /path/to/your/project
rebar adopt --tier 1          # scaffolding only; the default --tier 2 also asks an LLM to propose contracts
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
rebar run compute-registry    # writes architecture/CONTRACT-REGISTRY.md
```

For a new repository, `rebar new my-project -d "what it does"` does the same
in a fresh directory and makes the first commit (see
[QUICKSTART.md](QUICKSTART.md)). Either way the project gets `.rebarrc`
([settings reference](docs/CONFIGURATION.md)),
`.rebar-version`, a README badge, the Cold Start Quad, `CLAUDE.md`,
`architecture/` with the contract template, the role agents, `.mcp.json`,
`.claude/` (SessionStart hook and skills), and `scripts/` with two entry
points, `ci-check.sh` and `pre-commit.sh`, which run `rebar check`. Existing
files are left alone; the badge is added to an existing README.

**Copying by hand** instead works from a rebar checkout (`git clone
https://github.com/willackerly/rebar`), which holds the same templates. The
copied `scripts/` entry points call `rebar`, so rebar must still be on your
PATH:

```bash
PROJECT=/path/to/your/project
R=~/src/rebar                    # a rebar checkout
BOOTSTRAP="$R/templates/project-bootstrap"

# The Cold Start Quad (required)
cp "$BOOTSTRAP/README.md"        "$PROJECT/README.md"
cp "$BOOTSTRAP/QUICKCONTEXT.md"  "$PROJECT/QUICKCONTEXT.md"
cp "$BOOTSTRAP/TODO.md"          "$PROJECT/TODO.md"
cp "$BOOTSTRAP/AGENTS.md"        "$PROJECT/AGENTS.md"

# Claude Code config (required for Claude Code users)
cp "$BOOTSTRAP/CLAUDE.md"        "$PROJECT/CLAUDE.md"
cp -r "$BOOTSTRAP/.claude"       "$PROJECT/.claude"

# Metrics ground truth (recommended); declare how each key is computed
# with metric.<key>.* lines in .rebarrc
cp "$BOOTSTRAP/METRICS.md"       "$PROJECT/METRICS"

# Contract system (required)
mkdir -p "$PROJECT/architecture"
cp "$BOOTSTRAP/architecture/CONTRACT-TEMPLATE.md" "$PROJECT/architecture/"

# Cross-repo federation (optional — only if your project depends on
# contracts from other rebar repos). CHARTER §1.6. Adding entries to
# CONSUMES.md activates `rebar contract drift-check` as a required CI
# check via check-compliance. Single-repo projects skip.
cp "$BOOTSTRAP/CONSUMES.md"      "$PROJECT/CONSUMES.md"  # then edit or delete

# Enforcement entry points (recommended). The checks run from the rebar
# install; the project gets ci-check.sh + pre-commit.sh and tailors the
# checks in .rebarrc.
cp -r "$BOOTSTRAP/scripts"       "$PROJECT/scripts"

# Tier configuration (set your enforcement level)
cp "$BOOTSTRAP/.rebarrc"         "$PROJECT/.rebarrc"

# Version tracking: .rebar-version and the README badge must name the same
# version (check-compliance)
V=$(rebar version | awk '{print $2}')
echo "$V" > "$PROJECT/.rebar-version"
sed -i.bak "s/^> \*\*rebar v[^*]*\*\*/> **rebar $V**/" "$PROJECT/README.md" && rm "$PROJECT/README.md.bak"

mkdir -p "$PROJECT/.github"
cp "$R/.github/pull_request_template.md" "$PROJECT/.github/" 2>/dev/null || true

# CRITICAL: Install pre-commit hook — without this, nothing is enforced
ln -sf ../../scripts/pre-commit.sh "$PROJECT/.git/hooks/pre-commit"

# Role agents (architect, product, englead, steward, merger, featurerequest)
cd "$PROJECT" && ask init
```

rebar's methodology and reference docs (`DESIGN.md`, `conventions.md`,
`practices/`, `architecture/README.md`, the subagent templates in `agents/`)
ship with every install; read them there (`~/.rebar/current/DESIGN.md`,
`~/.rebar/current/practices/`) rather than copying them. Copies link to files elsewhere in the rebar repository, which
`check-doc-refs` then reports as broken in your project (allowlist them in
`.rebar/doc-refs-allow.txt` if you copy them anyway).

**The hook installation is not optional.** Checks without a hook are decorative —
a repo can go weeks with zero enforcement if the hook isn't linked. Verify
with: `ls -la .git/hooks/pre-commit`

If you already have `README.md`, `AGENTS.md`, or `CLAUDE.md`, diff the
templates against yours and merge the sections you're missing.

## Step 3: Customize Each File

Work through each file in this order. Every template has `<!-- comment -->`
blocks explaining what to customize. Remove comments when done.

### README.md (10 min)

The universal first-read. Every agent, every session, no exceptions.

1. **Rebar badge** — First line after `# Title` MUST be the rebar badge:
   ```markdown
   > **rebar v4.0.0** | **Tier 2: ADOPTED**
   ```
   This is validated by `check-compliance` (part of `rebar check`) against
   `.rebar-version`. Update both when you upgrade rebar
   ([UPGRADING.md](UPGRADING.md)). Update the tier when you change `.rebarrc`.
2. **Project name & description** — Replace `<PROJECT_NAME>`
3. **Architecture overview** — Briefly describe the contract structure
4. **Quick Start** — How to get the project running
5. **Project Structure** — Directory layout
6. **Core Tenets** — 3-5 non-negotiable principles

### QUICKCONTEXT.md (5 min)

The most volatile file. Update at the start and end of every session.

1. **Project** — One-liner
2. **Current Branch & State** — Branch, test counts, build status
3. **What's In Progress** — Active work right now
4. **What's Next** — Verify items haven't been implemented before listing
5. **Update the freshness date**

### TODO.md (5 min)

Tasks + known issues + blockers, all in one place.

1. **P0 items** — Your most urgent tasks
2. **Known Issues** — Active blockers, gotchas, workarounds
3. **Audit existing TODOs** — `grep -rn "TODO:" src/` — fix them or track them
4. **Update freshness and last-synced dates**

### AGENTS.md (10 min)

How agents work in this project. This file is now slim — mandatory
foundations only. Advanced practices live in `practices/`.

1. **Core Tenets** — Mirror from README.md
2. **Agent Autonomy** — Adjust "Requires Discussion" for your architecture
3. **Contract-Driven Development** — Customize for your contract categories
4. **Testing Cascade** — Fill in commands for your test runner

### CLAUDE.md (15 min)

Claude Code-specific configuration.

1. **Commands** — Build, test, lint commands
2. **Coding Style** — Conventions + contract linking rules
3. **Allowed Commands** — Adjust for your tools
4. **Environment Variables** — List your env vars

### architecture/ (10 min)

1. **Define your first contracts** — Start with the most important interfaces
2. **Generate CONTRACT-REGISTRY.md** — `git add` your contracts and source, then run `rebar run compute-registry` (again after adding or changing contracts; it counts implementing files git tracks)
3. **Convention** — Decide on your ID prefixes (S, C, I, P or your own)

### agents/ (10 min, optional)

1. **subagent-guidelines.md** — Customize for your project's sensitive areas
2. **subagent-prompts/** — Keep templates relevant to your profile, remove others
3. **Add .gitignore** for `agents/results/` if results are ephemeral

## Step 4: Verify

```bash
# Confirm Cold Start Quad exists
ls README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md

# Confirm architecture directory
ls architecture/CONTRACT-REGISTRY.md

# Confirm no leftover placeholders
grep -rn "<PROJECT_NAME>\|<pkg-manager>\|YYYY-MM-DD" \
  README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md

# Run the checks the hook and CI run (stage first: check-doc-refs only
# accepts links to files git tracks)
git add -A
rebar check --pre-commit
rebar check

# Confirm steward agent
[ -f agents/steward/AGENT.md ] && echo "Steward agent: ready" || echo "Steward agent: not found"

# Ground truth: METRICS keys are computed from metric.<key>.* lines in .rebarrc
# (it runs at tier 3, or at any tier with `checks_enable = check-ground-truth`)
rebar run check-ground-truth

# Test the agent experience: start a new Claude Code session
```

## Step 4b: Initialize Integrity Tracking

```bash
# rebar is on your PATH after `Install rebar` above. Working from a rebar
# checkout instead? Build it there (requires Go): cd cli && go build -o ../bin/rebar .

# Initialize integrity system
rebar init

# Verify everything is clean
rebar verify
rebar status
```

This creates `.rebar/integrity.json` (hash manifest), `.rebar/salt` (gitignored),
and tracks all enforcement scripts, contracts, and test files. From this point
forward, use `rebar commit` instead of `git commit` to ensure integrity tracking.

**`rebar init` also writes `.mcp.json`** — a Claude Code MCP config
that exposes ASK agents as first-class tools (`ask_<repo>_<role>`).
Reload Claude Code in this directory and the tools appear automatically.
If you prefer user-level config (available in all projects, not just
adopted ones), or if your layout needs a non-default `--repos-dir`,
see **[docs/MCP-SETUP.md](docs/MCP-SETUP.md)**.

The generated config runs `ask-mcp-server --stdio`, which opens no network
port. To share agents over HTTP (`ask serve` / `ask-mcp-server --port`),
note that the servers bind `127.0.0.1` by default and refuse a public
`--host` without an API key (`--api-key` / `ASK_API_KEY`; or
`--insecure-no-auth` on a trusted network) — see
[docs/MCP-SETUP.md § Security considerations](docs/MCP-SETUP.md#security-considerations).

**Commit `.mcp.json`?** It contains absolute paths specific to the
machine that ran `rebar init`. Solo devs: commit. Team projects with
heterogeneous dev environments: add to `.gitignore` and let each dev
regenerate via `rebar init`. Rebar does not auto-gitignore it.

## Step 5: Commit

```bash
git status        # review what rebar added (.rebar/salt stays untracked)
git add -A
rebar commit -m "docs: adopt contract-driven rebar methodology

Add Cold Start Quad (README, QUICKCONTEXT, TODO, AGENTS), Claude Code
config, methodology, contract system, and agent orchestration templates.
See: https://github.com/willackerly/rebar"
```

## Adoption Timing

**If you already have architecture docs and API specs,** contract
adoption is a 2-3 hour reformatting exercise with 6-way worktree
fanout. The agents reformat existing knowledge into the contract
template — they don't invent contracts from scratch.

**If you don't have architecture docs,** write those first. You can't
formalize contracts for systems you haven't documented.

**Recommended adoption sequence for existing projects:**

1. Doc consistency (fix stale refs, numeric claims)
2. API spec parity (one spec per route module)
3. Contract scaffolding (methodology, conventions, architecture/, agents/)
4. Contract creation (worktree fanout from existing docs)
5. Header stamping (Tier 1 then Tier 2, both via worktree fanout)
6. Role definitions (AGENT.md files with project-specific context)
7. ASK CLI integration (immediate payoff from role definitions)
8. BDD Gherkin scenarios (capstone — last, not first)

This is the reverse of the greenfield sequence (BDD → contracts → code)
but the only practical order for existing codebases.

## Ongoing Maintenance

| Cadence | Action |
|---------|--------|
| **Every session start** | Agent reads Cold Start Quad, verifies freshness |
| **Every session end** | Update QUICKCONTEXT "In Progress" / "Recently Complete" |
| **Every commit** | Run TODO two-tag check, update `METRICS` if counts changed, update docs if needed |
| **Every new source file** | Add `CONTRACT:` header comment |
| **Weekly** | Scrub TRACKED-TASK comments, review TODO staleness |
| **Monthly** | Full doc-drift audit (or use the doc-drift-detector template) |
| **Per contract change** | `grep -rn "CONTRACT:{id}"` → update all implementing code |

## Troubleshooting

**Agent ignores the templates:**
Claude Code reads `CLAUDE.md` automatically. `CLAUDE.md`'s Cold Start section
tells the agent to read README → QUICKCONTEXT → TODO → AGENTS.

**Templates feel too heavy for a small project:**
Start with README.md, QUICKCONTEXT.md, and CLAUDE.md. Add contracts and TODO
tracking as the project grows. Check your [profile](profiles/) for what to skip.

**Agent still makes mistakes I've corrected before:**
The correction belongs in a template or guideline. Add it to the relevant
subagent template's Anti-Patterns section, or to AGENTS.md.

**Contract system feels like overhead:**
Start with just 2-3 contracts for your most important interfaces. The system
proves its value the first time an agent modifies code and checks the contract
first instead of guessing at the intended behavior.
