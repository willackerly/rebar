# MCP Implementation for ASK - Complete Technical Guide

> **Implementation status (rebar 3.2):** the MCP server is native Go in the
> `rebar` binary — `rebar mcp serve` (package `cli/internal/mcp`) — built on
> the official MCP Go SDK, `github.com/modelcontextprotocol/go-sdk` (v1.8.0).
> The SDK owns the protocol: version negotiation (2024-11-05 through
> 2026-07-28, including the sessionless `server/discover` handshake Claude
> Code 2.1.3xx uses), message framing and validation, input-schema
> validation, and the Streamable HTTP transport. rebar adds the ASK tools
> and resources and three small adapters (see
> [Wire behaviour](#wire-behaviour-rebar-32) below). Tool names, descriptions
> and input schemas are the python server's, unchanged.
>
> The python snippets below are the original design (the python server was
> removed in rebar 3.2). The python server's wire format survives only in
> git history (`cli/cmd/testdata/mcp` before S9 of the go-native stack); the
> goldens there now record the SDK server, and
> `cli/cmd/native_mcp_compat_test.go` replays sessions recorded from Claude
> Code 2.1.224 and 2.1.311.

## What I Built: Full MCP Enablement

This document explains **exactly** what I implemented to make ASK fully MCP-enabled, serving as a worked example for understanding MCP implementation.

---

## Before vs After

**Before (ASK v0):**
- HTTP server with custom REST API (`/v1/ask`, `/v1/agents`)
- Multi-user capability via server architecture
- **NOT** MCP-compliant

**After (ASK MCP-enabled):**
- Full JSON-RPC 2.0 MCP protocol implementation
- ASK agents exposed as **MCP tools**
- Agent memory/logs exposed as **MCP resources**
- Standards-compliant MCP server that any MCP client can use
- **Backward compatible** - old ASK API still works

---

## The 4 Key MCP Components I Implemented

### 1. MCP Tool Discovery Endpoints 🔧

**What it does:** Exposes each ASK agent as a callable MCP tool

**Implementation in `ask-mcp-server`:**

```python
def _handle_tools_list(self, params, req_id):
    tools = []

    # Convert each agent into an MCP tool
    for agent_info in self.registry.list_all_agents():
        repo = agent_info["repo"]
        role = agent_info["role"]
        agent_id = agent_info["id"]  # "repo:role"

        tool = {
            "name": f"ask_{agent_id.replace(':', '_')}",
            "description": self._get_agent_description(repo, role),
            "inputSchema": {
                "type": "object",
                "properties": {
                    "question": {
                        "type": "string",
                        "description": "Question to ask the agent"
                    },
                    "verbose": {
                        "type": "boolean",
                        "description": "Request verbose response",
                        "default": False
                    }
                },
                "required": ["question"]
            }
        }
        tools.append(tool)
```

**Example Request/Response:**
```json
// Request
{"jsonrpc": "2.0", "method": "tools/list", "id": 1}

// Response
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "ask_rebar_architect",
        "description": "Agent architect in repository rebar",
        "inputSchema": {
          "type": "object",
          "properties": {
            "question": {"type": "string", "description": "Question to ask"},
            "verbose": {"type": "boolean", "default": false}
          },
          "required": ["question"]
        }
      }
    ]
  }
}
```

### 2. Standardized Agent Capabilities as MCP Tools ⚙️

**What it does:** Each agent becomes a callable tool with proper schema

**Implementation:**
```python
def _handle_tools_call(self, params, req_id):
    tool_name = params.get("name")
    arguments = params.get("arguments", {})

    # Parse tool name: "ask_repo_role" -> "repo:role"
    agent_id = tool_name[4:].replace("_", ":")
    repo, role = agent_id.split(":", 1)

    question = arguments.get("question")
    verbose = arguments.get("verbose", False)

    # Execute the actual ask command
    result = self._execute_ask(repo, role, question, verbose)

    return {
        "jsonrpc": "2.0",
        "id": req_id,
        "result": {
            "content": [{"type": "text", "text": result["answer"]}]
        }
    }
```

**Key Innovation:** Tool names are systematically generated from agent IDs:
- Agent `rebar:architect` becomes tool `ask_rebar_architect`
- Agent `billing:steward` becomes tool `ask_billing_steward`

### 3. MCP-Compliant Schema Definitions 📋

**What it does:** Every tool and resource has proper JSON Schema definitions

**Tool Schema Pattern:**
```json
{
  "name": "ask_<repo>_<role>",
  "description": "<extracted from AGENT.md>",
  "inputSchema": {
    "type": "object",
    "properties": {
      "question": {
        "type": "string",
        "description": "Question to ask the agent"
      },
      "verbose": {
        "type": "boolean",
        "description": "Request verbose response with rationale",
        "default": false
      }
    },
    "required": ["question"]
  }
}
```

**Resource Schema Pattern:**
```json
{
  "uri": "ask://<type>/<repo>:<role>",
  "name": "<repo>:<role> <Type>",
  "description": "<human description>",
  "mimeType": "text/markdown"
}
```

**Resource URI Design:**
- `ask://memory/rebar:architect` - Agent's current memory
- `ask://log/rebar:architect` - Agent's interaction history
- `ask://agent/rebar:architect` - Agent's role definition

### 4. MCP Resource Discovery & Access 📚

**What it does:** Exposes agent internal state as readable MCP resources

**Resource List Implementation:**
```python
def _handle_resources_list(self, params, req_id):
    resources = []

    for agent_info in self.registry.list_all_agents():
        repo = agent_info["repo"]
        role = agent_info["role"]
        agent_id = agent_info["id"]

        # Memory resource
        resources.append({
            "uri": f"ask://memory/{agent_id}",
            "name": f"{agent_id} Memory",
            "description": f"Current distilled memory for {role} in {repo}",
            "mimeType": "text/markdown"
        })

        # Log resource
        resources.append({
            "uri": f"ask://log/{agent_id}",
            "name": f"{agent_id} Log",
            "description": f"Full interaction history for {role} in {repo}",
            "mimeType": "text/markdown"
        })

        # Agent definition
        resources.append({
            "uri": f"ask://agent/{agent_id}",
            "name": f"{agent_id} Definition",
            "description": f"Agent role definition for {role} in {repo}",
            "mimeType": "text/markdown"
        })
```

**Resource Read Implementation:**
```python
def _handle_resources_read(self, params, req_id):
    uri = params.get("uri")

    # Parse: ask://type/repo:role
    parts = uri[6:].split("/", 1)
    resource_type, agent_id = parts
    repo, role = agent_id.split(":", 1)

    content = self._get_resource_content(resource_type, repo, role)

    return {
        "jsonrpc": "2.0",
        "id": req_id,
        "result": {
            "contents": [{
                "uri": uri,
                "mimeType": "text/markdown",
                "text": content
            }]
        }
    }
```

---

## Complete MCP Protocol Implementation

### JSON-RPC 2.0 Base Layer

**All requests follow this structure:**
```json
{
  "jsonrpc": "2.0",
  "method": "<mcp-method>",
  "params": { ... },
  "id": <unique-id>
}
```

**All responses follow this structure:**
```json
{
  "jsonrpc": "2.0",
  "id": <matching-id>,
  "result": { ... }
}
```

### Implemented MCP Methods

1. **`initialize`** / **`server/discover`** - Start a session (or, on
   2026-07-28, discover the server without one) and negotiate the protocol
   version
2. **`tools/list`** - Discover available agent tools
3. **`tools/call`** - Execute an agent query
4. **`resources/list`** - Discover agent resources
5. **`resources/read`** - Read agent memory/logs/config
6. **`ping`**, **`prompts/list`** (empty) and the other SDK built-ins

### Transport Options

**Stdio Transport** (what `.mcp.json` runs):
```bash
rebar mcp serve --stdio --repos-dir /path/to/repos
# newline-delimited JSON-RPC over stdin/stdout; banners on stderr
```

**Streamable HTTP Transport** (MCP spec 2025-03-26 and later):
```bash
rebar mcp serve --port 7232 --repos-dir /path/to/repos   # or bin/ask-mcp-server …
# MCP endpoint: http://localhost:7232/mcp  (POST/GET/DELETE, Mcp-Session-Id)
# plus GET /mcp/schema and, unless --mcp-only, the classic /v1/* API
```

Claude Code connects to it with `"type": "http"`:
```json
{"mcpServers": {"rebar-ask": {"type": "http", "url": "http://localhost:7232/mcp",
  "headers": {"Authorization": "Bearer <key>"}}}}
```

---

## Usage Examples

### From Any MCP Client

Use an MCP client library (it does the handshake, session id and protocol
headers for you), e.g. with the Go SDK:

```go
client := mcp.NewClient(&mcp.Implementation{Name: "my-client", Version: "1"}, nil)
session, _ := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://localhost:7232/mcp"}, nil)
res, _ := session.CallTool(ctx, &mcp.CallToolParams{
    Name: "ask_rebar_architect", Arguments: map[string]any{"question": "What contracts exist?"}})
// res.IsError reports a failed ask; res.Content[0] holds the answer text
```

By hand, a Streamable HTTP request must accept both JSON and event streams
and, after `initialize`, carry the session id it returned:

```bash
curl -si http://localhost:7232/mcp -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
# → Mcp-Session-Id: <id>; send it (and Mcp-Protocol-Version) on every later request
```

### Testing the Implementation

```bash
# Start MCP server
rebar mcp serve --port 7232 --repos-dir ..

# Test with the provided client (the SDK's Streamable HTTP client)
rebar mcp client http://localhost:7232
```

---

## Wire behaviour (rebar 3.2)

| Situation | Answer |
|-----------|--------|
| `initialize` with a version the server speaks | that version (2024-11-05, 2025-03-26, 2025-06-18, 2025-11-25) |
| `initialize` with another version | 2025-11-25 (the client decides whether to disconnect) |
| `server/discover` (2026-07-28) | `supportedVersions`, capabilities, serverInfo; later requests carry the version in `_meta` |
| `tools/call` succeeds | `{"content": [{"type": "text", "text": <answer>}]}` |
| `tools/call` fails (ask exits non-zero, times out, empty question, arguments that break the schema) | an `isError: true` result whose text says why (exit code, stderr, cmd, cwd) — the model can read it |
| `tools/call` of an unknown tool | JSON-RPC error -32602 `unknown tool "<name>"` |
| `resources/read` of an unknown URI | JSON-RPC error -32602 `Resource not found` |
| unknown method | JSON-RPC error -32601 |
| a request before `initialize` (old protocol) | JSON-RPC error -32600 `method "<m>" is invalid during session initialization` (the SDK's message; rebar sets the code, which SDK v1.8.0 leaves at 0 and v1.9.0 sets to -32600 itself) |
| notifications | never answered |

rebar's adapters around the SDK (`cli/internal/mcp`):

- **Aliases** (`compat.go`): tool names and resource URIs match
  case-insensitively — `ask_tdflite_architect` reaches `ask_TDFLite_architect`
  (a real user hit this; red-team finding C2, 2026-04-28).
- **Stdio hygiene** (`stdio.go`): the SDK ends a session at the first
  malformed line; rebar logs it on stderr
  (`ask-mcp-server: ignored invalid message (<why>): <line>`) and reads on, and
  never answers with `"id": null` (Claude Code's validator rejects it).
- **Shutdown** (`shutdown.go`, `stdio.go`, `main.go`): at the end of input
  (the client exited or restarts the server) or on SIGINT/SIGTERM, the server
  stops reading requests (HTTP: stops accepting connections), cancels the
  tool calls still running and waits at most **2 s** (`ShutdownGrace`) for
  the answers due — quick requests answer, a cancelled call answers with an
  `isError` "Agent query cancelled" result — then exits 0. Each ask runs in
  its own process group: cancelling it sends the group SIGTERM, then SIGKILL
  after 1 s (`KillDelay`), and whatever is left of the groups when the server
  exits is killed, so no `rebar ask`, `claude` or `git` outlives the server.
  2 s because Claude Code's stdio client (the TypeScript SDK) closes stdin
  and sends SIGTERM 2 s later: the server is gone before it has to. (The
  background memory compaction `rebar ask` may start after answering is
  detached on purpose and not killed.) `printf '…' | rebar mcp serve --stdio`
  still prints every quick answer, but a `tools/call` piped that way is
  cancelled at the end of input — keep stdin open until it answers.
- **Error code** (`compat.go`): the SDK answers a request before
  `initialize` with code 0; rebar rewrites it to -32600 (invalid request) on
  stdout and on `/mcp` — the code the SDK uses itself from v1.9.0.
- **Guard**: `"arguments": null` counts as absent (it panicked inside the SDK
  v1.8.0), and a panic in a handler is an internal error, not a crash.

## What This Achieves

### 1. **Standards Compliance** ✅
- The official MCP Go SDK: JSON-RPC 2.0, protocol versions 2024-11-05 → 2026-07-28
- Tool failures as `isError` results, protocol errors as JSON-RPC errors
- Schema validation for all inputs

### 2. **Tool Discovery** ✅
- Agents automatically exposed as tools
- Rich schema definitions with validation
- Human-readable descriptions from AGENT.md files

### 3. **Resource Access** ✅
- Agent memory accessible as MCP resources
- Interaction logs readable via MCP
- Agent configs exposed for introspection

### 4. **Multi-Transport** ✅
- Streamable HTTP for web/API clients
- Stdio for command-line MCP clients
- Same protocol over both transports

### 5. **Backward Compatibility** ✅
- Existing `/v1/ask` API unchanged
- All existing ASK clients continue working
- MCP layer is additive, not replacing

---

## Technical Innovations

### Agent-to-Tool Mapping

**Challenge:** Convert dynamic agent discovery into static tool definitions

**Solution:** Systematic naming convention
```
Agent: repo:role  → Tool: ask_repo_role
rebar:architect   → ask_rebar_architect
billing:steward   → ask_billing_steward
```

### Resource URI Scheme

**Challenge:** Expose agent internals as resources

**Solution:** Custom URI scheme
```
ask://memory/repo:role  → agents/role/memory.md
ask://log/repo:role     → agents/role/memory.log.md
ask://agent/repo:role   → agents/role/AGENT.md
```

### Schema Generation

**Challenge:** Generate JSON schemas from agent capabilities

**Solution:** Standardized schema template with agent-specific descriptions
- Extract description from AGENT.md first paragraph
- Standard parameters (question, verbose) for all agents
- Consistent validation rules across all tools

### Error Handling

**Challenge:** Map ASK errors to MCP

**Solution (rebar 3.2):** a failing ask is a *tool execution error* — an
`isError: true` result carrying the ask's stderr, exit code, command and
working directory — so the model sees why and can retry or rephrase. Only
protocol problems are JSON-RPC errors (-32602 unknown tool or resource /
invalid params, -32601 unknown method). The python server answered failing
asks with -32603 "Tool execution failed".

---

## Files Created

1. **`cli/internal/mcp/`** - the MCP server, ask-server and test client (Go; `rebar mcp serve`, `rebar ask serve`, `rebar mcp client`)
2. **`bin/ask-mcp-server`, `bin/ask-server`** - sh shims that run those commands (for `.mcp.json` files and scripts that name them)
3. **`docs/MCP-IMPLEMENTATION.md`** - This documentation

---

## What You Can Do Now

**Any MCP client can:**
- Discover all your ASK agents as callable tools
- Execute agent queries via standard MCP protocol
- Read agent memory, logs, and configs as resources
- Use ASK in any MCP-compatible workflow

**ASK is now a true MCP server** - not just "MCP-ready" but fully MCP-compliant with working tool and resource discovery.

The transformation from "shared service architecture" to "MCP-enabled swarm coordination platform" is complete! 🎉