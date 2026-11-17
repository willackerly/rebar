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
| `check-jtbd-presence.sh` | Latest-version contracts carry non-empty Why/Who/Scenarios sections |
| `check-prefix-uniqueness.sh` | No prefix number (S1, I3, …) claimed by two different contract IDs |
| `sync-bootstrap.sh --check` | `templates/project-bootstrap/scripts/` holds only thin entry points; the template's `CONTRACT-TEMPLATE.md` matches `architecture/`; the binary's embedded copy (`cli/internal/assets/files/`) matches the template and the `bin/` companions |

## Composite Runners

| Script | When to Run |
|--------|-------------|
| `ci-check.sh` | CI pipeline — runs all checks including steward |
| `pre-commit.sh` | Git hook — fast checks (TODOs + contract refs) |
| `cold-start-checks.sh` | Session start (via the `SessionStart` hook in `.claude/settings.json`) — enforcement quad + maturity counts, always exits 0, `<rebar-cold-start>` wrapped |

`rebar check`, `rebar check --pre-commit` and `rebar check --cold-start` run
native (Go) versions of these three with the same order, output, `SKIP_*`
variables and exit codes, plus the project's `checks_extra` /
`checks_extra_pre_commit` ([docs/CONFIGURATION.md](../docs/CONFIGURATION.md));
`REBAR_CHECK_IMPL=shell` runs the scripts. A project's own customized copy of
a runner, or of a check a runner calls, still runs (with a deprecation notice).
`steward.sh` has a native version too (`rebar run steward`, and the Steward
step of `rebar check`): same arguments, output and report files, no jq needed.

## Session Utilities

| Script | Notes |
|--------|-------|
| `inbox-watch.sh` | Long-running peer-inbox watcher (runs until killed) — armed at coordination-seat cold start per `practices/session-lifecycle.md` step 4; NOT a 0/1 check, never wire into CI |
| `rebar-doc.sh` | Resolver of record for `rebar:<kind>/<name>` refs (`conventions.md` §Cross-Repo References): local → `$REBAR_ROOT` → discovered checkout → upstream URL (exit 4). `--cat` streams the doc. Go twin: `rebar doc` |
| `refresh-context.sh` | QUICKCONTEXT / TODO freshness, commits since the last sync, untracked TODO comments, worktrees; `--quick`, `--test-baseline` |
| `agent-health.sh` | `source` it in a worktree agent's shell for `agent_checkpoint` / `agent_heartbeat` / `agent_metric` / `agent_rmse`; run as a command it runs one (`agent-health.sh checkpoint "msg"`) |
| `scan-consumers.sh` / `flush-notifications.sh` | Federation: which repos consume a contract (`REBAR_REPOS` or the ASK registry), and dispatching the outbox's pending change notices through `ask <consumer>:featurerequest` (`practices/federation-outbox.md`) |

`rebar run <name>` runs native (Go) versions of all of these with the same
arguments, output and exit codes (`REBAR_CHECK_IMPL=shell` runs the scripts).

## Installation

Adopting projects don't copy these scripts: `rebar check` and `rebar run
<script>` run them from the rebar install against the project, and the
project's `scripts/` holds only the two thin entry points from
`templates/project-bootstrap/scripts/`. See [SETUP.md](../SETUP.md#install-rebar)
(and [§ Run rebar in CI](../SETUP.md#run-rebar-in-ci)); projects with older
copies: [UPGRADING.md](../UPGRADING.md).

In this repository:

```bash
# Pre-commit hook
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit

