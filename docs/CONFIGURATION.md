# Configuring rebar: `.rebarrc`

rebar's checks are built into the `rebar` binary (`rebar check`, `rebar run
<check>`); your repo carries no copies of them. Anything specific to your repo goes in
`.rebarrc` at the repo root: which tier you enforce, where your source and
contracts live, and how METRICS values are computed. The template
[`templates/project-bootstrap/.rebarrc`](../templates/project-bootstrap/.rebarrc)
has every key commented out with an example. Copy the lines you need.

## File format

```
# comment
tier = 2
source_dirs = api sdk cmd
```

- One `key = value` per line. Spaces around `=` are optional.
- Keys start at the beginning of the line and are case-sensitive.
- `#` starts a comment anywhere on the line, so values cannot contain `#`.
- If a key appears more than once, the **first** line wins.
- Lists are space-separated.
- **Precedence:** environment variable (where one exists) > `.rebarrc` > built-in default.
- `rebar migrate` adds its lines between `# --- rebar migrate … ---` markers
  ([UPGRADING.md](../UPGRADING.md)). You can edit or move them like any other line.

## Tier and which checks run

| Key | Env override | Default | Meaning |
|-----|--------------|---------|---------|
| `tier` | `REBAR_TIER` | `3` | Enforcement tier: `1` Partial, `2` Adopted, `3` Enforced. Values other than 1–3 are ignored. |
| `checks_enable` | `REBAR_CHECKS_ENABLE` | — | Checks to run below their tier, each as it runs at tier 3. Names are script basenames without `.sh`, as in `rebar run <name>`. Unknown names are warned about and ignored. |

`tier` gates these checks; every other check runs at every tier, including
the registry check (`compute-registry --check`).

| Check | Below its tier | From its tier |
|-------|----------------|---------------|
| `check-contract-headers` | tier 1: SKIP | tier 2+ |
| `check-freshness` | tier 1: SKIP | tier 2+ |
| `check-jtbd-presence` | tier 1: SKIP | tier 2: advisory, tier 3: blocking |
| `check-prefix-uniqueness` | tier 1: SKIP | tier 2: advisory, tier 3: blocking |
| `check-ground-truth` | tiers 1-2: SKIP | tier 3 |
| `check-compliance` | tier 1: AGENTS.md sections not checked | tier 2+ |

`checks_enable` runs named checks regardless of the tier, each as it would
run at tier 3 (so a named jtbd or prefix check blocks):

```
tier = 1
checks_enable = check-ground-truth check-contract-headers
```

Valid names: `check-bypass-flags check-compliance check-contract-headers
check-contract-refs check-decay-patterns check-doc-refs check-fix-commit
check-freshness check-ground-truth check-jtbd-presence check-prefix-uniqueness
check-todos compute-registry steward sync-bootstrap` (a `.sh` suffix is
accepted). Naming an ungated check is allowed and changes nothing. An unknown
name prints one `rebar: WARN:` line on stderr per run and is ignored; it never
fails the run. `rebar status` shows the tier and the enabled checks; `rebar
audit --all` shows them in the tier column.

## Your own checks: `checks_extra`

| Key | Env override | Default | Meaning |
|-----|--------------|---------|---------|
| `checks_extra` | `REBAR_CHECKS_EXTRA` | — | Your own check programs, run by `rebar check` after rebar's checks. Space-separated paths, relative to the project root or absolute. |
| `checks_extra_pre_commit` | `REBAR_CHECKS_EXTRA_PRE_COMMIT` | — | The same for `rebar check --pre-commit`. List a check in both keys to run it in both suites. |

```
checks_extra = scripts/check-openapi.sh tools/check-licenses.py
checks_extra_pre_commit = scripts/check-openapi.sh
```

Each entry is an executable file in any language. It runs in the project
root with `REBAR_PROJECT_ROOT` set, after rebar's checks, and is listed in the
summary by its file name without the extension (`PASS  extra: check-openapi`).
A non-zero exit fails the suite like any check; a missing or non-executable
file is a failure, not a skip. `SKIP_CHECKS_EXTRA=1` skips them all for one
run. `rebar run check-openapi` runs one by that name (rebar's own checks keep
their names). The suites built into rebar run these; a project's own legacy
copy of `ci-check.sh` / `pre-commit.sh` does not (`rebar check` says so).

## Where things live

| Key | Env override | Default | Used by |
|-----|--------------|---------|---------|
| `contracts_dir` | `REBAR_CONTRACTS_DIR` | `architecture` | Every contract check, the registry, the steward. Holds `CONTRACT-*.md`, `CONTRACT-REGISTRY.md` and `.state/`. Other design docs can stay in `architecture/` either way. |
| `source_dirs` | — | `src internal cmd client packages lib app` | check-todos, check-contract-headers. Directories that don't exist are skipped. |
| `source_extensions` | `CONTRACT_EXTENSIONS` (headers only) | `go ts tsx js py rs jsx` | check-todos, check-contract-refs, check-contract-headers |
| `source_exclude_dirs` | — | `node_modules vendor dist` | check-todos, check-contract-refs, check-registry. Matched as directory **names** anywhere in the tree, never as substrings. `.git` and `.claude` are always skipped. |
| `freshness_max_days` | — | `14` | check-freshness: days before a `freshness:` / `last-synced:` marker counts as stale. |

