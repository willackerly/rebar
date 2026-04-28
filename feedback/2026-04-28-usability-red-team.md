# Feedback: Usability Red Team — Cold Start, MCP, Daily Friction, Debug, Docs

**Date:** 2026-04-28
**Source:** internal red-team across `rebar new`, `rebar adopt`, `ask` CLI, MCP stdio path, docs flow
**Type:** improvement | bug | confusion | missing-feature
**Status:** proposed
**Template impact:** `cli/cmd/new.go`, `cli/cmd/adopt.go`, `cli/cmd/root.go`, `bin/ask`, `bin/ask-mcp-server`, `templates/project-bootstrap/`, `QUICKSTART.md`, `SETUP.md`
**From:** maintainer-direct (assistant-conducted across all 5 pillars)

## What Happened

User asked for a broader usability red team across 5 pillars: (1) cold start
install→first-answer, (2) MCP spinup, (3) daily friction, (4) debugging,
(5) docs flow. Conducted live testing — `rebar new` in tmp dir, MCP stdio
handshake, malformed/edge-case CLI invocations, debug output, docs scan.

## What Was Expected

Adopters should be able to go `rebar new foo` → `cd foo` → `ask architect "X"`
without help. Cross-repo MCP calls should be case-tolerant. Common typos and
malformed invocations should produce actionable errors. The `--version`
convention should work. Error paths shouldn't fall through to misleading
"network failure" messages when the real issue is local.

## Findings

Severity tags: 🔴 critical (broken or misleading) | 🟡 medium (friction) |
🟢 low (polish). Pillar tag in brackets.

---

### 🔴 Critical

**C1. `rebar new` / `rebar adopt` create `.mcp.json` but NO `agents/`** [Pillar 1]

A fresh `rebar new test -d "..."` produces a project with `.mcp.json`
wired to `ask-mcp-server`, but the `agents/` directory doesn't exist.
Running `ask architect "X"` immediately fails:
```
ask: error: agent 'architect' not found (no directory at ./agents/architect)
ask: no agents directory at ./agents
```
The welcome message says "Next: cd test && rebar context" — never mentions
`ask init`. The `templates/project-bootstrap/` template also has no `agents/`
directory. Same gap in `rebar adopt`.

**Net effect:** Anyone following QUICKSTART end-to-end without you in the
loop hits a dead end. The MCP tool list in Claude Code will be empty until
they discover `ask init` (which neither QUICKSTART nor SETUP currently
documents).

**Fix:** `rebar new` and `rebar adopt` should run `ask init` (or inline the
equivalent agent-dir creation) as part of the bootstrap. Welcome message
should mention `ask init` as a fallback if it's not auto-run. Bootstrap
template should include skeleton `agents/<role>/AGENT.md` files.

---

**C2. MCP `tools/call` is case-sensitive — wrong case returns −32603** [Pillar 2]

Reproduced the friend's "can't see TDFLite" bug. `ask_tdflite_architect`
returns:
```
{"error": {"code": -32603, "message": "Tool execution failed",
           "data": "Repository tdflite not found"}}
```
…while `ask_TDFLite_architect` works. The `_handle_tools_call` does
`repo, role = agent_id.split(":", 1)` and looks up `repo` exactly. On
case-sensitive filesystems (Linux), the local `bin/ask` has the same
problem.

**Fix:** Server-side normalize to a case-insensitive lookup against the
registered repo set; return the canonical name in resolutions. Apply
the same pattern to `bin/ask` `resolve_project_agent` for cross-repo
calls and to `require_agent_exists` for local role lookup.

---

**C3. `rebar ask featurerequest "test"` FAILS with claude flag-combo error** [Pillar 3]

Newly introduced by the 2026-04-28 featurerequest commit. Reproducer:
```
$ rebar ask featurerequest "test"
ask: error: claude invocation failed
Error: When using --print, --output-format=stream-json requires --verbose
```
Root cause: `bin/ask` line 1136-1138 forces `--output-format stream-json`
when `WRITE_MODE=1`, but doesn't add `--verbose`. Pre-this-commit, write
mode was only triggered by explicit `-w`; now featurerequest auto-enables
it, exposing the latent bug.

