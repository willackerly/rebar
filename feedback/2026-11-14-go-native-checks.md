# Go-native checks: one self-contained binary per platform

**Source:** the 3.1.0 release stack (#14 self-contained install, #8
contract_namespace, #19 source-scan excludes, #17 versioned installer, #18
rebar migrate, #20 CLI fixes), virtru-analyze as the acceptance adopter,
2026-11-14
**Type:** proposal (propose-then-ship) with a first proof slice in the same
branch
**Target:** 3.2 (not the 3.1 rc)
**Status:** open — questions for the maintainer at the end

Claims are labelled per `practices/inbox-protocol.md`: `[M]` measured here,
`[V]` read from source or git history, `[R]` reported in the session that
produced the 3.1 stack and not re-measured.

## Summary

Move the logic of rebar's checks from `scripts/*.sh` into the cross-compiled
`rebar` binary, behind the surface adopters already use: `rebar check`,
`rebar check --pre-commit | --cold-start`, `rebar run <name>`, and `.rebarrc`.
Nothing an adopter types or configures changes. A release becomes one binary
per platform; the scripts stay selectable (`REBAR_CHECK_IMPL=shell`) for one
minor while a parity harness holds the two implementations to the same
output, then retire.

This branch ships the mechanism and one real port: **check-todos**, with a
reusable source-tree walker, and parity tests that run the script and the
Go check on the same fixtures (including the existing
`source-scan-excludes.test.sh` assertions, run against the Go check).

## The case

### 1. The 3.1 stack was mostly shell-portability work

Each of these was found, reproduced and fixed during the 3.1 stack. None is
a rebar logic error; each is the host's shell or userland disagreeing with
the developer's.

| Bug | Where | Evidence |
|-----|-------|----------|
| bash 3.2 treats an empty `"${arr[@]}"` as unbound under `set -u`; the one-line installer died after cloning | setup-rebar.sh | `1a7a5cb` [V] |
| BSD `date -j -f` fills in the current time; a second ticking between `TODAY` and the parse made every age a day short on macOS | check-freshness.sh | `12043c9` (on `feat/rebar-migrate`) [V]: "STALE (133 days)" vs "(134 days)" on CI |
| `ls` order follows the locale; the macOS runner sorted `README.md` last | migrate test T5 | `ce76bdd` [V] |
| `sort -u` / `comm` under the ambient locale re-emitted inbox memos every poll | inbox-watch.sh | `3801733` [V] |
| `grep` exits 1 on no match; under `set -euo pipefail` check-registry died silently at the first orphan | check-registry.sh | `98879a9` [V]; same class in compute-registry, `0abb549` [V] |
| `grep -q` closing a pipe early under `pipefail` (SIGPIPE, exit 141) | installer / tests | [R] |
| `local -n` namerefs need bash 4.3; compute-registry silently failed on stock macOS | compute-registry.sh | `0abb549` [V] |
| `grep -v "node_modules\|vendor\|dist"` on whole `file:line:text` output dropped `.github/`, `distance.go` and any line mentioning those words; three review rounds across check-todos, check-contract-refs, check-registry, compute-registry | 4 scripts | `fa63f56`, `e044d6c` [V] |
| BSD vs GNU `grep` / `sed` option and output differences | several | [R]; two new instances below [M] |

### 2. Config is parsed twice, and kept equal by tests

`.rebarrc` is read by `_rebar_config` (bash) and `config.RCValue` (Go) with
"the same rules"; `_REBAR_KNOWN_CHECKS` and `config.KnownChecks` are two
lists kept equal by `TestKnownChecksMatchScripts`; the namespace validation
exists as `_rebar_contract_namespace_valid` and `validContractNamespace`;
`9c6c200` had to add whitespace trimming to bash because Go already trimmed
[V]. The tier is read with `grep '^tier' | sed | tr` in bash (a
`tier_note = …` line would match) and with a case-insensitive key compare in
Go [V]. Every new key is two implementations and a parity test.

### 3. Distribution needs a source tree

Since #14 the CLI runs the install's own scripts, so:

- a release tarball is the whole repository plus a binary
  (`scripts/package-release.sh`: "Each tarball is the full rebar tree") [V];
- `findRebarRoot` tries `$REBAR_ROOT`, the binary's `../`, the working
  directory, `~/.rebar/current`, `~/.rebar`, `~/dev/rebar`, `~/src/rebar`,
  `~/code/rebar` [V];
- a CI job that installs only the binary gets `rebar check` → "ci-check.sh
  not found", and `--pre-commit` / `--cold-start` skip with a notice, so a
  hook can silently check nothing [V] (`cli/cmd/check.go`).

### 4. Process spawning is the runtime

- compute-registry `--check` on a scratch clone of virtru-analyze: 6.3 s
  wall, 2.0 s user, **10.8 s sys** [M] (16 s reported on the user's checkout
  [R]); sys ≫ user is fork/exec, not work.
- `rebar migrate` on virtru-analyze: 3–4 min [R], mostly running candidate
  scripts repeatedly.
- check-contract-refs forks several processes per match (two `cut`, then
  `grep -o | head | sed`, plus `sed` and `awk` with a namespace) [V].

### 5. The reason for shell is mostly gone

Pre-#14, adopters copied the scripts and customized them, and shell was the
right medium for that. Since #14 adopters carry `.rebarrc` only, the thin
wrappers `exec rebar check`, and `rebar migrate` exists to delete the copies
[V]. Tier 1 already needs the `rebar` binary to run checks the supported way.

### 6. Found while building the proof slice [M]

The parity work turned up two platform-dependent behaviours in the
*current* check-todos.sh, both invisible to its tests:

- **A TODO in a non-UTF-8 file depends on the locale.** macOS `/usr/bin/grep`
  under `LANG=en_US.UTF-8` (Terminal.app's default) does not match a line
  containing a Latin-1 byte: `printf '// caf\xe9 TODO: x\n'` is reported
  under `LC_ALL=C` and silently passes under UTF-8. GNU grep treats such a
  file as binary and reports it on stderr, which check-todos discards.
- **A binary file is an untracked TODO on macOS only.** BSD grep prints
  `Binary file src/x.go matches` on stdout; check-todos counts and prints it
  as a finding. GNU grep (3.5+) sends that message to stderr: no finding.
- **`rebar check` on rebar itself fails Script Tests** (pre-existing, not
  from this branch; reproduced on `ac38adc`): the CLI exports
  `REBAR_PROJECT_ROOT`, lib.sh did not clear it, and merge-friendly.test.sh
  then ran compute-registry against rebar instead of its fixture ("T2:
  expected a stale registry"). Fixed in this branch (lib.sh unsets it).

The Go walker picks one behaviour everywhere: bytes, not locale; files with
a NUL byte are skipped (GNU); symlinks met while walking are skipped (both);
a symlinked operand is followed (GNU; BSD skips it).

## Target architecture

### Surface: unchanged

`rebar check`, `rebar check --pre-commit`, `rebar check --cold-start`,
`rebar run <name> [args]`, the `SKIP_*` variables, the summary format, each
check's output text and exit codes, and every `.rebarrc` key stay as they
are. Thin wrappers keep calling `rebar check`. The CONTRACT: tags, METRICS,
contracts and `grep -rn CONTRACT:` workflows of DESIGN.md are untouched:
this changes rebar's enforcement tooling, not the plain-text substrate.

### Native checks (`cli/internal/checks`)

One file per check, registered under its script name:

```go
type Check struct {
    Name string                                  // "check-todos"
    Run  func(ctx *Context, args []string) int   // script args in, exit code out
}
type Context struct{ Root string; Stdout, Stderr io.Writer }
```

A check reproduces its script: same arguments, same `.rebarrc` keys, same
lines on stdout, same exit code. Tier gating moves into a shared helper that
prints the scripts' `SKIP:` / `NOTE:` lines (`_rebar_skip`), driven by
`config.CheckTier`. `config.KnownChecks` becomes the registry's names, which
deletes the parity test.

### One config reader

`internal/config` is the only `.rebarrc` reader. This branch adds
`config.RCValueOr(root, key, envVar, default)`, the Go twin of
`_rebar_config`, used by the walker. Once the last script that sources
`_rebar-config.sh` is ported, the bash reader goes.

### Shared helpers (built as the ports need them)

- `internal/sourcescan` (this branch): `ScopeFor` (source_dirs,
  source_extensions, source_exclude_dirs + .git/.claude), `Filter.Files`
  (grep's `--include` / `--exclude` / `--exclude-dir` selection, lexical
  order) and `Search` (grep `-n` lines). Reused by check-contract-refs,
  check-registry, compute-registry, check-contract-headers.
- `internal/contracts`: contract file discovery, header parsing, `CONTRACT:`
  reference extraction with namespaces (`config.LocalContractRef` exists).
- `internal/gitx`: thin `exec git` wrapper (ls-files, log, show) for
  compute-registry, check-fix-commit, check-bypass-flags.
- `internal/suite`: the ci-check / pre-commit / cold-start runners in Go,
  same headers and summary, same `SKIP_*` variables.

### Assets via `go:embed`

`templates/`, `agents/`, the contract template and the bootstrap wrappers
(~410 KB [M]) are embedded, so `rebar new / adopt / init` need no install
tree. The binary is ~6 MB today [M].

### Release = binary

The release workflow publishes `rebar_<ver>_<os>_<arch>` binaries (plus
checksums) next to the tarballs during 3.x. **Caveat:** ASK (`bin/ask`,
`bin/ask-server`, `bin/ask-mcp-server`, `bin/ask-agent-loop`: ~3,900 lines of bash and
Python [M])
is not part of this proposal. `rebar ask` keeps needing the install tree
until ASK gets its own decision; the binary alone covers checks, `new /
adopt / init`, status and audit — exactly what CI needs.

### What happens to …

- **`REBAR_ROOT` / `findRebarRoot`:** no longer needed for checks or
  templates. It survives for ASK and as a developer override ("run these
  scripts instead"); the guessed home-directory candidates can go in 4.0.
- **The installer:** downloads one binary into `~/.rebar/versions/<ver>/bin/`
  and verifies its checksum; the `current` symlink and side-by-side versions
  stay. The source fallback becomes `go install`-shaped (no `git archive`).
- **`rebar migrate`:** its job (decide whether a legacy copy may be deleted by
  running it and comparing with the install) is unchanged, but the reference
  side runs in process, so the 3–4 min is mostly the legacy scripts' own
  runtime.
- **Thin wrappers:** unchanged.
- **Adopters still running legacy copies:** a project's own
  `scripts/<check>.sh` keeps winning, with the existing deprecation notice,
  in both implementations (a copy may carry customizations the native check
  lacks). Copies source their own sibling `_rebar-config.sh`, so they need
  nothing from the install. Support ends in 4.0.

### Extension point: `checks_extra`

So nobody forks rebar to add a check:

```
checks_extra = scripts/check-openapi.sh scripts/check-licenses.sh
checks_extra_pre_commit = scripts/check-openapi.sh
```

`rebar check` runs each after the built-in checks, in the project root with
`REBAR_PROJECT_ROOT` set, and lists it in the summary (`PASS  extra:
check-openapi`); a non-zero exit fails the suite like any check; a missing or
non-executable file is a FAIL, not a skip. `rebar run check-openapi` finds
it too. Any language works; that is where shell stays first-class.

## Transition mechanics (built in this branch)

- **`REBAR_CHECK_IMPL=shell|go`**, default `shell`. Read in one place,
  `checks.Impl()`; anything else warns once and means shell.
- **`rebar run <check>`** under `go` runs the native check in process when
  one is registered, even when no install has the script; a project's legacy
  copy still wins.
- **Inside the suites:** ci-check.sh / pre-commit.sh / steward.sh keep calling
  the check scripts. Under `go`, the CLI exports `REBAR_NATIVE_CHECKS` (what
  this binary implements) and `REBAR_BIN` (itself); a ported script calls
  `_rebar_native <name> "$@"` right after sourcing `_rebar-config.sh`, which
  `exec`s `"$REBAR_BIN" native <name> "$@"`. A script run by hand, by an
  older CLI, or with a mismatched install never hands off (no list, no
  binary), so version skew cannot route to a missing implementation.
- **`rebar native <check> [args]`** (hidden) always runs the native code and
  never a script, so the hand-off cannot loop; with no arguments it lists the
  native checks.
- **Flip criteria per check:** parity tests green on Linux and macOS (bash
  3.2 and 5), parity runs on rebar, virtru-analyze and a fresh `rebar new`
  project identical, one alpha with no drift reports. Then the default for
  that check flips; `REBAR_CHECK_IMPL=shell` remains until the minor after.

## Migration order

| Order | Check | Bash lines | Why here |
|------:|-------|-----------:|----------|
| 1 | check-todos | 89 | done here; builds `sourcescan`; pre-commit hot path; #19 bug class |
| 2 | check-freshness | 106 | BSD date bug; `time.Parse` makes it trivial; land after `12043c9` merges |
| 3 | check-contract-refs | 111 | per-match spawning; namespace logic already in Go; pre-commit hot path |
| 4 | check-registry | 119 | same walker; orphan/pipefail history |
| 5 | check-contract-headers, check-prefix-uniqueness, check-jtbd-presence | 94 + 137 + 176 | small; build `internal/contracts` |
| 6 | compute-registry | 462 | biggest runtime win; merge-friendly bytes need golden files; after 3–5 settle the parsing helpers |
| 7 | check-ground-truth | 206 | metric DSL; `rebar migrate` already has a Go translator to reuse |
| 8 | check-fix-commit, check-bypass-flags, check-version-bump | 126 + 142 + 114 | git log parsing; `internal/gitx` |
| 9 | check-doc-refs, check-decay-patterns, check-compliance | 220 + 232 + 440 | heaviest sed/awk; least bug history |
| 10 | ci-check, pre-commit, cold-start-checks → `internal/suite` | 126 + 76 + 168 | once every check they call is native, the hand-off goes |
| 11 | steward | 657 | `CONTRACT:S1-STEWARD.2.0` (plan mode); last |

Stays bash, not shipped: `tests/run.sh`, `sync-bootstrap.sh`,
`package-release.sh`, `test-e2e-live.sh` (rebar's own dev tooling).
Separate decision: `inbox-watch.sh`, `flush-notifications.sh`,
`refresh-context.sh`, `rebar-doc.sh`, `scan-consumers.sh`, `agent-health.sh`.

## Parity strategy

1. **Side by side on fixtures** (Go test per check): build a tree, run
   `bash scripts/<check>.sh`, the native check in process, and the script
   handing off to the binary; require equal exit codes and output, with
   finding lines compared as a sorted set (grep -r lists files in directory
   order, the native checks in lexical order). `TestTodosParity` has seven
   fixture trees.
2. **The existing shell tests, against Go:** `REBAR_TEST_NATIVE_BIN=<rebar>`
   makes lib.sh's `run_in` hand off, so `source-scan-excludes.test.sh` runs
   its assertions against the native check
   (`TestShellRegressionTestsAgainstNative`). New shell tests get Go coverage
   for free.
3. **Real repos:** compare script vs `rebar native` on rebar, a scratch clone
   of virtru-analyze, and a fresh `rebar new` project, per port. A `rebar
   native --compare <check>` mode (runs both, diffs) would let adopters do
   it on their own trees; see question 6.
4. **Golden output** for generators (compute-registry `--stdout`): byte
   comparison against checked-in expected files.
5. **Deliberate divergences** (the walker's choices above) are listed per
   check in the CHANGELOG, each with the case that differs and on which
   platform the old behaviour was wrong.

## Red team

- **Adopters see different results.** Lexical finding order; TODOs found in
  Latin-1 files that a UTF-8 macOS shell missed; no "Binary file" findings
  on macOS. Each is a gate result that previously depended on the machine;
  call them out in the CHANGELOG and keep `REBAR_CHECK_IMPL=shell` for the
  transition minor.
- **Two implementations at once.** Double maintenance, and a fix to one may
  miss the other. Mitigation: parity tests fail CI on drift; the window is
  one minor per check; ported scripts get no new features (fixes only).
- **Contributors and the maintainer prefer bash.** Shell is quicker to
  hack, and DESIGN.md's comparison table says "bash + jq + grep: zero
  dependencies beyond a Unix shell". CHARTER §1.3 says the Go and Python
  binaries "augment but never replace the plain-text substrate" and "a team
  can adopt rebar at Tier 1 with zero binaries built." The substrate
  (markdown, CONTRACT: tags, grep) is unchanged, a prebuilt download is not
  a build, and #14 already made the binary the entry point, but the wording
  needs an amendment (question 1).
- **Agents debugging a failure** used to read the script that failed. They
  now read Go in the rebar repo; the output text stays the same, and each
  check's doc comment names its script for history.
- **Customization.** Forks of scripts lose their path; `checks_extra` and
  `.rebarrc` keys are the answer. A request that needs a new key is a rebar
  change, as it already is since #14.
- **Windows.** Go makes native Windows possible (scripts never were); paths
  in output must stay `/`-joined (the walker already does). Not a 3.2 goal.
- **Performance.** Go start-up plus an empty scan is ~8 ms [M]; during the transition a
  hand-off costs one bash start plus `_rebar-config.sh` (~20 ms [M]).
- **Binary size and patching.** Embedded templates add ~0.4 MB; a site can
  no longer patch a check in place (`checks_extra`, or `REBAR_ROOT` + shell
  during 3.x).
- **Version skew.** Old CLI + new scripts, new CLI + old scripts, and legacy
  copies all fall back to the script, because the hand-off needs both
  `REBAR_NATIVE_CHECKS` from the binary and `_rebar_native` in the script.

## Deliverables (line estimates, Go incl. tests)

| Phase | Contents | Lines |
|-------|----------|------:|
| 0 (this branch) | mechanism, `sourcescan`, check-todos, parity harness, proposal | ~1,100 Go (half tests) + ~40 shell [M] |
| 1 (3.2.0-alpha.1) | tier-gate helper, check-freshness, check-contract-refs, check-registry, contract-headers / prefix / jtbd, `internal/contracts` | ~2,000 |
| 2 (3.2.0-alpha.2) | compute-registry with golden files, check-ground-truth, git-log gates | ~2,200 |
| 3 (3.2.0-beta) | doc-refs, decay-patterns, compliance; `internal/suite`; `checks_extra`; flip defaults for green checks | ~2,500 |
| 4 (3.2.0) | `go:embed` assets; binary release assets; installer downloads one binary; CHARTER/DESIGN wording | ~800 + docs |
| 5 (3.3) | steward (plan mode); remove `REBAR_CHECK_IMPL=shell` for checks flipped in 3.2 | ~1,500 |
| 6 (4.0) | delete ported scripts and `_rebar-config.sh`; legacy-copy support ends | −5,000 |

## Open questions for the maintainer (lean in brackets)

1. **CHARTER §1.3 / DESIGN tooling row.** Amend to "the plain-text
   substrate is markdown, CONTRACT: tags, grep and git; enforcement ships as
   one prebuilt binary"? [Yes, in phase 4, plan-mode CHARTER amendment.]
2. **Default flip.** Per check once green, or all at once? [Per check, in
   3.2.0-beta; shell selectable until 3.3.]
3. **Output fidelity.** Byte-identical text including order, or identical
   text with lexical order? [Identical text, lexical order: deterministic
   across filesystems is worth one-time noise.]
4. **Divergences where the shell is platform-dependent** (binary files,
   locale, symlinked operands): take the walker's choices and document them?
   [Yes; matching one of BSD or GNU is still matching nothing for the other.]
5. **`checks_extra` shape.** Two flat keys as above, or a dotted
   `check.<name>.cmd / .pre_commit` group? [Flat keys; add dotted only if
   per-check options appear.]
6. **`rebar native --compare`** for adopters to run parity on their own
   trees during the transition? [Yes, small, and it doubles as the drift
   report.]
7. **git access.** `exec git` or go-git? [`exec git`: already required,
   no new dependency, same semantics as the scripts.]
8. **ASK.** Out of scope here, so the binary alone does not cover `rebar
   ask`. Separate proposal, or keep tarballs for ASK indefinitely?
   [Separate proposal; tarballs stay until then.]
9. **`rebar native` visibility.** Hidden during the transition, removed with
   the switch? [Yes.]
10. **Ported scripts during the window:** fixes only, no features? [Yes;
    features land in Go.]

## Proof slice in this branch

- `cli/internal/sourcescan` — scope, grep-equivalent file selection and
  `-n` search; unit tests for selection, operands, symlinks, CRLF, missing
  trailing newline, NUL and Latin-1 bytes.
- `cli/internal/checks` — registry, `REBAR_CHECK_IMPL`, `ScriptEnv`, and
  `check-todos` (79 lines for an 89-line script).
- `config.RCValueOr`; `rebar native`; `rebar run` and the script runner use
  the switch; `_rebar_native` and the one-line hook in check-todos.sh;
  lib.sh's `REBAR_TEST_NATIVE_BIN`.
- Parity: `TestTodosParity` (7 trees × script / in-process / hand-off),
  `TestShellRegressionTestsAgainstNative`, `TestCheckImplSwitch` (`rebar run
  check-todos`, `rebar check --pre-commit` under both settings; Go works with
  no install at all). Mutating the native check (drop the TODO.md exclusion,
  the excluded directories, or change one output word) fails them [M].

**Evidence [M]** (mean of 10–20 runs, Apple Silicon; output compared with
`cmp`):

| Tree | Result (both) | Output | Shell | Go | Hand-off |
|------|---------------|--------|------:|---:|---------:|
| virtru-analyze scratch clone, `.rebarrc` scope from its legacy copy | OK, 7 tracked | byte-identical | 147 ms | 37 ms | 61 ms |
| same, with two probe TODOs (CRLF; `distance.ts` mentioning vendor) | FAIL, 2 | byte-identical | | | |
| virtru-analyze, whole tree (`.`) | FAIL, 18 | same lines, file order differs | 247 ms | 72 ms | 97 ms |
| rebar, default scope | no source dirs | byte-identical | 47 ms | 8 ms | 28 ms |
| rebar, `cli bin scripts templates` | FAIL, 1 | byte-identical | 74 ms | 11 ms | 33 ms |

check-todos is one grep pipeline, so the gain is ~4×; the per-match
spawners (check-contract-refs, compute-registry) are where the large wins
are expected. virtru-analyze's own legacy check-todos.sh misses the
`distance.ts` probe TODO (the #19 substring filter) and counts 10 tracked
tasks to the install's 7 (it counts its own source lines).

## Disposition

Pending maintainer triage.
