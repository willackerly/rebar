# rebar Charter

> **rebar v2.0.0** | **Tier 3: ENFORCED**

The IS / IS NOT scope statement for rebar. **Authoritative for feature-request
triage** — every `ask featurerequest` submission is scored against this doc
before being filed.

If a request can't be tied to one of the **IS** lines below — or can be tied
to one of the **IS NOT** lines — it doesn't get filed. The asker is told
exactly which §reference applies and why.

For anything beyond a typed missing-feature ask (open-ended discussion,
design-shape collaboration, deep methodology questions), the right path is to
**clone rebar locally, engage with the source, and open a PR** — not to keep
asking through the intake pipe. The intake exists for the narrow case where
"X is a clear gap, please track it"; everything beyond that benefits from
direct repo access.

---

## §1 — What rebar IS

### §1.1 Information Organization Model
A convention for structuring repository knowledge so both humans and AI agents
can navigate at scale: contracts (architectural specifications), BDD scenarios
(product requirements), tier-progressive enforcement (T0–T5 testing), role-based
agent boundaries, persistent cross-session memory.

### §1.2 Swarm Coordination Platform
A protocol substrate for 10+ parallel agents working a single codebase without
collisions: worktree isolation, commit-per-chunk discipline, role-routed
ownership, post-fanout merge cherry-pick discipline, persistent ASK sessions,
collective failure-mode capture.

### §1.3 Plain-Text Substrate
The primary surface is markdown files, bash scripts, `grep`, `jq`, and `git`.
Optional Go and Python binaries (`rebar` CLI, ASK CLI, `ask-mcp-server`)
augment but never replace the plain-text substrate. A team can adopt rebar at
Tier 1 with zero binaries built.

### §1.4 Tier-Progressive Adoption
Five adoption tiers (1: Partial → 5: Federation) let a project absorb rebar in
proportion to its size and pain. Solo devs can land in 15 minutes; departments
take ~2 hours. The same conventions scale across.

### §1.5 Dogfooded Reference
The `rebar` repo itself runs at Tier 3 (ENFORCED) on its own conventions. Any
addition to rebar must pass `scripts/ci-check.sh` against rebar itself, not
just template projects.

---

## §2 — What rebar IS NOT

### §2.1 Not a build / CI / deployment system
rebar does not run builds, manage pipelines, deploy artifacts, or own release
processes. It assumes you have Make / npm / cargo / GitHub Actions / etc. and
sits alongside them. **Out of scope:** "add a build runner," "add deployment
patterns as code," "rebar should integrate with Jenkins."

### §2.2 Not a project management system
rebar does not track tickets, sprints, kanban, time, capacity, or task
assignments. INVENTORY.md is a feedback-disposition index, not a backlog.
**Out of scope:** "add a sprint planner," "add Jira sync," "track time
spent per contract."

### §2.3 Not an LLM / agent framework
rebar sits *on top of* Claude Code (and similar harnesses) — it does not
replace, abstract, or compete with the LLM tooling layer. The ASK CLI is a
persistent-session wrapper, not a model abstraction. **Out of scope:** "add
GPT/Gemini support," "build a model router," "abstract away Claude Code."

### §2.4 Not a knowledge graph / triplestore / vector DB
Contracts and discoveries are markdown with greppable headers. We do not
embed, index, or query semantically. The substrate is `grep` + `jq`.
**Out of scope:** "add embeddings," "build a contract similarity engine,"
"add vector search."

### §2.5 Not a code generator
rebar does not generate code from contracts. Contracts specify behavior;
implementation is a human (or human-orchestrated agent) task. The
`CONTRACT:` header is for navigation, not codegen.
**Out of scope:** "generate Go stubs from contracts," "scaffold tests from
BDD," "auto-implement spec'd interfaces."

### §2.6 Not opinionated about language or framework
rebar conventions apply identically to Go, TypeScript, Python, Rust, etc.
Profiles (`profiles/web-app.md`, `profiles/api-service.md`, etc.) suggest
shapes; they do not impose stacks. **Out of scope:** "add React-specific
contract patterns," "make rebar work better with FastAPI," "first-class
support for Tauri."

### §2.7 Not a competing-doctrine hub
rebar is opinionated about its conventions (contracts, BDD, tier
enforcement, append-only feedback). Requests to add alternative doctrines
in parallel ("add a Scrum profile," "add a feature-toggle methodology") are
out of scope. Fork and adapt if you want a different doctrine.

### §2.8 Not real-time / event-driven
All rebar state changes are explicit `git commit` operations. There is no
hot-reload, no daemon polling repo state, no push notifications. **Out of
scope:** "make MCP config hot-reload," "add file-watcher daemons,"
"contract change broadcasts."

### §2.9 Not orchestration-from-ASK
Per the existing rejection of `ask peek / diff / trace / broadcast / do`:
ASK roles answer questions and (for explicitly write-capable roles) file
structured artifacts. They do not execute arbitrary actions, run code,
trigger jobs, or send messages. The `featurerequest` role's filing
operation is the bounded exception — its write surface is a single
deterministic file shape in `feedback/FR-*.md`, validated by gates before
submission.

---

## §3 — Acceptance gates for feature requests

A request files an FR (`feedback/FR-YYYY-MM-DD-slug.md`) only when **all
four** are true:

1. **In-scope per §1** — the request maps to one or more IS-positives.
2. **Not in-scope per §2** — the request does not match any IS-NOT line.
3. **Concrete use case** — the asker named a specific scenario, not a
   hypothetical. ("In TDFLite we hit X when doing Y" qualifies; "would be
   cool if rebar supported Z" does not.)
4. **Novel** — no existing FR / Watchlist / Implemented entry covers it.
   (If a duplicate exists, the agent increments the vote count in
   INVENTORY.md instead of filing.)

If any of (1–4) fails, the agent **does not file** and instead returns:
- The §reference that disqualified the request (for §2 rejections), or
- The pointer to the existing entry (for §4 duplicates), or
- A request for the missing concrete use case (for §3 vagueness), or
- A "no in-scope match — consider forking" reply (for §1 misses).

---

## §4 — When to engage by fork instead

The `ask featurerequest` pipe is a thin, gated contribution channel for
**clear, typed, missing-feature asks**. It is not a discussion forum, a
design collaboration surface, or an open Q&A.

For any of these, **clone rebar and engage directly** instead:
- Open-ended methodology discussion
- Design-shaped proposals that reshape multiple files
- "Should rebar reconsider X philosophy?" questions
- Counterproposals to Charter §1 or §2 lines
- Cross-cutting refactor suggestions

The async friction of "open a PR with your proposal" is a feature, not a
bug — it filters speculative reshaping from concrete gaps. The vast
majority of substantive engagement should land via direct repo work, not
intake.

---

## §5 — Charter amendment process

This document is itself append-only at the section level. Amendments add
new IS / IS-NOT lines (numbered §1.6, §1.7, §2.10, etc.) rather than
rewriting existing ones. Amendments are made by the maintainer after a
clear pattern emerges across multiple FRs (e.g., "we keep getting requests
adjacent to area X — add an explicit §2.N"). See `feedback/INVENTORY.md`
Document History for the trigger events that motivated past amendments.

---

## Document History

- **2026-04-28** — Charter created. Motivating commit: introduction of
  `ask featurerequest` as a gated MCP intake channel. CHARTER is the
  in-scope/out-of-scope anchor that makes the gate deterministic.
