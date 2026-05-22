# Spec Package TODO

## High Priority - Additional Format Plugins

### 1. Protobuf Plugin
**Status:** Not started  
**Priority:** High (ubiquitous for gRPC/microservices)

**Implementation:**
- Extension: `.proto`
- SubDir: `protobuf/`
- Extract: Parse `.proto` service definitions + message types from contracts
- Generate: Create contract from `.proto` service/message definitions
- Detect: File extension + `syntax = "proto3"` or `service` keyword

**Use cases:**
- gRPC service contracts
- Message serialization schemas
- Cross-language type definitions

### 2. TypeSpec Plugin
**Status:** Not started  
**Priority:** High (growing adoption, multi-format generation)

**Implementation:**
- Extension: `.tsp`
- SubDir: `typespec/`
- Extract: Parse TypeSpec service definitions from contracts
- Generate: Create contract from `.tsp` models/operations
- Detect: File extension + TypeSpec syntax (`model`, `op`, `namespace`)

**Use cases:**
- Protocol-agnostic service definitions
- Generate OpenAPI, Protobuf, JSON Schema from single source
- Microsoft ecosystem integration

**Notes:**
- TypeSpec can emit OpenAPI/Protobuf, so consider workflow:
  - Option A: Store `.tsp` as canonical, generate OpenAPI/Proto on export
  - Option B: Treat `.tsp` as another format alongside OpenAPI/Proto
- May need TypeSpec compiler integration for multi-format export

## Medium Priority

### 3. AsyncAPI Plugin
**Status:** Not started  
**Priority:** Medium (event-driven architectures)

- Extension: `.yaml` (detect via `asyncapi:` version field)
- SubDir: `asyncapi/`
- Use cases: Kafka, AMQP, WebSocket event contracts

### 4. GraphQL Plugin
**Status:** Not started  
**Priority:** Medium (API-heavy projects)

- Extension: `.graphql` / `.gql`
- SubDir: `graphql/`
- Use cases: GraphQL schema definitions, queries, mutations

### 5. Smithy Plugin
**Status:** Not started  
**Priority:** Low (AWS-focused, niche)

- Extension: `.smithy`
- SubDir: `smithy/`
- Use cases: AWS SDK generation, protocol-agnostic services

## Architectural Improvements

### Refactor export/import to use plugin system
**Status:** Partially complete (plugin interface defined)  
**Next steps:**
- Refactor `export.go` to use `PluginRegistry`
- Refactor `import.go` to use `PluginRegistry`
- Remove hardcoded format logic, delegate to plugins
- Update tests for plugin architecture

### Multi-format compilation
**Status:** Not started  
**Goal:** Support formats that compile to other formats (TypeSpec → OpenAPI/Proto)

**Design considerations:**
- Should contracts store compiled or source format?
- How to track derived specs vs source specs in manifest?
- Compilation step in export workflow

## Testing

### Improve test isolation
**Status:** In progress (intermittent failures in full suite)  
**Issue:** TestExtractGherkinScenarios fails intermittently when run with full suite  
**Fix:** Investigate shared state or file system pollution between tests