**Fix:** Add `--verbose` to `claude_args` whenever `stream-json` is used
(or switch to `--output-format json` for the featurerequest role).

**Tag:** REGRESSION — should be fixed in the next push.

---

**C4. `ask architect -v` outputs claude version banner** [Pillar 3]

```
$ ask architect -v
2.1.121 (Claude Code)
```
The `-v` flag after the agent name leaks through to the claude binary as
its own `--version` flag. User sees the version of Claude Code, not their
agent's response. No error, no usage hint — looks like the system is
hallucinating.

**Fix:** Detect `-v` / `-d` / `-w` in CMD_WORD position and either
re-route to flag-parse OR error with "flag not allowed after agent name —
put it before: `ask -v <agent> \"<question>\"`".

---

### 🟡 Medium

**M1. Cross-repo case mismatch falls through to remote ASK_SERVER** [Pillar 3]

```
$ ask tdflite:architect "X"
ask: error: all servers failed for POST /v1/ask
  tried: 192.168.0.181:7232
```
Real cause: local case-mismatch (`TDFLite` ≠ `tdflite`). Misleading error
points the user at network/server when local lookup is the issue.

**Fix:** Case-insensitive repo lookup before remote fallback (same code
path as C2).

---

**M2. macOS-only filesystem case-insensitivity hides a Linux portability bug** [Pillar 3]

`ask Architect "test"` works on macOS (APFS case-insensitive by default)
but would fail on Linux. Mac users won't notice; Linux adopters will hit
it cold.

**Fix:** Same as C2/M1 — explicit script-level normalization removes the
filesystem dependence.

---

**M3. `--version` unsupported on both `ask` and `rebar`** [Pillar 3]

| CLI | `--version` | `version` subcommand |
|-----|-------------|----------------------|
| `ask` | ❌ "agent '--version' not found" | n/a |
| `rebar` | ❌ silent empty output (looks broken) | ✅ `rebar version` works |

`rebar --version` returning empty is the worst kind of bug — user can't
tell if their install is broken or the flag is unsupported.

**Fix:** Add `Version: "v2.0.0"` to cobra rootCmd in `cli/cmd/root.go`;
add `-V|--version` case to `bin/ask` flag-parse loop.

---

**M4. Agent list in `cmd_who` shows agent-voice prose instead of caller-facing** [Pillar 3]

```
$ ask architext "X"
...
featurerequest You are the feature-request intake agent for rebar. You...
```
The MCP server already has `_get_agent_description` that strips "You are..."
preambles for tool descriptions; the local `cmd_who` listing uses raw
sed/grep to extract from AGENT.md and gets the wrong audience.

**Fix:** Port the first-paragraph extraction logic from `bin/ask-mcp-server`
to `cmd_who`. (Or factor into a shared helper script that both call.)

---

**M5. Description truncation cuts mid-word at ~55 chars** [Pillar 3]

```
architect    Answer questions about system architecture and design p
```
The trailing "design p" is mid-word.

**Fix:** Word-break the description, or expand the column width to terminal-aware.

---

**M6. Unknown flags treated as agent names** [Pillar 3]

`ask --foo` → "agent '--foo' not found (no directory at ./agents/--foo)".
Hostile to typos. Should be obvious that `--foo` isn't an agent.

**Fix:** Add `--*` catch-all in the leading-flag parse loop emitting
"Unknown flag '--foo'. Try `ask --help`."

---

**M7. `rebar ask` and `ask` are duplicate-help paths** [Pillar 3]

`rebar ask --help` and `ask --help` show the same usage. Users have no
basis to choose between them. `rebar ask` adds no value (thin wrapper).

