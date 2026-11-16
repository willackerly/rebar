# MCP Implementation for ASK - Complete Technical Guide

> **Implementation status (rebar 3.2):** the MCP server is native Go in the
> `rebar` binary — `rebar mcp serve` (package `cli/internal/mcp`). The
> python snippets below are the original design (`bin/legacy/ask-mcp-server.py`,
> runnable with `REBAR_CHECK_IMPL=shell` until it is removed); the Go server
> keeps its wire behaviour: the same tool names, input schema, descriptions,
> resource URIs, error codes/messages/data, notification handling and
> `json.dumps` formatting, verified by parity tests that replay one JSON-RPC
> session and HTTP transcripts through both (`cli/cmd/native_mcp_test.go`).
>
> It is a minimal JSON-RPC 2.0 implementation rather than the official MCP
> Go SDK: the SDK negotiates its own protocol version and answers unknown or
> failing tools with its own error codes and `isError` results, which would
> change what deployed Claude Code sessions see. Deliberate differences from
> the python server: over HTTP the session (initialize) is kept across
> requests (python started a fresh, uninitialized session per request, so
> every `tools/list` after `initialize` failed with -32002), and a tool of a
> repo whose name contains `_` (`ask_dsp_2_8_steward`) resolves to that
> repo's agent (python split the name at the first `_`).

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

1. **`initialize`** - Start MCP session, exchange capabilities
2. **`tools/list`** - Discover available agent tools
3. **`tools/call`** - Execute an agent query
4. **`resources/list`** - Discover agent resources
5. **`resources/read`** - Read agent memory/logs/config

### Transport Options

**HTTP Transport** (default):
```bash
rebar mcp serve --port 7232 --repos-dir /path/to/repos   # or bin/ask-mcp-server …
# MCP endpoint: POST http://localhost:7232/mcp
```

**Stdio Transport** (for MCP clients):
```bash
rebar mcp serve --stdio --repos-dir /path/to/repos
# JSON-RPC over stdin/stdout
```

---

## Usage Examples

### From Any MCP Client

```python
import requests

# 1. Initialize
response = requests.post("http://localhost:7232/mcp", json={
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {"protocolVersion": "2024-11-05"},
    "id": 1
})

# 2. List tools
response = requests.post("http://localhost:7232/mcp", json={
    "jsonrpc": "2.0",
    "method": "tools/list",
    "id": 2
})
tools = response.json()["result"]["tools"]

# 3. Call agent
response = requests.post("http://localhost:7232/mcp", json={
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
        "name": "ask_rebar_architect",
        "arguments": {"question": "What contracts exist?"}
    },
    "id": 3
})
answer = response.json()["result"]["content"][0]["text"]
```

### Testing the Implementation

```bash
# Start MCP server
rebar mcp serve --port 7232 --repos-dir ..

# Test with provided client (bin/test-mcp-client runs the same)
rebar mcp client http://localhost:7232
```

---

## What This Achieves

### 1. **Standards Compliance** ✅
- Full JSON-RPC 2.0 implementation
- MCP protocol version 2024-11-05
- Proper error handling with standard codes
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
- HTTP for web/API clients
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

**Challenge:** Map ASK errors to MCP error codes

**Solution:** Systematic error code mapping
```python
# ASK errors → MCP errors
-32602: Invalid params (missing question, bad agent ID)
-32603: Internal error (ASK command failed, timeout)
-32002: Not initialized (MCP session not started)
-32601: Method not found (unknown MCP method)
```

---

## Files Created

1. **`cli/internal/mcp/`** - the MCP server, ask-server and test client (Go; `rebar mcp serve`, `rebar ask serve`, `rebar mcp client`)
2. **`bin/ask-mcp-server`, `bin/ask-server`, `bin/test-mcp-client`** - hand-offs to those commands (the python originals are in `bin/legacy/`)
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