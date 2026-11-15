# Go-native checks: one self-contained binary per platform

**Source:** the 3.1.0 release stack (#14 self-contained install, #8
contract_namespace, #19 source-scan excludes, #17 versioned installer, #18
rebar migrate, #20 CLI fixes), virtru-analyze as the acceptance adopter,
2026-11-14
**Type:** proposal (propose-then-ship) with a first proof slice in the same
branch
**Target:** 3.2 (not the 3.1 rc), delivered as a stack of PRs, S0 (#23,
this branch) to S8
**Status:** accepted with owner decisions (2026-11-15, [Owner decisions](#owner-decisions-2026-11-15));
the remaining open questions keep their leans

Claims are labelled per `practices/inbox-protocol.md`: `[M]` measured here,
`[V]` read from source or git history, `[R]` reported in the session that
produced the 3.1 stack and not re-measured.

## Summary

Move the logic of rebar's checks from `scripts/*.sh` into the cross-compiled
`rebar` binary, behind the surface adopters already use: `rebar check`,
`rebar check --pre-commit | --cold-start`, `rebar run <name>`, and `.rebarrc`.
Nothing an adopter types or configures changes. A release becomes one binary
per platform. Each check's Go port becomes its default as it lands, with the
script selectable (`REBAR_CHECK_IMPL=shell`) and held to the same output by
parity tests; the last PR of the stack deletes the scripts, and the parity
tests become golden-file tests.

This branch ships the mechanism and one real port: **check-todos**, with a
reusable source-tree walker, and parity tests that run the script and the
Go check on the same fixtures (including the existing
`source-scan-excludes.test.sh` assertions, run against the Go check).

## Owner decisions (2026-11-15)

These override the plan and the leans below where they differ.

- **The end state lands in this stack, not in 4.0.** The last PR (S8)
  deletes the ported scripts, `scripts/_rebar-config.sh` and the bash/Python
  ASK tools: rebar becomes Go-binary-only.
- **Go becomes the default per check as it is ported.** Each port PR flips
  the default for the checks it ports; `REBAR_CHECK_IMPL=shell` stays as the
  fallback until S8, and every port keeps a parity test against its script.
- **Parity tests become golden tests in S8:** the expected output is captured
  from the scripts before they are deleted, so the coverage survives them.
- **ASK is in scope:** `bin/ask`, `bin/ask-agent-loop` (bash) and
  `bin/ask-mcp-server`, `bin/ask-server`, `bin/test-mcp-client` (Python) are
  ported to Go in S6 and deleted in S8.
- Out of scope: `setup-rebar.sh` stays bash (it downloads the binary), and
  rebar's maintainer tooling (`scripts/tests/*`, `sync-bootstrap.sh`,
  `gen-released-blobs.sh`, `package-release.sh`, `test-e2e-live.sh`) may
  stay shell.

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

The parity work turned up platform-dependent behaviour in the *current*
check-todos.sh (`grep -rnI`), all invisible to its tests and all shared by
the other source scanners:

- **A TODO in a non-UTF-8 file depends on the locale.** macOS `/usr/bin/grep`
  under `LANG=en_US.UTF-8` (Terminal.app's default) does not match a line
  containing a Latin-1 byte: `printf '// caf\xe9 TODO: x\n'` is reported
  under `LC_ALL=C` and silently passes under UTF-8. GNU grep treats such a
  file as binary (per its manual), which `-I` skips.
- **A symlinked source directory is scanned on Linux only.** GNU grep -r
  follows a symlink given as an operand; macOS grep -r skips it silently.
- **`source_exclude_dirs` patterns mean different things.** Both greps
  match `--exclude-dir` with fnmatch(3) (`[!x]`, `[[:digit:]]`), but macOS
  grep also matches the path below the operand (`src/gen` or `*/gen` skips
  src/gen while walking src; GNU grep: never), keeps a trailing slash (`gen/`
  matches nothing; GNU drops it), and matches an operand by its whole path
  or last component only (GNU: any tail after a `/`).
- **Binary files:** both greps look for a NUL in the first 32 KiB; after a
  later NUL macOS grep keeps searching, GNU grep stops.

The Go walker picks one behaviour everywhere: bytes, not locale; symlinks
met while walking are skipped (both); a symlinked operand is followed
(GNU); exclude patterns are matched as GNU grep matches them; a file is
binary when its first 32 KiB hold a NUL (both), and is otherwise searched to
the end (BSD). `TestTodosParity` pins each choice to the grep that agrees
with it and runs on both in CI.

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

The release workflow publishes `rebar_<ver>_<os>_<arch>` binaries (darwin /
linux × amd64 / arm64) plus SHA256 checksums (S7; whether tarballs stay is
decided there). ASK (`bin/ask`, `bin/ask-server`, `bin/ask-mcp-server`,
`bin/ask-agent-loop`, `bin/test-mcp-client`: ~4,000 lines of bash and Python
[M]) is ported to Go in S6, with the same flags, MCP tool names and on-disk
state, and thin compat shims for adopters' `.mcp.json` and scripts that name
`bin/ask*`; so the binary alone covers checks, `new / adopt / init`, status,
audit and `rebar ask`.

### What happens to …

- **`REBAR_ROOT` / `findRebarRoot`:** no longer needed for checks,
  templates or ASK once assets are embedded (S7). It survives as a developer
  override during the stack; what remains of it after S8 is decided there.
- **The installer:** downloads one binary into `~/.rebar/versions/<ver>/bin/`
  and verifies its checksum (S7); the `current` symlink and side-by-side
  versions stay. The source fallback becomes `go install`-shaped (no `git
  archive`).
- **`rebar migrate`:** its job (decide whether a legacy copy may be deleted by
  running it and comparing with the install) is unchanged, but the reference
  side runs the native checks in process (S7), so the 3–4 min is mostly the
  legacy scripts' own runtime; it also learns to rewrite `.mcp.json` to the
  native ASK command (S6).
- **Thin wrappers:** unchanged.
- **Adopters still running legacy copies:** a project's own
  `scripts/<check>.sh` keeps winning, with the existing deprecation notice,
  in both implementations and after S8 (a copy may carry customizations the
  native check lacks, and it is the adopter's own bash). Copies source their
  own sibling `_rebar-config.sh`, so they need nothing from the install, and
  they never hand off to the binary (below).

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

- **`REBAR_CHECK_IMPL=shell|go`**. Read in one place, `checks.Impl()`;
  anything else warns once and means the default. S0 keeps `shell` as the
  default for every check; from S1 each port PR makes `go` the default for
  the checks it ports (check-todos with S1's), and `shell` selects the
  scripts until S8 deletes them.
- **`rebar run <check>`** under `go` runs the native check in process when
  one is registered, even when no install has the script; a project's legacy
  copy still wins.
- **Inside the suites** (until S4 moves them into `internal/suite`):
  ci-check.sh / pre-commit.sh / steward.sh keep calling the check scripts.
  Under `go`, the CLI exports `REBAR_NATIVE_CHECKS` (what this binary
  implements), `REBAR_BIN` (itself) and `REBAR_NATIVE_SCRIPTS` (the install's
  scripts directory, symlinks resolved); a ported script calls
  `_rebar_native <name> "$@"` right after sourcing `_rebar-config.sh`, which
  `exec`s `"$REBAR_BIN" native <name> "$@"` only when the script itself lives
  in `REBAR_NATIVE_SCRIPTS`. So a project's copy of a check, even one copied
  from a script with the hook and reached through the install's suite via
  `_rebar_script`, runs itself; the CLI clears the variables when it runs a
  legacy copy directly. A script run by hand, by an older CLI, or with a
  mismatched install never hands off (no list, no binary), so version skew
  cannot route to a missing implementation.
- **`rebar native <check> [args]`** (hidden) always runs the native code and
  never a script, so the hand-off cannot loop; like the script, it resolves
  arguments against its working directory and reads settings from the
  project root. With no arguments it lists the native checks.
- **Flip criteria per check** (met in the port PR that flips it): parity
  tests green on Linux and macOS (bash 3.2 and 5), and parity runs on rebar,
  virtru-analyze and a fresh `rebar new` project identical or differing only
  as documented.

## The stack

Each PR's base is the previous PR's branch; S0's base is
`release/v3.1.0-rc`. A PR that grows past ~2,500 changed lines (golden
fixtures aside) may be split.

| PR | Branch | Scope | Bash lines ported |
|----|--------|-------|------------------:|
| S0 | feat/go-native-checks (#23) | mechanism, `sourcescan`, check-todos (89), parity harness, this proposal; review fixes | 89 |
| S1 | feat/go-native-1-scanners | Go tier gate + `checks_enable` helper; `internal/contracts`; check-freshness (BSD date bug), check-contract-refs (per-match spawning), check-registry, check-contract-headers, check-prefix-uniqueness, check-jtbd-presence; Go default for these and check-todos | 106 + 111 + 119 + 94 + 137 + 176 |
| S2 | feat/go-native-2-registry | compute-registry (byte-identical incl. `--check` / `--stdout`, golden files), check-ground-truth (reuse `rebar migrate`'s metric translator) | 462 + 206 |
| S3 | feat/go-native-3-gates | check-fix-commit, check-bypass-flags, check-version-bump (`internal/gitx`), check-doc-refs, check-decay-patterns, check-compliance | 126 + 142 + 114 + 220 + 232 + 440 |
| S4 | feat/go-native-4-suites | ci-check, pre-commit, cold-start-checks → `internal/suite`; `checks_extra` / `checks_extra_pre_commit`; `rebar check` needs no bash for rebar's own checks | 126 + 76 + 168 |
| S5 | feat/go-native-5-steward-utils | steward (`CONTRACT:S1-STEWARD.2.0`), refresh-context, agent-health, scan-consumers, rebar-doc, inbox-watch, flush-notifications | 657 + … |
| S6 | feat/go-native-6-ask | ASK in Go: `rebar ask`, the agent loop, the MCP server (stdio), ask-server; compat shims for `bin/ask*`; `rebar migrate` rewrites `.mcp.json` | ~4,000 (bash + Python) |
| S7 | feat/go-native-7-distribution | `go:embed` the assets `new / adopt / init / migrate` read; the binary works without an install tree; release publishes per-platform binaries + SHA256; setup-rebar.sh downloads and verifies the binary; `rebar migrate` compares against the native checks | — |
| S8 | feat/go-native-8-remove-shell | delete the ported scripts, `_rebar-config.sh` and the bash/Python ASK tools; parity tests → golden tests; template references; docs (README, SETUP, UPGRADING, CONFIGURATION, scripts/README, CHARTER / DESIGN wording); CHANGELOG | −5,000 and more |

Stays bash: `setup-rebar.sh` (it downloads the binary) and rebar's own dev
tooling: `scripts/tests/*`, `sync-bootstrap.sh`, `gen-released-blobs.sh`,
`package-release.sh`, `test-e2e-live.sh`.

## Parity strategy

1. **Side by side on fixtures** (Go test per check): build a tree, run
   `bash scripts/<check>.sh`, the native check in process, and the script
   handing off to the binary; require equal exit codes and byte-identical
   output, after putting the script's finding lines into the native walk
   order (grep -r lists files in directory order). Cases where the script
   depends on the platform's grep stay in the table, marked with the grep
   that diverges and the native output expected; they are compared with the
   script on the other platform. `TestTodosParity` has 18 fixture trees.
2. **The existing shell tests, against Go:** `REBAR_TEST_NATIVE_BIN=<rebar>`
   makes lib.sh's `run_in` hand off (and sets `REBAR_CHECK_IMPL=go` for tests
   that drive the CLI). `TestShellRegressionTestsAgainstNative` runs every
   `scripts/tests/*.test.sh` that names a ported check that way and, for
   tests that use `run_in`, requires a logged hand-off per ported check. New
   shell tests get Go coverage for free.
3. **Real repos:** compare script vs `rebar native` on rebar, a scratch clone
   of virtru-analyze, and a fresh `rebar new` project, per port. A `rebar
   native --compare <check>` mode (runs both, diffs) would let adopters do
   it on their own trees; see question 6.
4. **Golden output** for generators (compute-registry `--stdout`): byte
   comparison against checked-in expected files.
5. **Deliberate divergences** (the walker's choices above) are listed per
   check in the CHANGELOG, each with the case that differs and on which
   platform the old behaviour was wrong.
6. **Golden tests after the scripts (S8):** before deleting a script, its
   output on every parity fixture is captured as an expected file; the
   parity tests become golden tests against those files.

## Red team

- **Adopters see different results.** Lexical finding order; TODOs found in
  Latin-1 files that a UTF-8 macOS shell missed; symlinked source
  directories scanned on macOS. Each is a gate result that previously
  depended on the machine; call them out in the CHANGELOG and keep
  `REBAR_CHECK_IMPL=shell` until S8.
- **Two implementations at once.** Double maintenance, and a fix to one may
  miss the other. Mitigation: parity tests fail CI on drift; the window ends
  with S8; ported scripts get no new features (fixes only).
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
  until S8).
- **Version skew.** Old CLI + new scripts, new CLI + old scripts, and legacy
  copies all fall back to the script, because the hand-off needs
  `REBAR_NATIVE_CHECKS` from the binary, `_rebar_native` in the script, and
  the script to live in the install the binary named.

## Deliverables

The stack table above replaces the earlier phase plan (alpha / beta / 3.3 /
4.0): everything through the deletion of the scripts lands in S0–S8. Line
estimates per PR, Go incl. tests: S0 ~2,050 [M]; S1 ~2,000; S2 ~2,200; S3
~2,500; S4 ~1,200; S5 ~2,000; S6 ~2,500 (may split into S6a / S6b); S7 ~800
+ docs; S8 mostly deletions and golden fixtures.

## Open questions for the maintainer (lean in brackets)

1. **CHARTER §1.3 / DESIGN tooling row.** Amend to "the plain-text
   substrate is markdown, CONTRACT: tags, grep and git; enforcement ships as
   one prebuilt binary"? **Resolved:** yes, in S8 with the docs, as a
   plan-mode CHARTER amendment (the binary-only end state requires it).
2. **Default flip.** Per check once green, or all at once? **Resolved:** per
   check, in the PR that ports it; shell selectable until S8.
3. **Output fidelity.** Byte-identical text including order, or identical
   text with lexical order? [Identical text, lexical order: deterministic
   across filesystems is worth one-time noise.]
4. **Divergences where the shell is platform-dependent** (binary files,
   locale, symlinked operands, exclude patterns): take the walker's choices
   and document them? [Yes; matching one of BSD or GNU is still matching
   nothing for the other. S0 applies this lean, see §6.]
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
   **Resolved:** ASK is ported to Go in S6 and its bash/Python tools are
   deleted in S8; whether tarballs stay is decided in S7.
9. **`rebar native` visibility.** Hidden during the transition, removed with
   the switch? **Resolved:** hidden through the stack; with no scripts left
   to hand off, it goes (or stays as a hidden debugging aid) in S8.
10. **Ported scripts during the window:** fixes only, no features? [Yes;
    features land in Go.]

## Proof slice in this branch (S0)

- `cli/internal/sourcescan` — scope, grep-equivalent file selection
  (fnmatch(3) globs, operand tails) and `-nI` search; unit tests for
  selection, patterns, operands, symlinks, CRLF, missing trailing newline,
  NUL and Latin-1 bytes.
- `cli/internal/checks` — registry, `REBAR_CHECK_IMPL`, `ScriptEnv`
  (install-only hand-off), `Context.Dir`, and `check-todos` (84 lines for an
  89-line script).
- `config.RCFile` / `RCValueIn` / `RCValueOr` (REBAR_RCFILE honored);
  `rebar native`; `rebar run` and the script runner use the switch;
  `_rebar_native` and the one-line hook in check-todos.sh; lib.sh's
  `REBAR_TEST_NATIVE_BIN`.
- Test helpers for the stack in `cli/internal/testutil`: `IsolatedEnv`,
  `RunEnv`, `Tags`, `RebarSource`.
- Parity: `TestTodosParity` (18 trees × script / in-process / hand-off),
  `TestShellRegressionTestsAgainstNative` (every script test naming a ported
  check), `TestNativeHandOffOnlyFromInstall`,
  `TestNativeArgsResolveAgainstWorkingDir`, `TestRCFileParity`,
  `TestCheckImplSwitch` (`rebar run check-todos`, `rebar check --pre-commit`
  under both settings; Go works with no install at all). Mutating the
  native check (drop the TODO.md exclusion, the excluded directories, or
  change one output word) fails them [M].

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

Accepted by the owner with the decisions above (2026-11-15); implemented
as the S0–S8 stack starting from #23.
