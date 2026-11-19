# ASK — Agent Scoped Knowledge

A Unix-like CLI for querying role-based agents. Each agent has bounded context,
persistent memory, and executable commands.

See the [root README](../README.md) for how ASK fits into the overall system.

## Quick Start

```bash
# Initialize (creates shared/, agent dirs)
./bin/ask init

# List agents
./bin/ask who

# Run an agent's default command
./bin/ask steward           # full quality scan
./bin/ask architect         # contract audit
./bin/ask englead           # enforcement status
./bin/ask product           # gap analysis

# Ask an agent a question
./bin/ask architect "What contract governs authentication?"
./bin/ask -v architect "Why was RSA chosen?"   # verbose mode
./bin/ask -w tester "Add unit tests for the auth module"  # write mode

# Run a specific command
./bin/ask steward summary   # one-line health check
./bin/ask steward json      # JSON to stdout
./bin/ask englead qa        # full QA flow
```

## Installation

```bash
# From the rebar repo root:
./bin/rebar-install

# Or with ASK_SERVER for remote agent access:
./bin/rebar-install --server 192.168.0.181:7232
```

This adds `bin/` to your PATH (in `.zshrc`/`.bashrc`/`.profile`), optionally
sets `ASK_SERVER`, and checks dependencies. Idempotent — safe to run again.

The shims (`ask`, `ask-agent-loop`, `ask-mcp-server`, `ask-server`),
`install` and `completions/` are also compiled into the `rebar` binary:
`rebar install-shims` writes them beside it (setup-rebar.sh does this for a
release binary; `--dir DIR` writes them elsewhere, `--force` overwrites
files that differ). After editing them here, run
`scripts/sync-bootstrap.sh` to refresh the embedded copy.

Then apply with `source ~/.zshrc` (or open a new terminal).

**Dependencies:** the `rebar` binary and the claude CLI. ASK is built into
`rebar` (`rebar ask …`); `bin/ask`, `bin/ask-agent-loop`, `bin/ask-server`
and `bin/ask-mcp-server` are small sh shims that run the `rebar` beside them
(else `REBAR_BIN`, else the one on `PATH`), so the two spellings behave the
same. No bash implementation, jq, uuidgen or python is needed (rebar 4.0
removed them; `rebar mcp client` replaces `bin/test-mcp-client`).

## Commands

### Agent Interaction

| Command | Description |
|---------|-------------|
| `ask <agent> "<question>"` | Ask a question (natural language) |
| `ask -w <agent> "<question>"` | Ask with write access (agent can edit files) |
| `ask <agent>` | Run default command (if configured) |
| `ask <agent> <command> [args]` | Run a named command |

### Management

| Command | Description |
|---------|-------------|
| `ask who` | List available agents |
| `ask watch <agent>` | Follow agent's incremental progress |
| `ask init` | Initialize directory structure + create agents |
| `ask where <agent>` | Show agent directory path |
| `ask status <agent>` | Check if running, inbox depth |
| `ask log <agent>` | View interaction history |
| `ask peek <agent>` | Inspect pending inbox messages |
| `ask tail` | Follow live event log |
| `ask up <agent>` | Start agent loop (background listener) |
| `ask reset <agent>` | Clear session, start fresh |

### Cross-Project

| Command | Description |
|---------|-------------|
| `ask register [name]` | Register current project |
| `ask projects` | List registered projects |
| `ask project:agent "question"` | Query agent in another project |

## Two Interaction Modes

**Quoted = question.** Multi-word or quoted text routes to the agent persona
for a natural-language answer via Claude:

```bash
ask architect "What are the system boundaries?"
```

**Unquoted = command.** A single word matching a script in
`agents/<role>/commands/` runs that script directly. Only plain names count
(letters, digits, `.`, `_`, `-`; no `..`; not starting with `.` or `-`), and
the script must be an executable file directly in `commands/`; anything else
(`../x`, `-x`, `two words`) is a question. `ask --question <role> "<q>"` is
always a question — the MCP server and `ask serve` ask that way, so text from
the network never runs a script:

