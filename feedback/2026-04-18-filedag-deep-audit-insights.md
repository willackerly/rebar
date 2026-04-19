# Feedback: filedag Deep Audit Insights for REBAR

**Source project:** filedag (content-addressed file intelligence, Tier 2 REBAR adopter)
**Context:** 2-day deep audit across 17 phases — from federation-vision rubric (Phase 0) through end-state architecture manifest (Phase 11) and contract consolidation (Phase 12)
**Date:** 2026-04-18
**Authors:** Will Ackerly + Claude Opus 4.7 (1M context)

---

## Context

filedag is a content-addressed file intelligence service being reframed as a **personal data sovereignty node in a federated, mutually-delegating agent graph**. Between 2026-04-17 and 2026-04-18 we ran a deep audit applying REBAR v2.0.0 Tier 2 adoption to a mid-sized project with real complexity:

- 25 contracts (post-Phase-12 consolidation)
- 46+ React components
- Dual DB backends (SQLite + PostgreSQL)
- Continuous-daemon operational model
- 40 REST endpoints + WebSocket
- Agent-swarm history (40-parallel Stage 1-4 migration)

**What worked, what didn't, what should evolve in peer REBAR is captured below.** Where possible, insights are paired with concrete template proposals REBAR could adopt.

---

## 5 key insights for REBAR methodology evolution

### 1. Operational contracts gap — formalize the `O-` prefix

filedag's continuous pipeline daemon had no formal contract. Tactical (C/S/I/P) contracts describe structure + interfaces; operational concerns (uptime, indexing lag, cycle time, memory ceiling, error-rate budgets) were implicit.

During Phase 12 consolidation we created two new contracts using an `O-` prefix:

- `CONTRACT-O1-PIPELINE-DAEMON.1.0.md` — daemon SLOs: ≥99% uptime, <60s indexing lag, <30min cycle completion, <4GB resident, <1% enrichment error rate
- `CONTRACT-O2-API-GATEWAY.1.0.md` — API SLOs: p50 <50ms, p95 <500ms, p99 <2s; <0.1% 5xx rate; ≥100 concurrent

**Proposal for REBAR:**

Add `O-` prefix to `CONTRACT-TEMPLATE.md` with fields:

- SLO table (uptime, latency percentiles, error rate, concurrent capacity)
- Startup invariants (fail-fast config validation, migration check, bind order)
- Shutdown invariants (graceful drain, SIGTERM handling, timeout behavior)
- Error recovery (retry policy, stagnation detection, auto-restart)
- Health signals (heartbeat file, endpoint, monitoring cron)
- Tactical counterparts (cross-reference to C/S/I/P contracts)

filedag's O1 and O2 are reference implementations; ready to copy-paste as templates.

---

### 2. Daemon-aware session lifecycle

Peer REBAR's `practices/session-lifecycle.md` assumes stateless sprint work (session = commits). filedag's continuous daemon introduces **long-running operational sessions with no commit granularity** — the daemon accumulates state across hundreds of incremental indexing cycles.

During Phase 1 reality calibration we discovered our own QUICKCONTEXT had drifted (Vitest count claimed 39, actual 5 — metric fabrication) partly because session-end protocol was informal.

**Proposal for REBAR:**

Extend `practices/session-lifecycle.md` with daemon-aware triggers:

- **Checkpoint-by-telemetry** (document-count, cycle count, memory pressure) — not just commit count
- **Heartbeat file + cron monitor + auto-restart** pattern — filedag's `scripts/pipeline-health.sh` is a reference
- **Stagnation detection** — no-progress threshold (default 24h) triggers ops warning
- **Daemon operational session-end protocol** — different from sprint-end; invariants include "finish current cycle, flush buffers, close WAL, checkpoint"

Daemon-mode projects (search indexers, pipeline orchestrators, ML training runs) benefit. Not just filedag.

---

### 3. Federation peer feedback loop formalization

The filedag ↔ peer-REBAR feedback loop is **active in both directions** but informal:

- filedag → REBAR: absorbed parallel-agent protocol, session-lifecycle gap, continuous-daemon findings, MCP scalability need
- REBAR → filedag: session lifecycle pattern, swarm orchestration SOP, scalability assessment

Phase 6 audit proposed formalizing this loop.

**Proposal for REBAR:**

Codify a "field-lab ↔ methodology" feedback cycle:

- **Quarterly sync** between field-lab repos (filedag, Dapple SafeSign, OpenDocKit) and REBAR core
- **Explicit agenda template** (what was discovered, what was painful, what worked, what should template-ize)
- **"Absorption" markers** when REBAR adopts a pattern — trace back to source repo + date for attribution
- **Pilot candidates** — filedag offers to be the formal pilot field-lab (we already have the audit structure + findings)

This makes REBAR evolution traceable and prevents pattern loss.

---

### 4. Seam contract metadata for cross-repo verification

Phase 12 produced `CONTRACT-P4-DELEGATION.0.1.md` — a **cross-repo-promotable contract** for cryptographic delegation chains (certificate-chain analogue). Core structure + verification semantics are invariant across projects; each project customizes scope grammar, key management, trust anchor provisioning.

We identified adapters for: filedag (reference), blindpipe (KAS-managed keys), TALOS (per-node Ed25519), OpenDocKit (PKI cert chain), TDFLite (KAS-based).

**Proposal for REBAR:**

Add a seam contract metadata template:

- `signature` — who published this seam version (Ed25519)
- `version` — semantic version of the seam
- `source_of_truth` — canonical location of the contract definition
- `adapter_manifest` — table of repos consuming this seam with their customizations (scope grammar, key management, trust anchor)
- `lifecycle` (across repos) — computed from each adapter's implementation status

A contract like P4-DELEGATION becomes discoverable across Will's ecosystem: any repo can opt into the delegation pattern with a specific adapter declaration.

---

### 5. Computed lifecycle automation — catch shadow/ghost/zombie contracts

Peer REBAR's `scripts/compute-registry.sh` automates contract-count + lifecycle tracking. filedag's Phase 1 audit found that manual registry maintenance produced:

- **4 lifecycle lies** (S1-API, S2-RESPONSE-PIPELINE, C1-METADATA-STORE, C5-WEB-UI — all understated)
- **2 zombies** (C6-NODE, P1-THEME with 0 code refs)
- **1 ghost** (C7-CONTENT referenced in registry summary with no contract file)
- **1 unlisted** (P2-ABAC.2.0 file exists but not in registry)

In a 14-day-old registry.

**Proposal for REBAR:**

Extend `compute-registry.sh` to auto-detect:

- **Drift:** computed lifecycle differs from registry → flag + optional auto-PR with the diff
- **Shadow contracts:** `// CONTRACT:<ID>` references in code pointing to IDs not in registry
- **Ghost references:** registry rows with no contract file
- **Zombies:** 0-ref contracts (deletion candidates)
- **Unlisted:** contract files not in the registry

Auto-PR on divergence eliminates the drift window entirely. filedag's manual Phase 12 correction took ~2 hours; automation would have caught it in minutes.

---

## Bonus: REBAR Deep-Review template proposal

filedag's audit structure is proposed as a **reusable REBAR template for deep project reviews**. Full draft at `/Users/will/dev/filedag/docs/audits/2026-04-18-rebar-deep-review-TEMPLATE.md`.

Key elements:

- **17-phase structure:** Audit (0-10) → Architectural Consolidation (11-14) → Execution Sequencing (15-17) → Retrospective (18)
- **7 core design principles** (customizable per project: filedag's are federation-shaped; crypto-library adds "no fallback paths"; data pipeline adds "no silent failures"; etc.)
- **12-primitive rubric** (customizable per project)
- **User-stories-as-coverage-lens method** (Phase 2 depth vector; every primitive scored against story coverage)
- **Max-de-risk architectural commits pattern** — lock architectural doors now, phase implementation (filedag uses 9 commits)
- **Fan-out vs main-thread-judgment strategy** — evidence-gathering to research agents; synthesis + judgment on main thread
- **Outputs:** findings + architectural commitments + phased roadmap (always 3 artifacts)

**Production-tested on filedag** (2 days, 17 phases, 9 new contracts, 8 audit findings docs, 3 consolidation deliverables, 1 retrospective scheduled). Ready for promotion into REBAR as `templates/deep-review.md`.

---

## What filedag asks of REBAR next

1. **Review and refine the REBAR Deep-Review template** — adopt as `templates/deep-review.md` or fork with your own additions.
2. **Formalize the `O-` contract prefix** in the CONTRACT-TEMPLATE set; filedag's O1/O2 are reference implementations.
3. **Accept (or counter-propose) the `federation-node` profile** — pending Phase 14 adoption plan in filedag; we'll submit a PR if useful.
4. **Codify cross-repo seam contract metadata** — P4-DELEGATION.0.1 as first use case.
5. **Evolve `compute-registry.sh`** to detect shadow/ghost/zombie contracts + optional auto-PR on drift.

---

## Active feedback loop commitment

filedag commits to continue feeding insights back as the federation architecture implementation progresses:

- Phase 13 SoT Refactor Plan — expected end Q2 2026
- Phase 14 REBAR Adoption Plan — expected Q2-Q3 2026; this is where we execute on peer REBAR adoption including `compute-registry.sh`, session-end protocol, federation-node profile
- Phase 17 Phased Roadmap — Q3 2026; 6/12/24-month federation milestones

Next expected feedback cycle: **Q3 2026**. Topics anticipated: real-world federation-node profile learnings, ABAC 3.0 implementation experience, cross-repo delegation chain first exchange.

---

## References

### filedag artifacts referenced in this feedback

- `docs/END-STATE-ARCHITECTURE.md` v4 — federation-reframed manifest
- `product/FEDERATION-STORIES-DRAFT.md` — 8 user stories + 12 primitives + 7 core principles
- `docs/audits/2026-04-17-phase1-reality-delta.md` — registry drift evidence
- `docs/audits/2026-04-18-phase6-rebar-deep.md` — peer alignment gap analysis
- `docs/audits/2026-04-18-rebar-deep-review-TEMPLATE.md` — proposed REBAR template
- `architecture/CONTRACT-P4-DELEGATION.0.1.md` — cross-repo promotable contract
- `architecture/CONTRACT-O1-PIPELINE-DAEMON.1.0.md` + `O2-API-GATEWAY.1.0.md` — operational contract references

### REBAR peer references

- `agents/subagent-prompts/*.md` — filedag adopted 4 of these; our 2 feedback files in this directory (parallel agent protocol, continuous daemon)
- `practices/session-lifecycle.md` — insight #2 proposes daemon-aware extension
- `profiles/` — insight: filedag needs new `federation-node` profile
- `scripts/compute-registry.sh` — insight #5 proposes drift/ghost/zombie extensions
