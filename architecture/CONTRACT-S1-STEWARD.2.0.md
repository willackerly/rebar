# CONTRACT-S1-STEWARD.2.0

SUPERSEDES: CONTRACT-S1-STEWARD.1.0

**Version:** 2.0
**Status:** active
**Owner:** rebar maintainer
**Type:** Service
**Cross-repo Promotability:** Yes — every rebar-adopted repo ships its own steward
**Source:** `practices/red-team-protocol.md`, `DESIGN.md` §9 (The Steward), and
`feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md` §1
(the `verified` → `impl-present` rename, decision D4 in `docs/v3-beta-plan.md`)

## Why this exists

rebar promises that adopters get **facts about project health, not opinions**.
Without an automated scanner, that promise is aspirational — adopters would
have to manually grep for drift, re-derive contract lifecycles, count tests,
and verify enforcement coverage. The Steward is the mechanism that turns
"contracts are the operating system" from a doctrine into a measurable thing
on every CI run.

Version 2.0 exists because 1.0's top lifecycle state lied. It was named
`verified` but computed purely from **file presence** (spec sections + impl
files + test files, none executed) — a tak-tdf session documented contracts
reported `verified` whose load-bearing code did not even compile. 2.0 renames
that computed state to **`impl-present`**, which claims exactly what the
Steward can prove. The word `verified` now belongs exclusively to the
*declared* maturity vocabulary (contract `**Status:**` headers), where it
means "active + passing tests/scenarios prove it."

## Who needs this

- **Every adopter's CI pipeline** — invokes `scripts/steward.sh` to gate merges
- **Every adopter's developers** — invoke `ask steward summary` interactively
  to see one-line health and `ask steward check <ID>` to drill into a single
  contract
- **rebar maintainers** — audit fleet-wide compliance via `rebar audit --all
  ~/dev`, which depends on Steward state files
- **MCP-connected Claude Code instances** — call `ask_<repo>_steward` as a
  first-class tool when a question lands on quality / health / drift
- **`agents/englead/AGENT.md` and `agents/architect/AGENT.md`** — read the
  Steward report as their primary signal for routing follow-up work
- **Anything parsing `steward-report.json` or per-contract state JSON** —
  the 2.0 output schema is the breaking surface this version bump exists for

## Scenarios (illustrative)

### Scenario 1 — pre-commit drift catch

A developer in `filedag` modifies a contract file. `scripts/pre-commit.sh`
runs the Steward in `--check` mode. Steward derives the contract's new
lifecycle from `CONTRACT:` ref counts + test counts, writes the per-contract
JSON to `architecture/.state/`, and returns nonzero if the dev forgot to
update `CONTRACT-REGISTRY.md`. The commit blocks until the developer
regenerates the registry.

### Scenario 2 — the honesty boundary (why 2.0 exists)

Maya, auditing a Tier-3 repo, sees a contract at lifecycle `impl-present`.
She knows exactly what that claims: spec sections complete, implementing
files and test files exist — and nothing more. Whether those tests ran green
is answered by the *declared* `**Status:**` field on the contract (only a
human/agent who watched them pass may write `verified` there). Under 1.0 she
would have seen `verified` and either trusted a counterfeit or spent a
30-agent fan-out re-deriving ground truth.

### Scenario 3 — role routing

`ask steward "what needs attention?"` in `pdf-signer-web` returns: 2
DRAFT contracts (architect should complete spec), 1 TESTING contract
without test files (developer should add tests), 3 DISCOVERIES in TODO.md
(product should triage). Each item names the role responsible. The user
runs `ask architect ...`, `ask product ...` next.

## Interfaces

```bash
# Full scan → JSON + markdown report
scripts/steward.sh

# Aggregate JSON to stdout
scripts/steward.sh --json

# One-line health summary
scripts/steward.sh --summary

# Single contract scan
scripts/steward.sh --check C1
```

ASK CLI mirrors the surface:

```bash
ask steward          # → commands/default.sh (full scan)
ask steward summary  # → commands/summary.sh
ask steward json     # → commands/json.sh
ask steward check C1 # → commands/check.sh C1
```

Output schema (the breaking change vs 1.0):

- Per-contract JSON `lifecycle` field values: `draft` | `active` | `testing`
  | `impl-present` (was `verified`)
- Aggregate `summary.contracts` object keys: `total`, `draft`, `active`,
  `testing`, `impl_present` (was `verified`; underscore key matches the
  file's snake_case JSON key style — the lifecycle *value* string keeps the
  hyphen: `impl-present`)
- `--summary` one-liner: `Steward: N contracts (Xd/Ya/Zt/Wip), ...`
  (was `.../Wv`)
- `STEWARD_REPORT.md` summary row and Contract Status table print
  `impl-present` (was `verified`)

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| Lifecycle computation | State (DRAFT/ACTIVE/TESTING/IMPL-PRESENT) is **derived** from `CONTRACT:` ref counts + test counts, never declared |
| Lifecycle vocabulary | Top computed state is `impl-present` — impl + test files exist. The Steward MUST NOT emit `verified` as a lifecycle value; `verified` is reserved for the DECLARED maturity vocabulary in contract headers |
| Computed vs declared | The Steward never reads or writes contract `**Status:**` (declared maturity) fields — that surface belongs to humans/agents and `check-compliance.sh` |
| Spec gate | A contract is gated on presence of: Interfaces, Behavioral, Errors, Tests, Implementing sections |
| State output | JSON written to `architecture/.state/<contract-id>.<version>.json` per contract + `steward-report.json` aggregate |
| Markdown output | `STEWARD_REPORT.md` is auto-generated; gitignored (regenerated on every scan) |
| Per-role routing | DRAFT contracts → architect, TESTING gaps → englead, DISCOVERY entries → product |
| Exit code | A completed scan exits 0 regardless of findings — the scan is a reporter, not a gate. Gating belongs to `ci-check.sh`, which runs the enforcement scripts individually. Non-zero exits are reserved for invocation errors (unknown `--check` ID → 1, missing `jq` → 2) |
| Bash version | Compatible with bash 3.2 (macOS default) — no `local -n` namerefs |

