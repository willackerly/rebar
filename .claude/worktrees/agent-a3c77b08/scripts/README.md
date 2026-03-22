# Scripts

Enforcement scripts for the contract-driven methodology. Copy these into
your project's `scripts/` directory and wire them into your CI and git hooks.

## Individual Checks

Each script is standalone, runs in <5 seconds, and exits 0 (pass) or 1 (fail).

| Script | What It Checks | When to Run |
|--------|---------------|-------------|
| `check-contract-headers.sh` | Every source file has a `CONTRACT:` header | CI, periodic |
| `check-contract-refs.sh` | Every `CONTRACT:` ref points to a real file | Pre-commit, CI |
| `check-todos.sh` | No untracked `TODO:` comments (two-tag system) | Pre-commit, CI |
| `check-freshness.sh` | Doc freshness dates aren't stale (>14 days) | CI, weekly |
| `check-registry.sh` | Contract registry matches files, no untracked orphans | CI, periodic |

## Composite Checks

| Script | What It Does | When to Run |
|--------|-------------|-------------|
| `ci-check.sh` | Runs all checks, reports summary | CI pipeline |
| `pre-commit.sh` | Runs fast checks (TODOs + contract refs) | Git pre-commit hook |

## Installation

### Pre-commit Hook

```bash
# Option 1: Copy
cp scripts/pre-commit.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit

# Option 2: Symlink (tracks updates)
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit
```

### CI Pipeline

```yaml
# GitHub Actions example
- name: Contract & doc checks
  run: ./scripts/ci-check.sh --strict

# Skip specific checks if needed
- name: Contract checks (no freshness)
  run: SKIP_FRESHNESS=1 ./scripts/ci-check.sh --strict
```

### Make all scripts executable

```bash
chmod +x scripts/*.sh
```

## Configuration

| Environment Variable | Default | Purpose |
|---------------------|---------|---------|
| `CONTRACT_EXTENSIONS` | `.go .ts .tsx .js .jsx .py .rs` | File extensions to scan |
| `SKIP_CONTRACT_HEADERS` | `0` | Skip header check in ci-check.sh |
| `SKIP_CONTRACT_REFS` | `0` | Skip ref check in ci-check.sh |
| `SKIP_TODOS` | `0` | Skip TODO check in ci-check.sh |
| `SKIP_FRESHNESS` | `0` | Skip freshness check in ci-check.sh |
| `SKIP_REGISTRY` | `0` | Skip registry check in ci-check.sh |

## Automation Hierarchy

```
Pre-commit (fast, <5s)        CI (thorough, <30s)        Periodic (comprehensive)
├── check-todos.sh            ├── check-contract-headers  ├── check-freshness.sh (14d)
└── check-contract-refs.sh    ├── check-contract-refs     └── check-registry.sh
                              ├── check-todos.sh
                              ├── check-freshness.sh
                              └── check-registry.sh
```
