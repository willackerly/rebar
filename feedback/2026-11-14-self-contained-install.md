# Self-contained install: adopters carry configuration, not rebar's scripts

**Source:** willackerly/rebar#8 (`f/namespace-no-scripts`, "reduce scripts in
the user repo"), virtru-analyze as the acceptance adopter, 2026-11-14
**Type:** feature (propose-then-ship; implementation in the same branch)
**Status:** implemented on `feat/thin-adoption-scripts`; open questions below

## Problem

Every adopter copies ~25 scripts from `templates/project-bootstrap/scripts/`
into its own `scripts/` and then edits them:

- `check-ground-truth.sh` ships with `compute_metrics()` marked
  CUSTOMIZATION REQUIRED, so METRICS cannot work without editing rebar's code.
- virtru-analyze forked `compute-registry.sh` (scope prefixes, purpose
  heading, locale) and `check-todos.sh` / `check-contract-refs.sh` (source
  roots, `.sh` files).
- Fixes in rebar (the #9 merge-friendly registry, the `>=` METRICS floors)
  reach an adopter only if someone re-copies the scripts and re-applies the
  local edits. In practice the copies drift: analyze's header still says
  "Based on REBAR v2.0.0".

PR #8 started to remove the copies but was built on an older main, bundled
an installer rewrite and contract namespaces, and had three functional gaps
(see "What came from #8").

## Goal

All rebar logic lives in the rebar install (the CLI plus the `scripts/` of
the install clone). An adopting repo carries **configuration only**:
`.rebarrc`, `METRICS`, its contracts, and at most two trivial entry points
that call the installed `rebar`.

## Design

### 1. The install executes, the project configures

`rebar check`, `rebar run <script>`, `rebar contract verify`, `rebar commit`
and `rebar push` resolve each script the same way
(`cli/cmd/scripts.go: resolveScript`):

1. the project's own `scripts/<name>` if it is a full copy (legacy; prints a
   deprecation notice), else
2. `<rebar install>/scripts/<name>`.

The script runs with the project root as working directory and
`REBAR_PROJECT_ROOT=<project>`. Every script that used to derive its root
from its own location (`scripts/..`) honors that variable, as do the
`.rebarrc` readers in `_rebar-config.sh`. Without the variable nothing
changes, so rebar's own CI and legacy copies behave exactly as before.

Inside the suite, `ci-check.sh` / `pre-commit.sh` / `cold-start-checks.sh` /
`steward.sh` pick each check through `_rebar_script`, which applies the same
rule: a project copy of a single check (say a customized
`check-ground-truth.sh`) keeps running, with the deprecation notice, until
its settings move to `.rebarrc`.

### 2. Entry points

`templates/project-bootstrap/scripts/` now holds:

| File | Body |
|------|------|
| `ci-check.sh` | `exec rebar check "$@"` (exit 1 if rebar is missing) |
| `pre-commit.sh` | `exec rebar check --pre-commit` (skips if rebar is missing) |
| `README.md` | what they are, how to configure, how to migrate |

