# `rebar migrate`: legacy script copies become `.rebarrc` settings, verified by running them

**Source:** follow-up to `feedback/2026-11-14-self-contained-install.md`
(#14), virtru-analyze as the acceptance adopter, 2026-11-14
**Type:** feature (addendum to #14; implementation in the same branch)
**Status:** implemented on `feat/rebar-migrate` (stacked on
`feat/thin-adoption-scripts`); open questions below

## Problem

#14 moved every check into the rebar install and made repo-specific
behavior a `.rebarrc` setting. An existing adopter still has ~25 copied
scripts, some customized, and the per-script override keeps running those
copies (with a deprecation notice) until someone moves the customizations
into `.rebarrc` by hand and deletes them. #14's appendix shows what that
took for virtru-analyze: reading four scripts, writing ~70 lines of
`metric.*` and registry settings, and checking 13 values and a registry
byte for byte. The override is due to go in v4; adopters need a way across
that does not depend on someone reading shell carefully.

## Goal

One command that, for each legacy copy, decides **by behavior** whether the
install (with derived settings) does the same thing, writes the settings,
deletes only what it verified, and says plainly what it could not move.

## Design

### Which files are legacy copies

The same rule the override uses (`cli/cmd/scripts.go: resolveScript`,
`_rebar_script`): a regular file `scripts/<name>.sh` in the project for which
the install has `scripts/<name>.sh`, unless it carries `# rebar-thin-wrapper`.
Symlinks are skipped. `_rebar-config.sh` is included (only copies source it),
and the old template's `scripts/README.md` when it is byte-identical to a
released one. The project's own scripts (analyze's `rebar-ci.sh`,
`deploy.sh`) are never touched. Running in the rebar repo itself is an error.

### How each copy is decided

Every probe runs with `REBAR_TIER=3` (so tier gates don't hide behavior),
`REBAR_PROJECT_ROOT=<project>`, stdin closed, a per-run timeout and its own
process group (killed on timeout). Candidate settings are tried through
`REBAR_RCFILE` (new: points `_rebar_config` at a temp file holding the
project's `.rebarrc` plus the candidate block), so the dry run never writes
the project.

| Copy | Decided by |
|------|------------|
| `check-ground-truth.sh` | metric values: legacy (its `compute_metrics()`) vs install (`--print`, new) with translated `metric.<key>.*` lines, plus the same exit status of the full check |
| `compute-registry.sh` | `--stdout` byte-identical with the smallest set of derived settings; `--health` compared as advisory |
| read-only checks (`check-todos`, `check-contract-refs`, `check-contract-headers`, `check-freshness`, `check-jtbd-presence`, `check-prefix-uniqueness`, `check-registry`, `check-doc-refs`, `check-compliance`, `check-decay-patterns --all`, `check-fix-commit`, `check-bypass-flags`, `refresh-context`) | same exit status and same output (or the same findings and counts with different wording) on the project; the three source scanners also on a **probe tree** |
| everything else (`steward`, `inbox-watch`, `flush-notifications`, `scan-consumers`, `rebar-doc`, `agent-health`, `check-version-bump`, `cold-start-checks`, `ci-check`, `pre-commit`) | not safe to run (writes state, runs forever, needs arguments): deleted only when byte-identical to a blob rebar has shipped (git history of the install); `ci-check.sh` / `pre-commit.sh` are **replaced** by the thin wrappers so CI and hook paths keep working |
| `_rebar-config.sh` | unmodified, or reports the same `_rebar_tier` / `_rebar_contracts_dir` as the install's |

**Ground truth.** Values come from a harness that sources the copy with its
output discarded and calls `compute_metrics()` from an EXIT trap, so both
guarded copies (analyze) and copies that verify at top level and `exit`
(the old template) yield their values; `$0` is set beside the copy so
`$(dirname "$0")/..` is the project. Fallback: the OK/DRIFT lines of a
normal run. The translator (`cli/internal/migrate/translate.go`) reads
`compute_metrics()` and the helpers it calls and understands:

- `find PATHS [-mindepth/-maxdepth] [-type f|d] -name / ( -name … -o … ) / ! -name / -not -name / ! -path '*/D/*' / -name D -prune -o … [-print]`
- `-exec grep -l|-q [-E|-F] PAT {} +|\;` → `contains` (BRE converted to ERE)
- `-exec basename {} \;` / `-printf '%f\n'` / `| xargs basename` / `| sed 's|.*/||'`, then `| sed [-E] 's/RE/\1/' | sort -u` → `distinct`
- `grep -rl [-E] PAT PATHS --include=G --exclude-dir=D` and `ls DIR/GLOB…`
- `| wc -l` with `tr -d ' '`, `xargs`, `awk '{print $1}'`; `2>/dev/null`
- helpers with positional arguments, `VAR=$(…)` then `echo "k=$VAR"`
- existence loops: `count=0; for x in a b; do [ -d|-f "$ROOT/dir/$x" ] && count=$((count + 1)); done; echo "$count"` → `paths = dir`, `names = a b`, depth 1
- root variables: anything assigned from `REBAR_PROJECT_ROOT`, `git rev-parse --show-toplevel`, or `..` of the script directory; `$SCRIPT_DIR/..` resolves

Anything else (a pipe through `grep -v`, `cat | wc -l`, arithmetic,
`-type l`, `-iname`, a top-level `-o` mixing tests, a value with `#`) is
refused with the reason. A metric is accepted only when the install,
with its translated lines, computes the **same value** as the copy on the
project; a translation looser than the original (find without `-type`, `ls`
of a glob) carries a note in the report. There is no escape hatch: per
#14 Q2, metrics stay declarative. The copy is deleted only if every metric
it computes is accepted, the install would not also check extra metrics,
and the full check exits the same.

**Registry.** Candidates come from the copy: scope prefixes stripped in its
classifier (`${id#SDK-}`), the purpose heading alternation in its awk
pattern, its `git grep` pathspecs, a non-default contracts directory, and
`registry_impl_counts = false`, plus `registry_purpose_style = bytes`.
Subsets are tried smallest first, so the result is the smallest set that
makes `--stdout` byte-identical (often none), and a candidate that would
break the output (say `registry_impl_counts = false` for a copy that shows
counts) is never kept; then one more candidate is added if it also makes
`--health` equal. **The registry bytes are preserved by
default**; when `bytes` was needed the report says so, and
`--regenerate-registry` instead leaves it out and regenerates
`CONTRACT-REGISTRY.md` once on apply.

**Probe trees.** Equal output on a clean project proves little about a
scanner's scope (both report 0 TODOs whatever directories they read). For
`check-todos`, `check-contract-refs` and `check-contract-headers`, migrate
also builds a scratch tree with a probe file (a `TODO:`, a broken
`CONTRACT:` ref, a headerless file) of every candidate extension in every
candidate directory, the project's top-level directories and the install
defaults, each also under `deep/`, `node_modules/`, `vendor/`, `dist/`, and
runs the copy (in the tree's `scripts/`, with the project's
`_rebar-config.sh` and `.rebarrc`) and the install on it. The findings
must match.

**Settings for other checks.** For checks whose hard-coded values the
install reads from `.rebarrc` (`source_dirs`, `source_extensions`,
`contracts_dir`, `freshness_max_days`), candidates come from the copy
(`for d in sdk mcp …; do [ -d`, `--include="*.sh"`, `${1:-30}`); the same
smallest-subset search keeps the needed ones. After all scripts are
decided, every probe-verified check is rerun with the final settings, since
`source_dirs` accepted for `check-todos` also changes
`check-contract-headers`.

**Dependencies.** A copy that a kept copy references (by file name) is kept
too, so the install suite's fallback to the kept copy doesn't break. Project
files that are not rebar copies but call a deleted copy (analyze's
`rebar-ci.sh`, `rebar-fix.sh`, `.pre-commit-config.yaml`) are reported with
file:line; migrate does not edit them.

**METRICS.** Deleting `.sh` copies changes counts like `shell_scripts`.
The deleted files are laid out alone in a scratch tree and counted with the
final metric settings; `key = N` lines that matched before are set to the
new value, and `>=` floors above it are lowered. `distinct` metrics touched
by the deletions are flagged for a look after apply instead.

**Tier.** migrate never changes `tier`. When a migrated check has an install
tier gate above the project's tier and the copy had none (analyze is tier 1;
the install's ground truth is tier 3), the report says `rebar check` will
SKIP it and how to run it anyway. Post-apply checks run at
`max(project tier, script gate)` and say so.

### Output and apply

Dry run (default) prints the `.rebarrc` block to add (between
`# --- rebar migrate … ---` markers, grouped by source script, aligned; keys
the file already has are never duplicated or edited), the files to delete
or replace with reasons and per-metric results, the files kept with reasons,
METRICS edits and notes. `--apply` writes exactly that, then reruns each
check that had a legacy copy (via the install, or the kept copy), and exits
1 if one that passed before fails. A second run proposes nothing.

`.rebarrc` is merged by `MergeRC`: existing lines untouched, a later run
appends inside the existing block.

## Evidence [M]

**virtru-analyze** (scratch `git clone --shared` of
`chore/rebar-merge-friendly`, user checkout untouched; customized
`check-ground-truth.sh` with 13 metrics, `compute-registry.sh`,
`check-todos.sh`, `check-contract-refs.sh`, `refresh-context.sh`; tier 1):

- Dry run (201 s): `git status` clean afterwards. All **13 metrics
  translated, each with the same value** from the install (sdk_languages 2,
  mcp_servers 6, helm_mcp_servers 4, helm_support_services 2, go_modules 17,
  mcp_shared_packages 7, contract_files 71, contract_ids 56,
  source_files_with_contract_headers 214, test_files 353, shell_scripts 16,
  role_agents 6, ci_workflows 36). Registry: two settings
  (`registry_scope_prefixes = SDK- MCP-`, `registry_purpose_heading =
  Purpose|Why this exists`) make `--stdout` byte-identical; `bytes` was not
  needed. `check-todos.sh`: `source_dirs` + `source_extensions`, same
  findings on the project and the probe tree. Kept: `check-contract-refs.sh`
  (probe tree: the install ignores refs under `.github/`, see Findings) and
  `refresh-context.sh` (different version, exit 141 vs 1).
- `--apply` (196 s): `.rebarrc` block written, three copies deleted,
  `shell_scripts = 16 → 13`; post-apply checks all as before (ground truth
  run at tier 3 and reported as such). Then `rebar check --pre-commit` exit
  0 (ground truth SKIPs at tier 1, as reported), `rebar run compute-registry
  --check` "Registry is current", `REBAR_TIER=3 rebar run
  check-ground-truth` all 13 OK, `git diff architecture/` empty.
- Second run (40 s): "Nothing to migrate".
- Against #14's hand-written appendix the generated `.rebarrc` has the
  same 13 metric groups line for line; it omits `registry_purpose_style =
  ellipsis` (now the default) and `registry_impl_globs` (does not change the
  committed registry; `--health` differs either way because the install's
  default `registry_exclude_dirs` skips the four tracked `dist/*.js`), and
  lists `source_extensions` in the copy's order.
- Reported callers of deleted copies: `scripts/rebar-ci.sh`,
  `scripts/rebar-fix.sh` (sources `check-ground-truth.sh` for
  `compute_metrics`), `scripts/tests/compute-registry.test.sh`,
  `.pre-commit-config.yaml`; and the integrity manifest.

**Fresh project from origin/main's template** (all 26 copies unmodified):
everything verified — 13 checks by behavior (3 also on probe trees), the
rest by provenance; `ci-check.sh` / `pre-commit.sh` / `README.md` replaced
by the current template files; `.rebarrc` gains nothing; `rebar check
--pre-commit` exit 0 before and after; the full `rebar check` fails the same
three checks before and after (doc-refs, registry missing, compliance
badge); second run "No legacy script copies".

**Untranslatable metric** (`scripts/tests/migrate.test.sh` T3, Go
`TestUntranslatableMetricKeepsCopy`): `sh_lines=$(cat tools/*.sh | wc -l)`
→ `kept      sh_lines = 2 — untranslatable: unsupported command "cat"`;
the copy stays, the expressible metric moves to `.rebarrc`.

## Tests

- Go, `cli/internal/migrate`: translator (analyze's shapes, the old
  template's examples, helpers with arguments, prune, BRE, `-printf`,
  refusals), lexer and paths, BRE→ERE; `.rebarrc` merge (new block, keys
  kept, second block avoided, idempotent, unwritable values, alignment);
  output comparison and value parsing; METRICS edits; and end to end against
  this checkout's scripts: dry run leaves the tree byte-identical, `--apply`
  deletes the verified copy and keeps a customized `steward.sh`, METRICS
  edit, second run empty; untranslatable and mismatched metrics keep the
  copy; registry settings derived minimally (`bytes` reported;
  `--regenerate-registry` regenerates); refusing the install itself.
- Shell, `scripts/tests/migrate.test.sh` (builds the CLI; SKIP without Go):
  dry run leaves `git status` clean; a customized `check-todos.sh` becomes
  `source_dirs` / `source_extensions` and the install then catches a TODO in
  the custom root; untranslatable metric kept and reported; second run
  empty; the pre-#14 template from git history migrates completely with the
  pre-commit result unchanged and a second run empty.
  `declarative-config.test.sh` T6 covers `--print` and `REBAR_RCFILE`.
- Not covered by tests: the provenance path without git history, timeouts,
  `_rebar-config.sh` behavior comparison, and the reference/integrity
  notes (exercised by the analyze run only).

## Red team

- *A copy that is only equal on today's tree.* Value and output equality on
  one state is weak for scanners — mitigated by probe trees, which is how
  the analyze `check-contract-refs` difference surfaced. Metrics: the
  translation is semantic (each find predicate maps to one field), the value
  check confirms it; loosened forms are reported. Remaining gap: a
  translation can match today and diverge on a future tree for constructs
  the notes don't cover (e.g. `-f` follows symlinks, `find -type f` does
  not).
- *"Wording differs" hides a real difference.* Facts are the file
  references and the numbers in the output (list numbering ignored), sorted,
  plus the exit status. A check whose output changes meaning without
  changing either would slip through; none of rebar's checks reports that
  way.
- *Running a customized copy is running arbitrary code.* It is the
  adopter's own code that `rebar check` already runs on every commit; only
  read-only checks are run, with stdin closed and a timeout. Scripts that
  write state are never run.
- *Smallest input (agreement 06).* A fresh template: every copy unmodified,
  `.rebarrc` gains nothing, all go. A copy computing zero metrics is
  equivalent to the install without `metric.*` lines. A project with no
  `scripts/`: "No legacy script copies".
- *Self-reference.* Probe files build `CONTRACT:` and `TODO:` from parts so
  rebar's own scans don't count them.
- *Deleting what another file calls.* Kept copies keep their dependencies;
  other callers are listed, not edited.
- *Install without git history* (tarball): provenance can't be checked, so
  the unprobeable copies are kept with that reason; probed ones still go.
- *Speed.* The virtru-analyze dry run takes ~3.5 minutes: each registry
  run is ~2.5 s and the subset search tries up to 11 settings combinations
  (stdout and `--health` in parallel); the legacy `check-contract-refs.sh`
  loops in bash over every probe hit. Install runs are cached per settings.
  It is a one-time command; per-run `--timeout` bounds a hung script.

## Findings about the install (not fixed here)

1. `check-contract-refs.sh` filters with `grep -v "node_modules\|vendor\|dist\|\.git"` on
   the output lines, so it ignores refs under `.github/` and in any path
   containing `dist` or `vendor` (`distribution/`). virtru-analyze's copy uses
   `--exclude-dir` and finds them; migrate therefore keeps that copy.
2. `refresh-context.sh` exits 141 (SIGPIPE) on virtru-analyze where the
   copy exits 1.

## Open questions (lean in brackets)

1. **Fix `check-contract-refs.sh` to use `--exclude-dir`?** It is a
   behavior change (more refs scanned → possibly new BROKEN findings for
   adopters) but the current filter is plainly wrong for `.github/`.
   [Lean: yes, as a `fix:` on #14, after which analyze's copy migrates too.]
2. **Edit callers of deleted copies?** migrate lists them; rewriting
   `scripts/check-todos.sh` to `rebar run check-todos` in arbitrary files is
   easy to get wrong. [Lean: keep reporting only.]
3. **Accept wording-only differences?** Strict byte equality would keep
   analyze's `check-todos.sh` ("task(s)" vs "tasks") forever. [Lean: keep
   facts equality, report it as "wording differs".]
4. **Unmodified-but-older checks whose output changed upstream** are kept
   (the copy and install differ). [Lean: keep; an `--accept-upstream` flag
   if adopters hit it.]
5. **Tier.** Analyze needs ground truth at tier 1; #14 Q1 proposed
   `checks_enable =`. migrate reports the gap and doesn't change tier.
   [Lean: land `checks_enable` with the v4 override removal.]
6. **Integrity manifest.** Projects with `.rebar/integrity.json` must
   `rebar commit` the deletions. [Lean: report only; migrate does not
   commit.]

## Disposition

Implemented on `feat/rebar-migrate`. Open questions await maintainer
triage; close into `feedback/processed/` once answered.