**Fix:** Pick one of (a) deprecate `rebar ask` with a warn-and-redirect,
(b) make `rebar ask` differentiate (auto-detect repo root, suggest
agents based on tier, etc.). Whichever, document the choice.

---

**M8. No "did you mean?" suggestion on typos** [Pillar 3]

`ask architext "X"` lists all 7 agents but doesn't say "did you mean
architect?". Closest-match suggestion would catch the most common typos
at first contact.

**Fix:** Add Levenshtein-based suggestion with threshold ≤4 chars in
`require_agent_exists` failure path. Bash 3.2 + awk implementation; ~20
lines. Apply same logic to project-not-found path in `resolve_project_agent`.

---

**M9. Malformed `repo:role` falls through to remote without validation** [Pillar 3]

`ask :architect "X"` and `ask rebar: "X"` both reach for ASK_SERVER
without warning that the split is malformed. Should reject empty repo or
role part.

**Fix:** Validate non-empty parts in `_is_remote_query` or `resolve_project_agent`.

---

**M10. QUICKSTART + SETUP don't document `ask init`** [Pillar 5]

Compounds C1: even a careful user reading docs end-to-end has no
instruction to run `ask init`. The documents reference
`agents/README.md` but never the bootstrap step itself.

**Fix:** Add "Step N: Run `ask init` to create role-based agents" to
QUICKSTART after the bootstrap copy step. SETUP.md per-profile sections
should mention it. (If C1 lands and `rebar new` auto-runs `ask init`,
this becomes informational rather than critical.)

---

**M11. MCP tool list polluted by stale dapple worktrees** [Pillar 2]

Live tools/list dump shows 7 dapple worktree projects all registered:
```
dapple-wt-bugfixes: 5    dapple-wt-coldstart: 5
dapple-wt-contracts: 5   dapple-wt-fallback: 5
dapple-wt-helpers-api: 5 dapple-sdk: 5
```
35 tool slots burnt on near-duplicate names. Pattern: `*-wt-*` is the
worktree convention. Real adoption-pattern issue (this is rebar's own
convention biting it).

**Fix:** Either (a) a `.rebar-skip-mcp` sentinel file that excludes a
dir from registry scan, or (b) pattern-exclude `*-wt-*` by default in
the registry walker. Probably (a) — explicit > magical.

---

### 🟢 Low

**L1. No tab completion for `ask` / `rebar`** [Pillar 3]

No `_ask` / `_rebar` completion files in repo. Subcommand discovery is
all manual / `--help`-driven.

**Fix:** Ship `bin/completions/_ask` (zsh) and `bin/completions/ask.bash`,
similarly for rebar. Wire from `bin/install`.

---

**L2. `ask log <agent>` dumps raw markdown** [Pillar 4]

Output is the raw `memory.log.md` content. Hard to skim — every Q/A pair
has YAML-ish front-matter and then prose. Would benefit from pagination
+ per-entry formatting or auto-pipe to `less -R` on TTY.

**Fix:** Format output for terminals (separator lines, role colorization);
auto-pipe through `less -R` when stdout is a TTY.

---

**L3. Compliance score after `rebar adopt` shows 0.9/10 without context** [Pillar 1]

```
Phase 1: Assessment
Current compliance: 0.9/10
```
A user reading "0.9/10" thinks "did this fail?" The score is structurally
low because the adopted repo doesn't have contracts yet — that's the
point of adopting. Should annotate.

**Fix:** Add explanatory line below the score: "Score is low because no
contracts exist yet — this is normal for fresh adoption. Run `rebar audit`
after writing your first contract to recheck."

---

**L4. `rebar context` doesn't suggest next-step** [Pillar 1]

After `rebar new`, the user runs `rebar context` and sees AI-generated
README + status, but no nudge toward `ask architect` or actually using
the system.