Both carry `# rebar-thin-wrapper` on a line of its own. The CLI never runs a
marked file, so `ci-check.sh → rebar check → ci-check.sh` cannot loop.
`sync-bootstrap.sh --check` (in rebar's ci-check) fails if anything else
appears in the template's `scripts/`, or a wrapper loses its marker.

The SessionStart hook calls `rebar check --cold-start` directly, falling back
to a legacy `scripts/cold-start-checks.sh`. Everything else is reachable as
`rebar run <script>` (`rebar run` with no arguments lists them); template
docs and skills were rewritten to say so.

### 3. Configuration schema (`.rebarrc`)

One flat `key = value` file, read by `_rebar_config` (bash) and
`config.RCValue` (Go) with the same rules: the first line that starts with the
key wins, `#` starts a comment, env overrides where noted. Structured
settings use dotted keys. All keys are optional; defaults equal the old
behavior.

| Key | Default | Used by |
|-----|---------|---------|
| `contracts_dir` (env `REBAR_CONTRACTS_DIR`) | `architecture` | every contract-aware script and CLI path; see §4 |
| `source_dirs` | `src internal cmd client packages lib app` | check-todos, check-contract-headers |
| `source_extensions` (env `CONTRACT_EXTENSIONS` for headers) | `go ts tsx js py rs jsx` | check-todos, check-contract-refs, check-contract-headers |
| `freshness_max_days` | `14` | check-freshness |
| `registry_impl_counts` (env `REBAR_REGISTRY_IMPL_COUNTS`) | `true` | compute-registry (from #9) |
| `registry_scope_prefixes` | none | compute-registry: stripped before Services/Components/… classification |
| `registry_purpose_heading` | `## Purpose` only | compute-registry: heading text (ERE); numbered headings also match |
| `registry_purpose_style` | `bytes` (first 80 bytes) | `ellipsis`: escape `\|`, cut to 77 chars + `...` |
| `registry_impl_globs` | `*.go *.ts *.tsx *.js *.py *.rs *.java *.rb *.jsx *.sh bin/` | compute-registry: where CONTRACT: headers count |
| `registry_exclude_dirs` | `node_modules vendor dist` | compute-registry |
| `metric.<key>.<field>` | none | check-ground-truth; see below |

**Declarative metrics.** Each METRICS key is computed from a group of
`metric.<key>.*` lines instead of shell in `compute_metrics()`:

| Field | Meaning |
|-------|---------|
| `type` | `files` (default) or `dirs` |
| `paths` | where to look, relative to the repo root (default `.`; missing paths count 0) |
| `names` / `exclude_names` | name globs to count / leave out |
| `exclude_dirs` | directory names pruned anywhere (`.git` always) |
| `min_depth` / `max_depth` | as `find` |
| `contains` | ERE; count only files whose content matches |
| `distinct` | ERE with one group matched against each name; count distinct group values (a non-matching name counts as itself, matching `sed` semantics) |

METRICS keeps `key = N` (exact) and `key >= N` (floor) from #9 unchanged.

### 4. `contracts_dir`

Names the directory holding `CONTRACT-*.md`, `CONTRACT-REGISTRY.md` and the
steward's `.state/` (including the federation outbox). It names the
**contracts location**, not "the architecture folder": several repos keep
design notes, diagrams and ADRs in `architecture/` alongside or instead of
contracts, and with `contracts_dir = specs` those are simply ignored
(including a stray `CONTRACT-*.md` left behind there).

Honored by compute-registry (registry location, and the directory left out
of implementing-file counts), check-registry, check-contract-refs,
check-jtbd-presence, check-prefix-uniqueness, check-compliance,
check-version-bump, flush-notifications, steward, cold-start-checks; in Go by
`config.Config.ContractsDir`, the integrity scanner, audit, context, agent
role permissions, status, contract drift-check (owner repo's own setting),
new/init/adopt; and by `agents/*/commands/default.sh` and `ask`'s project
summary. Only relative paths inside the repo are accepted.

## Evidence [M]

**virtru-analyze, acceptance run** (scratch clone of
`chore/rebar-merge-friendly`, user checkout untouched). With the `.rebarrc`
in the appendix and analyze's four rebar-derived scripts deleted:

- `rebar run check-ground-truth` (install engine) reports the same 13 values
  as analyze's own `scripts/check-ground-truth.sh`: sdk_languages 2,
  mcp_servers 6, helm_mcp_servers 4, helm_support_services 2, go_modules 17,
  mcp_shared_packages 7, contract_files 71, contract_ids 56,
  source_files_with_contract_headers 214, test_files 353, shell_scripts 16
  (12 once the four scripts are gone), role_agents 6, ci_workflows 36.
  Raising `test_files >= 354` fails with DRIFT.
- `compute-registry.sh --stdout` from the install is **byte-identical** to
  analyze's committed `architecture/CONTRACT-REGISTRY.md` with three keys
  (scope prefixes, purpose heading, ellipsis); without them 140 lines differ.
- `--health` matches except `SDK-A1-AUTH-PROVIDER` (24 vs 20): analyze counts
  four tracked `sdk/webauth/dist/*.js` build outputs; rebar's default
  `registry_exclude_dirs` skips `dist/`. Committed registry unaffected
  (`registry_impl_counts = false`).
- `rebar check --pre-commit` passes (0 untracked TODOs; 751 contract refs,
  0 broken; all 13 metrics OK).

**Every metric analyze computes was expressible declaratively.** None needed
the escape hatch discussed below.

## What came from #8

| #8 commit | Taken | Left / reworked |
|-----------|-------|-----------------|
| 708ba42 installer | the 23 template deletions bundled in it | the installer rewrite (versioned dirs, binary download) — separate change |
| 8c66877 thin wrappers | the model, README intent | `ci-check.sh` called `rebar audit`, which prints a score and **always exits 0**: CI could never fail. Now `rebar check`. |
| b9ea3ae / eab6c56 namespaces | — | separate change |
| 7b413a2 pre-commit from install | running checks from the install; the root-marker fix | it ran install scripts with the **install as working directory**, so they checked rebar, not the project; it dropped ground truth from pre-commit; `findRebarRoot` keyed on `setup-rebar.sh` (now `templates/project-bootstrap/` + `scripts/steward.sh`) |

It also left `cold-start-checks.sh` in the template, which would have printed
`[MISS]` for every check once the others were gone, and left ~40 template doc
and skill references to deleted scripts.

## Migration for existing adopters

Nothing breaks on upgrade: relaxation, not migration.

- A full project `scripts/ci-check.sh` (no marker) still runs as before
  under `rebar check`, with a deprecation notice; so do individual check
  copies picked by the install suite.
- To migrate: express customizations in `.rebarrc` (the appendix is a worked
  example), replace `ci-check.sh` / `pre-commit.sh` with the template's two
  files (or call `rebar check` from CI directly), delete the rest. A
  `METRICS` key like `shell_scripts` will drop accordingly.
- Hooks: `.git/hooks/pre-commit → ../../scripts/pre-commit.sh` keeps
  working; the target just gets shorter.

Known differences an adopter may hit when switching to `rebar check`:

1. **Tier gates apply.** The install's check-ground-truth is Tier 3 only,
   contract headers / registry / jtbd Tier 2+. virtru-analyze is `tier = 1`
   but runs ground truth and registry through its own `rebar-ci.sh`, whose
   copies have no gate; under the install they would SKIP. (Open question 1.)
2. **More checks.** `rebar check` runs the full 15-check suite; analyze's CI
   runs 5. At tier 1 analyze fails doc-refs (10 untracked link targets) and
   compliance (README badge says v2.0.0) — real findings, but new ones.
   Calling `rebar run <check>` per check keeps today's scope.
3. **Version skew.** The wrappers call whichever `rebar` is on PATH, and the
   CLI runs the scripts of the install it finds (`REBAR_ROOT`, its own
   `../`, then `~/.rebar`, …). A stale `~/.rebar` clone next to a newer
   binary runs old scripts; `rebar version` and `REBAR_ROOT` disambiguate.
4. **Standalone runs.** `~/.rebar/scripts/check-todos.sh` run by hand from a
   project reads `~/.rebar/.rebarrc`, not the project's. Use
   `rebar run check-todos` (sets `REBAR_PROJECT_ROOT`).

## Red team

- *A wrapper that calls `rebar check`, which runs `ci-check.sh` …* — marker
  on its own line; `resolveScript` skips it; Go test covers it, plus a test
  that a script merely *mentioning* the marker is not treated as one.
- *Honest-answer lens (working agreement 06).* A fresh template project
  fails the same three checks on main and on this branch (doc-refs on
  `../DESIGN.md`, missing registry, badge version) — parity, measured. An
  adopter with zero metrics declared still passes ("No metrics declared").
- *Self-reference.* The template `.rebarrc` documents `metric.*` keys only
  in comments; the parser matches `^metric\.` so the examples are inert. The
  new tests build `CONTRACT:` references from parts so rebar's own registry
  scan does not count them (it did on the first try: the registry went
  STALE with a shadow `S1-FOO`).
- *`#` in values.* Comments are stripped, so a `contains` regex cannot use
  `#`. Documented; no current metric needs it.
- *Deprecation noise.* Legacy adopters see one stderr line per legacy
  script per run. Loud but not blocking (agreement 05).
- *`contracts_dir` pointing outside the repo.* Rejected in Go (falls back to
  `architecture`); bash takes the value as given and finds nothing.

## Open questions (lean in brackets)

1. **Per-check opt-in independent of tier?** analyze wants ground truth and
   registry at tier 1. Options: `checks = ground-truth registry` in
   `.rebarrc` that force-enables named checks, or tell adopters to raise
   the tier. [Lean: add `checks_enable =` as a follow-up; it is the one gap
   the acceptance run hit.]
2. **Escape hatch for metrics the schema cannot express.** None found in
   analyze. Candidates: `metric.<key>.command = <shell>` (arbitrary shell
   in config — exactly what we are removing), or a constrained
   `metric.<key>.exists = path1 path2` for existence counts. [Lean: no
   escape hatch until an adopter needs one; `names` + `paths` already covers
   existence counts like analyze's helm templates.]
3. **Make `ellipsis` the registry default?** It is strictly better (`|`
   escaped, no split UTF-8), but changing the default regenerates every
   adopter's registry. [Lean: flip it in the next breaking release.]
4. **Drop the per-check project override?** It exists only for migration.
   [Lean: keep through v3.x with the notice; remove in v4.]
5. **rebar's own METRICS is unverified.** rebar's `compute_metrics` was the
   template no-op, so its METRICS (e.g. `practice_docs = 8`, actual 18) never
   ran. Now it can be declared in rebar's `.rebarrc`. [Lean: separate PR
   that declares the metrics and fixes the numbers.]
6. **Contract namespace / versioned installer from #8.** Out of scope here.
   [Lean: separate PRs rebased on this one.]

## Appendix — virtru-analyze `.rebarrc`

```
# REBAR Configuration
tier = 1                      # see open question 1; the acceptance run used 3
registry_impl_counts = false
registry_scope_prefixes = SDK- MCP-
registry_purpose_heading = Purpose|Why this exists
registry_purpose_style = ellipsis
registry_impl_globs = *.go *.ts *.tsx *.js *.jsx *.py *.rs *.java *.rb *.sh

source_dirs       = sdk mcp e2e examples charts scripts
source_extensions = go ts tsx js jsx py rs sh

metric.sdk_languages.type      = dirs
metric.sdk_languages.paths     = sdk
metric.sdk_languages.min_depth = 1
metric.sdk_languages.max_depth = 1
metric.sdk_languages.names     = datalake-sdk-go datalake-sdk-web

metric.mcp_servers.type      = dirs
metric.mcp_servers.paths     = mcp
metric.mcp_servers.min_depth = 1
metric.mcp_servers.max_depth = 1
metric.mcp_servers.names     = *-server

metric.helm_mcp_servers.paths     = charts/mcp-servers/templates
metric.helm_mcp_servers.max_depth = 1
metric.helm_mcp_servers.names     = deployment-embed.yaml deployment-iceberg.yaml deployment-secure-postgres.yaml deployment-trino.yaml

metric.helm_support_services.paths     = charts/mcp-servers/templates
metric.helm_support_services.max_depth = 1
metric.helm_support_services.names     = deployment-ollama.yaml deployment-tei.yaml

metric.go_modules.names        = go.mod
metric.go_modules.exclude_dirs = vendor node_modules

metric.mcp_shared_packages.type      = dirs
metric.mcp_shared_packages.paths     = mcp
metric.mcp_shared_packages.min_depth = 1
metric.mcp_shared_packages.max_depth = 1
metric.mcp_shared_packages.names     = bedrock config mcptypes ollama sqlrewrite tei tenancy

metric.contract_files.paths         = architecture
metric.contract_files.max_depth     = 1
metric.contract_files.names         = CONTRACT-*.md
metric.contract_files.exclude_names = CONTRACT-TEMPLATE.md CONTRACT-REGISTRY.md CONTRACT-REGISTRY.template.md *.impl.md

metric.contract_ids.paths         = architecture
metric.contract_ids.max_depth     = 1
metric.contract_ids.names         = CONTRACT-*.md
metric.contract_ids.exclude_names = CONTRACT-TEMPLATE.md CONTRACT-REGISTRY.md
metric.contract_ids.distinct      = ^CONTRACT-(.*)\.[0-9]+\.[0-9]+\.md$

metric.source_files_with_contract_headers.paths        = sdk mcp e2e examples charts
metric.source_files_with_contract_headers.names        = *.go *.ts *.tsx *.js *.jsx *.py *.rs
metric.source_files_with_contract_headers.exclude_dirs = node_modules vendor dist
metric.source_files_with_contract_headers.contains     = CONTRACT:[A-Za-z0-9_-]*\.[0-9]*\.[0-9]*

metric.test_files.paths        = api dcr-api sdk mcp e2e examples charts
metric.test_files.names        = *_test.go *.test.ts *.test.tsx *.test.js *.spec.ts *.spec.tsx test_*.py *_test.py
metric.test_files.exclude_dirs = node_modules vendor dist

metric.shell_scripts.paths     = scripts
metric.shell_scripts.max_depth = 1
metric.shell_scripts.names     = *.sh

metric.role_agents.paths     = agents
metric.role_agents.min_depth = 2
metric.role_agents.max_depth = 2
metric.role_agents.names     = AGENT.md

metric.ci_workflows.paths     = .github/workflows
metric.ci_workflows.max_depth = 1
metric.ci_workflows.names     = *.yml *.yaml
```

## Disposition

Implemented on `feat/thin-adoption-scripts` in the commits that follow this one. Open
questions await maintainer triage; close into `feedback/processed/` once
answered.