```bash
ask steward summary      # runs agents/steward/commands/summary.sh
ask architect audit      # runs agents/architect/commands/audit.sh
```

### Adding Commands

Create an executable script at `agents/<role>/commands/<name>.sh`:

```bash
#!/usr/bin/env bash
# Description of what this command does
REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
# ... your logic here
```

It's immediately available as `ask <role> <name>`. The comment on line 2 is
used as help text.

## Write Mode (`-w`)

By default, agents are read-only — they answer questions using Read, Grep,
and Glob. With `-w`, the agent gets Edit, Write, and Bash access and can
modify the repo directly.

```bash
# Agent can read and write
ask -w tester "Add missing tests for the auth module"
ask -w englead "Fix the broken import in src/utils.ts"
ask -w architect "Create a new contract for the billing service"
```

Write mode also enables **incremental progress streaming**. While the agent
works, it writes each tool call to `agents/<role>/.progress` so you can
watch in real time:

```bash
# Terminal 1: start the work
ask -w tester "Add tests for the auth module"

# Terminal 2: watch progress as it happens
ask watch tester
```

The progress file shows each step as it happens:

```
# ask tester — 2026-03-21T14:22:11Z
# Q: Add tests for the auth module
# status: running
---
[1] Read src/auth/login.ts
[2] Glob **/*.test.ts
[3] Read src/auth/__tests__/login.test.ts
  ... Analyzing existing test coverage...
[4] Edit src/auth/__tests__/login.test.ts
[5] Bash npx vitest run src/auth/__tests__/login.test.ts
---
# status: done — 2026-03-21T14:23:45Z
```

Combine flags: `ask -w -v tester "question"` for write mode + verbose output.

## Session Persistence

Agents maintain sessions across questions. The first question sends full
context; follow-ups resume the session without re-sending everything.

```bash
ask architect "What contracts exist?"       # full context sent
ask architect "Which one governs signing?"  # resumes session
ask architect "Show me the interface"       # still in session
```

Sessions auto-reset at 70% context usage (configurable via `ASK_CONTEXT_LIMIT`).
Manual reset: `ask reset <agent>`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ASK_AGENTS_DIR` | `./agents` | Agents directory |
| `ASK_SHARED_DIR` | `./shared` | Shared directory |
| `ASK_TIMEOUT` | `120` | Timeout (seconds) for inbox/outbox mode |
| `ASK_MODEL` | `sonnet` | Claude model |
| `ASK_CONTEXT_LIMIT` | `70` | Auto-reset session at this context % |
| `ASK_VERBOSE` | `0` | Debug output on stderr |
| `ASK_WRITE` | `0` | Enable write mode by default |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Agent could not answer |
| 2 | Agent not available |
| 3 | Timeout |
| 4 | Internal error |

## Composability

Output is plain text, pipe-friendly:

```bash
ask architect "what contracts exist?" | grep BLOBSTORE
ask product "requirements?" > requirements.txt
echo "is offline needed?" | ask product
```

## Agent Directory Structure

```
agents/<role>/
  AGENT.md              # Role definition, responsibilities, context rules
  commands/             # Executable command scripts
    default.sh          #   runs on `ask <role>` (no args)
    <name>.sh           #   runs on `ask <role> <name>`
  memory.md             # Distilled current state
  memory.log.md         # Append-only interaction history
  inbox/                # Incoming messages (inbox/outbox mode)
  outbox/               # Outgoing responses
  .session-id           # Current session ID (auto-managed)
  .progress             # Incremental progress (write mode, auto-managed)
```

## Enterprise Server

Host agents from multiple repos on a single endpoint. Any machine can query
any repo's agents over HTTP.

### Start the server

```bash
# Serve all repos in a directory
ask serve --port 7232 --repos-dir /srv/repos

# Serve specific repos
ask serve --port 7232 --repos /srv/repos/billing,/srv/repos/auth

