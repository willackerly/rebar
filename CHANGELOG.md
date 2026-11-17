# Changelog

All notable changes to rebar. Versioned with [semver](https://semver.org/).

**Versioning policy:**
- **Major** — breaking changes to contract format, agent role structure, CLI interface, or enforcement script API
- **Minor** — new agents, templates, practices, profiles, scripts
- **Patch** — doc fixes, script bug fixes, template clarifications
- **Pre-release suffixes (`-alpha`, `-beta`) are the annealing channel:**
  work we believe in but that needs field time ships **on `main`,
  honestly labeled** — the version string and per-artifact `Status:`
  markers carry the anneal state, not branch topology (parallel lines
  rot). Each pre-release label is earned by an explicit checklist and
  graduates by explicit criteria recorded in its release plan doc —
  never by the passage of time alone.

---

## Unreleased

- **`rebar mcp serve` shuts down promptly and takes its children with it.**
  At the end of stdin (Claude Code exiting or restarting the server) and on
  SIGINT/SIGTERM — stdio and HTTP — the server stops reading requests (HTTP:
  stops accepting connections and shuts down gracefully), cancels the tool
  calls still running (they answer with an `isError` "Agent query
  cancelled" result) and exits 0 within 2 s. Previously the end of input
  waited up to 310 s for running calls, and a signal killed the server and
  orphaned the `rebar ask` / `claude` processes it had started. Each ask now
  runs in its own process group: cancelling it (client cancellation,
  timeout, shutdown) sends the group SIGTERM, then SIGKILL after 1 s, and
  what is left of it is killed when the server exits. 2 s is the window
  Claude Code's stdio client gives a server between closing stdin and
  sending SIGTERM. Behaviour change for scripts: a `tools/call` piped in with
  `printf … |` is cancelled at the end of input — keep stdin open until it
  answers (`scripts/test-e2e-live.sh` now does, over a FIFO). `rebar ask
  serve` gets the same signal handling and cancels `/v1/ask` asks when
  their client disconnects. Tests: `TestMCPStdioShutdown` (end of input,
  SIGTERM, SIGINT; asks that ignore SIGTERM), `TestMCPHTTPShutdown`,
  `TestRunAskCancelKillsGroup`, `TestKillChildren`, `TestStdinFilterGrace`.
- **`rebar mcp serve` runs on the official MCP Go SDK**
  (`github.com/modelcontextprotocol/go-sdk` v1.8.0) instead of the
  hand-written JSON-RPC layer, so the server follows the MCP spec rather than
  the python server's quirks. Tool names, descriptions and input schemas are
  unchanged, and tool names and `ask://` resource URIs still match
  case-insensitively. **Client-visible changes:** the protocol version is
  negotiated (2024-11-05 … 2025-11-25 over `initialize`; 2026-07-28 over
  `server/discover`, which Claude Code 2.1.3xx tries first; the server used
  to answer 2024-11-05 to everyone); a failing ask, a timeout, an empty
  `question` or arguments that break the schema are tool errors
  (`isError: true` results with the reason) instead of JSON-RPC -32603/-32602
  errors; an unknown tool is -32602 `unknown tool "<name>"` (was
  -32603 "Repository … not found"); an unknown resource is -32602 "Resource
  not found"; a request before `initialize` gets the SDK's "invalid during
  session initialization" error with code -32600 invalid request (was -32002;
  rebar rewrites the SDK v1.8.0's code 0 to what SDK v1.9.0 sends); responses are compact JSON
  (`{"jsonrpc":"2.0",…}`, not python `json.dumps` spacing) and may arrive out
  of order; `serverInfo.version` is rebar's version (was "1.0.0"); `ping` and
  an empty `prompts/list` are answered; batches are refused after
  2025-06-18. Over stdio a malformed line is still logged and skipped (no
  `id: null` reply), invalid JSON-RPC objects and fractional ids are now
  skipped too. **HTTP:** `/mcp` is
  MCP's Streamable HTTP transport (`Accept: application/json,
  text/event-stream`, `Mcp-Session-Id`, 202 for notifications, DELETE ends a
  session; loopback requests need a loopback `Host`), so Claude Code's
  `"type": "http"` servers work; plain one-shot JSON-RPC POSTs no longer do.
  `rebar mcp client` uses the SDK client. The classic `/v1/*` API and
  `rebar ask serve` are unchanged. The binary grows by ~1.9 MB (stripped).
  Tests: goldens in `cli/cmd/testdata/mcp` regenerated, and a Claude Code
  compatibility test replays sessions recorded from Claude Code 2.1.224 and
  2.1.311.
- **rebar is a single Go binary: the shell implementations are gone.**
  rebar no longer ships `scripts/*.sh` for its checks, suites, steward and
  utilities, nor `scripts/_rebar-config.sh`, the bash `ask` /
  `ask-agent-loop`, the python `ask-mcp-server` / `ask-server` /
  `test-mcp-client` (`bin/legacy/`). Every check, suite and utility runs from
  the binary (`rebar check`, `rebar run <name>`, `rebar ask`, `rebar mcp
  serve`), and `REBAR_CHECK_IMPL` is ignored (no `shell` fallback, no
  hand-off). **What adopters keep:** `.rebarrc`, `METRICS`, their contracts,
  and the two thin entry points `scripts/ci-check.sh` / `scripts/pre-commit.sh`
  (they run `rebar check`). `bin/ask`, `bin/ask-agent-loop`,
  `bin/ask-mcp-server` and `bin/ask-server` stay as sh shims that run
  `rebar` (from `rebar install-shims` / the installer), so PATH entries and
  `.mcp.json` files that name them keep working; `rebar mcp client` replaces
  `bin/test-mcp-client`. **Legacy copies:** a project's own copy of one of
  rebar's old scripts (`scripts/check-todos.sh` and the like) is the
  adopter's code: `rebar run <name>` and the suites still run it instead of
  the built-in check, with a deprecation notice, and `checks_extra` still
  runs the project's own scripts. A copy that sources `_rebar-config.sh`
  still finds it only if the project copied that file too. `rebar migrate`
  recognizes copies of every released script from the list compiled into
  the binary (no rebar tree needed) and replaces those it verifies.
  **Behaviour changes:** outside a rebar checkout the full suite's Bootstrap
  Sync and Script Tests steps are listed as `SKIP … (rebar source repo
  only)` instead of running a script that skipped itself (the summary counts
  them as skipped, not passed); `rebar run` lists the built-in checks and the
  project's own scripts; check hints name `rebar run compute-registry`,
  `rebar run flush-notifications`, `rebar run refresh-context` and `rebar run
  steward` instead of the deleted scripts. **Tests:** the parity tests are
  golden tests now: goldens captured from the scripts before deletion (the
  scanner checks got theirs in this change), `runGolden` runs every case in
  process and in parallel, ASK and MCP sessions compare with goldens (and
  through the `bin/ask` / `bin/ask-mcp-server` shims); the cmd package runs in
  about 45 s instead of 3.5 min. `scripts/tests/*.test.sh` run the binary
  (`run.sh` builds it from `cli/`, or `REBAR_TEST_BIN`); CI's check step runs
  `rebar check` built from the commit. The rebar tree marker is
  `templates/project-bootstrap/` plus `cli/internal/assets/` (or, for older
  trees, `scripts/steward.sh`). `scripts/` now holds maintainer tooling only
  (`sync-bootstrap.sh`, `tests/`, `package-release.sh`,
  `gen-released-blobs.sh`, `test-e2e-live.sh`); the checks reference moved to
  `docs/CONFIGURATION.md`.

- **The `rebar` binary is self-contained, and releases are binaries.**
  `rebar new / init / adopt` write the bootstrap template compiled into the
  binary (`cli/internal/assets`, a `.embed`-suffixed copy that
  `scripts/sync-bootstrap.sh` refreshes and, with `go test`, checks), so
  `new / adopt / init / migrate / check / run / ask / mcp` need no rebar tree
  and no `REBAR_ROOT`; only `REBAR_CHECK_IMPL=shell` (until S8) and `rebar
  run` of rebar's maintainer scripts still use one. New `rebar install-shims
  [--dir DIR] [--force]` writes the `ask*` shims, `bin/install` and the
  completions beside the binary. The release workflow publishes
  `rebar_<ver>_<os>_<arch>` binaries plus `rebar_<ver>_checksums.txt` (no
  tarballs) and smoke-tests the linux/amd64 one with no tree;
  `setup-rebar.sh` downloads and verifies the binary and runs `install-shims`
  (whole-tree tarballs of v3.1.0 and earlier, and source builds, still
  install as trees; `.rebar-install` records `layout=`). `rebar run` lists
  the built-in checks; `rebar audit` and `rebar context session-start` count
  them as available; with no rebar tree, `rebar check` marks Bootstrap Sync
  and Script Tests "(rebar source repo only)". `bin/install` no longer asks
  for jq or python3. Docs: SETUP's CI section (the "pinned binary, no tree"
  trap is gone; the pinned wrapper now downloads and verifies the release
  binary), UPGRADING, README, and CHARTER §1.7 / the DESIGN tooling row
  ("enforcement ships as one prebuilt binary").
- **`rebar migrate` compares copies with the built-in checks, in process.**
  The reference side is the native check, run with the candidate `.rebarrc`
  as an explicit rc file and a scrubbed environment (`config.WithEnv`)
  instead of the install's script under `REBAR_RCFILE`; legacy copies still
  run with bash. A rebar tree is optional (its git history adds released
  versions). The migrate test package runs in about half the time. New: a
  `ci-check.sh` / `pre-commit.sh` copy that is a released version plus calls
  to project scripts becomes the thin wrapper with `checks_extra` /
  `checks_extra_pre_commit` when rebar's suite with them exits as the copy
  does; otherwise it is kept with the reason.
- **Tests: parity cases run in parallel; goldens stop changing with the
  date.** `runParity` runs each case in parallel (the in-process side under
  `config.WithEnv` with the case's environment instead of `t.Setenv`): the
  cmd package takes about 3.5 minutes instead of 8 locally, and CI and the
  release workflow pass an explicit `go test -timeout 20m`. The ci-check and
  refresh-context goldens held dates (and, for refresh-context, commit ids)
  computed from the fixtures' relative dates, so they failed from the day
  after they were written; goldens now keep the `{{ago:N}}` tokens and
  refresh-context's commit ids are normalized.
- **ASK's servers run natively.** `rebar mcp serve` is the MCP server
  (was the python `bin/ask-mcp-server`): stdio and HTTP, the same flags,
  tool names (`ask_<repo>_<role>`), input schema, descriptions, resources
  (`ask://memory|log|agent/<repo>:<role>`), JSON-RPC error codes, messages
  and data, notification handling, and Python's `json.dumps` output byte for
  byte. It is a minimal JSON-RPC implementation, not the official MCP Go
  SDK, whose protocol negotiation and error shapes would change what Claude
  Code sees. `rebar ask serve` is the enterprise server (was the python
  `bin/ask-server`): the same `/v1/*` endpoints, statuses and bodies.
  `rebar mcp client` replaces `bin/test-mcp-client` (no `requests` module
  needed). tools/call and `/v1/ask` run `rebar ask` (or `--ask-bin` /
  `ASK_BIN`) in the repo. No python is needed any more: `bin/ask-mcp-server`,
  `bin/ask-server` and `bin/test-mcp-client` are POSIX sh hand-offs to the
  binary, so adopters' `.mcp.json` files keep working; the python originals
  moved to `bin/legacy/*.py` and run under `REBAR_CHECK_IMPL=shell` until
  they are removed. `rebar init/new/adopt` write a `.mcp.json` that runs
  `rebar mcp serve --stdio` (through `~/.rebar/current/bin/rebar`), and
  `rebar migrate` reports, and with `--apply` writes, the rewrite of
  `.mcp.json` servers that run `ask-mcp-server` (directly or via python3)
  to `rebar mcp serve` with the same arguments. Parity: one recorded stdio
  session (44 messages: lifecycle, every method, every error path, odd ids,
  parse errors, notifications) byte-identical, HTTP transcripts of both
  servers structurally identical (`cli/cmd/native_mcp_test.go`, goldens in
  `cli/cmd/testdata/mcp`; skipped without python3). Deliberate deviations:
  over HTTP the MCP session survives across requests (python made a new,
  uninitialized one per request, so `tools/list` after `initialize` always
  failed with -32002); `ask_<repo>_<role>` resolves repos with `_` in their
  name (python split at the first `_`); usage/help text is rebar's own
  (exit codes match).
- **ASK runs natively.** `rebar ask` no longer execs `bin/ask`: the Go
  port in the binary implements every subcommand (`who`, `where`,
  `status`, `log`, `peek`, `tail`, `watch`, `up`, `reset`, `compact`,
  `register`, `projects`, `init`, `agents`, agent commands, cross-project
  `project:agent`, remote mode over `ASK_SERVER`), its flags, `ASK_*`
  variables, exit codes and on-disk state (`.session-id`, `.pid`,
  `.progress`, `.last-stderr`, inbox/outbox, `memory.log.md`,
  `shared/messages/events.jsonl`, `~/.config/ask/projects`,
  `.rr-index`), and the inbox loop (`ask up`, `bin/ask-agent-loop`, now
  also `rebar ask-agent-loop`). It works in any directory, as `bin/ask`
  did, and needs no jq (it writes jq's bytes itself). `bin/ask` and
  `bin/ask-agent-loop` hand off to the `rebar` beside them (else
  `REBAR_BIN`, else `PATH`), so scripts and `run.sh` files that call them
  keep working; `REBAR_CHECK_IMPL=shell` runs the bash. `rebar init/new`
  scaffold `agents/` and `rebar contract upstream` dispatch in process
  (`ASK_BIN` still picks another ask for the latter). Parity:
  `TestAskParity` (110 cases: bash, native and the shim against a stub
  claude and test ASK servers; output, exit code, files, claude calls and
  HTTP requests) and `TestAskLoopParity`, goldens in
  `cli/cmd/testdata/ask`. Deliberate deviations from the bash:
  `ask --help` / `ask -h` show the help (the flag loop rejected them as
  unknown flags); agent names resolve to the directory's own spelling on
  every file system (on a case-insensitive one the script kept the
  spelling as typed, in prompts, events and `ask where`);
  `LAST_ACTIVE` of `ask status` is computed on every platform (the
  script's `stat -f %m` crashed it with GNU stat); re-registering a moved
  project that was the registry's only entry works (the script's `grep -v`
  exited it silently); the background compaction does not auto-register
  its directory; on a terminal the answer header is capitalised and
  wrapped as GNU sed and `fmt` did (BSD sed left it as is).
- **The utility scripts run natively.** `rebar run refresh-context`,
  `agent-health`, `scan-consumers`, `rebar-doc`, `inbox-watch` and
  `flush-notifications` run Go ports, with the scripts' arguments,
  environment variables (`REBAR_REPOS`, `ASK_REGISTRY`, `REBAR_OUTBOX`,
  `REBAR_NOTIFY_DEFAULT`, `ASK_BIN`, `AGENT_ID`, `AGENT_PROGRESS_FILE`,
  `REBAR_ROOT`, `HOME`), output, files and exit codes. The install's scripts
  hand off to the binary under `rebar run`; `REBAR_CHECK_IMPL=shell` runs
  the scripts. `inbox-watch` waits on a timer and SIGINT/SIGTERM together,
  as the script's background `sleep` + `wait` did, so a kill still lands at
  once and releases the per-inbox lock (the lock holds the same PID: the
  hand-off `exec`s). `agent-health.sh` stays a library to `source` into an
  agent's shell; run as a command it now takes the function to run
  (`agent-health.sh checkpoint "msg"`, `heartbeat`, `metric K V`, `rmse D B
  A`), which is what `rebar run agent-health …` runs natively.
  `flush-notifications` calls ask through `ASK_BIN` as before and reads
  scan-consumers' JSON as the script did with jq. `rebar doc` and
  `rebar-doc` now share one kind table, so `rebar doc` also rejects a name
  with whitespace or `..` (exit 2), as the script did. Kept for parity, to
  fix later in both implementations: refresh-context ends with exit 128
  outside a git repository (and before any commit), right after "Commits
  since last sync:", because of `set -e` with pipefail; scan-consumers ends
  with exit 1 at the first consumer section missing `version_pinned`,
  `pin_date`, `notify_on_change` or `rationale`, though the template marks
  `notify_on_change` optional (so flush-notifications sees no consumers for
  that contract); flush-notifications exits 1 after every run that is not
  `--dry-run`; the TODO counts print `0<newline>0` when there are none.
  Platform choices: dates are read as GNU date reads them (a date that does
  not exist gives no staleness line; BSD rolled it over), binary source
  files are not untracked TODOs, the `--dry-run` message length counts
  characters (bash 3.2 in the C locale counted bytes).

- **steward runs natively.** `rebar run steward` and the Steward step of
  `rebar check` run a Go port of `scripts/steward.sh`
  (the S1-STEWARD 2.0 contract) with the same arguments (`--json`,
  `--summary`, `--check <id>`, anything else a full scan), output, exit codes
  and files, byte for byte as jq writes them: per-contract
  `<contracts_dir>/.state/<id>.<version>.json`, `steward-report.json` and
  `STEWARD_REPORT.md`. It runs the seven enforcement checks in process, or a
  project's own copy of one (a copy that is not executable counts as
  `skip`, as before). `REBAR_CHECK_IMPL=shell` runs the script, and the
  install's `steward.sh` hands off to the binary like the other ported
  scripts. It needs no jq. Where the script depended on the platform or
  locale, the port picks one behaviour: files, lines and contracts sort in
  byte order (the C locale; under a UTF-8 locale the script's `sort -u`
  ordered `implementing_files` by locale collation of `path:line:text`); a
  binary file holding a reference is not an implementation (BSD grep's
  "Binary file … matches" line became an implementing file named that); a
  directory named like a contract file is not a contract; lines are read
  as bytes (GNU grep under a UTF-8 locale dropped a TODO.md discovery line
  that is not valid UTF-8, taking it for binary data). Steward on rebar
  takes 0.07 s instead of 6 s; on virtru-analyze (60 contracts) 6.6 s
  instead of 48 s, most of it the project's own check copies, which still
  run as bash.
- **`rebar new` ignores the steward's reports.** The steward contract says
  `STEWARD_REPORT.md` is generated and gitignored, but `rebar new` wrote a
  `.gitignore` without it or `architecture/.state/*.json`, so a fresh
  project's first `rebar check` left both untracked. The bootstrap
  template's `.gitignore` listed `architecture/.state/steward-report.md`, a
  file nothing writes, instead of `STEWARD_REPORT.md`; fixed too.

- **The suites run natively: `rebar check` needs no bash for rebar's
  checks.** `rebar check`, `rebar check --pre-commit` and `rebar check
  --cold-start` (and `rebar commit`, `rebar push`, `rebar run ci-check`) run
  Go versions of ci-check, pre-commit and cold-start-checks in process, with
  the scripts' check order, output, `SKIP_*` variables, summary line, strict
  semantics (`--strict` or no argument) and exit codes. Steward,
  sync-bootstrap and `scripts/tests/run.sh` still run as scripts from the
  install; a binary-only install lists them as "script not found". A
  project's own customized `ci-check.sh` / `pre-commit.sh` still runs
  instead, and so does a project's own copy of a check a suite calls, with
  the deprecation notice; thin wrappers go native. `REBAR_CHECK_IMPL=shell`
  runs the scripts. On rebar itself `rebar check` takes 1.4 s instead of
  16 s (0.3 s instead of 11 s without steward, which S5 ports).
- **`checks_extra` / `checks_extra_pre_commit`.** List your own check
  programs (any language) in `.rebarrc`; `rebar check` (`--pre-commit`)
  runs them after rebar's checks, in the project root with
  `REBAR_PROJECT_ROOT` set, and counts them in the summary (`PASS  extra:
  check-openapi`). A non-zero exit or a missing file fails the suite;
  `SKIP_CHECKS_EXTRA=1` skips them; `rebar run <name>` runs one. Env
  overrides `REBAR_CHECKS_EXTRA` / `REBAR_CHECKS_EXTRA_PRE_COMMIT`. Only the
  built-in suites run them (`rebar check` warns when the shell suite or a
  project copy runs instead). See docs/CONFIGURATION.md.
- **check-doc-refs, check-decay-patterns and check-compliance run natively.**
  The `rebar` binary runs all three by default (`REBAR_CHECK_IMPL=shell`
  selects the scripts), with the same arguments, output and exit codes.
  check-doc-refs reads `git ls-files` as the script does (so a non-ASCII
  path, which git quotes, still counts as untracked), folds `./` and `../`
  the way its sed does, and matches a directory link as a grep pattern
  (`d.cs/` matches `docs/`). check-decay-patterns scans the same files
  (staged, working tree, `--all`, `--paths`) for the same patterns.
  check-compliance keeps every check, message and badge rule, the tolerant
  `Status:` parse and the maturity arithmetic (a status outside the
  vocabulary still counts twice). On adopter repos check-doc-refs takes
  0.1–0.2 s instead of 1–25 s and `check-decay-patterns --all` 0.3–0.9 s
  instead of 6–12 s. Under macOS bash 3.2 the script crashed ("Bus error")
  or hung on a scan set of a few hundred files. Where grep's handling of
  binary files differs between platforms, the native checks do what GNU
  grep does: a binary spec file yields no findings and a binary contract
  has no `Status:` (BSD grep's "Binary file … matches" line was read as a
  finding and as status "binary"). A directory link whose path is not a
  valid pattern (an unbalanced `[`) is reported as broken without grep's
  error message, and a link such as `a//../b` no longer hangs the check (the
  script's `../` folding looped forever on it).
- **The git gates run natively.** check-fix-commit (Gate G),
  check-bypass-flags (Gate I) and check-version-bump (the post-commit
  outbox hook) are ported to the `rebar` binary and are its default
  (`REBAR_CHECK_IMPL=shell` selects the scripts). They run the same git
  commands as the scripts (new `internal/gitx`), so commit selection
  (`--range` takes any `git rev-list` arguments), path quoting and rename
  detection stay git's, merge commits and shallow clones included, and give
  the same output and exit codes in every mode (HEAD, commit-msg file,
  `--range`, `--warn`, `--help`). Kept as the scripts have them: the
  `Reproduced on:` / `Bypass tickets:` evidence must contain a character
  that is not POSIX whitespace (a vertical tab does not count), the
  "matched:" list shows the first three matches, and check-version-bump
  reports "00 pending" when a contract was renamed to another one and the
  outbox holds nothing pending, and exits with git's status in a repository
  without commits. One difference: `--range` without a range says so and
  exits 1 (the scripts died on `set -u` with "$2: unbound variable").
- **compute-registry and check-ground-truth run natively too.** Both are
  ported to the `rebar` binary and are its default (`REBAR_CHECK_IMPL=shell`
  selects the scripts). compute-registry writes, `--check`s, prints
  (`--stdout`) and `--health`s the same bytes as the script for every
  `registry_*` key (impl counts on or off, `ellipsis` / `bytes` purposes,
  scope prefixes, numbered purpose headings), in a git work tree (tracked
  files, as `git grep`) and outside one (as `find`); on virtru-analyze
  `--check` takes 0.02 s instead of 1.9 s and `--health` 0.09 s instead of
  6.1 s (the script ran one `git grep` per contract). check-ground-truth
  computes the `metric.<key>.*` DSL as the script's `find` does (depths,
  pruning, symlinks not followed, `contains`, `distinct`) and compares
  METRICS the same way, `--print` and the tier gate included, with 3.1's
  `metrics_strict` (env `REBAR_METRICS_STRICT`; no declarations fail too, and
  a strict failure that is not drift ends "Ground truth not verified"),
  `metric.<key>.tracked` (git
  tracked files and the directories holding one) and colour only on a
  terminal without `NO_COLOR` (`checks.UseColour`, shared by the native
  checks that colour). The expected outputs live as golden files
  in `cli/cmd/testdata/`, so the coverage outlives the scripts. Platform
  quirks are settled one way: a status right after its colon
  (`Status:stable`) keeps its first letter (BSD sed read `\s` as `s`), and a
  binary contract file is a draft (BSD grep's "Binary file … matches" became
  its status). `rebar new` writes the new project's registry with the
  native compute-registry.
- **Native (Go) checks are the default for seven checks.** check-todos and
  six more ported scanners, check-freshness, check-contract-refs,
  check-registry, check-contract-headers, check-prefix-uniqueness and
  check-jtbd-presence, now run in the `rebar` binary unless
  `REBAR_CHECK_IMPL=shell` (the scripts stay until the end of the stack).
  Same arguments, `.rebarrc` keys, tier gate (`REBAR_TIER`, `checks_enable`,
  the `SKIP:` / `NOTE:` lines), output and exit codes; on virtru-analyze the
  six scanners take 0.2 s instead of 19 s (check-contract-refs and
  check-registry spawned processes per match). Where the scripts depended on
  the platform, the native checks pick one behaviour: findings are listed in
  file-name order (find and grep -r used directory order) and contracts,
  registry IDs and JTBD findings in byte order whatever the locale (bash globs
  and `sort` used its collation); check-freshness treats a date that does not
  exist (`2025-02-30`) as GNU date does on every platform (counted, not aged;
  BSD date rolled it into the next month and could report it stale) and a
  binary `.md` with a marker as macOS did (counted, not aged; GNU grep 3.5+
  made it unmarked); and check-contract-refs / check-registry select files as
  check-todos does (below). The scripts' quirks are kept: `tier = 1 # note`
  in `.rebarrc` still reads as tier 3, and contract IDs are still matched as
  patterns (a `.` matches any character). The script tests pin
  `REBAR_CHECK_IMPL=shell`; `REBAR_TEST_NATIVE_BIN` runs them against the
  binary.
- **Experimental: native (Go) checks behind `REBAR_CHECK_IMPL=go`.** First
  PR of a stack that ends with a Go-binary-only rebar (proposal and plan:
  `feedback/2026-11-14-go-native-checks.md`). Here the default stays `shell`
  (the entry above makes `go` the default) and nothing changes unless the
  variable is set. With `REBAR_CHECK_IMPL=go`,
  `rebar check` (all modes) and `rebar run check-todos` use a Go port of
  check-todos.sh; `rebar run` works for it even without a rebar install. Only
  the install's scripts hand off to the binary: a project's own copy of a
  check runs itself, customizations included, even when it was copied from a
  script with the hand-off hook. Same output and exit code, except where the
  script depended on the platform's grep: findings are listed in file-name
  order (grep -r used directory order); a TODO on a line that is not valid
  UTF-8 is reported whatever the locale; `.rebarrc source_exclude_dirs`
  patterns follow GNU grep on macOS too (a pattern with a `/` matches an
  operand, not a directory below one; a trailing `/` is ignored); a symlinked
  source directory is scanned on macOS too (BSD grep skipped it); and a file
  is binary when its first 32 KiB hold a NUL, as macOS grep decides, also on
  Linux (GNU grep checked a larger first block and stopped at a later NUL). `config.RCValue` now honors
  `REBAR_RCFILE`, as the scripts do.

---

## v3.1.0 — 2026-11-16

**Adopters carry configuration, not rebar's scripts.** Checks run from a
versioned, self-contained rebar install (`rebar check`, `rebar run <check>`);
everything repo-specific lives in `.rebarrc`
([docs/CONFIGURATION.md](docs/CONFIGURATION.md)); `rebar migrate` converts a
project's copied, even customized, scripts into settings, verified by running
them; and the installer keeps versions side by side under `~/.rebar/versions/`,
installing checksum-verified prebuilt downloads. Upgrade with
[UPGRADING.md](UPGRADING.md); CI needs a full rebar tree, not just the binary
([SETUP.md § Run rebar in CI](SETUP.md#run-rebar-in-ci)).

Field-tested on virtru-analyze: upgrades from v2.0.0 and v3.0.0-beta.4 copies,
and the adopter's own CI running a pinned 3.1 tree. The full list of changes is
in the release candidates below; since rc.3:

- **`rebar new` writes the contract registry,** so a new project passes its
  first `rebar check`. The registry check runs at every tier and failed with
  "CONTRACT-REGISTRY.md does not exist" (15/16) until the user ran
  `rebar run compute-registry`; the registry is now generated before the
  initial commit and included in it.
- **`metrics_strict = true` also fails when `.rebarrc` declares no metrics.**
  In rc.3, a strict check with no `metric.<key>.*` lines passed ("No metrics
  declared"), so deleting the declarations silently turned enforcement off.
  A strict failure for a missing `METRICS` file or missing declarations now
  ends with "Ground truth not verified" instead of the misleading "Metric
  drift detected".

---

## v3.1.0-rc.3 — 2026-11-16

Third release candidate for 3.1.0, still a **pre-release** (default "latest"
stays `v3.0.0`). Fixes from running rc.2 in a real adopter's CI
(virtru-analyze #457), and the released-blobs CI guard (#30):

- **Root flags before a pass-through command work:** `rebar --repo-root X run
  check-todos` (and `-v`, `--json`) failed with `no script "--repo-root"`,
  because `run` takes its arguments verbatim and cobra handed it the root
  flags too. Root flags before `run`, `ask` or `push` are now parsed by rebar;
  for `run` also between `run` and the script name. Everything after the
  script name (for `ask`/`push`, after the command name) still passes through
  untouched, so `rebar ask -v …` keeps meaning ask's own `-v`.
- **SETUP.md's pinned CI wrapper no longer leaks a temp clone per cold run.**
  It set `trap 'rm -rf "$tmp"' EXIT` and then `exec`'d the pinned rebar, which
  replaces the shell before any EXIT trap can run. It now removes the temp dir
  (and drops the trap) once the tree is in place. If you copied the snippet,
  add `rm -rf "$tmp"; trap - EXIT` after its `mv`. A new test runs the snippet
  straight from SETUP.md.
- **`rebar migrate` keeps a `check-ground-truth.sh` that callers source for
  other functions.** A file sourcing the copy got the thin wrapper, which
  defines only `compute_metrics()`, so a caller using the copy's `count_*`
  helpers (virtru-analyze's `rebar-fix.sh`) would break. migrate now keeps
  the copy, names the caller and functions, and notes how to migrate the
  caller (read `rebar run check-ground-truth --print`). It also recognizes
  `. "$(dirname "$0")/x.sh"` as sourcing (a path with spaces inside a
  command substitution was missed).
- **`.rebarrc` `metrics_strict = true`** (env `REBAR_METRICS_STRICT`) makes
  `check-ground-truth` fail when `METRICS` is missing and when a declared
  metric has no `METRICS` line; by default these still pass (SKIP, NEW).
  `rebar migrate` now also compares the copy and the install in those two
  cases: it proposes `metrics_strict = true` when the copy failed both (as
  virtru-analyze's did; rc.2 judged them equivalent and dropped the
  failures), and keeps a copy that fails only one of them.
- **`metric.<key>.tracked = true`** counts only files git tracks (for
  `type = dirs`, directories holding a tracked file), so gitignored or
  untracked local files no longer inflate a count such as `shell_scripts`.
  Outside a git work tree it has no effect (every file counts, with a note on
  stderr). `rebar migrate` reads and keeps the field on metrics already in
  `.rebarrc`; it does not derive it from a legacy copy.
- **Colour only on a terminal:** `check-ground-truth`, `refresh-context` and
  `test-e2e-live` printed ANSI colour codes into CI logs and pipes. They now
  colour only when stdout is a terminal and `NO_COLOR` is unset.
- **Release branches can't go stale for the release workflow:** CI now runs
  the release workflow's released-blobs coverage test against the head of a
  `release/**` branch on every push (#30), so a merge that leaves
  `released-blobs.txt` incomplete fails at once, not at tag time (rc.2's
  first tag did).

---

## v3.1.0-rc.2 — 2026-11-15

Second release candidate for 3.1.0, still a **pre-release** (default "latest"
stays `v3.0.0`). One fix found by testing rc.1 on a real adopter:

- **`refresh-context.sh` no longer dies with exit 141** in a repo with many
  commits since QUICKCONTEXT was last synced. It listed them with
  `git log | head -20` under `pipefail`, so git got SIGPIPE and `set -e` ended
  the script; it now uses `git log -n 20`. `test-e2e-live.sh` dropped the same
  pattern (`git for-each-ref --count=1`, `find -print -quit`).

---

## v3.1.0-rc.1 — 2026-11-15

First release candidate for 3.1.0, a **pre-release**: the installer's default
"latest" stays on `v3.0.0`; install it with an explicit version
(`curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.0-rc.1/setup-rebar.sh | bash -s -- v3.1.0-rc.1`)
or `--pre`. The theme is **adopters carry configuration, not rebar's
scripts**: the checks run from a versioned, self-contained install, everything
repo-specific moves into `.rebarrc`, and `rebar migrate` converts existing
copied (even customized) scripts, verified by running them. Upgrade steps:
[UPGRADING.md](UPGRADING.md).

- **Implementing-file counts are off by default** (`.rebarrc`
  `registry_impl_counts`, now `false`): the committed registry has no
  `Impl Files` column and no Contract Health section, because any PR that adds
  a `CONTRACT:` header changes them and two such PRs merge into a wrong count.
  `rebar run compute-registry --health` prints them. **Upgrade note:** a
  registry with counts reports STALE once — regenerate it (UPGRADING.md step
  5), or set `registry_impl_counts = true` to keep them. `rebar migrate`
  follows the new default and regenerates unless the project opted in.
- **The registry's regenerate hint names the CLI:** `<!-- Regenerate: rebar run
  compute-registry -->` (it said `./scripts/compute-registry.sh`, which adopters
  no longer have). `compute-registry.sh --check` ignores that comment, as it
  ignores the old timestamp, so a registry with the old hint stays current and
  picks up the new one when next regenerated; `rebar migrate` compares
  registries the same way.
- **The source scanners skip binary files** (`grep -I`). A matching binary was
  reported as "Binary file … matches" and read as a reference or a TODO — a
  built `bin/rebar` that embeds `CONTRACT:` strings made the registry STALE.

- **Upgrading:** [UPGRADING.md](UPGRADING.md) is the runbook for moving an
  adopting project from v2.0.0 or v3.0.x (install side by side, bump
  `.rebar-version` and the badge, `rebar migrate`, registry, `checks_enable`,
  integrity manifest, CI, verify). Install a release candidate with its own
  tag's installer:
  `curl -fsSL https://raw.githubusercontent.com/willackerly/rebar/v3.1.0-rc.1/setup-rebar.sh | bash -s -- v3.1.0-rc.1`
  ([SETUP.md § Testing a release candidate](SETUP.md#testing-a-release-candidate)).
  CI must install a full rebar tree, not just the binary
  ([SETUP.md § Run rebar in CI](SETUP.md#run-rebar-in-ci)).
- **`rebar migrate`** turns an adopter's legacy, possibly customized,
  script copies into `.rebarrc` settings, decided by running them: each
  `compute_metrics()` line of a custom `check-ground-truth.sh` becomes
  `metric.<key>.*` only if the install computes the same value, on the
  project and on a probe tree built from the metric; registry settings are
  kept only if `compute-registry.sh --stdout` stays byte-identical
  (`registry_purpose_style = bytes` when the old cut is needed); other checks
  must give the same exit status and findings, and the source scanners also
  on a probe tree with exclusion bait. All probes run in a scratch copy of
  the project, and a copy that writes files is kept. Dry run by default;
  `--apply` refuses a dirty work tree (`--allow-dirty`), writes `.rebarrc`
  atomically, deletes only verified copies (a copy other files call becomes
  a `# rebar-thin-wrapper` instead, so callers keep working), adjusts METRICS
  counts the deletions change, reruns the checks and rolls everything back on
  a regression. It refuses symlinked `scripts/` outside the project and any
  rebar checkout, ignores `REBAR_*` in the environment, never changes `tier`,
  and keeps checks whose copies ran ungated below the install's tier gate
  running with a proposed `checks_enable`. Unmodified copies are recognized
  from a released-blobs list embedded in the CLI
  (`scripts/gen-released-blobs.sh`, a release step), so installs without git
  history work. A scanner copy that is a released version plus settings
  (its directories, extensions) is migrated even though the install now
  excludes directories by name (#19), with the newly reported findings
  listed before `--apply`; old substring filters map to
  `source_exclude_dirs`. The CLI now honors `REBAR_PROJECT_ROOT` for repo
  detection, as the scripts do.
  `check-ground-truth.sh --print` and `REBAR_RCFILE` support it. Design:
  `feedback/2026-11-14-rebar-migrate.md`.
- **CLI errors are printed.** A failing command printed nothing and exited 1
  (cobra's printing was off and `main` didn't print), so hints like
  "rebar install not found (set REBAR_ROOT …)" never reached the user. They now
  go to stderr as `rebar: <error>`.
- **`rebar commit` drops manifest entries for staged deletions** (a rename
  counts as delete + add). They used to stay, so `rebar verify --strict`
  reported the files as Missing — e.g. after `rebar migrate --apply`.
- The registry check's hint says `rebar run compute-registry` instead of the
  project-local `scripts/compute-registry.sh`.

- **Versioned installer.** `setup-rebar.sh [<version>|latest|<branch>]`
  installs side by side under `~/.rebar/versions/<version>/`, with
  `~/.rebar/current` pointing at the active one and `~/.rebar/current/bin`
  on PATH. "latest" (the default) is the highest stable tag (the highest
  pre-release only while no stable tag exists); `--pre` includes
  pre-releases. Tags download a
  prebuilt tarball verified against the release's SHA-256 checksums, and
  build from source (git + Go) when the release has no assets; branches and
  full commit SHAs always build from source, replacing the committed darwin/arm64
  `bin/rebar`. `--pre`, `--no-switch`, `--source`. An existing `~/.rebar` clone is
  left untouched; versions are added inside it. Design:
  `feedback/2026-11-14-versioned-installer.md` (from #8).
- **Release workflow.** Pushing a `v*` tag builds darwin/linux ×
  amd64/arm64 with `scripts/package-release.sh` and publishes a GitHub
  Release (a pre-release for suffixed tags) with the tarballs and
  `rebar_<ver>_checksums.txt`.
- **`rebar version` can be stamped:** `-ldflags "-X
  github.com/willackerly/rebar/cli/cmd.Version=<ver>"`; branch builds from
  the installer report `<branch>@<sha7>`. README badges written by `rebar
  new` / `rebar adopt` still use the release string.
- **Adopters carry configuration, not rebar's scripts.** `rebar check`,
  `rebar check --pre-commit|--cold-start` and the new `rebar run <script>`
  run the rebar install's scripts against the project
  (`REBAR_PROJECT_ROOT`). The bootstrap template's `scripts/` is down to
  two `# rebar-thin-wrapper` entry points. Legacy project copies keep
  running, with a deprecation notice. Design and migration:
  `feedback/2026-11-14-self-contained-install.md` (from #8).
- **Repo-specific settings in `.rebarrc`:** declarative METRICS
  (`metric.<key>.*`, same `=` / `>=` semantics), registry classification
  and purpose settings, `source_dirs`, `source_extensions`,
  `freshness_max_days`.
- **`contracts_dir`** (default `architecture`) moves the contracts,
  registry and steward state anywhere in the repo; honored by every script
  and the CLI.
- **Registry purposes are cut cleanly by default** (`registry_purpose_style
  = ellipsis`): `|` is escaped so it can't break the table, and long
  purposes are cut to 77 characters plus `...` instead of at byte 80, which
  could split a multibyte character. **Upgrade note:** a registry generated
  with the old cut reports STALE once; regenerate it, or set
  `registry_purpose_style = bytes` to keep the old output.
- **`checks_enable`** (`.rebarrc`, env `REBAR_CHECKS_ENABLE`) runs named
  checks regardless of the tier, each as it would run at tier 3:
  `tier = 1` + `checks_enable = check-ground-truth` runs ground truth.
  Names are check script basenames without `.sh`; an unknown name warns
  once per run and is ignored. `rebar status` and `rebar audit --all` show
  the enabled checks. Unset, nothing changes. The registry check was never
  tier-gated (docs said tier 2+); they now say it runs at every tier.
- **`contract_namespace`** (optional, e.g. `github.com/org/repo`): source
  may cite this repo's contracts as `CONTRACT:<namespace>:<ID>.<v>` as well
  as the short form. Steward, compute-registry, check-contract-refs,
  check-registry and `rebar context developer` treat both alike
  (check-contract-headers only shows the qualified form in its hint); references
  qualified with another repo's namespace are counted, never treated as
  ours. Filenames stay unnamespaced; composes with `contracts_dir`; unset
  behaves exactly as before. From #8; see
  `feedback/2026-11-14-contract-namespace.md`.
- **Fix: the source scanners skipped too much.** check-todos, check-contract-refs,
  check-registry and compute-registry's non-git fallback dropped any match whose
  path or line text contained `dist`, `vendor`, `node_modules` or `.git`
  (a broken reference under `.github/` or in `distance.go`, a TODO mentioning
  `TODO.md`, every match when the project's own path contained `dist`). They now
  skip directories by name: `.rebarrc` `source_exclude_dirs` (default
  `node_modules vendor dist`) plus `.git` and `.claude` (agent worktrees, which
  were scanned before). `contracts_dir` is normalised (`./docs/specs/` →
  `docs/specs`); check-registry also scans `*.sh` and `bin/` and skips
  `CONTRACT-SEAM-TEMPLATE.md`. **Upgrade note:** checks can report findings
  they used to miss — e.g. TODOs in compiled `dist-esm/` output now block
  pre-commit; add such directories to `source_exclude_dirs`.
- **Fix:** `check-registry.sh` stopped silently (exit 1, no ORPHAN line)
  at the first contract with no implementing file.
- `sync-bootstrap.sh --check` also keeps the bootstrap template's
  `CONTRACT-TEMPLATE.md` identical to `architecture/`.

---

## v3.0.0 — 2026-11-14

Graduation of the v3 line. The four annealing betas (`v3.0.0-beta`,
`-beta.2`, `-beta.3`, `-beta.4`) shipped on `main`, honestly labeled; this
release drops the pre-release suffix. One fix since `v3.0.0-beta.4`; otherwise
only version strings, the rebuilt `bin/rebar`, and the recorded decision.

- **`rebar new` projects passed their own compliance check again.** `rebar new`,
  `rebar init` and `rebar audit --fix` wrote a hardcoded `v3.0.0-beta` into
  `.rebar-version` while the README badge used the CLI's version, so a fresh
  project failed `check-compliance.sh` ("Badge says v3.0.0 but .rebar-version
  says v3.0.0-beta"). All three now write the CLI's version, and the bootstrap
  template's `.rebar-version` and badge say `v3.0.0`. Broken since beta.3.

**What v3 is:** badges that stop lying by default. Contracts declare their
maturity (`Status:` stub → verified) and the compliance badge is weighted by
it; the steward's computed `verified` is renamed `impl-present` so the two
vocabularies never share a word; a SessionStart hook makes the cold start a
harness fact; and the field-proven peer-inbox paradigm, Claude Skills, and
abstract `rebar:` refs ship alongside. Details and upgrade notes are in the
beta entries below, starting with `v3.0.0-beta`.

**Graduation decision.** Decided by the project owner on 2026-11-14, not
measured to completion. Of the four criteria in `docs/v3-beta-plan.md`
§Graduation, criteria 2, 3 and 4 were met on 2026-08-02. Criterion 1 (two
migrated repos clean for ≥2 weeks) had its repo count met and a clean window
expected to close 2026-08-16, but it was never re-verified, and beta.3 and
beta.4 shipped changes after that date. It was accepted by the owner's
decision, recorded as D12 in the plan's decision log.

---

## v3.0.0-beta.4 — 2026-11-14

Installer fix. **Not** graduation; the §Graduation scoreboard was not
rescored.

- **`setup-rebar.sh`:** the one-line `curl … | bash` install failed on a
  stock Mac. `/bin/bash` 3.2 treats an empty `"${install_args[@]}"` as
  unbound under `set -u`, so the installer died after cloning unless
  `--server` was passed. Present since at least beta.2 (#12).
- `bin/rebar` rebuilt so `rebar version` reports `v3.0.0-beta.4`.

---

## v3.0.0-beta.3 — 2026-11-14

Third annealing increment on the v3 line, driven by an adopter
(virtru-analyze) whose parallel PRs conflicted on rebar's generated files.
**Not** graduation: the `docs/v3-beta-plan.md` §Graduation scoreboard was not
rescored for this release.

### Merge-friendly generated files for busy repos

Parallel PRs no longer conflict on rebar's generated files when they change
unrelated things. See `practices/busy-repos.md`.

- **`rebar commit` / `rebar sign`** no longer restamp `generated_at` /
  `generated_by` in `.rebar/integrity.json`; the header is written once by
  `rebar init`. `rebar status` shows it as "Created".
- **Ratchet floors are derived** from per-file entries instead of a stored,
  constantly raised `min`. Legacy stored floors stay as a frozen lower bound;
  `current` is no longer written.
- **`compute-registry.sh`** drops the `Generated:` timestamp and pins
  `LC_ALL=C`, so output is identical on every machine. New `.rebarrc` key
  `registry_impl_counts = false` (env `REBAR_REGISTRY_IMPL_COUNTS`) keeps the
  code-derived `Impl Files` column and Contract Health section out of the
  committed registry; `--health` prints them.
- **`compute-registry.sh`** counts implementing files over tracked files only
  (`git grep`) in a git work tree, so untracked scratch files and gitignored
  build output no longer change the registry; outside git it falls back to
  `grep -r`. The exclusions now match path components instead of substrings:
  only the top-level `architecture/` is skipped (a nested
  `website/src/pages/architecture/` file now counts), as are `node_modules/`,
  `vendor/` and `dist/` directories. Counts can shift once on regeneration.
- **`check-ground-truth.sh`** accepts `key >= N` floors in `METRICS`.
- **`_rebar-config.sh`** gains `_rebar_config <key> <ENV> <default>`.
- Regression tests: `scripts/tests/merge-friendly.test.sh`,
  `cli/internal/integrity/manifest_test.go`.

### Release housekeeping

- `rebar version` now reports the release (`v3.0.0-beta.3`). It had stayed
  at `v3.0.0-beta` through beta.2. `rebar new` and `rebar adopt` stamp the
  README badge from the same constant.
- `bin/rebar` rebuilt from this source (darwin/arm64).

---

## v3.0.0-beta.2 — 2026-08-02

Second annealing increment on the v3 line. **Not** graduation to `v3.0.0` —
scored against the four criteria in `docs/v3-beta-plan.md` §Graduation on
2026-08-02: criterion 2 met (a live seat armed the canonical watcher in real
cross-repo traffic and caught a genuine inbound); criterion 1 met on repo count
(`office180`, `go-tak-server`) but its clean window reset the same day; criterion
3 not met (three v3-feature defects filed 2026-08-02); criterion 4 unverified. No
criterion was amended or skipped — per D1, the tag is earned, not declared.

### Changed

- **Contract prefix numbering retired — the full descriptive name is the key.**
  `CONTRACT-BLOBSTORE.2.1.md`, not `CONTRACT-C1-BLOBSTORE.2.1.md`. The number was
  bookkeeping that collided on merge and broke every `CONTRACT:` reference when
  resolved; the type letter duplicated the contract's own `Type:` field. Verified
  by probe that the tooling never required either — `compute-registry.sh`,
  `check-contract-refs.sh`, `check-prefix-uniqueness.sh`, and `check-registry.sh`
  all accept unnumbered IDs unchanged. **Relaxation, not migration:** existing
  numbered contracts stay, never renumber, never mass-rename. Promoted to a
  top-level README callout. (`README.md`, `conventions.md` §Contract Naming,
  `architecture/README.md`, `CONTRACT-TEMPLATE.md`, `DESIGN.md`,
  `practices/contract-supersession.md`, bootstrap copies.)
- **Peer-inbox memo grammar reconciled** to the TYPE-bearing form
  (`YYYY-MM-DD-<sender>-<TYPE>-<description>.md`) that
  `practices/inbox-protocol.md` and the user-level skill already taught;
  `conventions.md` had diverged. The date is now normatively the sender's
  **local** deposit date — agents read UTC out of tooling, which west of
  Greenwich silently dates a memo tomorrow and breaks the arrival-order sort.
  Deposited memos are never renamed.
- `check-prefix-uniqueness.sh` marked **LEGACY-ONLY** in both copies. Behavior
  unchanged — a collision between two still-numbered contracts is still a real
  defect — but a passing result must not be read as "numbering is required."

### Added

- **`practices/inbox-protocol.md`** — the memo grammar itself: naming, the ten
  memo types, `[M]/[V]/[S]/[R]/[U]` confidence labels, crossed-memo handling, and
  the memo-vs-synchronous-ask rule. Distilled from the TDFLite ↔ nanotdf2
  exchange; complements `inbox-watch.md` (how you learn a memo arrived) and
  `federation.md` (the principles). Recommends a short root `INBOX.md` per repo,
  since coordination is load-bearing from a session's first minute.
- **`practices/diagram-and-document-assets.md`** — rebar's first diagram
  convention. A standalone `.pptv.svg` atom is the default when a figure outgrows
  an ASCII fence; markdown + `md2docx` when a document must open in Word. Adopted
  from `office180-md-office-converter` and pinned to `0.1.0-alpha.4`; the alpha
  pin is bounded because an atom degrades to a plain renderable SVG if the tooling
  disappears. Verified end to end before adoption.
- **Federation Principle 5 — "a held inbox is a watched inbox"**
  (`practices/federation.md`; owner-ratified 2026-07-11). Every repo
  holding a peer `inbox/` arms the **same canonical watcher**
  (`scripts/inbox-watch.sh`) on its **own** inbox at session start; no
  per-repo watcher variants. Generalizes the 2026-07-06 field SOP
  (own-inbox-only, stale-watcher check) from
  `feedback/2026-07-06-inbox-watch-self-echo-scope-to-own-inbox.md`.
- `scripts/inbox-watch.sh`: arm-time Principle-5 checks — a **per-inbox PID
  lock** (`.inbox-watch.lock`, a hidden dotfile invisible to the watch) that
  warns only when another **live** watcher already holds the **same** inbox
  (real double-coverage), and reclaims a crashed watcher's stale lock silently;
  plus an own-inbox scope warning when a watched dir resolves outside the
  current repo. Warn-only; never blocks. The lock replaced an earlier
  process-global `pgrep inbox-watch` check that false-positived on every
  legitimate sibling seat in a single-machine, multi-seat setup (go-tak-server
  / _atlas, 2026-07-11) — cry-wolf that defeats the warning's purpose. Lock is
  git-ignored (bootstrap + rebar). Template copy synced byte-identical
  (`templates/project-bootstrap/scripts/inbox-watch.sh`).
- Project bootstrap now ships a top-level `inbox/` (with a one-line
  README) so adopter repos are born holding a peer inbox.
- Peer-Inbox Convention (`conventions.md`): Principle-5 bullet and an
  optional `reply-by:` frontmatter field (courtesy deadline signal for
  coordination seats; absence means human-ish timescales).
- `practices/inbox-watch.md`: Scope SOP section (own inbox only, stale
  watchers); `practices/session-lifecycle.md` step 4 aligned.

### Fixed

- `scripts/inbox-watch.sh`: **full-backlog re-emit on inbox git operations.**
  The per-poll baseline update replaced the seen-set with the current listing
  (`mv cur snap`), so a git op (merge/checkout/stash) that transiently
  removes-and-restores tracked inbox files reset the baseline to the empty/
  partial mid-operation state — and the whole backlog then re-diffed as new on
  the next poll. Now the seen-set is an **append-only ledger** (`sort -u` union
  each poll): a filename, once reported, stays reported for the watcher's life,
  so transient disappearance-and-return cannot re-notify. Field-reported by the
  go-tak-server seat (2026-07-11, one 8-file burst); reproduced and regression-
  tested (old logic emits the backlog, new logic emits only genuine deposits).
  Matters under Principle 5: a fleet-wide re-emit would train every seat to skim
  NEW-deposit events — the exact reflex the principle exists to prevent.
  Trade-off (correct for the inbox convention): a deleted-then-recreated
  same-name file will not re-notify; inbox names are dated, unique, append-only.
  Template copy synced byte-identical.
- `scripts/inbox-watch.sh`: **collation follow-up to the ledger fix — phantom
  re-emit every poll under a non-C locale.** The first ledger fix unioned with a
  bare `sort -u`, which re-collates under the ambient locale, while `list_dir`
  emits `LC_ALL=C`-sorted listings. With mixed-case filenames (the memo norm),
  the ledger and the current listing then sorted differently and `comm` merge-
  walked two mis-ordered files, reporting a stable subset as new **every poll,
  permanently** — worse than the git-op bug it replaced. Now `LC_ALL=C` is
  pinned on both the union sort and the `comm` walk, so every sort/merge over
  these files shares the one C collation. Field-reported same day by the
  go-tak-server stress rig; the earlier regression test missed it because its
  fixtures were lowercase-only (identical under C and locale collation).

### Added

- `scripts/tests/inbox-watch.test.sh`: regression guard for both inbox-watch
  bugs above. Uses mixed-case, digit-heavy fixtures and forces a divergent
  UTF-8 locale (skips loudly if none is installed, rather than passing falsely
  under C), then asserts a full lifecycle — armed baseline silent, one emit per
  genuine deposit, silent through a transient remove/restore — emits **exactly**
  the genuine deposits. Confirmed to fail against both pre-fix revisions.
- `scripts/tests/inbox-watch-lock.test.sh`: regression guard for the per-inbox
  lock — asserts the lock is written with the watcher's PID and hidden from
  `ls -1`, a second watcher on the same inbox warns and does not steal it, a
  clean exit releases it, and a stale (dead-PID) lock is reclaimed silently.

## v3.0.0-beta (2026-07-04)

Seven clusters consolidated onto one trunk — the alpha→beta hard move.
Decision log: `docs/v3-beta-plan.md`. The `v3.0.0-alpha`/`v3.0.1-alpha`
branches are **retired** (kept on origin for provenance; all their
content is on this line).

### Breaking (migration notes for upgraders)

1. **Steward's computed top lifecycle state renamed `verified` →
   `impl-present`** (S1-STEWARD 2.0). Anything parsing steward output
   must migrate: JSON `summary.contracts.verified` →
   `summary.contracts.impl_present`; report row and `--summary` suffix
   (`Xv` → `Xip`) likewise. Rationale: computed-from-file-presence
   "verified" overclaimed (`feedback/2026-06-19-trustable-status-…`).
2. **Contracts carry a declared-maturity `Status:` header** —
   `stub | draft | in-progress | active | verified` (canonical
   definitions: `conventions.md` §Declared Maturity). The old
   `draft | active | deprecated` template vocabulary is replaced;
   `superseded`/`deprecated`/`retired` are terminal and excluded from
   weighting. `check-compliance.sh` Check 9 weights the tier badge:
   <33% stub-or-draft → as declared; 33–66% → "— IN PROGRESS"
   annotation; >66% → demoted one tier (exit 1). **Repos with no
   `Status:` fields are treated as pre-v3: advisory only, no penalty** —
   upgrade by adding one line per contract, honestly.
3. **SessionStart hook expectation.** `.claude/settings.json` runs
   `scripts/cold-start-checks.sh` on session start (guarded — prints an
   install pointer if the script is absent). `rebar init/new/adopt` now
   install the hook, the four skills, and the core scripts.
   Adopting by copy: use `cp -r templates/project-bootstrap/. <target>/`
   — the old `/*` glob silently dropped `.claude/`.
4. **`ci-check.sh` gained two unconditionally-registered checks**
   (`check-jtbd-presence.sh`, `check-prefix-uniqueness.sh`) and
   `check-decay-patterns.sh` gained the P8 demo-bypass pattern — Tier-3
   repos that were green on v2 can newly fail. Escape hatches while you
   migrate: `SKIP_JTBD=1`, `SKIP_PREFIX_UNIQUENESS=1`; P8 findings mean
   a demo/UAKS spec is faking user actions — fix the spec.

### Added

- **Peer-inbox paradigm** (field-verified, tak cluster 2026-07): repo-level
  `inbox/` convention in `conventions.md`, `scripts/inbox-watch.sh`
  persistent watcher, coordination-seat cold-start hygiene
  (`practices/session-lifecycle.md` step 4, `practices/inbox-watch.md`)
- **Claude Skills packaging**: `rebar-coldstart`, `rebar-feedback`,
  `rebar-audit`, `rebar-inbox-watch` in `.claude/skills/` + bootstrap
  template — skills are pointers to practices, never copies
- **`scripts/cold-start-checks.sh`**: enforcement quad + maturity counts,
  `<rebar-cold-start>` wrapped, always exits 0
- **`practices/test-fidelity.md`**: five-rung fidelity ladder, UAKS tier,
  closed-loop demo gate; `check-decay-patterns.sh` P8-demo-bypass
- **`agents/FANOUT_PATTERN.md`** + `subagent-guidelines.md` §Verify
  Before Relying
- **`practices/spike-first-contracts.md`**, **`practices/contract-supersession.md`**,
  **`scripts/check-jtbd-presence.sh`**, **`scripts/check-prefix-uniqueness.sh`**
  (both checks wired into `ci-check.sh` — now 15 checks)
- **conventions.md**: Declared Maturity, Peer-Inbox Convention, and
  "Reaching the Agent: Hook vs Skill vs Ask" doctrine

### Changed

- `conventions.md` computed-lifecycle table: `VERIFIED` → `IMPL-PRESENT`
  with rename note
- `installer`: thin curl|bash shim over the canonical CLI (carried from
  the alpha line, #2)

---

## v2.0.1 (2026-04-26)

### Fixed
- **`bin/ask` resilient session resume** — when `claude --resume <id>` returns "No conversation found with session ID …" (vanished `~/.claude` store, interrupted first call, server-side retention boundary), `_ask_direct` now wipes the stale `.session-id`, regenerates a fresh UUID, and replays the question once with full agent context. Prints `[<agent>] session expired, restarting fresh` to stderr so the recovery is visible. Retry is capped at one attempt and only fires on the specific stale-session signature — auth/network/rate-limit errors still surface immediately.

---

## v2.0.0 (2026-04-01)

### Added
- **Session lifecycle protocol** — `practices/session-lifecycle.md`: start/checkpoint/end framework with wrapup template, marathon session guidance, architect review checkpoints
- **Red team protocol** — `practices/red-team-protocol.md`: 5-persona adversarial review strategy (adversarial user, performance, security, fidelity, API/contract)
- **Visual fidelity methodology** — `practices/visual-fidelity.md`: ground truth, RMSE measurement, oracle pattern, human emulator tests, regression prevention
- **Red team subagent template** — `agents/subagent-prompts/red-team.md`: multi-persona template with structured JSON output and fix DAG
- **Product review subagent template** — `agents/subagent-prompts/product-review.md`: BDD alignment, persona fit, flow completeness, scope assessment
- **Seam contracts** — new contract type for integration points across language/protocol boundaries; `architecture/CONTRACT-SEAM-TEMPLATE.md`
- **`rebar context` CLI command** — context shepherd that cats role-relevant files in reading order (`rebar context`, `rebar context architect`, `rebar context session-start`, etc.)
- **Context refresh script** — `templates/project-bootstrap/scripts/refresh-context.sh`: automated QUICKCONTEXT staleness checker
- **Feedback status tracking** — `feedback/README.md` now includes `Status:` and `Template impact:` fields

### Changed
- **Cold Start Quad enhanced** — staleness verification step added between QUICKCONTEXT and TODO reads
- **AGENTS.template.md** — session lifecycle reference, priority tracking rule (QUICKCONTEXT is single source of truth), issue dedup rule
- **CLAUDE.template.md** — session-end protocol added, `refresh-context.sh` in health check
- **TODO.template.md** — forward-looking only (<50 lines open items), completed items in collapsed `<details>` section
- **QUICKCONTEXT.template.md** — "What's Next" section as canonical priority list
- **Worktree collaboration** — fan-out merge ordering strategy (HOT/WARM/COLD), shared mock consolidation rule, worktree lifecycle checklist, cherry-pick best practices
- **Multi-agent orchestration** — file-level conflict matrix and interface-change sequencing in pre-launch audit
- **UX review template** — interaction stability (human emulator) dimension added
- **Security surface scan template** — red team mode (adversarial mindset) section added
- **DESIGN.md** — seam contracts in §3 (The Contract System), session lifecycle in §5 (The Information Environment)
- **README.md** — battle-tested results updated with OpenDocKit (15+ agents, 8K tests) and filedag (40+ agents, 62 commits in 48hrs)

### Why v2.0

Three field reports from production deployments (Dapple SafeSign, OpenDocKit, filedag) converged on the same meta-insight: rebar's structural protocols work; its behavioral protocols don't. v2.0 addresses the session lifecycle gap, adds adversarial review and visual fidelity practices, introduces seam contracts for integration points, and adds the `rebar context` CLI command. The core contract system is unchanged — this is a methodology evolution, not a rewrite.

### Migration from v1.2.0
1. Copy new practice files: `practices/session-lifecycle.md`, `practices/red-team-protocol.md`, `practices/visual-fidelity.md`
2. Copy new subagent templates: `agents/subagent-prompts/red-team.md`, `agents/subagent-prompts/product-review.md`
3. Copy `architecture/CONTRACT-SEAM-TEMPLATE.md`
4. Copy `templates/project-bootstrap/scripts/refresh-context.sh` to your project's `scripts/`
5. Re-diff your AGENTS.md against the new template — session lifecycle and priority tracking sections are new
6. Re-diff your CLAUDE.md — session-end protocol and staleness verification are new
7. Consider shortening your TODO.md — move completed items to a collapsed section
8. Add a "What's Next" section to QUICKCONTEXT.md as your canonical priority list
9. Rebuild rebar CLI: `cd cli && go build -o ../bin/rebar .`
10. Update `.rebar-version` to `v2.0.0`

---

## v1.2.0 (2026-03-20)

### Added
- **Scalability overhaul** — AGENTS.template.md slimmed from 917 to 382 lines
- **`practices/` directory** — multi-agent orchestration, E2E testing, deployment patterns, worktree collaboration extracted as reference guides
- **Computed registry** — `scripts/compute-registry.sh` generates CONTRACT-REGISTRY.md from contract files on disk (replaces manual maintenance)
- **Memory compaction** — `ask compact <agent>` summarizes old memory entries, auto-triggers at 50KB threshold
- **Script versioning** — all scripts have `# rebar-scripts: YYYY.MM.DD` headers; steward detects stale copies
- **Tier-aware enforcement** — `.rebarrc` configures tier (1=partial, 2=adopted, 3=enforced); scripts skip inapplicable checks
- **Team-size profiles** — `profiles/solo-dev.md`, `profiles/small-team.md`, `profiles/department.md`
- **Conventions minimum viable** — Tier 1 section at top of conventions.md
- **Rebar version tracking** — `.rebar-version` file + README badge for adopting repos
- **Adoption level validation** — steward checks README badge against actual compliance
- **CHANGELOG.md** — you're reading it

### Changed
- `AGENTS.template.md` — mandatory foundations only; advanced practices moved to `practices/`
- `architecture/CONTRACT-REGISTRY.template.md` — now documents the computed format
- `scripts/check-registry.sh` — deprecated in favor of `compute-registry.sh`
- `scripts/steward.sh` — uses `compute-registry.sh --check`, adds compliance validation
- `scripts/ci-check.sh` — uses `compute-registry.sh --check`
- `conventions.md` — added minimum viable section for Tier 1
- `SETUP.md` — added tier selection, team-size profiles, practices/ copy step
- All profile files — updated section references for practices/

### Migration from v1.1.0
1. Copy `practices/` directory into your project
2. Copy `scripts/compute-registry.sh` and `scripts/_rebar-config.sh`
3. Create `.rebarrc` from `.rebarrc.template` (set your tier)
4. Create `.rebar-version` with `v1.2.0`
5. Add rebar badge to top of your README.md (see README.template.md)
6. Re-diff your AGENTS.md against the new AGENTS.template.md — moved sections are now in `practices/`
7. Run `scripts/compute-registry.sh` to generate your registry
8. Optional: update all scripts from rebar (check `# rebar-scripts:` dates)

---

## v1.1.0 (2026-03-19)

### Added
- Merger agent — branch integration + conflict resolution (actor agent)
- Subagent LOE levels in prompt index
- Scout Rule — zero tolerance for skipped or failing tests
- Adoption badges — PARTIAL / ADOPTED / ENFORCED tiers
- AI-native contracts feedback — cross-repo namespacing (`CONTRACT:namespace/ID`)
- Blindpipe adoption feedback — ASK as context preservation, role discipline pattern

### Changed
- `ask init` now creates merger agent directory
- `AGENTS.template.md` — added Scout Rule section, merge coordinator flow

### Migration from v1.0.0
1. Copy `agents/merger/` directory
2. Merge Scout Rule section into your AGENTS.md (§Testing Expectations)
3. Optional: add adoption badge to README.md

---

## v1.0.0 (2026-03-17)

### Added
- Cold Start Quad templates — README, QUICKCONTEXT, TODO, AGENTS
- CLAUDE.md template with full Claude Code configuration
- Contract system — CONTRACT-TEMPLATE, CONTRACT-REGISTRY, naming conventions
- Methodology.md — full philosophy document
- Conventions.md — branch naming, commits, headers, discovery taxonomy
- 5 enforcement scripts — contract-headers, contract-refs, TODOs, freshness, ground-truth
- Steward — automated project health scanner with JSON + markdown output
- CI check runner and pre-commit hook
- ASK CLI — role-based agent queries with persistent sessions
- 8 subagent templates — code review, contract audit, security scan, UX review, doc drift, feature inventory, test shard, merge coordinator
- 4 project profiles — web-app, api-service, crypto-library, cli-tool
- SETUP.md — step-by-step adoption guide
- METRICS template with ground truth verification
- Learnings from OpenDocKit — 37KB of battle-tested patterns

### Migration
This is the initial release. Follow SETUP.md.
