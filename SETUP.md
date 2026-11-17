# Setup Guide

How to adopt the rebar kit in your project.

**Start by reading [DESIGN.md](DESIGN.md)** to understand the
philosophy. Then pick your [project profile](profiles/) to know which
parts to adopt.

## Prerequisites

- A git repository
- Claude Code (or another AI coding agent that reads markdown context files)
- 30 minutes for initial setup
- `jq` (for steward quality scanning — `brew install jq` / `apt install jq`)
- The rebar install — see [Install rebar](#install-rebar)

## Install rebar

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.0/setup-rebar.sh | bash -s -- v3.1.0
```

That installs v3.1.0 and puts `~/.rebar/current/bin` (`rebar`, `ask`,
`ask-mcp-server`) on your PATH via your shell RC; open a new terminal (or
source the RC) and check with `rebar version`. Fetch `setup-rebar.sh` from the
tag you install, as above: the copy on `main` is whatever `main` holds. With no
version argument the installer takes the latest stable release (the highest
`vX.Y.Z` tag, ignoring pre-releases such as `v3.2.0-rc.1`; if no stable tag
exists yet, the highest pre-release). Variants (`…` is the `curl` part):

| Command | Installs |
|---------|----------|
| `… \| bash -s -- v3.1.0` | that release |
| `… \| bash -s -- --pre` | the newest tag, pre-releases included |
| `… \| bash -s -- main` | a branch, built from source; `rebar version` says `main@<sha>` |
| `… \| bash -s -- v3.1.0 --no-switch` | side by side, without changing the active version |
| `… \| bash -s -- v3.1.0 --source` | from source even if a prebuilt download exists |
| `… \| bash -s -- v3.1.0 --dir /opt/rebar` | under `/opt/rebar` instead of `~/.rebar` (CI) |

Each version is a complete rebar tree in `~/.rebar/versions/<version>/`;
`~/.rebar/current` is a symlink to the active one, so switching never edits
your RC again (`ln -sfn versions/<version> ~/.rebar/current` switches by
hand; `rm -rf ~/.rebar/versions/<version>` removes one). Releases with
prebuilt assets download as a tarball verified against the release's SHA-256
checksums (a mismatch aborts); releases without assets, branches and commits
(by full 40-character SHA) build from source, which needs `git` and Go 1.25
or newer (use a [supported release](https://go.dev/doc/devel/release#policy):
the binary includes its standard library; see [SECURITY.md](SECURITY.md)). Branch
names are made path-safe for the directory (`feat/x` → `versions/feat-x`); if
two refs map to the same name, the second install is refused rather than
replacing the first. `./setup-rebar.sh --help` lists every option.

The CLI runs the scripts of the tree it belongs to, so `rebar check` always
uses the checks of the version you run. Upgrading a project that already uses
rebar: [UPGRADING.md](UPGRADING.md).

### Testing a release candidate

Release candidates are tags like `v3.2.0-rc.1`, published as GitHub
pre-releases; the default "latest" install skips them. Use the installer from
the candidate's own tag (or `--pre` for the newest pre-release):

```bash
curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.2.0-rc.1/setup-rebar.sh \
  | bash -s -- v3.2.0-rc.1 --no-switch
~/.rebar/versions/v3.2.0-rc.1/bin/rebar version     # rebar v3.2.0-rc.1
```

`--no-switch` keeps your current version active (except on a first versioned
install, which always becomes current), so you can try the candidate by path
on one project, then switch with
`ln -sfn versions/v3.2.0-rc.1 ~/.rebar/current`. Then follow
[QUICKSTART.md](QUICKSTART.md) for a new project or
[UPGRADING.md](UPGRADING.md) for an existing one, and report problems as
issues on the repository.

Name the candidate explicitly: without a version the installer takes the latest
*stable* release, so a candidate is only picked by `--pre` (the newest tag,
pre-releases included).

**Upgrading from a `~/.rebar` clone** (what the installer made before versioned installs): the
installer leaves the clone as it is — not moved, not updated — and adds
`versions/` and `current` inside it (git-excluded). The new PATH entry comes
later in your RC, so it wins; `.mcp.json` files and `REBAR_ROOT` that point at
the clone keep working until you re-run `rebar init` or change them. If
`REBAR_ROOT` is set, the CLI runs that tree's scripts, not
`~/.rebar/current`'s; unset it to use the installed version.

## Run rebar in CI

A project's checks run from the rebar install (`rebar check` executes the
install's `scripts/` against the project), so CI needs a complete rebar tree
at the version you pin, not just the `rebar` binary.

**GitHub Actions, with the installer:**

```yaml
      - uses: actions/checkout@v4
      - name: Install rebar
        env:
          REBAR_VERSION: v3.1.0
        run: |
          curl -fsSL "https://raw.githubusercontent.com/willackerly/rebar/${REBAR_VERSION}/setup-rebar.sh" \
            | bash -s -- "$REBAR_VERSION" --dir "$RUNNER_TEMP/rebar"
          echo "$RUNNER_TEMP/rebar/current/bin" >> "$GITHUB_PATH"
      - name: rebar checks
        run: rebar check            # or ./scripts/ci-check.sh
```

The release's prebuilt tarball needs only `curl`, `tar` and `shasum` or
`sha256sum`. A tag without prebuilt assets falls back to a source build, which
needs `git` and Go (add `actions/setup-go`). The checks themselves need `jq`
and `git`. The installer also appends a PATH line to the runner's shell RC,
which is harmless there.

**How the CLI finds its scripts.** In order (`findRebarRoot` in
`cli/cmd/init.go`), the first directory that holds both
`templates/project-bootstrap/` and `scripts/steward.sh` wins:

1. `$REBAR_ROOT`
2. the directory above the running binary, after resolving symlinks
   (`~/.rebar/current/bin/rebar` → `~/.rebar/versions/<v>/`)
3. the current directory (working on rebar itself)
4. `~/.rebar/current`, `~/.rebar`, `~/dev/rebar`, `~/src/rebar`, `~/code/rebar`

If none matches, `rebar check` fails (`rebar: ci-check.sh not found in the
project or a rebar install`) and `rebar check --pre-commit` prints
`pre-commit.sh not found … skipping` and **exits 0** (a commit hook must not
block on a missing install). A CI step that runs `--pre-commit` without an
install therefore passes without checking anything.

**The trap: pinned binary, no tree.** A wrapper that builds only the `rebar`
binary from a pinned commit into a cache directory (virtru-analyze's
`scripts/rebar.sh` did) worked while the project carried copies of rebar's
scripts. Under 3.1 that binary finds no `scripts/` beside it; it falls through
to `~/.rebar/current` (a different version on a laptop, nothing on a CI
runner). Keep the whole tree of the pinned commit and build the binary into
it:

```bash
#!/usr/bin/env bash
# scripts/rebar.sh — run the rebar version this repository pins, with that
# version's own scripts. Usage: scripts/rebar.sh check | run <script> | ...
set -euo pipefail

REBAR_PIN="v3.1.0"                          # a release tag
REBAR_PIN_COMMIT="<commit the tag points at>" # git rev-parse 'v3.1.0^{commit}' in a clone
REBAR_REPOSITORY="${REBAR_REPOSITORY:-https://github.com/willackerly/rebar.git}"
ROOT="${REBAR_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/rebar}/$REBAR_PIN"

if [ ! -x "$ROOT/bin/rebar" ] || [ "$(cat "$ROOT/.pinned-commit" 2>/dev/null)" != "$REBAR_PIN_COMMIT" ]; then
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/rebar-pin.XXXXXX")"
  trap 'rm -rf "$tmp"' EXIT
  git -C "$tmp" init -q src
  git -C "$tmp/src" fetch -q --depth 1 "$REBAR_REPOSITORY" "refs/tags/$REBAR_PIN"
  [ "$(git -C "$tmp/src" rev-parse 'FETCH_HEAD^{commit}')" = "$REBAR_PIN_COMMIT" ] \
    || { echo "rebar $REBAR_PIN does not point at $REBAR_PIN_COMMIT" >&2; exit 1; }
  # The whole tree, not just the CLI: rebar check runs the tree's scripts/.
  mkdir "$tmp/tree"
  git -C "$tmp/src" archive FETCH_HEAD | tar -x -C "$tmp/tree"
  (cd "$tmp/tree/cli" && CGO_ENABLED=0 go build -trimpath -o ../bin/rebar .)
  echo "$REBAR_PIN_COMMIT" > "$tmp/tree/.pinned-commit"
  mkdir -p "$(dirname "$ROOT")" && rm -rf "$ROOT" && mv "$tmp/tree" "$ROOT"
  rm -rf "$tmp"; trap - EXIT   # exec below replaces the shell: no EXIT trap runs
fi

export REBAR_ROOT="$ROOT"
export PATH="$ROOT/bin:$PATH"   # nested `rebar` calls get the same version
exec "$ROOT/bin/rebar" "$@"
```

The binary would find `$ROOT` through its own `../` anyway; `REBAR_ROOT` makes
the choice explicit and wins over a stray `~/.rebar`. The project's
`scripts/ci-check.sh` and `scripts/pre-commit.sh`, and the wrappers
`rebar migrate` leaves for copies that other files call, run whatever `rebar`
is on PATH. So a project script that CI runs directly should call the pinned
wrapper (`scripts/rebar.sh run check-todos`), not those files, unless the CI
job puts the pinned `bin/` on PATH. In CI, cache
`$REBAR_CACHE_DIR` between runs if the build time matters. Without Go on the
runner, use the installer instead:
`setup-rebar.sh "$REBAR_PIN" --dir "$ROOT_BASE" --no-switch`, then run
`"$ROOT_BASE/versions/$REBAR_PIN/bin/rebar"`.

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

**Copying by hand** instead works from the install tree, which holds the same
templates (`~/.rebar/current`; any rebar checkout works too). The copied
`scripts/` entry points call `rebar`, so rebar must still be on your PATH:

```bash
PROJECT=/path/to/your/project
R=~/.rebar/current
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
   > **rebar v3.1.0** | **Tier 2: ADOPTED**
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
