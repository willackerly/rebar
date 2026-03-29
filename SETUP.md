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

## Step 2: Copy the Core Files

```bash
# From the rebar directory:
PROJECT=/path/to/your/project

# The Cold Start Quad (required)
cp README.template.md        "$PROJECT/README.md"
cp QUICKCONTEXT.template.md  "$PROJECT/QUICKCONTEXT.md"
cp TODO.template.md          "$PROJECT/TODO.md"
cp AGENTS.template.md        "$PROJECT/AGENTS.md"

# Claude Code config (required for Claude Code users)
cp CLAUDE.template.md        "$PROJECT/CLAUDE.md"

# Methodology (required — the philosophy)
cp DESIGN.md            "$PROJECT/DESIGN.md"

# Contract system (required)
cp -r architecture/          "$PROJECT/architecture/"

# Agent orchestration (recommended)
cp -r agents/                "$PROJECT/agents/"

# Practice reference guides (recommended)
cp -r practices/             "$PROJECT/practices/"

# Enforcement scripts and conventions (recommended)
cp -r scripts/               "$PROJECT/scripts/"
cp conventions.md            "$PROJECT/conventions.md"
cp METRICS.template           "$PROJECT/METRICS"
# State directory for steward
mkdir -p "$PROJECT/architecture/.state"
touch "$PROJECT/architecture/.state/.gitkeep"

# Tier configuration (set your enforcement level)
cp .rebarrc.template         "$PROJECT/.rebarrc"

# Version tracking (so you know which rebar you adopted)
echo "v1.2.0" > "$PROJECT/.rebar-version"

mkdir -p "$PROJECT/.github"
cp .github/pull_request_template.md "$PROJECT/.github/"
chmod +x "$PROJECT/scripts/"*.sh

# Install pre-commit hook
ln -sf ../../scripts/pre-commit.sh "$PROJECT/.git/hooks/pre-commit"
```

If you already have `README.md`, `AGENTS.md`, or `CLAUDE.md`, diff the
templates against yours and merge the sections you're missing.

## Step 3: Customize Each File

Work through each file in this order. Every template has `<!-- comment -->`
blocks explaining what to customize. Remove comments when done.

### README.md (10 min)

The universal first-read. Every agent, every session, no exceptions.

1. **Rebar badge** — First line after `# Title` MUST be the rebar badge:
   ```markdown
   > **rebar v1.2.0** | **Tier 2: ADOPTED**
   ```
   This is validated by `scripts/check-compliance.sh`. Update version when you
   upgrade rebar. Update tier when you change `.rebarrc`.
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
2. **Generate CONTRACT-REGISTRY.md** — Run `scripts/compute-registry.sh`
3. **Convention** — Decide on your ID prefixes (S, C, I, P or your own)

### agents/ (10 min, optional)

1. **subagent-guidelines.md** — Customize for your project's sensitive areas
2. **subagent-prompts/** — Keep templates relevant to your profile, remove others
3. **Add .gitignore** for `agents/results/` if results are ephemeral

## Step 4: Verify

```bash
# Confirm Cold Start Quad exists
ls README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md DESIGN.md

# Confirm architecture directory
ls architecture/CONTRACT-REGISTRY.md

# Confirm no leftover placeholders
grep -rn "<PROJECT_NAME>\|<pkg-manager>\|YYYY-MM-DD" \
  README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md

# Confirm no untracked TODOs
grep -rn "TODO:" --include="*.ts" --include="*.go" --include="*.py" src/ || echo "Clean"

# Confirm steward script
[ -x scripts/steward.sh ] && echo "Steward: ready" || echo "Steward: not found or not executable"

# Confirm steward agent
[ -f agents/steward/AGENT.md ] && echo "Steward agent: ready" || echo "Steward agent: not found"

# Confirm ground truth script has at least one metric defined
# (remove the no-op line and uncomment at least one metric)
grep -q 'echo "' scripts/check-ground-truth.sh && echo "Ground truth: metrics defined" \
  || echo "Ground truth: customize compute_metrics() in scripts/check-ground-truth.sh"

# Test the agent experience: start a new Claude Code session
```

## Step 4b: Initialize Integrity Tracking

```bash
# Build and install the rebar CLI (requires Go 1.22+)
cd cli && go build -o ../bin/rebar . && cd ..

# Initialize integrity system
rebar init

# Verify everything is clean
rebar verify
rebar status
```

This creates `.rebar/integrity.json` (hash manifest), `.rebar/salt` (gitignored),
and tracks all enforcement scripts, contracts, and test files. From this point
forward, use `rebar commit` instead of `git commit` to ensure integrity tracking.

## Step 5: Commit

```bash
git add README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md DESIGN.md
git add architecture/ agents/ .rebar/integrity.json .rebarrc
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
