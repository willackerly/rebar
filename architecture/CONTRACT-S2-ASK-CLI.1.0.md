# CONTRACT-S2-ASK-CLI.1.0

**Version:** 1.0
**Status:** active
**Owner:** rebar maintainer
**Type:** Service
**Cross-repo Promotability:** Yes — every rebar-adopted repo invokes this CLI
**Source:** `bin/README.md`, `DESIGN.md` §10 (Why agent commands are unquoted)

## Why this exists

Ephemeral subagents pay 10× context per question — every `Agent(prompt:
"...")` call inside Claude Code re-ingests the project's cold-start files.
For a developer asking the architect 10 questions in a row, that's 10 cold
loads. ASK CLI maintains a **persistent role-specific session** per repo per
role, so the second through tenth questions cost ~1× context, not 10×.

ASK is also the substrate the MCP server (`CONTRACT:S3-MCP-SERVER`) wraps —
without ASK, the MCP tools have nothing to invoke. And it's the substrate the
Steward (`CONTRACT:S1-STEWARD`) and other agent commands plug into via the
`ask <role> <command>` convention.

## Who needs this

- **Developers in adopted repos** — interactive shell use:
  `ask architect "..."`, `ask product "..."`, `ask steward summary`
- **CI / automation** — scripted invocations with `--json` output for parsing
- **`bin/ask-mcp-server`** — runs ASK as a subprocess to fulfill MCP tool calls
- **`scripts/ci-check.sh` and Steward** — invoke ASK commands like
  `ask steward` as part of composite checks
- **Cross-repo asks** — `ask <peer-repo>:<role> "..."` lets a developer in
  one repo query the architect role in another (e.g.,
  `ask filedag:architect "..."` from inside `pdf-signer-web`)

## Scenarios (illustrative)

### Scenario 1 — interactive role consultation

A developer in `TDFLite` is deciding whether the sealed policy bundle should
be one file or split per attribute family. They run
`ask architect "should the sealed policy bundle be monolithic or split?"`.
ASK loads `agents/architect/AGENT.md` + `agents/architect/memory.md`,
forwards the question to the LLM with project context, returns the answer,
and appends the exchange to `agents/architect/memory.log.md`. A follow-up
question 5 minutes later reuses the warm session — no re-ingest of project
files.

### Scenario 2 — agent command invocation

`ask steward summary` resolves to `agents/steward/commands/summary.sh` and
executes it directly. Output goes to stdout. The convention is "quoted
argument = question" / "unquoted argument = command name." This lets each
role expose a slice of project health without per-call LLM context.

### Scenario 3 — cross-repo ask via MCP

Claude Code in `office180` is implementing a feature that depends on
`filedag`'s ABAC contract. It calls the MCP tool `ask_filedag_architect` with
the question. The MCP server (`CONTRACT:S3-MCP-SERVER`) shells out to
`ask filedag:architect "..."` running in `~/dev/filedag`. Filedag's
architect agent reads filedag's contracts and answers. Office180's Claude
gets the answer in-tool, no shell roundtrip.

## Interfaces

```bash
# Question form (quoted)
ask <role> "<question>"

# Default command (no args)
ask <role>

# Named command
ask <role> <command> [args]

# Cross-repo
ask <repo>:<role> "<question>"

# Lifecycle
ask reset <role>     # clear the persistent session
ask who              # list available agents in current repo
ask help             # surface help
```