## Contract namespace

| Key | Env override | Default |
|-----|--------------|---------|
| `contract_namespace` | `REBAR_CONTRACT_NAMESPACE` | unset |

When set, code can cite this repo's contracts as
`CONTRACT:<namespace>:<ID>.<v>` as well as the short `CONTRACT:<ID>.<v>`.
A reference qualified with a different namespace points at another repo and
is not checked here.
- **Allowed characters:** letters, digits, `. _ / -`.
- **Not allowed:** a value shaped like `ID.version`, such as `acme.1.2`.

Leave it unset until references cross repos. See
[conventions.md](../conventions.md).

## Contract registry

Settings for `compute-registry`, which generates
`<contracts_dir>/CONTRACT-REGISTRY.md` and checks it is current.

| Key | Default | Meaning |
|-----|---------|---------|
| `registry_impl_counts` | `false` (env `REBAR_REGISTRY_IMPL_COUNTS`) | `true` adds the Impl Files column and the Contract Health section. They change with almost every code change, which is why they are off by default ([practices/busy-repos.md](../practices/busy-repos.md)). |
| `registry_impl_globs` | `*.go *.ts *.tsx *.js *.py *.rs *.java *.rb *.jsx *.sh bin/` | The files where `CONTRACT:` headers are counted, when counts are on. |
| `registry_exclude_dirs` | `node_modules vendor dist` | Directory names left out of those counts. |
| `registry_scope_prefixes` | — | Prefixes stripped, in order, before classifying. `SDK- MCP-` files `SDK-S1-FOO` as a Service. |
| `registry_purpose_heading` | `Purpose` | Heading (ERE) the purpose is taken from, such as `Purpose\|Why this exists`. Numbered headings (`## 2. Purpose`) also match. |
| `registry_purpose_style` | `ellipsis` | `ellipsis` escapes `\|` and cuts the purpose to 77 characters plus `...`. `bytes` keeps the pre-3.0 behavior: the first 80 bytes, `\|` unescaped. |

## METRICS ground truth

`check-ground-truth` (tier 3, or listed in `checks_enable`) compares each
`METRICS` line (`key = N` for an exact value, `key >= N` for a floor) with a
value computed from `metric.<key>.*` lines. Every field is optional.

| Field | Meaning |
|-------|---------|
| `type` | `files` (default) or `dirs` |
| `paths` | Where to look, relative to the repo root (default `.`) |
| `names` | Name globs to count (default: everything) |
| `exclude_names` | Name globs to leave out |
| `exclude_dirs` | Directory names to skip anywhere (`.git` is always skipped) |
| `min_depth` / `max_depth` | As `find -mindepth` / `-maxdepth`, relative to each path |
| `contains` | ERE; count only files whose content matches |
| `distinct` | ERE with one group, matched against each name; counts the distinct values of the group |
| `tracked` | `true`: count only files git tracks (`git ls-files`), and for `type = dirs` only directories holding a tracked file, so gitignored or untracked local files don't change the count. Outside a git work tree it has no effect: every file counts, with a note on stderr. |

```
metric.test_files.paths        = api sdk
metric.test_files.names        = *_test.go *.test.ts
metric.test_files.exclude_dirs = node_modules vendor dist
metric.contract_ids.paths      = architecture
metric.contract_ids.max_depth  = 1
metric.contract_ids.names      = CONTRACT-*.md
metric.contract_ids.exclude_names = CONTRACT-TEMPLATE.md CONTRACT-REGISTRY.md
metric.contract_ids.distinct   = ^CONTRACT-(.*)\.[0-9]+\.[0-9]+\.md$
metric.shell_scripts.paths     = scripts
metric.shell_scripts.names     = *.sh
metric.shell_scripts.tracked   = true
```

Check the computed values with `rebar run check-ground-truth`.

| Key | Env override | Default | Meaning |
|-----|--------------|---------|---------|
| `metrics_strict` | `REBAR_METRICS_STRICT` | `false` | `true` fails the check when there is no `METRICS` file (otherwise SKIP), when a declared metric has no `METRICS` line (otherwise reported as NEW), and when `.rebarrc` declares no metrics at all. `rebar migrate` proposes it when your old copy of the check failed in both cases. |

## What `rebar check` runs

`rebar check` is the full suite (CI), `rebar check --pre-commit` the fast
checks for the git hook (TODOs, contract references, ground truth at its
tier), `rebar check --cold-start` the advisory session-start summary (always
exits 0, wrapped in `<rebar-cold-start>`). Each check also runs alone with
`rebar run <name>`; `rebar run` lists them.