**Fix:** Append "Try: `ask architect 'what should I implement first?'`"
to `rebar context` output when run in a project with empty
`memory.log.md` (signal that nothing's happened yet).

---

## Working correctly (verified)

These were tested and behave as advertised — listed for confidence:

- ✅ `ask --help`, `ask -h`, `ask` (bare), `ask help` all show usage [Pillar 3]
- ✅ `rebar --help`, `rebar -h`, `rebar` (bare) show usage [Pillar 3]
- ✅ `ask architect` (no question) runs `commands/default.sh` [Pillar 3]
- ✅ ENV vars documented in `ask --help` [Pillar 3]
- ✅ `ask -d <agent>` shows session ID, tokens, context%, claude command [Pillar 4]
- ✅ Stale `.session-id` recovery: "session expired, restarting fresh" works [Pillar 4]
- ✅ Exit codes match documentation (2 for "agent not found") [Pillar 4]
- ✅ MCP stdio handshake works on cold launch [Pillar 2]
- ✅ MCP server correctly returns no response for JSON-RPC notifications [Pillar 2]
- ✅ README links to QUICKSTART, FEATURE-DEVELOPMENT, SETUP, CHARTER are consistent [Pillar 5]
- ✅ `rebar version` (subcommand) prints `rebar v2.0.0` [Pillar 3]
- ✅ MCP server auto-strips `ASK_SERVER` env var to prevent recursive routing [Pillar 2]
- ✅ MCP server prints clean banner on stderr, not stdout [Pillar 2]
- ✅ `ask init` fully populates 6 agent dirs including `featurerequest` [Pillar 1]

## Suggestion

Fix in 5 themed clusters, prioritized:

### Cluster C — Regression + claude flag bugs (C3, C4) — DO FIRST

Just-shipped feature is broken (C3). Fix `--verbose` requirement when
stream-json; fix `-v` after agent name leaking to claude. Both in
`bin/ask`. ~10 lines. Critical because we can't ship more features on a
broken intake gate.

### Cluster A — Cold-start completeness (C1, M10, L3, L4)

Make `rebar new` / `rebar adopt` get the user to a working `ask architect`
state. Auto-run `ask init` from the bootstrap. Update QUICKSTART/SETUP to
match. Annotate the alarming compliance score. ~40 lines across
`cli/cmd/new.go`, `cli/cmd/adopt.go`, `templates/project-bootstrap/`,
QUICKSTART.md, SETUP.md.

### Cluster B — Case insensitivity everywhere (C2, M1, M2)

`bin/ask`: case-insensitive lookup for both repo names and role names in
`resolve_project_agent` and `require_agent_exists`. `bin/ask-mcp-server`:
case-insensitive tool name resolution in `_handle_tools_call` and
`_handle_resources_read`. Solves the friend's bug + Linux portability.
~30 lines across two files.

### Cluster D — Error path polish + did-you-mean (M3-M9)

Add `--version` to both CLIs. Port MCP description extraction to
`cmd_who`. Levenshtein typo suggestions. Validate cross-repo split.
Reject unknown flags. ~80 lines, mostly `bin/ask`.

### Cluster E — Polish & adoption hygiene (M7, M11, L1, L2)

Reconcile `rebar ask` vs `ask`. Add MCP-skip sentinel for worktrees. Tab
completion. Log formatting. Lower priority — none of these block adopters.

**Recommended landing:** C → A → B → D as a single themed PR labeled
"v2.1.0 UX pass" (or 2-3 sub-commits). E in a follow-up. Whole cluster
fits in ~250 lines + doc updates; should land in one focused session.

## Provenance notes

- Test commands run live against current `main` (commit `6df6b3d` —
  CHARTER + featurerequest landing).
- Cold-start tests in `mktemp -d` to avoid contaminating real projects.
- MCP stdio tests via raw JSON-RPC piped to `ask-mcp-server --stdio`.
- All 7 findings from prior 2026-04-28 narrow red-team folded into
  this report (case-insensitivity, --version, did-you-mean,
  descriptions, etc. → C2, M1, M2, M3, M4, M5, M8 here).