# Serve other machines: bind a public address AND set an API key
ask serve --host 0.0.0.0 --port 7232 --repos-dir /srv/repos --api-key SECRET
```

`ask serve` (like `rebar mcp serve` in HTTP mode) listens on 127.0.0.1 by
default and then answers only requests whose `Host` is a loopback name. A
non-loopback `--host` without `--api-key`/`ASK_API_KEY` is refused (exit 2)
unless `--insecure-no-auth`. POST bodies must be `Content-Type:
application/json`, at most 1 MiB (413; `/v1/ask` also needs a
`Content-Length`, 411), sent within 60 s; at most `--max-asks` (default 8)
asks run at once (503 beyond). Repos with `.rebar-skip-mcp` are not served.
A failed ask returns its last error line; the details are in the server log.

### Query from any machine

```bash
# Set the server address
export ASK_SERVER=server-host:7232
export ASK_API_KEY=SECRET  # if server uses auth

# List all available agents
ask agents

# Query a remote agent
ask billing:architect "What contracts does the payment flow touch?"
ask -v auth:steward "What's the contract health?"

# Management
ask status billing:architect
ask log billing:architect
ask reset billing:architect
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/health` | Server health check |
| GET | `/v1/agents` | List all repo:role pairs |
| GET | `/v1/repos` | List repos with agent counts |
| POST | `/v1/ask` | Ask a question (`{repo, role, question, verbose?}`) |
| GET | `/v1/status/<repo>/<role>` | Agent status |
| GET | `/v1/log/<repo>/<role>` | Agent memory log |
| POST | `/v1/reset/<repo>/<role>` | Reset agent session |

### How it works

The server (`rebar ask serve`; `bin/ask-server` runs it) is a thin HTTP
wrapper around the `ask` CLI. Each request invokes `rebar ask` (or
`--ask-bin` / `ASK_BIN`) with `cwd` set to the target repo — all existing logic
(sessions, memory, context loading) works unchanged. Agents spin up
on demand, not ahead of time.

Local agents (no `:` prefix) always run locally. The `ASK_SERVER`
variable only affects `repo:agent` cross-project queries.

**Dependencies:** the `rebar` binary and the claude CLI.

### MCP server

`rebar mcp serve` (`bin/ask-mcp-server` runs it) exposes every registered ASK agent as an MCP tool
(`ask_<repo>_<role>`) plus resources (`ask://memory/...`,
`ask://log/...`, `ask://agent/...`) via stdio or HTTP transport.

- `rebar init` / `rebar adopt` auto-write `.mcp.json` for Claude Code
  (`rebar mcp serve --stdio`); `rebar migrate` points older ones that run
  `bin/ask-mcp-server` at it
- User-level wiring (available in all projects) via `~/.claude.json`
- Full setup + troubleshooting: **[docs/MCP-SETUP.md](../docs/MCP-SETUP.md)**
- Protocol-level implementation: [docs/MCP-IMPLEMENTATION.md](../docs/MCP-IMPLEMENTATION.md)

### Future

- SSE transport for MCP-over-HTTP streaming
- TLS / HTTPS (use a reverse proxy for now)
- Write mode over network (`-w` flag)
- PROGRAM concept (bundled multi-repo scopes)
- Company-level shared agents
- Horizontal scaling / load balancing
- Agent session pooling / warm instances
- WebSocket streaming for long-running queries
- Rate limiting per client/repo
- Audit logging / admin dashboard
- Agent-to-agent cross-repo queries via server
- Team/user access scoping

## Known Limitations (v0)

This is a bash proof-of-concept. See [IMPLEMENTATION.md](../docs/IMPLEMENTATION.md)
for the v1/v2 roadmap.

- No concurrent access protection (race conditions with inbox/outbox)
- No structured memory summarization (log grows unbounded)
- No agent-to-agent `ask` (re-entrant calls)
- No graceful shutdown / message draining
- No transport abstraction (filesystem only)