| Check | What it checks |
|-------|----------------|
| `check-contract-headers` | Every source file has a `CONTRACT:` header |
| `check-contract-refs` | Every `CONTRACT:` reference points to a real contract file |
| `check-doc-refs` | Every `[text](path)` link in tracked `*.md` resolves to a tracked file |
| `check-todos` | No untracked `TODO:` comments (two-tag system) |
| `check-freshness` | `freshness:` / `last-synced:` markers aren't stale |
| `compute-registry --check` | `CONTRACT-REGISTRY.md` is current (`rebar run compute-registry` regenerates it) |
| `check-jtbd-presence` | Latest-version contracts carry non-empty Why/Who/Scenarios sections |
| `check-prefix-uniqueness` | No prefix number (S1, I3, …) claimed by two contract IDs |
| `check-ground-truth` | METRICS matches the values computed from `metric.<key>.*` |
| `check-compliance` | README badge, tier and AGENTS.md sections are consistent |
| `check-decay-patterns` | Soft-hardening anti-patterns in spec and test files |
| `check-fix-commit` | Gate G: HEAD's `fix:`/`regression:` commit has `Reproduced on:` |
| `check-bypass-flags` | Gate I: HEAD's bypass mentions carry `Bypass tickets:` |
| `steward` | Contract lifecycle, enforcement and discoveries → `architecture/.state/`, `STEWARD_REPORT.md` |
| Bootstrap Sync, Script Tests | rebar's own source only (`scripts/sync-bootstrap.sh --check`, `scripts/tests/run.sh`); skipped in other projects |

Utilities (`rebar run <name>`, not part of a suite): `refresh-context`
(QUICKCONTEXT/TODO freshness; `--quick`, `--test-baseline`), `inbox-watch`
(long-running peer-inbox watcher; never wire it into CI), `agent-health`
(`checkpoint|heartbeat|metric|rmse` for worktree agents), `rebar-doc`
(resolver for `rebar:<kind>/<name>` refs; also `rebar doc`), `scan-consumers`
and `flush-notifications` (federation, [practices/federation-outbox.md](../practices/federation-outbox.md)),
`check-version-bump` (post-commit contract-version notices).

A project's own copy of one of rebar's old scripts (`scripts/check-todos.sh`
and the like, from before rebar was a single binary) still runs instead of
the built-in check, with a deprecation notice, from `rebar run` and from the
suites: it is your code. `rebar migrate` replaces the copies it can verify.

### Skipped directories

check-todos, check-contract-refs and check-registry skip the directory names
in `source_exclude_dirs` (default `node_modules vendor dist`) anywhere in the
tree, plus `.git` and `.claude` (agent worktrees) always. Names are matched as
`grep --exclude-dir` matched them, never as substrings of a path or line, so
`.github/`, `dist-esm/` or `distance.go` are scanned. compute-registry uses
`registry_exclude_dirs` (same default) for implementing-file counts.

## Skipping a check for one run

Skips are environment variables, not `.rebarrc` keys, for example
`SKIP_STEWARD=1 rebar check`. A check is skipped only when its variable is
exactly `1`.

| Variable | Skips |
|----------|-------|
| `SKIP_CONTRACT_HEADERS` | check-contract-headers |
| `SKIP_CONTRACT_REFS` | check-contract-refs |
| `SKIP_DOC_REFS` | check-doc-refs |
| `SKIP_TODOS` | check-todos |
| `SKIP_FRESHNESS` | check-freshness |
| `SKIP_REGISTRY` | the registry check |
| `SKIP_JTBD` | check-jtbd-presence |
| `SKIP_PREFIX_UNIQUENESS` | check-prefix-uniqueness |
| `SKIP_GROUND_TRUTH` | check-ground-truth |
| `SKIP_COMPLIANCE` | check-compliance |
| `SKIP_DECAY_PATTERNS` | check-decay-patterns |
| `SKIP_BOOTSTRAP_SYNC` | the template drift check (rebar source only) |
| `SKIP_SCRIPT_TESTS` | the shell regression tests (rebar source only) |
| `SKIP_FIX_COMMIT` | Gate G |
| `SKIP_BYPASS_FLAGS` | Gate I |
| `SKIP_STEWARD` | the steward scan |
| `SKIP_CHECKS_EXTRA` | every `checks_extra` / `checks_extra_pre_commit` entry |

`CONTRACT_EXTENSIONS` (default `.go .ts .tsx .js .jsx .py .rs`) overrides the
file extensions check-contract-headers scans.

## Moving settings out of copied scripts

Older adopters customized rebar by editing their copies of its scripts.
`rebar migrate` moves those edits into the `.rebarrc` keys above, then removes
or wraps each copy once it has verified that the built-in check behaves the same:
[UPGRADING.md](../UPGRADING.md).
