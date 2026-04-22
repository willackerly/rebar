# Feedback Inventory

Index of every feedback item's disposition + vote accumulator for deferred
proposals. The goal: when multiple independent projects ask for the same
thing, the accumulation is visible at a glance and we can promote to action.

**Source files:**
- Active (awaiting implementation): `feedback/*.md`
- Decided (implemented / deferred / rejected / redirected): `feedback/processed/*.md`

**Flow:**
1. New feedback lands in `feedback/`
2. Triage assigns disposition → updates this file
3. Source file moves to `processed/` unless pending implementation (Wave 1/2)
4. Deferred items accumulate votes here; when threshold hit → promote to Queued

**Promotion criteria (Watchlist → Queued):**
- 2+ independent projects request the same item, OR
- 1 measured pain point (specific cost in hours/regressions/incidents), OR
- Low-effort items can promote on the next adjacent doc/code touch (opportunistic)

---

## 🔥 Queued for Action

Accepted items with decision made, pending implementation. Source files
remain in `feedback/` root until implementation lands, then move to processed/.

### Wave 1 — Doc-only, ~1 day

| # | Item | Source | Effort |
|---|------|--------|--------|
| W1-1 | Numeric drift principle in DESIGN.md (§Anti-Drift) | [digital-signer-feedback.md](digital-signer-feedback.md) | S |
| W1-2 | Single Source of Truth Table section in AGENTS.template.md | [digital-signer-feedback.md](digital-signer-feedback.md) | S |
| W1-3 | Deploy confirmation TTY-guard pattern in AGENTS.template.md | [digital-signer-feedback.md](digital-signer-feedback.md) | XS |
| W1-4 | Zero-tolerance testing doctrine (AGENTS.template.md + DESIGN.md ref) | [zero-tolerance-testing-feedback.md](zero-tolerance-testing-feedback.md) | S |
| W1-5 | CHANGELOG per-version `### Migration` subsections | [versioning-and-upgrade-path-2026-03-20.md](versioning-and-upgrade-path-2026-03-20.md) | S |

### Wave 2 — Script + template surgery, ~1 day

| # | Item | Source | Effort |
|---|------|--------|--------|
| W2-1 | `O-` operational-contract prefix in CONTRACT-TEMPLATE.md + filedag O1/O2 as reference examples | [2026-04-18-filedag-deep-audit-insights.md](2026-04-18-filedag-deep-audit-insights.md) #1 | M |
| W2-2 | Extend `compute-registry.sh` to detect drift / shadow / ghost / zombie / unlisted contracts | [2026-04-18-filedag-deep-audit-insights.md](2026-04-18-filedag-deep-audit-insights.md) #5 | M |

### Wave 2.5 — MCP activation (COMPLETED 2026-04-20)

Turning a latent MCP server into a discoverable first-class tool for
Claude Code instances. Moved to Implemented section below.

---

## 👀 Watchlist — Deferred, Awaiting Accumulation

Items worth doing but not yet — waiting for a 2nd independent request, a
measured pain point, or an opportunistic doc/code touch. Each row shows the
source project(s) so accumulation is visible.

### Architecture & Contracts

| Item | Votes | Sources | Effort | Rationale for defer |
|------|------:|---------|--------|---------------------|
| Cross-repo `CONTRACT:namespace/ID` syntax | 1 | Office180 | XS | Tiny change — add when 2nd ask lands or a real multi-repo dep appears |
| YAML frontmatter on contracts (`id`/`version`/`namespace`/`depends_on`/`implements`/`mcp_tools`/`tags`) | 1 | Office180 | M | Template friction for solo users; no measured pain yet |
| `security_tier: critical/standard/internal` field on contracts | 1 | Office180 (scalability) | S | Defer with YAML frontmatter; no crypto-team-review workflow yet |
| Contract tiering (Tier-1 contract-owning / Tier-2 architecture-belonging / Tier-3 no-header) formal framework | 1 | Digital Signer | S | conventions.md already has the distinction; formalization waits for explicit adopter confusion |
| `CONTRACT-GAPS.md` template + `check-contract-gaps.sh` | 1 | Digital Signer | S | Redundant with W2-2 extended registry; kill if W2-2 handles it |
| ADR pattern (`decisions/NNNN-title.md`) | 1 | filedag | M | Adds a new convention; wait for 2nd ask OR dogfood in rebar repo itself first |
| Contract impact DAG (`depends_on`/`consumed_by` frontmatter + `check-contract-graph.sh`) | 1 | filedag | L | Piggy-backs on YAML frontmatter decision above |
| Amendment discipline lint (structured `## Amendment <L>` template + >2-amendments warning) | 1 | filedag | S | Over-engineered for current maturity — no amendment rot problem yet |
| Cold-start coherence check (`check-cold-start-coherence.sh`) | 1 | filedag | S | Interesting layering on ground-truth; watchlist for 2nd ask |
| Seam-contract metadata w/ Ed25519 signatures, adapter manifests | 1 | filedag | L | Pre-emptive; wait for 2nd federation use case |
| Deep-review 17-phase audit template | 1 | filedag | L | filedag-shaped; link from CASE-STUDIES.md instead |
| `federation-node` profile | 1 | filedag | M | Pending Phase 14; wait for filedag to finish trial |

