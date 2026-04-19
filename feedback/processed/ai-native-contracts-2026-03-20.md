# Feedback: AI-Native Contracts — Cross-Repo Namespacing + Machine-Readable Metadata

**From:** Office 180 project (bootstrapped with rebar)
**Date:** 2026-03-20

---

## Problem

Rebar contracts are great for single-repo agent workflows. Two gaps emerged when building a multi-repo product suite (Office 180 + blindpipe + OpenDocKit + TDFLite):

### 1. No cross-repo contract references

When office180's Docs app implements blindpipe's blobstore contract, there's no way to express that dependency. `CONTRACT:C1-BLOBSTORE.2.1` is ambiguous — is that office180's C1 or blindpipe's C1?

### 2. Contracts are human-readable but not machine-parseable

AI agents can read the markdown, but MCP servers, automated validators, and cross-repo tooling need structured data. Currently there's no way to:
- Programmatically extract a contract's interface definition
- Discover what MCP tools a contract exposes
- Build a dependency graph across repos
- Auto-generate MCP tool definitions from contracts

---

## Proposed Solutions (implemented in Office 180)

### Cross-Repo Namespacing

Simple prefix convention compatible with existing grep-based tooling:

```
CONTRACT:C1-APP-REGISTRY.1.0           ← local (no prefix = this repo)
CONTRACT:blindpipe/C1-BLOBSTORE.2.1    ← cross-repo (namespace = repo name)
```

- Unqualified = local to this repo
- Qualified (`namespace/ID`) = external repo
- `grep -rn "CONTRACT:blindpipe/"` finds all cross-repo deps instantly
- Zero new tooling required — just a naming convention

The contract registry gets an **External Dependencies** section:

```markdown
| Namespace | Contract | Version | Used By | Purpose |
|-----------|----------|---------|---------|---------|
| `blindpipe` | C1-BLOBSTORE | 2.1 | File Enclaves | Encrypted blob storage |
```

### AI-Native Frontmatter

Add a structured YAML block to every contract (inside a fenced code block so it renders cleanly in markdown). Machines parse the YAML; humans read the prose below it.

```yaml
id: "C1-APP-REGISTRY"
version: "1.0"
namespace: "office180"
status: "draft"
type: "component"

depends_on:
  - "S2-AUTH.1.0"                     # local
  - "blindpipe/C1-BLOBSTORE.2.1"     # cross-repo

implements:
  - path: "src/registry/app-registry.ts"
    lang: "typescript"

interface: |
  export interface AppRegistry {
    register(app: AppDefinition): void;
    getAll(): AppDefinition[];
    getById(id: string): AppDefinition | undefined;
  }

mcp_tools:
  - name: "app_registry_list"
    description: "List all registered apps"
    input_schema:
      type: "object"
      properties:
        persona: { type: "string" }

tags: ["registry", "apps", "persona"]
```

**What each field enables:**

| Field | Consumer | Purpose |
|-------|----------|---------|
| `id`, `version`, `namespace` | Cross-repo tooling | Unique contract identity |
| `depends_on` | Dependency graphing | Build order, impact analysis |
| `implements` | Steward, CI | Traceability without grep |
| `interface` | AI agents | Understand the shape without reading full contract |
| `mcp_tools` | MCP servers | Auto-register tools from contracts |
| `tags` | Search/discovery | Agents find relevant contracts |

### MCP Tool Schemas

When a contract exposes operations an AI agent might invoke, include MCP tool definitions in the frontmatter. This follows the [Model Context Protocol](https://modelcontextprotocol.io/) spec: `name`, `description`, `input_schema` (JSON Schema).

Not every contract needs MCP tools — only those with operations that make sense for agent invocation. Guidelines:
- Name tools as `{contract_noun}_{verb}` (e.g., `blobstore_put`, `app_registry_list`)
- Descriptions should be self-contained (agent has no other context)
- Include `output_schema` when the response shape matters
- Tag with required clearance level if ABAC-gated

### OpenAPI Integration

For contracts that define HTTP endpoints, reference an OpenAPI spec:

```yaml
openapi: "docs/specifications/app-registry.openapi.yaml"
```

This is optional — many contracts (components, interfaces, protocols) don't have HTTP endpoints.

---

## Recommendations for Rebar

1. **Add `namespace` to CONTRACT-TEMPLATE.md** — defaults to repo name
2. **Add YAML frontmatter block** — structured metadata inside fenced code block
3. **Add `mcp_tools` as optional field** — for AI-invocable contracts
4. **Add External Dependencies section** to CONTRACT-REGISTRY template
5. **Update conventions.md** with cross-repo CONTRACT: header format
6. **Update steward.sh** to parse YAML frontmatter (future — validate depends_on, check implements paths)
7. **Add "AI Native" as a recommended principle** in profiles that use contracts

The beauty is this is all backwards-compatible. Existing contracts without frontmatter still work — the YAML block is additive. Existing single-repo CONTRACT: refs still work — the namespace prefix is optional (unqualified = local).