# CI pipeline (GitHub Actions example)
# - run: ./scripts/ci-check.sh --strict
```

## Configuration

`.rebarrc` keys: [docs/CONFIGURATION.md](../docs/CONFIGURATION.md). Environment
variables:

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
| `SKIP_JTBD` | `0` | Skip JTBD-presence contract check |
| `SKIP_PREFIX_UNIQUENESS` | `0` | Skip contract prefix-number uniqueness check |
| `SKIP_BOOTSTRAP_SYNC` | `0` | Skip templates/scripts drift check (rebar-source-only) |
| `SKIP_SCRIPT_TESTS` | `0` | Skip the shell regression tests, `scripts/tests/run.sh` (rebar-source-only) |
| `SKIP_STEWARD` | `0` | Skip steward scan |
| `SKIP_COMPLIANCE` | `0` | Skip rebar compliance check |
| `SKIP_FIX_COMMIT` | `0` | Skip Gate G (HEAD's `fix:`/`regression:` needs `Reproduced on:`) |
| `SKIP_BYPASS_FLAGS` | `0` | Skip Gate I (HEAD's bypass mentions need `Bypass tickets:`) |
| `SKIP_CHECKS_EXTRA` | `0` | Skip the project's own `checks_extra` / `checks_extra_pre_commit` (native suites only) |

### Skipped directories (`.rebarrc`)

check-todos, check-contract-refs and check-registry skip the directory names in
`source_exclude_dirs` (default `node_modules vendor dist`) anywhere in the
tree, plus `.git` and `.claude` (agent worktrees) always. Names are matched
with `grep --exclude-dir`, never as substrings of a path or line, so
`.github/`, `dist-esm/` or `distance.go` are scanned. compute-registry uses
`registry_exclude_dirs` (same default) for implementing-file counts.

### Tiers and `checks_enable` (`.rebarrc`)

`tier` (env `REBAR_TIER`, default 3) gates these checks; every other check
runs at every tier, including the registry check (`compute-registry.sh --check`).

| Check | Below its tier | From its tier |
|-------|----------------|---------------|
| `check-contract-headers` | tier 1: SKIP | tier 2+ |
| `check-freshness` | tier 1: SKIP | tier 2+ |
| `check-jtbd-presence` | tier 1: SKIP | tier 2: advisory, tier 3: blocking |
| `check-prefix-uniqueness` | tier 1: SKIP | tier 2: advisory, tier 3: blocking |
| `check-ground-truth` | tiers 1-2: SKIP | tier 3 |
| `check-compliance` | tier 1: AGENTS.md sections not checked | tier 2+ |

`checks_enable` (env `REBAR_CHECKS_ENABLE`) runs named checks regardless of the
tier, each as it would run at tier 3 (so a named jtbd or prefix check blocks):

```
tier = 1
checks_enable = check-ground-truth check-contract-headers
```

Names are the check script basenames without `.sh`, as in `rebar run <name>`:
`check-bypass-flags check-compliance check-contract-headers check-contract-refs
check-decay-patterns check-doc-refs check-fix-commit check-freshness
check-ground-truth check-jtbd-presence check-prefix-uniqueness check-todos
compute-registry steward sync-bootstrap`. Naming an ungated check is allowed and
changes nothing. An unknown name prints one `rebar: WARN:` line on stderr per run
and is ignored; it never fails the run. `rebar status` lists the enabled checks;
`rebar audit --all` shows them in the tier column. Unset, tiers behave as before.

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
- **jq** — required by the steward.sh script (the native steward that `rebar
  run steward` and `rebar check` use needs none)
- **grep, find** — standard Unix tools
- **node** (optional) — only for `templates/scripts/check-tag-ci-coverage.mjs`,
  which is project-specific and not in the universal `ci-check.sh` flow

## Maintainer-facing tools

| Script | What it does |
|--------|--------------|
| `package-release.sh <version>` | Build the release binaries (`rebar_<ver>_<os>_<arch>`, self-contained, one per platform) and `rebar_<ver>_checksums.txt`; what `.github/workflows/release.yml` runs on a `v*` tag. `--targets`, `--out`, `--ref`, `--repo` |
| `release-notes.sh <tag>` | Print CHANGELOG.md's `## <tag>` section, the GitHub Release body `.github/workflows/release.yml` publishes (it fails a tag without one). `--link-base URL` makes relative links absolute |
| `sync-bootstrap.sh` (+ `--check`) | Keep `templates/project-bootstrap/scripts/` down to the thin entry points and refresh the binary's embedded copy of the template and `bin/` companions (run it after editing them); caught by ci-check and `go test` |
| `test-e2e-live.sh` | End-to-end smoke against a live LLM. Gated on `claude` CLI; per-repo gates skip cleanly when adopted repos aren't on disk. Includes version triple-check (binary / `.rebar-version` / latest tag), MCP server discovery + tools/list smoke, ASK HTTP server smoke (if running), and 4 live LLM keyword-acceptance queries (rebar:steward, rebar:architect, local-mode, cross-repo). Run after every meaningful update — single command answer to "is this still working?" |

## Source-of-truth structure

`/scripts/` is the only copy. Adopting projects don't carry these scripts:
`rebar check` and `rebar run <script>` run them from the rebar install against
the project (`REBAR_PROJECT_ROOT`), and the project tailors them in `.rebarrc`
([docs/CONFIGURATION.md](../docs/CONFIGURATION.md)).
`templates/project-bootstrap/scripts/` holds just the two thin entry points
(`ci-check.sh`, `pre-commit.sh`) that adopters get from
`cp -r templates/project-bootstrap/.`; `sync-bootstrap.sh --check` (wired into
`ci-check.sh`) keeps anything else from creeping back in.
