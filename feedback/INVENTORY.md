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

### Wave 3 — Regression-fix mechanical gates, ~1 day

Source: [`feedback/processed/2026-04-24-process-gates-G-through-L.md`](processed/2026-04-24-process-gates-G-through-L.md). Self-postmortem of the same Dapple SafeSign session that produced the testing-rigor + fidelity-decay feedback. Author's closing thesis: *"prose-form REBAR guidance does not bind agent behavior; what binds behavior is mechanical gates that fail closed."* Aligns with the 2026-04-25 wave (check-doc-refs, check-decay-patterns, sync-bootstrap drift check). Top 3 author-ranked gates plus a doctrine doc.

| # | Item | Gate | Effort |
|---|------|------|--------|
| W3-1 | `practices/regression-fix-protocol.md` — codify Gates G/H/I/J/K/L as a single practice doc adopters reference per-project | all six | S |
| W3-2 | `scripts/check-fix-commit.sh` — commit-msg lint: `fix:`/`regression:`-prefixed commits must contain a `Reproduced on:` line referencing a SHA / deploy URL / log excerpt | Gate G | S |
| W3-3 | `scripts/check-bypass-flags.sh` — commit-msg lint: when commit body mentions `--skip-*` / `--no-verify` / `--force` usage, require a `Bypass tickets:` line listing the broken-test IDs | Gate I | S |
| W3-4 | AGENTS.template.md doctrine: Gates H (single-fix-isolation: each `fix:` commit pairs with a verify step) and L (fix-your-own-test-drift: when your change breaks tests, those tests are part of the same PR or it's incomplete) | Gates H, L | XS |

**On completion:** source already in `processed/` (no implementation-pending file at root); update Implemented section.

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
| ~~Oracle Pattern as DESIGN.md primitive~~ | ~~1~~ | ~~OpenDocKit~~ | ~~XS~~ | **IMPLEMENTED** — DESIGN.md §"Debugging with Cross-Representation Oracles" already exists. Discovered during 2026-04-26 triage (this Watchlist entry was stale). Moved to Implemented below. |
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

### Regression-fix process (from 2026-04-24-process-gates-G-through-L.md)

The 3 universal gates (G, I, L) are queued as Wave 3 above. The remaining 3 are project-specific in their mechanism — Watchlist for now.

| Item | Votes | Sources | Effort | Rationale for defer |
|------|------:|---------|--------|---------------------|
| Gate H — `fix:` commit PR-size limit (≤2 files unless commit body explains why isolation isn't possible) | 1 | Dapple SafeSign 2026-04-24 | S | Project-specific repo-size norms; doctrine in AGENTS.template.md (W3-4) covers the spirit. Promote to Queued if a 2nd adopter reports the spray-fix pattern. |
| Gate J — Test-fixture matrix (`test-fixture-matrix.md` per category enumerating dimensions) | 1 | Dapple SafeSign 2026-04-24 | M | Test infra varies by project (PDF formats, browser DPRs, etc.); rebar can't ship a universal matrix. Could ship a template skeleton. Promote when 2nd adopter codifies one. |
| Gate K — Trust-state-as-variable doctrine for external-verifier tests | 1 | Dapple SafeSign 2026-04-24 | XS | Domain-bound (trust stores, OS keychains, Adobe trust list) — not every project has external verifiers. Add as text in `practices/regression-fix-protocol.md` (W3-1) when written. |

### Cross-language signed-bytes pattern (from 2026-04-26-webcrypto-ed25519-quirks.md)

| Item | Votes | Sources | Effort | Rationale |
|------|------:|---------|--------|-----------|
| `templates/canonical-fixture-pattern.md` — when two repos must agree on signed bytes (assertion chains, audit logs, federated query receipts), freeze a canonicalization spec + ship a fixture both impls test against | 1 | filedag DP3c (D2-RECEIPT cross-impl Go ↔ WebCrypto) | S | Distinct from the existing Oracle Pattern (debugging by closest-to-truth) — this one is about *byte-level cross-language agreement*. Promote when a 2nd cross-language pair lands (likely TALOS ↔ blindpipe assertion chains). |

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

### Cross-Representation Oracle Pattern (rediscovered 2026-04-26)

Already shipped in DESIGN.md §"Debugging with Cross-Representation Oracles" (the "implementation closest to ground truth becomes oracle for debugging the others" pattern from OpenDocKit). The Watchlist entry for this was stale; flagged during 2026-04-26 webcrypto-ed25519 triage when filedag's fixture pattern landed adjacent to it. The fixture-byte-agreement pattern from filedag is a sibling, not a duplicate — that one stays on Watchlist as a separate entry.

### Bootstrap drift + bash 3.2 path-norm + steward arg-skip (2026-04-25)

Filed during TDFLite's Tier-3 push. Three independent bugs, all
script-side, surfaced by an actual adopter doing real Tier-3 work.
Source: `feedback/processed/2026-04-25-bootstrap-template-script-drift-and-bash3.2.md`.

| Item | Disposition |
|------|-------------|
| Bug 1: `check-doc-refs.sh` `${var/\/.\//\/}` produces literal backslashes on macOS bash 3.2 | **Implemented** — replaced parameter expansion with sed (portable across bash 3.2/4+/zsh). Both canonical + bootstrap copy. |
| Bug 2: bootstrap copy of `check-freshness.sh` was stale relative to canonical | **Already fixed** by the 2026-04-25 sync-bootstrap run earlier in the day. The infrastructure exists — `sync-bootstrap.sh --check` is wired into ci-check.sh. The drift the feedback caught is what the check is designed to prevent going forward. |
| Bug 3: `steward.sh` silently downgrades arg-bearing checks to `skip` (e.g., `compute-registry.sh --check`) | **Implemented** — split script-name from args in the for-loop body so `-x` test resolves and the args are passed through. Steward enforcement count corrected (now 7/7, was 6/7 due to silent skip). |
| Bonus diagnosis correction | The feedback hypothesized that a remote `ASK_SERVER=192.168.0.181:7232` env var inherited via `~/.zshrc` was causing MCP tool/call -32603 errors. The code at `bin/ask-mcp-server:482` already strips ASK_SERVER from the subprocess env. Real cause was opaque because stderr was `.strip()`-ed and the error message defaulted to "Ask command failed". **Implemented** — error result now forwards the full stderr verbatim plus exit code + cmd + cwd. Future -32603s will name their actual cause. Plus a startup note when ASK_SERVER is in env to defuse the misdiagnosis. |

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
| **Wave 2.5 follow-ups (2026-04-22 → 2026-04-24)** | |
| Notification handling fix in `ask-mcp-server` | `0db9073` — server was replying to JSON-RPC notifications with id=null, tripping Claude Code's Zod validator and silently dropping the connection. The MCP wiring landed but didn't actually work end-to-end until this. |
| First-paragraph extraction for tool descriptions | `bc936cf` — descriptions were truncating mid-sentence at line wraps (~80 chars). Now extracts full paragraph. |
| Caller-facing role preambles (centralized A1) | `2f52983` — `ROLE_DESCRIPTIONS` dict in MCP server gives each tool description a caller-facing lead ("Owns X. Best for: Y. Prefer over grep when: Z."). All 32 tools now read as tool descriptions, not agent instructions. |

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

## 🧰 Maintainer Queue (rolled in from NEXT-SESSION-TODO 2026-04-25)

Items that are concrete implementation actions but don't fit the
feedback-driven Watchlist / Queued shape. Pick up between feedback waves.

### MCP / ASK follow-ups

- [ ] **A2 — Fix fontkit AGENT.md copy/paste error.** `~/dev/fontkit/agents/architect/AGENT.md` says *"You are the architect agent for **rebar**"* instead of fontkit. 30-second edit, foreign repo. Track here so it doesn't get lost.
- [ ] **A3 — Optional `examples` array in tools/list inputSchema** (MCP server). Helps Claude formulate better queries. ~30 min; low priority.

### Triage backlog

- [ ] **`feedback/2026-04-22-testing-rigor-six-moments.md` per-proposal disposition.** Watchlist has all six items; needs explicit "Queued / Defer / Reject" decisions. Proposal 5 has a working prototype lifted into `templates/scripts/check-tag-ci-coverage.mjs` (2026-04-25). Proposal 6 (security-test commit template) is still unassigned.
- [ ] **`feedback/2026-04-24-contract-discipline-and-jtbd-framing.md` — full disposition.** Why/Who/Scenarios already shipped in `architecture/CONTRACT-TEMPLATE.md` (2026-04-25). Remaining proposals: spike-first practice doc, contract-supersession practice doc, JTBD-presence ci-check, prefix-number-uniqueness ci-check, cross-repo promotion checklist enforcement.
- [ ] **`feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md` — followups.** Grep-detectable patterns (P1, P4, P5, P7) shipped in `scripts/check-decay-patterns.sh` (2026-04-25). Semantic patterns (P2 inverted assertions, P6 hermeticity, P8 single-key gates) still need self-audit-prompt addition to AGENTS.template.md.

### Promotion candidate (opportunistic)

- [ ] **Session-start repo-state check** — currently 1 vote in Watchlist (Session Lifecycle section). Promote when a 2nd adopter reports the same drift surprise OR while you're already editing `rebar status` / `practices/session-lifecycle.md` for another reason.

### Repo housekeeping

- [ ] Working tree has untracked deletions from stale worktrees: `.claude/worktrees/agent-a28b156a/` and `.claude/worktrees/agent-a3c77b08/`. Either `git clean` them or commit the deletions.
- [ ] `.DS_Store` modified in root — confirm it's in `.gitignore`.
- [ ] `bin/__pycache__/` appears as untracked — add to `.gitignore` if not already.

---

## Document History

- **2026-04-26** — Triaged 2026-04-24-process-gates-G-through-L.md (Wave 3 queued for the 3 universal gates G/I/L, Watchlist for project-specific H/J/K) and 2026-04-26-webcrypto-ed25519-quirks.md (Watchlist entry for cross-language canonical-fixture pattern; rediscovered Oracle Pattern was already implemented in DESIGN.md and moved its stale Watchlist entry to Implemented). Both source files moved to `processed/`.
- **2026-04-25** — NEXT-SESSION-TODO.md folded into this file (Maintainer Queue section above) so there's one canonical planning surface, not two. Concurrent ship: `check-doc-refs.sh`, `check-decay-patterns.sh`, `templates/scripts/check-tag-ci-coverage.mjs`, `sync-bootstrap.sh` + drift check, bash 3.2 fixes for compute-registry.sh, Why/Who/Scenarios required in CONTRACT-TEMPLATE.md, README/QUICKSTART/SETUP cleanup. See commit log for the precise diffs.
- **2026-04-19** — Inventory created during full feedback scrub. 14 source files triaged (1 duplicate deleted, 9 moved to processed/, 4 kept in feedback/ as in-progress Wave 1/2).
