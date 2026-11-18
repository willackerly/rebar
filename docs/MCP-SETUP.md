# MCP Setup — Making ASK Available to Claude Code

**Goal:** Wire rebar's ASK agents into Claude Code as first-class MCP tools
so a coding agent working in your project can call `ask_<repo>_<role>` the
same way it calls `Grep` or `Bash` — without shelling out, without extra
context cost.

**Why it matters:** ASK's whole value proposition is persistent,
role-scoped agent sessions that save 10× context vs. ephemeral subagents.
That value only lands if coding agents actually *reach for* ASK. MCP tool
registration is what makes it a peer citizen in Claude Code's tool list
instead of a buried shell command.

---

## What you get

Once wired up, Claude Code sees tools like:

```
ask_rebar_architect       — Query rebar's architect about contracts/design
ask_filedag_englead       — Query filedag's eng lead about implementation
ask_blindpipe_product     — Query blindpipe's product agent about features
...
```

…plus resources (`ask://memory/<repo>:<role>`, `ask://log/<repo>:<role>`,
`ask://agent/<repo>:<role>`) for any repo under the configured
`--repos-dir` that has an `agents/` directory.

A coding agent in project *X* can ask the architect of a *sibling* project
*Y* — this is the cross-repo ASK value that falls out of the same
configuration.

---

## Two setup paths

### A. Project-level (`.mcp.json` at repo root) — recommended for team repos

When you run `rebar init` or `rebar adopt`, a `.mcp.json` is written to
the project root automatically:

```json
{
  "mcpServers": {
    "rebar-ask": {
      "command": "/absolute/path/to/rebar",
      "args": ["mcp", "serve", "--stdio", "--repos-dir", "/absolute/path/to/parent"]
    }
  }
}
```

Claude Code picks this up when you open the project. The MCP server is
built into the `rebar` binary (`rebar mcp serve`); `command` is that binary,
through `~/.rebar/current/bin/rebar` for a `setup-rebar.sh` install so it
follows version switches.

**Older `.mcp.json` files** run `…/bin/ask-mcp-server` (the python server of
rebar ≤ 3.1). They keep working: `bin/ask-mcp-server` is now a thin
hand-off that runs `rebar mcp serve` with the same arguments, so no python
is needed. `rebar migrate` reports the rewrite to the native command and
`rebar migrate --apply` writes it (same arguments, other servers and keys
kept).

**`--repos-dir`** is the parent of your project by default, so sibling
rebar-adopted repos register alongside this one. Adjust if your projects
live elsewhere.

**Commit it? Depends:**

| Team shape | Recommendation |
|------------|----------------|
| Solo dev, same machine | Commit — it documents intent |
| Team, shared dev environment | Commit, but absolute paths must match across machines (usually via a shared dev container or path convention) |
| Team, heterogeneous setups | Add `.mcp.json` to `.gitignore`; each dev runs `rebar init` locally to regenerate |

Rebar does **not** auto-gitignore `.mcp.json` — you choose.

### B. User-level (`~/.claude.json`) — recommended for single-developer, many-projects

If you work across many projects and want ASK available in all of them
without touching each, add rebar to your Claude Code user config:

```jsonc
// ~/.claude.json (top-level merge into existing mcpServers)
{
  "mcpServers": {
    "rebar-ask": {
      "command": "/Users/you/.rebar/current/bin/rebar",
      "args": ["mcp", "serve", "--stdio", "--repos-dir", "/Users/you/dev"]
    }
  }
}
```

**Tradeoff:** available everywhere (good for power users), including in
projects that haven't adopted rebar. If that noise bothers you, use
path A instead.

You can mix: user-level for ambient availability, project-level `.mcp.json`
overrides when a project has different `--repos-dir` needs.

---

## How `--repos-dir` works

