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

_Wave 1, Wave 2, and Wave 3 all implemented 2026-04-25 — see Implemented
section below._ Queue is empty pending the next batch of triaged feedback.

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
| ~~Cross-repo `CONTRACT:namespace/ID` syntax~~ | ~~1~~ | ~~Office180~~ | ~~XS~~ | **SUPERSEDED 2026-04-28** by `CONSUMES.md` declaration in cross-repo federation design (CHARTER §1.6). Cross-repo coordination uses owner-repo + contract-id + version_pinned in CONSUMES.md, not inline namespace syntax in CONTRACT: refs. Source: `feedback/2026-04-28-cross-repo-contract-federation.md`. |
| YAML frontmatter on contracts (`id`/`version`/`namespace`/`depends_on`/`implements`/`mcp_tools`/`tags`) | 1 | Office180 | M | Template friction for solo users; no measured pain yet |
| `security_tier: critical/standard/internal` field on contracts | 1 | Office180 (scalability) | S | Defer with YAML frontmatter; no crypto-team-review workflow yet |
| Contract tiering (Tier-1 contract-owning / Tier-2 architecture-belonging / Tier-3 no-header) formal framework | 1 | Digital Signer | S | conventions.md already has the distinction; formalization waits for explicit adopter confusion |
| `CONTRACT-GAPS.md` template + `check-contract-gaps.sh` | 1 | Digital Signer | S | Redundant with W2-2 extended registry; kill if W2-2 handles it |
| ADR pattern (`decisions/NNNN-title.md`) | 1 | filedag | M | Adds a new convention; wait for 2nd ask OR dogfood in rebar repo itself first |
| ~~Contract impact DAG (`depends_on`/`consumed_by` frontmatter + `check-contract-graph.sh`)~~ | ~~1~~ | ~~filedag~~ | ~~L~~ | **SUPERSEDED 2026-04-28** by `CONSUMES.md` + `rebar contract drift-check` in cross-repo federation design (CHARTER §1.6). The drift-check command derives the consumer→owner DAG from CONSUMES.md declarations and reports deltas. The reverse direction (owner→consumers) is via `scripts/scan-consumers.sh`. No frontmatter required. Source: `feedback/2026-04-28-cross-repo-contract-federation.md`. |
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
| ~~Contract catalog (git-repo-based, `catalog-collect.sh` + `build-index.sh`)~~ | ~~2~~ | ~~Office180, scalability-deep-review~~ | ~~M~~ | **REJECTED 2026-04-28** as superseded by the distributed federation model (CHARTER §1.6 + §2.10). Central catalog violates §2.10 ("not a federation registry"); consumer-side `CONSUMES.md` self-declaration achieves the same coordination goal without a central index. Adopters who need cross-machine discovery run `scripts/scan-consumers.sh` with explicit repos lists. Source: `feedback/2026-04-28-cross-repo-contract-federation.md`. |
| ~~CI-triggered catalog collection~~ | ~~1~~ | ~~scalability-deep-review~~ | ~~M~~ | **REJECTED 2026-04-28** — depended on the rejected catalog above. |
| ~~Cross-repo breaking-change detection script~~ | ~~1~~ | ~~scalability-deep-review~~ | ~~S~~ | **SUPERSEDED 2026-04-28** by `rebar contract drift-check` (consumer-side) + `scripts/check-version-bump.sh` (owner-side) in the federation design. |
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
| `ask peek` / `ask diff` / `ask trace` / `ask broadcast` | 1 | OpenDocKit | **REJECT** — expands ASK from interrogation into orchestration; muddies clean value prop. *(Scope reaffirmed 2026-04-28 alongside `ask featurerequest` — see note below.)* |
| `do <role> "..."` imperative variant of ask | 1 | OpenDocKit | **REJECT** — same rationale; keep ASK purely interrogative. *(Scope reaffirmed 2026-04-28 — see note below.)* |
| "Context preservation" reframe in ASK README | 1 | blindpipe | XS — opportunistic README copy-edit; low cost, clear improvement |
| `ask featurerequest` gated intake role | (maintainer) | rebar-direct 2026-04-28 | **IMPLEMENTED** — see CHARTER §2.9 + `agents/featurerequest/AGENT.md`. The bounded exception to the no-orchestration doctrine: write surface is a single deterministic file shape (`feedback/FR-*.md`), guarded by CHARTER §3 acceptance gates, no `git commit`. |

