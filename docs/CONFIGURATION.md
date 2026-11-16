# Configuring rebar: `.rebarrc`

rebar's checks run from the rebar install (`rebar check`, `rebar run <check>`),
not from copies in your repo. Anything specific to your repo goes in
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

The checks each tier gates:
[scripts/README.md § Tiers and `checks_enable`](../scripts/README.md#tiers-and-checks_enable-rebarrc).
`rebar status` shows the tier and the enabled checks.

```
tier = 1
checks_enable = check-ground-truth check-contract-headers
```

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

## Skipping a check for one run

Skips are environment variables, not `.rebarrc` keys. For example,
`SKIP_STEWARD=1 rebar check`. The full list is in
[scripts/README.md § Configuration](../scripts/README.md#configuration).

## Moving settings out of copied scripts

Older adopters customized rebar by editing their copies of its scripts.
`rebar migrate` moves those edits into the `.rebarrc` keys above, then removes
or wraps each copy once it has verified that the install behaves the same:
[UPGRADING.md](../UPGRADING.md).