### Swarm & Orchestration

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| GC protection + `git fsck --unreachable` recovery protocol in `practices/multi-agent-orchestration.md` | 1 | OpenDocKit | S | Real pattern, low cost — promote opportunistically on next orchestration-doc touch |
| Fan-out overlap detection (conflict matrix pre-launch checklist) | 1 | OpenDocKit | S | Add as text checklist when next editing orchestration doc |
| Oracle Pattern as DESIGN.md primitive | 1 | OpenDocKit | XS | Add as short section on next DESIGN.md touch — zero risk, low cost |
| Namespaced auto-generated outputs (P5.1: `<file>-<agent-id>.<ext>`) | 1 | OpenDocKit | S | Worth a note in orchestration doc; not enough pain to codify a pattern |
| Agent health monitor + heartbeat + shared progress JSONL (P0-P2) | 1 | OpenDocKit | M | Harness-level concern; Claude Code owns the session lifecycle |
| Swarm collective-intelligence (P6-P10: agent_broadcast, cross-repo swarm memory, role-routing, failure-lib, auto-retro) | 1 | OpenDocKit | XL | Different product direction — keep as north-star in DESIGN.md only |
| Semantic worktree branch names | 1 | OpenDocKit | - | Upstream (Claude Code harness) concern |

### Scalability / Multi-repo (Tier 3-4)

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| Contract catalog (git-repo-based, `catalog-collect.sh` + `build-index.sh`) | 2 | Office180, scalability-deep-review | M | No adopter at Tier 3 yet; premature to build |
| CI-triggered catalog collection | 1 | scalability-deep-review | M | Follows catalog; premature |
| Cross-repo breaking-change detection script | 1 | scalability-deep-review | S | Follows catalog |
| `WORKSTREAMS.md` split from AGENTS.md | 1 | scalability-deep-review | S | Document as optional in small-team profile instead of structural change |
| `compute-metrics.sh` (generate METRICS, not verify) | 1 | scalability-assessment | S | Tier 2 optimization; wait for real merge-conflict pain |
| Un-gitignore shared agent memory + summarizer | 1 | scalability-assessment | M | log/summary split done; summarizer waits for demand |
| Maturity model L1-L5 self-assessment doc | 1 | scalability-assessment | S | Add when 2nd adopter asks how to self-assess |
| Template profiles strip AGENTS.md bloat (solo gets 10KB not 39KB) | 1 | scalability-deep-review | M | solo-dev profile partially addresses; wait for bloat complaint |
| Rebar script version header + stale-copy detection | 1 | scalability-deep-review | S | Reasonable; wait for "your script is stale" incident |
| Tier-aware steward checks (don't enforce Tier-3 rules on Tier-1 projects) | 1 | scalability-deep-review | M | Add when Tier 3 rules actually exist to enforce |
| Optional-frontmatter mode (Tier 1-2 = prose only, Tier 3+ = YAML required) | 1 | scalability-deep-review | S | Depends on YAML frontmatter decision |
| "Minimum viable conventions" section in conventions.md | 1 | scalability-deep-review | XS | Opportunistic — add on next conventions.md touch |

### ASK CLI feature requests

| Item | Votes | Sources | Decision |
|------|------:|---------|----------|
| `ask peek` / `ask diff` / `ask trace` / `ask broadcast` | 1 | OpenDocKit | **REJECT** — expands ASK from interrogation into orchestration; muddies clean value prop |
| `do <role> "..."` imperative variant of ask | 1 | OpenDocKit | **REJECT** — same rationale; keep ASK purely interrogative |
| "Context preservation" reframe in ASK README | 1 | blindpipe | XS — opportunistic README copy-edit; low cost, clear improvement |

### Subagent Skills

| Item | Votes | Sources | Decision |
|------|------:|---------|----------|
| `/cherry-pick-resolve` auto keep-both resolver skill | 1 | OpenDocKit | **REJECT** — auto "keep both" resolution risks silent corruption of incompatible implementations |
| `/sbs-assess`, `/pincer-investigate` (visual-fidelity skills) | 1 | OpenDocKit | Domain-bound to rendering-fidelity; stays in OpenDocKit |
| `/fanout-audit` (automated overlap detection) | 1 | OpenDocKit | Reduce to checklist text under "Fan-out overlap detection" above |
| `/merge-worktree`, `/worktree-cleanup` | 1 | OpenDocKit | Existing `merge-coordinator.md` subagent covers this |
| `/sbs` intelligent-defaults wrapper | 1 | OpenDocKit | OpenDocKit-specific |

### Role Definitions

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| Expand role AGENT.md skeletons to 50-150 lines each (per-role context, decision frameworks, grep commands) | 1 | Dapple SafeSign | M | Skeletons exist; expansion waits for 2nd project to validate which sections carry real weight |
| Role discipline pattern in AGENTS.template.md ("every session asserts a role at startup") | 1 | blindpipe | XS | Opportunistic — add on next AGENTS.template.md touch |

### Memory System

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| Memory summarization (`/memory-compact`) | 1 | OpenDocKit | M | log/summary split exists; actual summarizer waits for memory-bloat incident |
| Hot/warm/cold memory tiering | 1 | OpenDocKit | M | Speculative — current system workable |

### Session Lifecycle

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| **Pre-flight repo-state check** (read `.rebar-version`, compare to latest tag, warn if behind, link CHANGELOG migration) | 1 | FontKit (via conversation 2026-04-19) | S | Real pain reported; if 1 more adopter hits this, promote. Candidate: `rebar doctor` or extend `rebar status`. |

### Testing rigor (from 2026-04-22-testing-rigor-six-moments.md)

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| **Tag-to-CI coverage check** — every `@<tag>` in spec files must have a path to CI or be allowlisted with a reason | 1 (measured pain) | Dapple SafeSign 2026-04-22 | S | **Prototype attached** in source project: `scripts/check-tag-ci-coverage.mjs` + allowlist JSON, battle-tested (surfaced 35 pre-existing orphan tags on first run). Caught the specific failure where `@security-audit` ran nowhere in CI. Lift directly into `templates/scripts/` if useful. |
| File-to-tier matrix (path → required tier must pass before commit) | 1 (measured pain) | Dapple SafeSign 2026-04-22 | S | Prevents the "web vitest green = committable" failure mode when editing files that only a slow tier exercises. Related to existing Wave 1 zero-tolerance testing doctrine but adds a *which tier must run* dimension. |
| Negative-control mandate for detection tests (stage the violation, prove the detector fires) | 1 (measured pain) | Dapple SafeSign 2026-04-22 | XS | Script-enforceable: spec-file linter greps for `.not.toContain / toBeNull / toEqual([])` patterns and requires a sibling `negative control` describe. Catches tautological tests that pass on a clean environment. |
| Test Fidelity Ladder — formalize `fidelity: tautology / surrogate / real-flow / mutation-proof` declaration in spec headers | 1 (measured pain) | Dapple SafeSign 2026-04-22 | S | References existing Fidelity Ladder concept; adds machine-checkable comment requirement. For `surrogate` decls, verify a matching `real-flow` test covering the same claim exists. |
| Drift-mode taxonomy for differential tests (enumerate `DriftModes: covered` / `NOT covered`) | 1 | Dapple SafeSign 2026-04-22 | XS | Convention + optional linter. Forces comparison-test authors to think through what their regex/fingerprint actually proves vs. misses. |
| Security-test commit-message template (required fields: Claim / Fidelity / Drift modes NOT covered / Negative control / CI job) | 1 | Dapple SafeSign 2026-04-22 | XS | `commit-msg` hook. Forces honesty at commit time when the claim is security-critical. Lowest priority of the six. |

---

## ✅ Implemented (recent commits)

These items were substantially addressed by commits in the last cycle.
Listed so future feedback on the same topic can see prior work and not
re-request.

### Wave 2.5 — MCP activation (2026-04-20)

Turning a latent MCP server into a discoverable, first-class tool for
Claude Code instances. Pre-this-cycle: server existed but stdio was
broken (stdout pollution) and nothing in the adoption flow wired it up.

| Item | Details |
|------|---------|
| Stdout pollution bug fix in `ask-mcp-server` | Status banner now on stderr. Was breaking stdio JSON-RPC handshake — Claude Code would fail to connect. This means the prior "MCP wiring" was never actually working end-to-end. |
| `rebar init` / `rebar adopt` emit project-local `.mcp.json` | `ensureMCPConfig` + `findMCPServerPath` in `cli/cmd/init.go`; auto-called from `bootstrapV2Files`. Tries same-bin, `findRebarRoot`, then PATH for `ask-mcp-server` |
| `docs/MCP-SETUP.md` — new user-facing setup guide | Covers project-level vs user-level paths, verify procedures, pitfalls, commit-or-gitignore guidance |
| README, QUICKSTART, SETUP, bin/README all link MCP-SETUP | MCP is a first-class setup step, not a footnote |
| Will's `~/.claude.json` configured | `rebar-ask` user-level entry; 32 tools across 7 rebar-adopted repos verified end-to-end (TALOS, blindpipe, filedag, fontkit, office180, pdf-signer-web, rebar). Makes ASK available in opendockit + OpenTDF/{TDFLite,otdfctl-main,platform-main} as consumers. |

### Prior cycle

| Item | Implemented By | Source Feedback |
|------|---------------|-----------------|
| MCP wiring for ASK CLI (`bin/ask-mcp-server`) | 24ea799 | ai-native-contracts (MCP schemas, partially), blindpipe (implicit) |
| `rebar init` v2 bootstrap | 92ee243 | versioning-and-upgrade (bootstrap path) |
| `rebar adopt` CLI for existing-project migration | 24ea799 | pdf-signer-migration (full migration guide ask) |
| `rebar audit` CLI | 24ea799 | (general housekeeping) |
| `rebar new` CLI command | 24ea799 | — |
| CLI help grouped by user journey | 44cadcf | — |
| 6 role AGENT.md skeletons (product/architect/englead/merger/steward/tester + engineer via tester) | prior cycle | role-definitions-feedback |
| `practices/session-lifecycle.md` (daemon-aware) | prior cycle | filedag-deep-audit #2 |
| `practices/multi-agent-orchestration.md` | prior cycle | swarm-orchestration-sop (partial) |
| `practices/red-team-protocol.md`, `visual-fidelity.md` | prior cycle | OpenDocKit feedback |
| `scripts/compute-registry.sh` (base functionality) | prior cycle | (precursor to W2-2) |
| `scripts/check-ground-truth.sh` | prior cycle | digital-signer-feedback (base) |
| 7 profiles: web-app, api-service, crypto-library, cli-tool, solo-dev, small-team, department | prior cycle | scalability-assessment, blindpipe-adoption |
| `CHANGELOG.md` + `v2.0.0` git tag | prior cycle | versioning-and-upgrade (core) |
| `.rebar-version` in templates/project-bootstrap | prior cycle | versioning-and-upgrade |
| `memory.md` + `memory.log.md` split | prior cycle | scalability-deep-review |
| `.github/pull_request_template.md` | prior cycle | scalability-assessment |
| 13 subagent prompts (code-review, contract-audit, doc-drift-detector, feature-inventory, merge-coordinator, test-shard-runner, red-team, product-review, ux-review, security-surface-scan, cleanroom-audit, rebar-compliance-audit, example-template) | prior cycle | blindpipe-adoption, OpenDocKit |
| `check-contract-headers.sh`, `check-contract-refs.sh`, `check-todos.sh`, `check-freshness.sh`, `check-compliance.sh` | prior cycle | various |
| `steward.sh` with JSON output | prior cycle | scalability-assessment (catalog precursor) |

---

## 🚫 Rejected / Redirected

| Item | Source | Reason |
|------|--------|--------|
| TDFLite entitlement consumption questions | [2026-04-19-tdflite-entitlement-consumption-ask.md](processed/2026-04-19-tdflite-entitlement-consumption-ask.md) | **Redirected** — cross-repo ask routing (filedag → TDFLite), not rebar methodology feedback. Belongs in filedag's outbox or TDFLite's inbox, not rebar's. |
| `/cherry-pick-resolve` auto-resolver | OpenDocKit | Auto "keep both" risks silent corruption |
| ASK orchestration commands (`peek`/`diff`/`trace`/`broadcast`/`do`) | OpenDocKit, SOP | Expands ASK beyond clean interrogation value prop |
| Swarm collective-intelligence platform (P6-P10) | OpenDocKit SOP | Different product direction — dilutes rebar's "plain files + bash + git" substrate |

---

## Document History

- **2026-04-19** — Inventory created during full feedback scrub. 14 source files triaged (1 duplicate deleted, 9 moved to processed/, 4 kept in feedback/ as in-progress Wave 1/2).
