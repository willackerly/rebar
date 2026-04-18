# CONTRACT-SEAM-{SOURCE}-{TARGET}.{MAJOR}.{MINOR}

<!-- Seam contracts define integration points between components that cross
     language, protocol, or service boundaries. They explicitly map how data
     transforms at each boundary crossing.
     
     Regular contracts define what a component does.
     Seam contracts define how two components talk to each other.
     
     Naming: CONTRACT-SEAM-{source-component}-{target-component}.{version}.md
     Example: CONTRACT-SEAM-API-FRONTEND.1.0.md
-->

## Overview

| Field | Value |
|-------|-------|
| **Source** | [Component/service that produces the data] |
| **Target** | [Component/service that consumes the data] |
| **Protocol** | [HTTP/JSON, gRPC, WebSocket, direct function call, etc.] |
| **Direction** | [unidirectional / bidirectional] |

**Why this seam exists:** [Brief explanation of why these components communicate]

**Implements:** [Link to BDD feature if applicable]

## Endpoint / Interface

<!-- For HTTP APIs: -->
```
METHOD /api/v1/path/{param}
Content-Type: application/json
Authorization: Bearer <token>
```

<!-- For function calls: -->
```typescript
function processDocument(input: SourceType): TargetType
```

<!-- For events/messages: -->
```
Channel: document.updated
Payload: { documentId: string, version: number, ... }
```

## Type Mapping

<!-- The critical section: how types transform across the boundary.
     This is where bugs hide — Go *bool becomes JSON null becomes TS undefined. -->

| Source Type | Wire Format | Target Type | Notes |
|-------------|------------|-------------|-------|
| `string` | `"value"` | `string` | Direct mapping |
| `*bool` (Go pointer) | `true` / `false` / `null` | `boolean \| undefined` | Nullable — null maps to undefined, not false |
| `[]string` (empty) | `[]` | `string[]` (empty) | Empty array, NOT null/undefined |
| `[]string` (nil) | `null` | `undefined` | Go nil slice serializes as null |
| `time.Time` | `"2026-03-21T..."` | `Date` (via `new Date()`) | ISO 8601 format |
| `int64` | `number` | `number` | Safe up to Number.MAX_SAFE_INTEGER (2^53-1) |
| `error` | `{ "error": "msg", "code": "ERR_X" }` | `throw new ApiError(code, msg)` | See Error Mapping below |

## Request Shape

```json
{
  "field1": "string — required",
  "field2": 42,
  "field3": null
}
```

**Validation rules:**
- `field1`: required, non-empty, max 255 characters
- `field2`: required, positive integer
- `field3`: optional (omit or null)

## Response Shape

```json
{
  "data": {
    "id": "uuid",
    "status": "active | pending | archived",
    "metadata": {}
  },
  "pagination": {
    "total": 100,
    "offset": 0,
    "limit": 20
  }
}
```

## Error Mapping

| Source Error | Wire Code | Wire Message | Target Error | HTTP Status |
|-------------|-----------|-------------|-------------|-------------|
| `ErrNotFound` | `NOT_FOUND` | `"Resource not found"` | `ResourceNotFoundError` | 404 |
| `ErrValidation` | `VALIDATION` | `"Invalid input: {details}"` | `ValidationError` | 400 |
| `ErrUnauthorized` | `UNAUTHORIZED` | `"Authentication required"` | `AuthError` | 401 |
| `ErrInternal` | `INTERNAL` | `"Internal server error"` | `ServerError` | 500 |

## Behavioral Contracts

<!-- What invariants hold across this seam? -->

| Behavior | Specification |
|----------|--------------|
| Empty collection | Returns `[]` (empty array), never `null` or omitted |
| Missing optional field | Omitted from response (not `null`, not `""`) |
| Concurrent requests | Safe — no request-ordering dependencies |
| Idempotency | `PUT` and `DELETE` are idempotent; `POST` is not |
| Pagination | Offset-based; total is always accurate at time of query |

## Test Requirements

<!-- How to verify this seam works correctly -->

**Contract tests:**
```bash
# From the source side: does the response match the declared shape?
go test ./internal/handlers/ -run TestSeam

# From the target side: does the client parse the response correctly?
npx vitest run src/api/__tests__/seam.test.ts
```

**Integration test:**
```bash
# End-to-end: source → wire → target → verify
npx playwright test e2e/api-contracts.spec.ts
```

## Implementing Files

<!-- Updated by: grep -rn "CONTRACT:SEAM-{SOURCE}-{TARGET}" src/ internal/ -->

**Source side:**
- `internal/handlers/resource.go` — produces the response
- `internal/handlers/resource_test.go` — contract tests

**Target side:**
- `src/api/resourceClient.ts` — consumes the response
- `src/api/__tests__/resourceClient.test.ts` — contract tests

**Shared:**
- `shared/types/resource.ts` — TypeScript types (if shared type generation)