**Note on no-orchestration doctrine (2026-04-28):** The `ask featurerequest`
role files structured artifacts as a side effect — formally a write op,
which the prior rejection of `do/peek/diff/trace/broadcast` would seem to
preclude. The reconciliation: featurerequest is still interrogative in
shape (caller asks, agent decides+responds), but produces a deterministic
filed artifact instead of a free-text answer when CHARTER §3 gates pass.
The rejected `do/peek/diff/trace/broadcast` proposals were unbounded
action surfaces; featurerequest's write surface is `feedback/FR-*.md`
plus vote increments on `INVENTORY.md` — bounded and auditable. CHARTER
§2.9 makes this carve-out explicit. Future write-capable ASK roles must
similarly justify their bounded surface vs the default no-orchestration
posture.

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

### Cross-Repo Contract Federation — CHARTER amendments (2026-04-28 eve)

CHARTER §1.6 (Cross-Repo Contract Federation) and §2.10 (Not a federation
registry / package manager) amendments landed. Source proposal:
`feedback/2026-04-28-cross-repo-contract-federation.md`. Subsequent
commits land the implementation: CONSUMES.md template, owner-side
scan/flush scripts + post-commit version-bump detector, consumer-side
`rebar contract drift-check` + `rebar contract upstream` commands,
compliance gating when CONSUMES.md present.

| Item | Disposition |
|------|-------------|
| CHARTER §1.6 (federation as IS-positive) | **Implemented** — pure addition; "discipline, not infrastructure" framing locks composition-over-inheritance + async outbox model. |
| CHARTER §2.10 (NOT a federation registry / package manager) | **Implemented** — preempts the central-registry temptation; mandates local-machine discovery via CONSUMES.md greping. |
| Cross-repo `CONTRACT:namespace/ID` syntax (Office180) | **Superseded** — Watchlist entry struck-through; CONSUMES.md achieves the same coordination via owner_repo + contract_id + version_pinned fields, no inline syntax change needed. |
| Contract impact DAG (filedag) | **Superseded** — Watchlist entry struck-through; `rebar contract drift-check` derives consumer→owner DAG from CONSUMES.md, no frontmatter required. |
| Contract catalog (Office180 + scalability-deep-review, 2 votes) | **Rejected** — Watchlist entry struck-through; central catalog violates new CHARTER §2.10. The 2-vote-promotion threshold doesn't override charter constraints. |
| CI-triggered catalog collection | **Rejected** — depended on rejected catalog. |
| Cross-repo breaking-change detection script | **Superseded** — covered by drift-check + check-version-bump. |

### CHARTER + `ask featurerequest` intake role (2026-04-28)

A formal scope anchor (`CHARTER.md`) and a gated MCP intake channel for
external callers who cannot push to rebar directly.

