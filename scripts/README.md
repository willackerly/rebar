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
| `check-doc-refs.sh` | Every `[text](path)` link in tracked `*.md` resolves to a tracked file |
| `check-todos.sh` | No untracked `TODO:` comments (two-tag system) |
| `check-freshness.sh` | Doc freshness dates aren't stale (>14 days) |
| `check-registry.sh` | Contract registry matches actual files |
| `check-ground-truth.sh` | METRICS file matches codebase reality |
| `check-decay-patterns.sh` | Soft-hardening anti-patterns in spec/test files |
| `sync-bootstrap.sh --check` | `templates/project-bootstrap/scripts/` matches `/scripts/` |

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
| `SKIP_DOC_REFS` | `0` | Skip cross-doc reference check |
| `SKIP_DECAY_PATTERNS` | `0` | Skip soft-hardening pattern check |
| `SKIP_BOOTSTRAP_SYNC` | `0` | Skip templates/scripts drift check (rebar-source-only) |
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

- **bash 3.2+** — all shell scripts are bash 3.2 compatible (macOS default)
- **jq** — required by steward.sh and ground truth verification
- **grep, find** — standard Unix tools
- **node** (optional) — only for `templates/scripts/check-tag-ci-coverage.mjs`,
  which is project-specific and not in the universal `ci-check.sh` flow

## Maintainer-facing tools

| Script | What it does |
|--------|--------------|
| `sync-bootstrap.sh` (+ `--check`) | Mirror `/scripts/` into `templates/project-bootstrap/scripts/`; drift caught by ci-check |
| `test-e2e-live.sh` | End-to-end smoke against a live LLM. Gated on `claude` CLI; per-repo gates skip cleanly when adopted repos aren't on disk. Includes version triple-check (binary / `.rebar-version` / latest tag), MCP server discovery + tools/list smoke, ASK HTTP server smoke (if running), and 4 live LLM keyword-acceptance queries (rebar:steward, rebar:architect, local-mode, cross-repo). Run after every meaningful update — single command answer to "is this still working?" |

The maintainer-facing scripts are NOT mirrored into the bootstrap copy
(they assume the rebar source repo's dev layout) — `sync-bootstrap.sh`
explicitly skips them.

## Source-of-truth structure

`/scripts/` is canonical. `templates/project-bootstrap/scripts/` is mirrored
from it by `sync-bootstrap.sh` so adopters running `cp -r templates/project-bootstrap/*`
get a working project in one command. Drift between the two trees is caught
by `sync-bootstrap.sh --check` (wired into `ci-check.sh`).
