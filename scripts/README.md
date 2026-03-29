# Scripts

Enforcement scripts and quality scanning for the contract-driven methodology.

See the [root README](../README.md) for how scripts fit into the overall system.

> **Note:** These scripts are now called via the `rebar` CLI for integrity tracking:
> - `rebar check` — runs `ci-check.sh` with integrity updates
> - `rebar commit` — runs `pre-commit.sh` + updates integrity manifest (no `--no-verify`)
> - `rebar verify` — checks hash integrity of all protected files (including these scripts)
>
> Direct invocation still works but bypasses integrity tracking.
> See `docs/REBAR-CLI-INTEGRITY.md` for the full design.

## Quality Scanner

| Script | What It Does | Invocation |
|--------|-------------|-----------|
| `steward.sh` | Full quality scan — contract lifecycle, enforcement, discoveries | `ask steward` or `./scripts/steward.sh` |
| `steward.sh --json` | Aggregate JSON to stdout | `ask steward json` |
| `steward.sh --summary` | One-line health summary | `ask steward summary` |
| `steward.sh --check <id>` | Single contract scan | `ask steward check C1` |

Output goes to `architecture/.state/` (JSON) and `STEWARD_REPORT.md` (human-readable).

## Enforcement Checks

Each script is standalone, runs in <5 seconds, and exits 0 (pass) or 1 (fail).

| Script | What It Checks |
|--------|---------------|
| `check-contract-headers.sh` | Every source file has a `CONTRACT:` header |
| `check-contract-refs.sh` | Every `CONTRACT:` ref points to a real contract file |
| `check-todos.sh` | No untracked `TODO:` comments (two-tag system) |
| `check-freshness.sh` | Doc freshness dates aren't stale (>14 days) |
| `check-registry.sh` | Contract registry matches actual files |
| `check-ground-truth.sh` | METRICS file matches codebase reality |

## Composite Runners

| Script | When to Run |
|--------|-------------|
| `ci-check.sh` | CI pipeline — runs all checks including steward |
| `pre-commit.sh` | Git hook — fast checks (TODOs + contract refs) |

## Installation

```bash
# Pre-commit hook (pick one)
cp scripts/pre-commit.sh .git/hooks/pre-commit     # copy
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit  # symlink

# Make all scripts executable
chmod +x scripts/*.sh

# CI pipeline (GitHub Actions example)
# - run: ./scripts/ci-check.sh --strict
```

## Configuration

| Environment Variable | Default | Purpose |
|---------------------|---------|---------|
| `CONTRACT_EXTENSIONS` | `.go .ts .tsx .js .jsx .py .rs` | File extensions to scan |
| `SKIP_CONTRACT_HEADERS` | `0` | Skip header check |
| `SKIP_CONTRACT_REFS` | `0` | Skip ref check |
| `SKIP_TODOS` | `0` | Skip TODO check |
| `SKIP_FRESHNESS` | `0` | Skip freshness check |
| `SKIP_REGISTRY` | `0` | Skip registry check |
| `SKIP_GROUND_TRUTH` | `0` | Skip ground truth check |
| `SKIP_STEWARD` | `0` | Skip steward scan |

## Automation Hierarchy

```
Pre-commit (fast, <5s)       CI (thorough, <30s)          Full scan (comprehensive)
├── check-todos.sh           ├── all pre-commit checks    ├── all CI checks
└── check-contract-refs.sh   ├── check-contract-headers   ├── steward.sh (lifecycle,
                             ├── check-freshness              discoveries, action items)
                             ├── check-registry            └── STEWARD_REPORT.md
                             ├── check-ground-truth
                             └── steward.sh
```

## Dependencies

- **bash** — all scripts are bash
- **jq** — required by steward.sh and ground truth verification
- **grep, find** — standard Unix tools
