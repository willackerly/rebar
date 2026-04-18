# My Project

> **rebar v2.0.0** | **Tier 1: PARTIAL** | [What is rebar?](https://github.com/willackerly/rebar)

**Description:** [Brief description of what this project does]

---

## Quick Start

```bash
# Install dependencies
[your install commands]

# Run tests
[your test commands]

# Start development
[your dev commands]
```

---

## Contract-Driven Development

This project uses **rebar contracts** to coordinate development across agents and humans:

- **Contracts in `architecture/`** — behavioral specifications for components
- **Role-based agents** — persistent experts for architecture, product, testing, etc.
- **Quality enforcement** — automated checks for contract compliance and documentation

### Key Files (Read These First)

1. **[QUICKCONTEXT.md](QUICKCONTEXT.md)** — Current project state
2. **[TODO.md](TODO.md)** — Active work and known issues
3. **[AGENTS.md](AGENTS.md)** — How we work with AI agents
4. **[architecture/](architecture/)** — Contract specifications

### Using Agents

```bash
# Get help with architecture decisions
ask architect "Should I add caching to the user service?"

# Product perspective on features
ask product "Does this contract meet our user requirements?"

# Quality and delivery coordination
ask englead "Are we ready to ship this feature?"

# Automated health checks
ask steward summary
```

---

## Project Structure

```
my-project/
├── QUICKCONTEXT.md         # Current state (updated by agents)
├── TODO.md                 # Active work and issues
├── AGENTS.md               # Agent coordination guidelines
├── CLAUDE.md               # Claude Code configuration
├── METRICS.md              # Ground truth metrics
├── architecture/           # Contract specifications
│   ├── CONTRACT-*.md      # Individual contracts
│   └── CONTRACT-REGISTRY.md
├── scripts/               # Quality enforcement
│   ├── check-*.sh        # Individual checks
│   ├── ci-check.sh       # Full CI pipeline
│   └── steward.sh        # Quality scanner
└── [your source code]
```

---

## Development Workflow

### Adding a Feature

1. **Define success** — BDD scenario in `features/` or user story
2. **Design contract** — behavioral specification in `architecture/`
3. **Coordinate agents** — get architecture, product, and engineering input
4. **Implement** — write code that fulfills the contract
5. **Quality cascade** — T0 (unit) → T1 (integration) → T2 (security) tests
6. **Integrate** — merge with automated quality gates

### Quality Gates

```bash
# Before committing
scripts/check-contract-refs.sh    # Contract links valid
scripts/check-todos.sh           # No untracked TODOs

# Before merging
scripts/ci-check.sh              # Full quality scan
ask steward "ready to ship?"     # Health check
```

---

## Configuration

**Rebar Tier:** 1 (Partial) — basic contract enforcement + TODO tracking
- Upgrade to Tier 2 for headers + freshness + registry enforcement
- Upgrade to Tier 3 for ground truth + strict steward scanning

**Agent Access:** Configure in CLAUDE.md for Claude Code integration

---

## Learn More

- **[Contract quickstart](https://github.com/willackerly/rebar/blob/main/CONTRACT-QUICKSTART.md)** — Write your first contract in 5 minutes
- **[Agent coordination](https://github.com/willackerly/rebar/blob/main/AGENTS-QUICKSTART.md)** — Role agents vs subagent templates
- **[Feature development](https://github.com/willackerly/rebar/blob/main/FEATURE-DEVELOPMENT.md)** — Complete BDD→Contract→Code workflow
- **[Case studies](https://github.com/willackerly/rebar/blob/main/CASE-STUDIES.md)** — Real-world solutions by problem type