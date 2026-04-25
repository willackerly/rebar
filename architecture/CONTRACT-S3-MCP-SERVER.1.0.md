# CONTRACT-S3-MCP-SERVER.1.0

**Version:** 1.0
**Status:** active
**Owner:** rebar maintainer
**Type:** Service
**Cross-repo Promotability:** No — single instance per Claude Code installation
**Source:** `docs/MCP-SETUP.md`, `docs/MCP-IMPLEMENTATION.md`, Wave 2.5 commits (`d9e68fc`, `0db9073`, `bc936cf`, `2f52983`)

## Why this exists

Claude Code can call MCP tools mid-conversation as first-class actions —
same surface as Read, Grep, Bash. Without an MCP bridge, ASK was reachable
only via shell-out, which is expensive (each call is a fresh subprocess
that re-loads project context) and indirect (Claude has to remember to
construct a Bash invocation).

`ask-mcp-server` exposes every role agent in every adopted repo as a
discoverable MCP tool: `ask_<repo>_<role>`. Claude Code's tool list now
includes (as of 2026-04-25) 37 such tools across 8 repos. Cross-repo asks
are first-class — Claude in pdf-signer-web can call `ask_filedag_architect`
without leaving its session.

Without this contract, the Wave 2.5 work that made ASK first-class would
not have a stable specification — and the recent fix sequence (notification
handling, first-paragraph extraction, role preambles, depth-2 discovery)
would not have a place to anchor.

## Who needs this

- **Claude Code clients** (every developer running Claude Code in any of
  the 8 adopted repos) — consumers of the MCP tools
- **`agents/<role>/AGENT.md` files in each adopted repo** — the
  underlying servers; the MCP tool list is built by scanning these
- **`rebar init` and `rebar adopt`** — write a project-local `.mcp.json`
  pointing at this server
- **Future MCP-compatible coding agents** beyond Claude Code — if they
  speak the protocol, they get the same surface

## Scenarios (illustrative)

### Scenario 1 — fresh install + first-class tool discovery

Will runs `rebar adopt` in a new repo. The CLI writes `.mcp.json`
referencing `bin/ask-mcp-server` with `--repos-dir ~/dev`. Will restarts
Claude Code in the repo. The tool list now shows
`ask_<repo>_architect`, `ask_<repo>_product`, etc. — alongside Grep,
Read, Bash. Claude reaches for the architect tool the same way it
reaches for grep, no Bash detour.

### Scenario 2 — cross-repo coordination

Claude Code in `office180` is implementing a feature spanning office180
and filedag. Mid-task, it calls `ask_filedag_architect` with the
relevant question. The MCP server routes the call: `cd ~/dev/filedag`
+ `ask architect "..."`. Filedag's architect agent has its own memory,
its own contracts, its own context. Office180's Claude gets the answer
in-tool and proceeds.

### Scenario 3 — depth-2 nested-repo discovery

A new repo lands at `~/dev/OpenTDF/TDFLite` (one level deeper than the
top-level scan). The MCP server's discovery descends into non-repo dirs
(those without `agents/`) one extra level, finds TDFLite, registers
its 5 role agents, and exposes them as
`ask_TDFLite_{architect,product,englead,steward,tester}`. No config
change; the single `--repos-dir ~/dev` setting suffices.

## Interfaces

CLI flags:

```bash
ask-mcp-server [OPTIONS]

  --stdio              Use stdio transport (for MCP clients)
  --port PORT          Port for HTTP transport (default: 7232)
  --host HOST          Bind host for HTTP (default: 0.0.0.0)
  --repos-dir DIR      Directory to scan for repos (subdirs with agents/)
  --repos PATHS        Comma-separated explicit repo paths
  --mcp-only           Only serve MCP, not classic ASK API
  --api-key KEY        Bearer-auth key
  --ask-bin PATH       Path to ask CLI (default: alongside this script)
```

JSON-RPC surface:

| Method | Behavior |
|--------|----------|
| `initialize` | Standard MCP handshake; advertises capabilities |
| `tools/list` | Returns one tool per (repo, role) pair, named `ask_<repo>_<role>`, with caller-facing description |
| `tools/call` | Routes `{ name, arguments: { question } }` to `ask <repo>:<role> "<question>"` and returns the answer |
| `resources/list` | Per-role memory + log files exposed as MCP resources |
| `resources/read` | Returns file contents (memory.md, memory.log.md) |
| Notifications (any method without `id`) | **Silently ignored** — never reply with `id: null` |

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| Tool naming | `ask_<repo-name>_<role>` — `repo-name` is the directory's basename, `role` matches `agents/<role>/AGENT.md` |
| Tool descriptions | Caller-facing first-paragraph extraction from `agents/<role>/AGENT.md`, prefixed with the centralized `ROLE_DESCRIPTIONS` preamble for each role (architect/product/englead/steward/tester/merger/engineer) |
| Discovery | `--repos-dir` recurses **depth-2** (top level + one nesting level) to find subdirs with `agents/`; skips hidden dirs and `node_modules`/`vendor`/`dist`/`build`/`target`/`out`/`.next`/`.nuxt`/`coverage`/`__pycache__` |
| Notification handling | JSON-RPC requests without `id` are notifications and MUST NOT receive a response; replying with `id: null` triggers Claude Code's Zod validator and silently breaks the session |
| Stdio hygiene | All status/banner output goes to stderr; stdout carries only JSON-RPC (otherwise the client can't parse the handshake) |
| Cross-repo invocation | `tools/call` with `name=ask_<repo>_<role>` runs `cd <repo-path> && ask <role> "<question>"` |
| Tool count discipline | At most one tool per (repo, role); duplicates are deduplicated by name |
| Authentication | If `--api-key` set, requires `Authorization: Bearer <key>` for HTTP transport; stdio is trusted (subprocess-of-Claude) |

## Error Contracts

| Error | When | Behavior |
|-------|------|----------|
| `--repos-dir` missing | Path doesn't exist | Warn on stderr, continue (allows running with `--repos` only) |
| `tools/call` for unknown tool | Name not in registry | JSON-RPC error code -32602, "Tool not found" |
| `ask` subprocess fails | Underlying ASK CLI error | Forward exit code + stderr in tool result with `isError: true` |
| `initialize` not called | Other method invoked first | Error -32002, "Not initialized — Call initialize first" |
| Method not implemented | Unknown method (non-notification) | Error -32601, "Method not found" |

## Dependencies

- Depends on: `CONTRACT:S2-ASK-CLI.1.0` (subprocess executor for every tools/call)
- Depends on: Python 3.10+
- Depends on: standard library only (no third-party deps — keeps install zero-friction)
- Configuration: `.mcp.json` in adopted repos (auto-written by `rebar init`)
  and `~/.claude.json` (user-level, manual)

## Cross-references

- **Doc:** `docs/MCP-SETUP.md` (adopter guide), `docs/MCP-IMPLEMENTATION.md` (protocol-level details)
- **Sister contract:** `CONTRACT:S2-ASK-CLI.1.0`
- **Wave 2.5 commits:** `d9e68fc` (activation), `0db9073` (notifications fix),
  `bc936cf` (paragraph extraction), `2f52983` (role preambles), `5800647`
  (depth-2 discovery)
- **Memory file:** `~/.claude/projects/.../memory/mcp-notifications-must-not-reply.md`

## Future evolution

- **Provisional:** the `ROLE_DESCRIPTIONS` dict is centralized in
  `bin/ask-mcp-server`. If/when adopters want repo-specific role copy in
  tool descriptions, distribute the override pattern (read from
  `agents/<role>/AGENT.md` first paragraph if it's caller-facing, fall
  back to centralized).
- **Provisional:** `tools/list` currently returns the same tool list to
  every Claude Code instance regardless of which repo Claude is running
  in. A future minor bump might filter to "this repo + cross-repo asks
  the user has explicitly enabled."
- **Major-bump trigger:** any breaking change to the `ask_<repo>_<role>`
  naming convention or the tool-result envelope shape.

## Retirement / supersession plan

This is the latest version. No predecessor.

## Implementing Files

- `bin/ask-mcp-server` — the entire server (Python, ~870 lines)
- `cli/cmd/init.go` — writes `.mcp.json` referencing this server (`ensureMCPConfig`, `findMCPServerPath`)
- `cli/cmd/adopt.go` — same, in the adoption flow

## Test Requirements

- [ ] `tools/list` returns one entry per (repo, role) pair
- [ ] `tools/call` against `ask_<repo>_<role>` routes correctly cross-repo
- [ ] Notifications (id-less requests) get no response
- [ ] `initialize` is idempotent and required before other methods
- [ ] Stdout is JSON-RPC-only; banners on stderr
- [ ] Depth-2 discovery finds nested repos and skips noise dirs
- [ ] `bin/ask-mcp-server --help` exits 0

## Change History

| Version | Date | Change | Migration |
|---------|------|--------|-----------|
| 1.0 | 2026-04-25 | Initial contract — formalizing Wave 2.5 + 2026-04-22→25 fixes | — |