The MCP server scans the directory you point it at and registers **every
immediate subdirectory that contains an `agents/` folder** (and, one level
further down, those inside subdirectories without one; hidden directories,
`node_modules`, `vendor`, build outputs and repos holding a
`.rebar-skip-mcp` file are skipped). Given:

```
~/dev/
├── rebar/           ← agents/ present → registered
├── filedag/         ← agents/ present → registered
├── blindpipe/       ← agents/ present → registered
├── opendockit/      ← no agents/ → skipped (still benefits as consumer)
└── some-other-repo/ ← no agents/ → skipped
```

…with `--repos-dir ~/dev`, you get tools for rebar/filedag/blindpipe.
From *inside* opendockit, Claude Code still has those tools — it can
ask filedag's architect about the integration. That's the cross-repo
federation story.

To register a non-standard location, use `--repos` instead of (or with)
`--repos-dir`:

```json
"args": ["mcp", "serve", "--stdio", "--repos", "/path/to/repo1,/path/to/repo2"]
```

---

## Verify it works

### 1. Dogfood the server directly

```bash
# stdio handshake — expect clean JSON-RPC on stdout, the banner on stderr
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"me","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' |
  rebar mcp serve --stdio --repos-dir /path/to/parent 2>/dev/null
```

Expect one JSON response per request, starting with
`{"jsonrpc":"2.0","id":1,"result":{"capabilities":…,"protocolVersion":"2025-11-25",…}}`.
Every status line (`scanning repos...`, `registered: …`) goes to stderr, and
so does `ask-mcp-server: ignored invalid message (…)` for a line that is not
JSON-RPC (the server reads on). The server exits at the end of input
(within 2 s, cancelling tool calls still running), so to try a `tools/call`
this way keep stdin open until it answers — `(printf '%s\n' …; sleep 60) |
rebar mcp serve …` — or use Claude Code.

### 2. From Claude Code

After registering, reload Claude Code for the project. Ask Claude:

> "What MCP tools do you have available for rebar?"

You should see `ask_<repo>_<role>` tools in the list. If they're not
visible:
- Confirm `.mcp.json` is at the project root (or `~/.claude.json` has the
  entry)
- Confirm the `command:` path exists and is executable (an old
  `bin/ask-mcp-server` needs a `rebar` beside it, in `REBAR_BIN` or on
  `PATH`)
- Check `--repos-dir` points at a directory that actually contains repos
  with `agents/` subdirs
- Run Claude Code with MCP debug output (see Claude Code docs)

### 3. Test a tool call

Ask Claude:

> "Use the rebar architect agent to summarize the contract system."

If the MCP server is wired correctly, the tool call resolves and returns
the architect agent's response. When the agent fails (e.g. `claude` is not
on the server's `PATH`), Claude Code shows the tool result as an error with
the ask's stderr, exit code, command and working directory — read it rather
than guessing.

### 4. Over HTTP (shared server)

`rebar mcp serve --port 7232 …` serves MCP's Streamable HTTP transport at
`/mcp`. Point Claude Code at it with an `http` server:

```json
{
  "mcpServers": {
    "rebar-ask": {
      "type": "http",
      "url": "http://ask-host:7232/mcp",
      "headers": {"Authorization": "Bearer <key>"}
    }
  }
}
```

`rebar mcp client http://ask-host:7232` walks the server through a session
(initialize, tools, resources, one call) to check it. Requests that reach
the server through a loopback address must name a loopback `Host` (DNS
rebinding protection); behind a proxy on the same machine, forward the
original host name as `localhost`.

---

## Protocol (rebar 3.2)