Exit codes: 0 success, 1 ask-level failure, 2 invocation error.

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| Persistent session | One per (repo, role); accumulates across calls until `ask reset <role>` |
| Memory durability | `agents/<role>/memory.md` is the durable distillation; reloaded each call |
| Memory log | `agents/<role>/memory.log.md` is append-only event log (gitignored) |
| Quoted vs unquoted | Quoted arg = question for LLM; unquoted = command name resolving to `agents/<role>/commands/<name>.sh` |
| Default command | `ask <role>` with no args runs `agents/<role>/commands/default.sh` |
| Cross-repo asks | `ask <repo>:<role> "..."` resolves repo via the registry the MCP server uses; `cd`s into that repo before invoking |
| --json mode | Wraps stdout in `{"role":..., "answer":...}` JSON shape for CI/automation |
| Auth (optional) | If `ASK_API_KEY` is set, ASK forwards as bearer to the LLM endpoint |
| Server endpoint | Reads `ASK_SERVER` env var; if unset, runs the LLM call directly via configured provider |

## Error Contracts

| Error | When | Behavior |
|-------|------|----------|
| Unknown role | `ask foo "..."` where `agents/foo/` doesn't exist | Exit 2, show available roles |
| Unknown command | `ask architect bogus` | Exit 2, list valid commands |
| Cross-repo target missing | `ask nonexistent:architect "..."` | Exit 2, list registered repos |
| LLM endpoint unreachable | Network or auth failure | Exit 1 with the underlying error message |
| Memory file corrupted | Malformed `memory.md` | Exit 1 with the parse error and offer to back up + reset |

## Dependencies

- Depends on: Python 3.10+ for `bin/ask`, `bin/ask-server`, `bin/ask-agent-loop`
- Depends on: `claude` CLI (or other LLM endpoint) reachable for question handling
- Optional: `ASK_SERVER` for remote-server mode (not required for local default)
- Configuration: `agents/<role>/AGENT.md` defines the role; `agents/<role>/memory.md` seeds durable state

## Cross-references

- **Doc:** `bin/README.md` (full reference), `DESIGN.md` §10 (design rationale)
- **Sister contract:** `CONTRACT:S3-MCP-SERVER.1.0` (the MCP bridge that wraps ASK)
- **Sister contract:** `CONTRACT:S1-STEWARD.1.0` (Steward exposes itself via `ask steward`)

## Future evolution

- **Provisional:** the wire protocol between `bin/ask` and `bin/ask-server`
  isn't yet a versioned contract; it should be `CONTRACT:I2-ASK-WIRE.1.0`
  once a 2nd implementation appears (e.g., a Go reimplementation).
- **Major-bump trigger:** changing the quoted-vs-unquoted convention, or
  changing how `ask <repo>:<role>` resolves repos.

## Retirement / supersession plan

This is the latest version. No predecessor.

## Implementing Files

- `bin/ask` — primary CLI entry point (Python, ~1900 lines)
- `bin/ask-server` — long-running session server
- `bin/ask-agent-loop` — agent-loop harness
- `bin/install` — PATH installer + dependency check
- `agents/<role>/AGENT.md` — per-role config, read by ASK on every call
- `agents/<role>/commands/*.sh` — unquoted-command implementations

## Test Requirements

- [ ] `ask who` lists every role with an AGENT.md
- [ ] `ask <role> "..."` succeeds against a live LLM endpoint
- [ ] Memory accumulates across calls (verified by inspecting `memory.log.md`)
- [ ] `ask reset <role>` clears the persistent session
- [ ] Cross-repo `ask <repo>:<role>` correctly `cd`s into the target repo
- [ ] `--json` output validates as JSON
- [ ] Quoted vs unquoted dispatch is unambiguous

## Cross-repo promotion notes

- **Universal invariants:** persistent sessions, memory.md/memory.log.md
  pair, quoted-vs-unquoted dispatch, the 5-or-6 standard role names
- **Per-project customization:** which roles a project enables (TDFLite
  has 5; rebar has 6; pdf-signer-web has 4 with a custom "engineer" role)
- **Specialization-contract naming:** adopters can add custom roles like
  `agents/curator/` and they'll be auto-discovered
- **Candidate adopting repos:** all 8 currently adopt this contract

## Change History

| Version | Date | Change | Migration |
|---------|------|--------|-----------|
| 1.0 | 2026-04-25 | Initial contract — formalizing the existing CLI | — |