## Error Contracts

| Error | When | Behavior |
|-------|------|----------|
| `architecture/` missing | No contracts directory | Scan runs against zero contracts and exits 0 (creates `architecture/.state/` as a side effect) — an empty repo is not an error to the reporter |
| `jq` not installed | JSON processing required | Exit 2 with brew/apt install instructions |
| Single-contract `--check` for unknown ID | `--check C99` where no such contract | Exit 1 with `Contract not found` + the list of valid IDs |

## Dependencies

- Depends on: `CONTRACT:S2-ASK-CLI.1.0` (ASK CLI invokes the steward via `ask steward`)
- Depends on: bash 3.2+, `jq`, GNU/BSD `grep`, `find`, `awk`, `sed`
- Configuration: `.rebarrc` (`tier = 1|2|3`)
- External: none

## Cross-references

- **Practices:** `practices/red-team-protocol.md` §Steward integration
- **Doc:** `DESIGN.md` §9 (Steward), `scripts/README.md`,
  `conventions.md` §Lifecycle Status Definitions (computed)
- **Findings:** `feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md`
  motivated the cross-doc consistency family;
  `feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md`
  motivated the 2.0 rename (D4)

## Future evolution

- **Provisional:** the JSON schema in `architecture/.state/` will get a
  formal versioned contract (likely `CONTRACT:I1-STEWARD-STATE.1.0`)
  once a 2nd consumer beyond `rebar audit` emerges.
- **Planned tier above `impl-present`:** a behavioral proof state (working
  name `exercised`) gated on a named milestone test that actually executed
  green, recorded as a `proof:` field written by the test run, not by
  file-grep. Deferred to v3.1-scale work (see the 2026-06-19 feedback,
  suggestion 1).
- **Planned stale detection:** `ask steward` warning when
  `steward-report.json` is >24h old. Not implemented in any ask-steward
  command today — listed here so nobody reads it back into the
  behavioral table before it exists.
- **Major-bump trigger:** if the spec-gate sections change (e.g.,
  Why/Who/Scenarios become required for spec-gate completeness), or if any
  lifecycle value string changes again.

## Retirement / supersession plan

- **Predecessor:** `CONTRACT-S1-STEWARD.1.0` — retirement criterion:
  `grep -rn "CONTRACT:S1-STEWARD.1.0"` (excluding `architecture/` and the
  1.0 contract file itself) returns zero, and no fleet repo's tooling still
  parses a `verified` lifecycle value
- **Migration deadline:** the `v3.0.0-beta` tag — rebar's own tree and
  bootstrap templates must be fully migrated before the tag lands
- **Migration owner:** rebar maintainer (Will Ackerly)

Migration for downstream parsers:

- `lifecycle == "verified"` → `lifecycle == "impl-present"`
- `.summary.contracts.verified` → `.summary.contracts.impl_present`
- `--summary` suffix `Nv` → `Nip`

## Implementing Files

- `scripts/steward.sh` — main scanner
- `scripts/_rebar-config.sh` — tier resolution helper
- `scripts/check-*.sh` — individual checks orchestrated by `ci-check.sh`
- `scripts/ci-check.sh` — composite runner that invokes Steward + checks
- `agents/steward/AGENT.md` — reader-role definition for ASK
- `agents/steward/commands/*.sh` — ask-steward subcommand executables
- `agents/architect/commands/default.sh` — parses `summary.contracts.impl_present`
- `templates/project-bootstrap/scripts/steward.sh` — adopter copy (synced via `sync-bootstrap.sh`)

## Test Requirements

- [ ] Bash 3.2 compatibility verified on stock macOS
- [ ] Completed scans exit 0 with seeded drift present (reporter, not
      gate); `--check <unknown-id>` exits 1 and lists valid IDs
- [ ] No output surface (JSON, markdown, one-liner) emits `verified` as a
      computed lifecycle value
- [ ] JSON output validates against the schema (when schema lands)
- [ ] State files survive a full scan + partial `--check` cycle

## Cross-repo promotion notes

- **Universal invariants:** lifecycle-computed-not-declared, the
  `impl-present` naming (adopters MUST NOT rename it back to `verified`),
  JSON state in `architecture/.state/`, scan-exits-0 reporter semantics,
  role routing
- **Per-project customization:** the `check-*.sh` set each repo enables
  via `.rebarrc` flags; project-specific metrics in `METRICS` file
- **Specialization-contract naming:** adopters don't typically write
  their own steward — they ship rebar's verbatim
- **Candidate adopting repos:** all 8 fleet repos; each picks up 2.0 by
  re-syncing `scripts/steward.sh` from the bootstrap template

## Change History

| Version | Date | Change | Migration |
|---------|------|--------|-----------|
| 2.0 | 2026-07-04 | Computed lifecycle `verified` renamed `impl-present` (D4); aggregate JSON key `summary.contracts.verified` → `impl_present`; `--summary` suffix `v` → `ip` | Update anything parsing steward output per the Retirement / supersession plan above |
| 1.0 | 2026-04-25 | Initial contract — formalizing what already shipped | — |