The server is built on the official MCP Go SDK. It negotiates the protocol
version with the client (2024-11-05 through 2025-11-25 over `initialize`,
2026-07-28 over `server/discover`, which Claude Code 2.1.3xx tries first),
reports a failed ask as a tool error (`isError: true` result) rather than a
JSON-RPC error, and answers an unknown tool with JSON-RPC error -32602. Tool
names and input schemas did not change; tool names and `ask://` resource
URIs still match case-insensitively. Details: [MCP-IMPLEMENTATION.md](MCP-IMPLEMENTATION.md#wire-behaviour-rebar-32).

---

## Common pitfalls

### Path is absolute and machine-specific

The generated `.mcp.json` contains absolute paths to
`rebar` and `--repos-dir`. Moving the rebar repo or the project
breaks the config — regenerate via `rebar init --force` or edit by hand.

### Which `rebar` `rebar init` writes

`rebar init` records the `rebar` binary that runs it (as
`~/.rebar/current/bin/rebar` when that is the active version), else the
rebar install's `bin/rebar`, else `rebar` on `PATH`.

### Stdio banner corrupting JSON-RPC

Prior to v2.0.0, the python `ask-mcp-server` printed a `scanning repos...` banner
to stdout. That's a protocol violation — MCP clients see it as a
malformed JSON-RPC message and disconnect. If you cloned rebar before
v2.0.0, pull the fix (commit on `main`) and Claude Code will reconnect.

### Multi-developer projects with different paths

If developers clone rebar at different locations, absolute paths in a
committed `.mcp.json` won't work universally. Options:

1. **Gitignore `.mcp.json`** and let each dev run `rebar init` locally
2. **Put `rebar` on a shared PATH location** (e.g. `/usr/local/bin/rebar`)
   and write `.mcp.json` with `"command": "rebar"` — portable but requires
   manual edit
3. **Env var expansion** — MCP config does not currently expand `${HOME}`,
   so this is not yet viable

---

## Security considerations

- The MCP server spawns `rebar ask` subprocesses (or `--ask-bin` /
  `ASK_BIN`), one per tool call, in the repo, with `ASK_SERVER` cleared. These inherit the
  user's environment and filesystem access. Treat ASK agents as trusted
  local code, not sandboxed.
- `--repos-dir` exposes every registered repo's `agents/memory.md` and
  `agents/*/memory.log.md` to the MCP client. Don't point `--repos-dir`
  at directories whose memory contents shouldn't be readable by every
  Claude Code session on the machine.
- Don't commit `.mcp.json` to a public repo if the command path reveals
  private directory structure you'd rather not disclose.
- Tool calls run `rebar ask --question -- <role> "<question>"`: the question is
  never taken as an agent command, and a role must be one of the repo's
  agents. A repo with `.rebar-skip-mcp` is not served.
- HTTP mode (`--port`) listens on 127.0.0.1 by default and answers only
  requests with a loopback `Host` (DNS rebinding) that are not browser
  cross-origin requests. To serve other machines bind `--host 0.0.0.0` with
  `--api-key KEY` (or `ASK_API_KEY`); without a key a non-loopback bind exits
  2 unless `--insecure-no-auth`. POST bodies: `application/json`, ≤ 1 MiB,
  sent within 60 s. At most `--max-asks` (default 8) asks run at once (503 /
  "Server busy"); idle sessions close after 15 min, at most 1000 are open.
  Remote callers get a short error for a failed ask; the details go to the
  server's stderr. A reverse proxy in front of a loopback-bound server must
  send `Host: localhost`.

---

## Commands reference

```bash
# Auto-generate .mcp.json via init
rebar init                              # new project
rebar adopt                             # existing project

# Start MCP server manually for debugging
rebar mcp serve --stdio --repos-dir ~/dev     # bin/ask-mcp-server runs this

# HTTP mode (enterprise server) — for multi-user orgs: Streamable HTTP at /mcp
# (127.0.0.1 by default; another address needs --api-key or --insecure-no-auth)
rebar mcp serve --host 0.0.0.0 --port 7232 --repos-dir /srv/repos --mcp-only --api-key "$KEY"

# Point an old .mcp.json at the native server
rebar migrate            # report
rebar migrate --apply    # rewrite
```

See also: [docs/MCP-IMPLEMENTATION.md](MCP-IMPLEMENTATION.md) for the
protocol-level implementation details.
