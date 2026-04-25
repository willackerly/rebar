# CONTRACT-S1-STEWARD.1.0

**Version:** 1.0
**Status:** active
**Owner:** rebar maintainer
**Type:** Service
**Cross-repo Promotability:** Yes — every rebar-adopted repo ships its own steward
**Source:** `practices/red-team-protocol.md` and `DESIGN.md` §9 (The Steward)

## Why this exists

rebar promises that adopters get **facts about project health, not opinions**.
Without an automated scanner, that promise is aspirational — adopters would
have to manually grep for drift, re-derive contract lifecycles, count tests,
and verify enforcement coverage. The Steward is the mechanism that turns
"contracts are the operating system" from a doctrine into a measurable thing
on every CI run.

If the Steward didn't exist, rebar would be just templates plus a methodology
doc — there would be no force-function for "contract drift between
declaration and reality" not to accumulate silently.

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

## Scenarios (illustrative)

### Scenario 1 — pre-commit drift catch

A developer in `filedag` modifies a contract file. `scripts/pre-commit.sh`
runs the Steward in `--check` mode. Steward derives the contract's new
lifecycle from `CONTRACT:` ref counts + test counts, writes the per-contract
JSON to `architecture/.state/`, and returns nonzero if the dev forgot to
update `CONTRACT-REGISTRY.md`. The commit blocks until the developer
regenerates the registry.

### Scenario 2 — fleet audit

A maintainer runs `rebar audit --all ~/dev`. Each adopted repo's
`architecture/.state/steward-report.json` is consulted (or regenerated if
missing). The aggregate scorecard ranks all 8 repos, surfaces the common
failures (e.g., 4 of 8 missing METRICS files), and identifies which adopters
need the most help.

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

## Behavioral Contracts

| Behavior | Specification |
|----------|--------------|
| Lifecycle computation | Status (DRAFT/ACTIVE/TESTING/VERIFIED) is **derived** from `CONTRACT:` ref counts + test counts, never declared |
| Spec gate | A contract is gated on presence of: Interfaces, Behavioral, Errors, Tests, Implementing sections |
| State output | JSON written to `architecture/.state/<contract-id>.<version>.json` per contract + `steward-report.json` aggregate |
| Markdown output | `STEWARD_REPORT.md` is auto-generated; gitignored (regenerated on every scan) |
| Stale detection | If `steward-report.json` is >24h old, `ask steward` warns and suggests re-running the scanner |
| Per-role routing | DRAFT contracts → architect, TESTING gaps → englead, DISCOVERY entries → product |
| Exit code | 0 = clean, 1 = enforcement failures detected |
| Bash version | Compatible with bash 3.2 (macOS default) — no `local -n` namerefs |

## Error Contracts

| Error | When | Behavior |
|-------|------|----------|
| `architecture/` missing | No contracts directory | Exit 1 with message naming `rebar init` as the fix |
| `jq` not installed | JSON processing required | Exit 2 with brew/apt install instructions |
| Single-contract `--check` for unknown ID | `--check C99` where no such contract | Exit 1 with list of valid IDs |
| Stale state without refresh | `--summary` against >24h-old report | Run silently; the staleness is reported in the summary text itself |

## Dependencies

- Depends on: `CONTRACT:S2-ASK-CLI.1.0` (ASK CLI invokes the steward via `ask steward`)
- Depends on: bash 3.2+, `jq`, GNU/BSD `grep`, `find`, `awk`, `sed`
- Configuration: `.rebarrc` (`tier = 1|2|3`)
- External: none

## Cross-references

- **Practices:** `practices/red-team-protocol.md` §Steward integration
- **Doc:** `DESIGN.md` §9 (Steward), `scripts/README.md`
- **Findings:** `feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md` motivated the cross-doc consistency family

## Future evolution

- **Provisional:** the JSON schema in `architecture/.state/` will get a
  formal versioned contract (likely `CONTRACT:I1-STEWARD-STATE.1.0`)
  once a 2nd consumer beyond `rebar audit` emerges.
- **Major-bump trigger:** if the spec-gate sections change (e.g.,
  Why/Who/Scenarios become required for spec-gate completeness).

## Retirement / supersession plan

This is the latest version. No predecessor.

## Implementing Files

- `scripts/steward.sh` — main scanner (624 lines)
- `scripts/_rebar-config.sh` — tier resolution helper
- `scripts/check-*.sh` — individual checks orchestrated by `ci-check.sh`
- `scripts/ci-check.sh` — composite runner that invokes Steward + checks
- `agents/steward/AGENT.md` — reader-role definition for ASK
- `agents/steward/commands/*.sh` — ask-steward subcommand executables
- `templates/project-bootstrap/scripts/steward.sh` — adopter copy (synced via `sync-bootstrap.sh`)

## Test Requirements

- [ ] Bash 3.2 compatibility verified on stock macOS
- [ ] Each `--<flag>` exits 0 on a clean repo, 1 on a repo with seeded drift
- [ ] JSON output validates against the schema (when schema lands)
- [ ] State files survive a full scan + partial `--check` cycle
- [ ] Stale-detection threshold (>24h) fires correctly

## Cross-repo promotion notes

- **Universal invariants:** lifecycle-computed-not-declared, JSON state
  in `architecture/.state/`, exit code 0/1 semantics, role routing
- **Per-project customization:** the `check-*.sh` set each repo enables
  via `.rebarrc` flags; project-specific metrics in `METRICS` file
- **Specialization-contract naming:** adopters don't typically write
  their own steward — they ship rebar's verbatim
- **Candidate adopting repos:** all 8 already adopt this contract
  implicitly; making it explicit is what this contract does

## Change History

| Version | Date | Change | Migration |
|---------|------|--------|-----------|
| 1.0 | 2026-04-25 | Initial contract — formalizing what already shipped | — |
