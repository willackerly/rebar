# <PROJECT_NAME>

<!-- One paragraph: what this project is, what problem it solves, and the
     core tech stack. This is the first thing any agent or human reads. -->

## Agent Cold Start

**Every agent session starts here.** Read these four files in order:

1. **This file** (`README.md`) → project orientation (you're here)
2. `QUICKCONTEXT.md` → what's true right now (branch, test counts, active work)
3. `TODO.md` → what needs doing + known issues + blockers
4. `AGENTS.md` → how we work (norms, testing cascade, contracts, collaboration)

Then: `CLAUDE.md` for Claude-specific configuration.

**Trust but verify:** After reading QUICKCONTEXT.md, run `git log --oneline -10`
and compare. If docs and reality disagree, reality wins.

## Architecture

This project uses contract-driven development. All contracts live in
`architecture/`. See [methodology.md](methodology.md) for the full philosophy.

```bash
# Find all contracts
ls architecture/CONTRACT-*.md

# Find what contract a code file implements
head -10 path/to/file.go    # read the CONTRACT: header

# Find all code implementing a specific contract
grep -rn "CONTRACT:C1-BLOBSTORE" src/ internal/
```

**Rule:** Don't implement without a contract. Don't modify code without
checking its contract. See `architecture/README.md` for details.

## Quick Start

<!-- How to get the project running from zero. Example:

```bash
git clone <repo-url>
cd <project>
<pkg-manager> install
<pkg-manager> dev
```
-->

## Project Structure

<!-- Map the directory layout. Example:
- `src/` or `internal/` — application code
- `pkg/` or `shared/` — shared contracts and types
- `architecture/` — contract documents
- `product/` — BDD features, personas, user stories
- `agents/` — subagent orchestration templates
- `tests/` — E2E and integration tests
- `deploy/` — Docker, Kubernetes, CI/CD configs
- `docs/` — additional documentation
-->

## Core Tenets

<!-- 3-5 non-negotiable architectural principles. Example:
1. **Zero-Knowledge** — The server never sees cleartext.
2. **Contract-First** — Write the contract, then the code.
3. **Offline-Capable** — Core features work without network.
-->

## Contributing

<!-- For human contributors. Reference AGENTS.md for agent-specific norms. -->

See `AGENTS.md` for development norms, testing cascade, and collaboration
patterns. All code must reference its architecture contract in the file header.

---

<!-- MAINTENANCE NOTES:
This file should be stable — it changes when the project's identity or
structure changes, not when daily work happens. Keep it short and focused
on orientation. Daily state goes in QUICKCONTEXT.md, not here.
-->
