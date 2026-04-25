# CONTRACT-{ID}-{NAME}.{MAJOR}.{MINOR}

<!-- Copy this file to create a new contract.
     Replace all {placeholders} with actual values.
     Remove these HTML comments when done. -->

<!-- VERSIONING:
     - When this contract is superseded, add: SUPERSEDED BY: CONTRACT-{ID}-{NAME}.{NEW}
     - When this contract supersedes another, add: SUPERSEDES: CONTRACT-{ID}-{NAME}.{OLD}
-->

**Version:** {MAJOR}.{MINOR}
**Status:** draft | active | deprecated
**Owner:** [team or person responsible]
**Type:** Service | Component | Interface | Protocol | Data Model | Operational | Integration Seam
**Cross-repo Promotability:** Yes/No — if yes, name candidate adopting repos
**Source:** [link to BDD feature file, e.g., `product/features/encrypted-storage.feature`]

## Why this exists

<!-- REQUIRED. Two or three sentences in domain language a non-engineer could
     read. What user need does this serve? What changes if we don't have it?
     This is the JTBD framing — a Job To Be Done, not an interface description.
     If this contract composes with others to serve the JTBD, name the
     composition explicitly ("this abstracts X so P5 can fan out without
     knowing the underlying source"). -->

## Who needs this

<!-- REQUIRED. Distinct list of consumers — other contracts, demo phases, user
     stories, future expected consumers. This list is the cross-reference the
     next reviewer uses to ask "if assumptions change, what re-evaluates?" -->

- **Consumer A** (e.g. `internal/server/chat.go`, Phase 3) — what they need from this contract
- **Consumer B** (e.g. external repo `<peer>`, Sprint 7) — what they need
- **Future Consumer C** (e.g. federation peer, Phase 7) — anticipated need

## Scenarios (illustrative)

<!-- REQUIRED. Two or three concrete walk-throughs that make this contract
     vivid. Use named personas from product/personas/ where they exist; invent
     reasonable ones (Casey, Maya, Sam) where they don't. Each scenario should
     answer: who initiates, what travels through this contract, what success
     looks like, what failure looks like.

     If scenarios are extensive, link to a dedicated doc and summarize here. -->

### Scenario 1 — short title

2-4 sentences. Show the contract being exercised concretely with a named persona.

### Scenario 2 — short title

2-4 sentences. Pick a different angle (failure path, multi-step, edge case).

## Interfaces

<!-- Define the public interface(s) this contract specifies.
     Use your language's idiom — Go interfaces, TypeScript types,
     Python protocols, etc. -->

```go
// Example: Go interface
type BlobStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Put(ctx context.Context, key string, data []byte) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
}
```

## Behavioral Contracts

<!-- Define behaviors that the type system can't enforce.
     These are the things contract tests verify. -->

| Behavior | Specification |
|----------|--------------|
| `Get` on missing key | Returns `ErrNotFound` (not generic error) |
| `Put` with empty data | Returns `ErrInvalidInput` |
| `Delete` on missing key | No-op (idempotent), returns nil |
| `List` with no matches | Returns empty slice, not nil |
| Concurrent safety | All methods safe for concurrent use |

## Error Contracts

<!-- Define the error types/codes this contract uses.
     Implementations MUST use these — not ad-hoc errors. -->

| Error | When | Code |
|-------|------|------|
| `ErrNotFound` | Key does not exist | `NOT_FOUND` |
| `ErrInvalidInput` | Invalid key or data | `INVALID_INPUT` |
| `ErrStorageFull` | Storage capacity exceeded | `STORAGE_FULL` |

## Dependencies

<!-- What does this component depend on? Other contracts, external services,
     configuration. -->

- Depends on: `CONTRACT:I2-KEY-EXCHANGE.1.0` for encryption keys
- Configuration: `BLOBSTORE_PATH` environment variable
- External: none (self-contained)

## Cross-references

<!-- Beyond formal dependencies — what user stories does this serve? What
     primitives does it implement? What memory or findings inform it?
     This is what makes the assumption-change re-evaluation set obvious. -->

- **User stories:** `product/features/<feature>.feature` — which scenarios are gated by this contract
- **Architectural primitives:** Primitive-N (if you maintain a primitives doc)
- **Findings / audits:** `agents/findings/<dated>.md` — evidence used while drafting

## Future evolution

<!-- OPTIONAL. Required when this contract is provisional or has known horizon
     limits. What assumptions are provisional? What would trigger a major
     version bump? Document this so future readers know what's load-bearing vs.
     what's expected to change. -->

- Provisional assumption 1 (e.g., "Go-level filtering; may absorb into DB pushdown when primitives mature").
- Major-bump trigger (e.g., "If wire shape changes, bump v1 → v2").

## Retirement / supersession plan

<!-- REQUIRED when this contract supersedes another OR is itself superseded.
     OPTIONAL otherwise. Without a deadline, superseded contracts accumulate
     indefinite-state lag (see filedag's C9-ABAC retirement-lag finding). -->

- **Predecessor:** `CONTRACT-<ID>.<old-version>` — retirement criterion: `grep -rn "<old-id>"` returns zero
- **Migration deadline:** YYYY-MM-DD or named phase boundary
- **Migration owner:** [team or person responsible for the cutover]

## Implementing Files

<!-- List all files that implement this contract.
     Keep updated — or regenerate with:
     grep -rn "CONTRACT:{ID}-{NAME}" src/ internal/ client/
-->

- `internal/blobstore/file.go` — file-backed implementation
- `internal/blobstore/memory.go` — in-memory implementation (tests)
- `internal/blobstore/blobstore_test.go` — contract tests

## Enforcement Matrix (optional — for security/access contracts)

<!-- When this contract defines a security control (auth, ABAC, rate limiting,
     encryption), list every endpoint/component and whether the control applies.
     This makes gaps discoverable by inspection rather than audit.

| Endpoint / Component | Enforced | Status | Notes |
|----------------------|----------|--------|-------|
| GET /api/v1/users    | Yes      | Active | Role-based access |
| GET /api/v1/stats    | No       | N/A    | Aggregate data only |
| POST /api/v1/upload  | Yes      | Active | Auth + size limit |
-->

## Test Requirements

<!-- What must be tested? Contract tests are king. -->

- [ ] Every method in the interface has at least one happy-path test
- [ ] Every behavioral contract in the table above has a test
- [ ] Every error contract has a test (trigger the error, verify type/code)
- [ ] Concurrent safety verified with `-race` flag
- [ ] Contract tests run against EVERY implementation (file, memory, etc.)

## Cross-repo promotion notes (required when Cross-repo Promotability: Yes)

<!-- Invariants that must hold across all adopting repos, plus per-project
     customization points. -->

- **Universal invariants:** what every adopter must preserve
- **Per-project customization:** which fields/behaviors adopters may specialize
- **Specialization-contract naming:** how adopters name their specialized version (e.g., `CONTRACT-<ID>-<REPO>.{MAJOR}.{MINOR}`)
- **Candidate adopting repos:** which projects are expected to consume this

## Companion File

<!-- Every contract MAY have a companion file: `CONTRACT-{ID}-{NAME}.impl.md`
     (no version number in the companion filename).

     The companion holds tribal knowledge that supports the contract but doesn't
     define behavior: implementation notes, debugging tips, performance
     characteristics, historical context, migration guides.

     Companion edits don't affect contract lifecycle — you can freely update
     the companion without bumping the contract version.

     The companion lives alongside the contract in architecture/:
       architecture/CONTRACT-C1-BLOBSTORE.2.1.md        ← the contract
       architecture/CONTRACT-C1-BLOBSTORE.impl.md        ← the companion
-->

## Change History

| Version | Date | Change | Migration |
|---------|------|--------|-----------|
| 1.0 | YYYY-MM-DD | Initial contract | — |

---

## Template usage notes

The Why / Who / Scenarios sections were promoted to required as of REBAR
v2.1 in response to filedag's 2026-04-24 architectural-spike retrospective.
Older contracts may not yet have them; back-fill opportunistically when you
revisit a contract for any reason. Newly-authored contracts must include them.

The point of Why / Who / Scenarios is **cross-referential integrity for
assumptions**: a future reader (or future-you) can see what user need a
contract serves, who consumes it, and what scenarios it was designed to
support. When assumptions change (a user story drops, a consumer set shifts),
the cross-refs make the re-evaluation set obvious.

Composition-aware framing is preferred over forced standalone-ness: a
contract may serve a JTBD that *composes* with others. State the
composition explicitly ("this abstracts X so P5 can fan out without
knowing the underlying source").
