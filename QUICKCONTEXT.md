# QUICKCONTEXT — rebar

last-synced: 2026-04-25
**Branch:** main
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology

---

## Current State

- **rebar v2.0.0** released (tag `v2.0.0`)
- **8 rebar-adopted repos** registered in MCP swarm: rebar, TALOS, blindpipe, filedag, fontkit, office180, pdf-signer-web, **TDFLite** (newly added 2026-04-24)
- **37 ASK agents** discoverable from any of the 8 repos via the rebar-ask MCP server
- **3 contracts** for rebar's own load-bearing components (S1-STEWARD, S2-ASK-CLI, S3-MCP-SERVER)
- **15 enforcement scripts** in `/scripts/` (mechanically synced into `templates/project-bootstrap/scripts/`)

## Recent Ship (2026-04-24 → 2026-04-25)

10 commits closing a multi-persona red-team review of rebar:

| Commit | Topic |
|--------|-------|
| `c610fcf` | Filed 2026-04-24 feedback (contract-discipline + fidelity-decay) |
| `3e403f5` | gitignore generated outputs + dev-machine artifacts |
| `0abb549` | bash 3.2 compatibility (compute-registry) + sync-bootstrap mechanism |
| `bd3b999` | check-doc-refs.sh + repaired 38 broken feedback links |
| `aafd9ba` | check-decay-patterns.sh — soft-hardening anti-pattern lens |
| `1ce0281` | tag-to-CI coverage Node prototype in templates/scripts/ |
| `66d002b` | CONTRACT-TEMPLATE — Why/Who/Scenarios required |
| `b79b56b` | Consolidated planning surfaces; deleted CONTRACT-QUICKSTART, AGENTS-QUICKSTART, SITE-MAP, NEXT-SESSION-TODO |
| `a06732b` | Untracked stale agent-harness worktrees (~75K LOC pruned) |
| `949fada` | Rebar CLI rebuild with Go 1.25 |
| `cdb2c45` | rebar audit --all recurses depth-2 |
| `5800647` | ask-mcp-server discovery recurses depth-2 |

## In Progress

- **Max-compliance dogfooding** (this push): adding the structural files
  rebar didn't have for itself (AGENTS.md, QUICKCONTEXT.md, TODO.md,
  METRICS, .rebar-version, contracts) so adopters consulting `ask_rebar_*`
  see exemplary state.

## What's Next

1. Land the 3 contracts (S1-STEWARD, S2-ASK-CLI, S3-MCP-SERVER) and tag source files with `CONTRACT:` headers.
2. Install pre-commit hook in `.git/hooks/pre-commit`.
3. Re-run `rebar audit` — target 9-10/10.
4. Triage the recently-arrived `feedback/2026-04-24-process-gates-G-through-L.md` (untracked).
5. Triage the active feedback files still pending Wave 1/2 implementation:
   - `digital-signer-feedback.md` (numeric drift, single-source-of-truth)
   - `versioning-and-upgrade-path-2026-03-20.md` (CHANGELOG migration sections)
   - `zero-tolerance-testing-feedback.md` (testing doctrine)
   - `2026-04-18-filedag-deep-audit-insights.md` (Wave 2 — O- prefix, registry extension)
   - `2026-04-21-filedag-cross-ref-and-federation-coord.md` (cross-ref check shipped; federation profile pending)

## Known issues / non-blockers

- `feedback/2026-04-24-process-gates-G-through-L.md` is untracked — Will dropped it in 2026-04-24, untriaged.
- 3 of the 4 ci-check failures in rebar's own audit are about being a
  meta-tool: the rebar source isn't an application repo with CONTRACT:
  headers in user-facing source. Tier 3 compliance reframes this — we add
  contracts for rebar's own infrastructure (S1/S2/S3) and tag the relevant
  bash/Python/Go source files.

## Verification (run before trusting this file)

```bash
git log --oneline -10
rebar audit
rebar audit --all ~/dev
scripts/ci-check.sh
```
