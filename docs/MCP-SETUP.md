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
      "command": "/absolute/path/to/ask-mcp-server",
      "args": ["--stdio", "--repos-dir", "/absolute/path/to/parent"]
    }
  }
}
```

Claude Code picks this up when you open the project.

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
      "command": "/Users/you/dev/rebar/bin/ask-mcp-server",
      "args": ["--stdio", "--repos-dir", "/Users/you/dev"]
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
immediate subdirectory that contains an `agents/` folder**. Given:

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
"args": ["--stdio", "--repos", "/path/to/repo1,/path/to/repo2"]
```

---

## Verify it works

### 1. Dogfood the server directly

```bash
# stdio handshake — expect clean JSON-RPC, no banner on stdout
python3 -c '
import json, subprocess
p = subprocess.Popen(
    ["/path/to/ask-mcp-server", "--stdio", "--repos-dir", "/path/to/parent"],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    text=True)
p.stdin.write(json.dumps({"jsonrpc":"2.0","id":1,"method":"initialize",
    "params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"t","version":"0"},
    "capabilities":{}}})+"\n"); p.stdin.flush()
print(p.stdout.readline())
'
```

Expect a JSON response starting with `{"jsonrpc":"2.0","id":1,"result":{...}}`.
If you see a `scanning repos...` banner, your `ask-mcp-server` is out of
date — that banner belongs on stderr (fixed in rebar v2.0.0+).

### 2. From Claude Code

After registering, reload Claude Code for the project. Ask Claude:

> "What MCP tools do you have available for rebar?"

You should see `ask_<repo>_<role>` tools in the list. If they're not
visible:
- Confirm `.mcp.json` is at the project root (or `~/.claude.json` has the
  entry)
- Confirm the `command:` path exists and is executable
- Check `--repos-dir` points at a directory that actually contains repos
  with `agents/` subdirs
- Run Claude Code with MCP debug output (see Claude Code docs)

### 3. Test a tool call

Ask Claude:

> "Use the rebar architect agent to summarize the contract system."

If the MCP server is wired correctly, the tool call resolves and returns
the architect agent's response.

---

## Common pitfalls

### Path is absolute and machine-specific

The generated `.mcp.json` contains absolute paths to
`ask-mcp-server` and `--repos-dir`. Moving the rebar repo or the project
breaks the config — regenerate via `rebar init --force` or edit by hand.

### `ask-mcp-server` not found by `rebar init`

`rebar init` tries three locations for `ask-mcp-server`:
1. Same directory as the `rebar` binary (`os.Executable()` + `../bin/`)
2. The rebar repo root via `findRebarRoot()` (checks `$HOME/dev/rebar`,
   `$HOME/src/rebar`, `$HOME/code/rebar`)
3. `PATH`

If all three fail, `.mcp.json` is **not** written and you'll see:

```
Skipped .mcp.json — ask-mcp-server not found; see SETUP.md §MCP to configure manually
```

Fix: either symlink `ask-mcp-server` into your `$PATH`, or write
`.mcp.json` by hand with the correct path.

### Stdio banner corrupting JSON-RPC

Prior to v2.0.0, `ask-mcp-server` printed a `scanning repos...` banner
to stdout. That's a protocol violation — MCP clients see it as a
malformed JSON-RPC message and disconnect. If you cloned rebar before
v2.0.0, pull the fix (commit on `main`) and Claude Code will reconnect.

### Multi-developer projects with different paths

If developers clone rebar at different locations, absolute paths in a
committed `.mcp.json` won't work universally. Options:

1. **Gitignore `.mcp.json`** and let each dev run `rebar init` locally
2. **Symlink `ask-mcp-server`** into a shared PATH location (e.g.,
   `/usr/local/bin/ask-mcp-server`) and write `.mcp.json` with
   `"command": "ask-mcp-server"` — portable but requires manual edit
3. **Env var expansion** — MCP config does not currently expand `${HOME}`,
   so this is not yet viable

---

## Security considerations

- The MCP server spawns subprocess `ask` invocations. These inherit the
  user's environment and filesystem access. Treat ASK agents as trusted
  local code, not sandboxed.
- `--repos-dir` exposes every registered repo's `agents/memory.md` and
  `agents/*/memory.log.md` to the MCP client. Don't point `--repos-dir`
  at directories whose memory contents shouldn't be readable by every
  Claude Code session on the machine.
- Don't commit `.mcp.json` to a public repo if the command path reveals
  private directory structure you'd rather not disclose.

---

## Commands reference

```bash
# Auto-generate .mcp.json via init
rebar init                              # new project
rebar adopt                             # existing project

# Start MCP server manually for debugging
ask-mcp-server --stdio --repos-dir ~/dev

# HTTP mode (enterprise server) — for multi-user orgs
ask-mcp-server --port 7232 --repos-dir /srv/repos --mcp-only
```

See also: [docs/MCP-IMPLEMENTATION.md](MCP-IMPLEMENTATION.md) for the
protocol-level implementation details.