| Item | Disposition |
|------|-------------|
| `CHARTER.md` (IS / IS NOT statement, §3 acceptance gates, §4 fork-instead doctrine) | **Implemented** — anchors all FR triage; amendment-only at section level per §5. Cited from feedback/README.md, agents/featurerequest/AGENT.md, and the bounded-exception note above. |
| `agents/featurerequest/AGENT.md` (four-path triage, append-only, no-commit, provenance fields) | **Implemented** — single ASK role with default file-write permission. Permissions explicitly restrict to `feedback/FR-*.md` (new files only) + INVENTORY.md vote increments. |
| `feedback/FR-TEMPLATE.md` (provenance + charter mapping + triage recommendation) | **Implemented** — required-fields shape for every filed FR. |
| MCP `ROLE_DESCRIPTIONS` entry for `featurerequest` | **Implemented** — `bin/ask-mcp-server` ROLE_DESCRIPTIONS dict; tool surface `ask_<repo>_featurerequest` discoverable to MCP callers. |
| `bin/ask` auto-enables WRITE_MODE for `featurerequest` role | **Implemented** — case-statement in `cmd_ask` so MCP callers (which don't pass `-w`) get the role's intended write surface. Other roles unchanged. |
| `feedback/README.md` documents the FR flow | **Implemented** — covers CHARTER §3 gates, lifecycle, what-the-agent-can/cannot-write, and the "fork instead" boundary. |
| Architect + Product role doctrine for routing missing-feature asks | **Implemented** — both AGENT.md files now point callers at `ask_rebar_featurerequest` instead of filing feedback themselves. Keeps interrogative ASK roles interrogative. |

### Wave 1 + Wave 2 + Wave 3 close-out (2026-04-25)

All three waves landed in a single push after rebar's own Tier-3 dogfooding.
Source files moved to `processed/`.

**Wave 1 — doc-only (5 items)**

| Item | Disposition |
|------|-------------|
| W1-1: Numeric drift principle in DESIGN.md §Anti-Drift | Already shipped during v2.0.0 evolution — DESIGN.md §"Numeric Claims: The Fastest Drift Vector" (line 482). Source now in `processed/digital-signer-feedback.md`. |
| W1-2: Single Source of Truth Table | Implemented for adopters in `templates/project-bootstrap/AGENTS.md` §"Single Source of Truth for Metrics". |
| W1-3: Deploy TTY-guard pattern | Implemented inline in `templates/project-bootstrap/AGENTS.md` §"Production Deploy Confirmation" — full pattern listed alongside link to `practices/deployment-patterns.md` for the broader catalog. |
| W1-4: Zero-tolerance testing doctrine | Implemented in `templates/project-bootstrap/AGENTS.md` §"The Scout Rule: Zero Tolerance for Broken Tests". Source now in `processed/zero-tolerance-testing-feedback.md`. |
| W1-5: CHANGELOG `### Migration` per-version subsections | Already shipped in CHANGELOG since v1.0.0 (every release entry has its own Migration subsection). Source now in `processed/versioning-and-upgrade-path-2026-03-20.md`. |

**Wave 2 — script + template surgery (2 items)**

| Item | Disposition |
|------|-------------|
| W2-1: `O-` operational-contract prefix + D / T prefixes | Added to DESIGN.md §"ID prefixes (suggested)" and `architecture/README.md` naming-convention table. Filedag's O1-PIPELINE-DAEMON and O2-API-GATEWAY cited as worked examples inline. CONTRACT-TEMPLATE.md already had `Operational` in the `Type` field as of 2026-04-25 dogfooding. Source now in `processed/2026-04-18-filedag-deep-audit-insights.md`. |
| W2-2: Extend `compute-registry.sh` for drift/shadow/ghost/zombie/unlisted | Implemented as a "Contract Health" section appended to the auto-generated registry. Drift caught by existing `--check` mode; zombies (0 impl refs) caught by existing orphan detection (renamed Zombies for clarity); shadows (CONTRACT: refs to nonexistent IDs) added as a new pass over source — same extension whitelist + bin/ second-pass as `count_implementations`. Ghost / unlisted are both architecturally prevented (registry is regenerated from files, never hand-maintained). Same source. |

**Wave 3 — regression-fix mechanical gates (4 items)**

| Item | Disposition |
|------|-------------|
| W3-1: `practices/regression-fix-protocol.md` codifying Gates G/H/I/J/K/L | Shipped as a single practice doc with the cross-cutting "enumerate dimensions before verifying" insight. |
| W3-2: `scripts/check-fix-commit.sh` (Gate G) | Shipped. commit-msg hook that fails `fix:`/`regression:` commits without a `Reproduced on:` line. Bash 3.2 compatible (uses `grep -E` instead of bash regex for portability). |
| W3-3: `scripts/check-bypass-flags.sh` (Gate I) | Shipped. commit-msg hook that fails commits mentioning `--skip-*` / `--no-verify` / `--force` / `SKIP_*=` without a `Bypass tickets:` line. |
| W3-4: AGENTS.template.md doctrine for Gates H + L | Added to both `project-bootstrap/AGENTS.md` (slim adopter copy) and `component-templates/AGENTS.template.md` (full reference) just above the Scout Rule, with cross-link to the practice doc. |

Both new scripts wired into `scripts/ci-check.sh` (Fix Commit Gate, Bypass Flags Gate); ci-check now runs 13 checks, 13/13 green on rebar itself. Bootstrap copies synced via `sync-bootstrap.sh`.

Side benefit: discovered and fixed a CONTRACT-shadow false-positive on `scripts/check-contract-headers.sh` example strings (same trick as the earlier `cli/cmd/context.go` fix — split the literal `CONTRACT:` prefix via shell variable so the regex can't construct it from source).

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

### Next big push (queued)

- [ ] **Opportunistic auto-federation experiments** — `feedback/2026-04-28-auto-federation-experiment.md`. 5 candidate experiments to close the federation loop end-to-end while preserving CHARTER §1.6 + §2.10 + `practices/federation.md` Principle 4 ("tries, doesn't require"). Test plan staged in 3 phases (manual baseline → one auto-arrow at a time → measure). 7 open questions await maintainer decisions (test partner repo, cron cadence, auto-PR bot identity, fatigue thresholds, etc.).

### Repo housekeeping

- [ ] Working tree has untracked deletions from stale worktrees: `.claude/worktrees/agent-a28b156a/` and `.claude/worktrees/agent-a3c77b08/`. Either `git clean` them or commit the deletions.
- [ ] `.DS_Store` modified in root — confirm it's in `.gitignore`.
- [ ] `bin/__pycache__/` appears as untracked — add to `.gitignore` if not already.

---

## Document History

- **2026-04-28 (eve)** — CHARTER §1.6 + §2.10 amendments for cross-repo
  contract federation landed. Three predecessor Watchlist items struck
  through (Office180 cross-repo syntax, filedag impact DAG, contract
  catalog) — first two superseded by CONSUMES.md + drift-check, third
  rejected as charter-incompatible (would have created a central
  registry, violating new §2.10). Source proposal:
  `feedback/2026-04-28-cross-repo-contract-federation.md`.
  Subsequent commits land the implementation across 5 themed clusters
  (CHARTER → CONSUMES.md → owner-side → consumer-side → compliance).
- **2026-04-28** — Landed CHARTER.md + `ask featurerequest` gated intake role. New artifacts: `CHARTER.md` (§1 IS, §2 IS NOT, §3 hard gates, §4 fork-instead, §5 amendment process), `agents/featurerequest/AGENT.md` (four-path triage doctrine), `feedback/FR-TEMPLATE.md` (provenance + charter mapping). MCP wiring: ROLE_DESCRIPTIONS entry in `bin/ask-mcp-server`; WRITE_MODE auto-enable in `cmd_ask` for `featurerequest`. Architect + product AGENT.md updated to route missing-feature asks through the intake gate. Bounded-exception carve-out in ASK CLI feature-requests table reconciles with prior rejection of `do/peek/diff/trace/broadcast`.
- **2026-04-25 (eve)** — Knocked out all queued waves (Wave 1, 2, 3 → Implemented). Net new artifacts: `practices/regression-fix-protocol.md`, `scripts/check-fix-commit.sh`, `scripts/check-bypass-flags.sh`, doctrine additions to AGENTS templates, D/O/T contract prefixes in DESIGN + architecture/README, "Contract Health" section in compute-registry.sh output. ci-check.sh now runs 13/13 enforcement on rebar itself. Five more source files moved to `processed/` (digital-signer, zero-tolerance, versioning, filedag-deep-audit + the two from the 2026-04-26 triage round).
- **2026-04-26** — Triaged 2026-04-24-process-gates-G-through-L.md (Wave 3 queued for the 3 universal gates G/I/L, Watchlist for project-specific H/J/K) and 2026-04-26-webcrypto-ed25519-quirks.md (Watchlist entry for cross-language canonical-fixture pattern; rediscovered Oracle Pattern was already implemented in DESIGN.md and moved its stale Watchlist entry to Implemented). Both source files moved to `processed/`.
- **2026-04-25** — NEXT-SESSION-TODO.md folded into this file (Maintainer Queue section above) so there's one canonical planning surface, not two. Concurrent ship: `check-doc-refs.sh`, `check-decay-patterns.sh`, `templates/scripts/check-tag-ci-coverage.mjs`, `sync-bootstrap.sh` + drift check, bash 3.2 fixes for compute-registry.sh, Why/Who/Scenarios required in CONTRACT-TEMPLATE.md, README/QUICKSTART/SETUP cleanup. See commit log for the precise diffs.
- **2026-04-19** — Inventory created during full feedback scrub. 14 source files triaged (1 duplicate deleted, 9 moved to processed/, 4 kept in feedback/ as in-progress Wave 1/2).
