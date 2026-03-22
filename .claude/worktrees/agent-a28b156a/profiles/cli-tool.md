# Profile: CLI Tool / Developer Utility

For command-line tools, build tools, code generators, dev utilities.

## Core Files — Copy All

| File | Priority | Notes |
|------|----------|-------|
| `README.template.md` | Required | Include usage examples, installation |
| `QUICKCONTEXT.template.md` | Required | Track version, release status |
| `TODO.template.md` | Required | Full two-tag system |
| `AGENTS.template.md` | Required | Slim down — skip deployment/E2E sections |
| `CLAUDE.template.md` | Required | Slim down — focused on build/test |
| `methodology.md` | Required | Reference — lighter touch on contracts |

## Architecture — Light Touch

| Item | Relevance |
|------|-----------|
| Contract system | **Medium** — useful for plugin interfaces, config schema |
| BDD features | **Medium** — useful for complex CLI workflows |
| Contract linking | **Low** — small codebases may not need per-file linking |

**When contracts are worth it:**
- Plugin/extension interfaces
- Configuration schema (what's valid, what's deprecated)
- Output format contracts (JSON output that downstream tools depend on)
- Inter-process communication (if the tool spawns subprocesses)

## Subagent Templates — Selective

| Template | Relevance |
|----------|-----------|
| `code-review.md` | **High** |
| `security-surface-scan.md` | **Medium** — important if handling user input/files |
| `test-shard-runner.md` | **Medium** — once test suite grows |
| `contract-audit.md` | **Low** — unless plugin interface exists |
| `doc-drift-detector.md` | **Low** — smaller doc surface |
| `feature-inventory.md` | **Low** — smaller codebase |
| `ux-review.md` | **N/A** |

## AGENTS.template.md — What to Keep

| Section | Action |
|---------|--------|
| Core Tenets | Customize: "Zero dependencies where possible", "Fast startup", "Unix philosophy" |
| Testing Cascade | Simplify: T0-T3 sufficient, skip T4-T5 |
| Agent Collaboration | Keep but scale down — fewer parallel agents needed |
| TODO Tracking | Keep — even small projects benefit |
| E2E Test Server Management | Skip |
| Deployment Traps | Skip (unless published to package registry) |

## What You Can Skip

- E2E Test Server Management (no servers)
- Deployment Traps (no deployed infrastructure)
- UX review template
- Visual/E2E testing tiers
- Most of the web-specific guidance
- Complex fan-out patterns (smaller codebase, less parallelism needed)

## What to Add

- **Release checklist** — version bump, changelog, package publish
- **Backwards compatibility policy** — CLI flag deprecation, output format stability
- **Cross-platform testing** — Linux/macOS/Windows matrix if applicable
