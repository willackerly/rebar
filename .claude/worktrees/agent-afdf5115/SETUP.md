# Setup Guide

How to adopt the rebar kit in your project.

**Start by reading [methodology.md](methodology.md)** to understand the
philosophy. Then pick your [project profile](profiles/) to know which
parts to adopt.

## Prerequisites

- A git repository
- Claude Code (or another AI coding agent that reads markdown context files)
- 30 minutes for initial setup
- `jq` (for steward quality scanning — `brew install jq` / `apt install jq`)

## Step 1: Pick Your Profile

Check [profiles/](profiles/) for your project type:
- [web-app.md](profiles/web-app.md) — SPA, SSR, frontend + API
- [api-service.md](profiles/api-service.md) — Backend API, microservice
- [crypto-library.md](profiles/crypto-library.md) — Security-critical library
- [cli-tool.md](profiles/cli-tool.md) — Command-line tool

Your profile tells you which files to copy and which sections to customize.

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
cp methodology.md            "$PROJECT/methodology.md"

# Contract system (required)
cp -r architecture/          "$PROJECT/architecture/"

# Agent orchestration (recommended)
cp -r agents/                "$PROJECT/agents/"

# Enforcement scripts and conventions (recommended)
cp -r scripts/               "$PROJECT/scripts/"
cp conventions.md            "$PROJECT/conventions.md"
cp METRICS.template           "$PROJECT/METRICS"
# State directory for steward
mkdir -p "$PROJECT/architecture/.state"
touch "$PROJECT/architecture/.state/.gitkeep"

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

1. **Project name & description** — Replace `<PROJECT_NAME>`
2. **Architecture overview** — Briefly describe the contract structure
3. **Quick Start** — How to get the project running
4. **Project Structure** — Directory layout
5. **Core Tenets** — 3-5 non-negotiable principles

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

### AGENTS.md (15 min)

How agents work in this project.

1. **Core Tenets** — Mirror from README.md
2. **Agent Autonomy** — Adjust "Requires Discussion" for your architecture
3. **Contract-Driven Development** — Customize for your contract categories
4. **Testing Cascade** — Fill in commands for your test runner
5. **Active Workstreams** — Current priorities

### CLAUDE.md (15 min)

Claude Code-specific configuration.

1. **Commands** — Build, test, lint commands
2. **Coding Style** — Conventions + contract linking rules
3. **Allowed Commands** — Adjust for your tools
4. **Environment Variables** — List your env vars

### architecture/ (10 min)

1. **Define your first contracts** — Start with the most important interfaces
2. **Fill in CONTRACT-REGISTRY.md** — Index your contracts
3. **Convention** — Decide on your ID prefixes (S, C, I, P or your own)

### agents/ (10 min, optional)

1. **subagent-guidelines.md** — Customize for your project's sensitive areas
2. **subagent-prompts/** — Keep templates relevant to your profile, remove others
3. **Add .gitignore** for `agents/results/` if results are ephemeral

## Step 4: Verify

```bash
# Confirm Cold Start Quad exists
ls README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md methodology.md

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

## Step 5: Commit

```bash
git add README.md QUICKCONTEXT.md TODO.md AGENTS.md CLAUDE.md methodology.md
git add architecture/ agents/
git commit -m "docs: adopt contract-driven rebar methodology

Add Cold Start Quad (README, QUICKCONTEXT, TODO, AGENTS), Claude Code
config, methodology, contract system, and agent orchestration templates.
See: https://github.com/willackerly/rebar"
```

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
