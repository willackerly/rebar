# Feedback: Cross-reference checks + cross-repo federation coordination

**Date:** 2026-04-21
**Source:** `profiles/*/ci-check.sh` (Tier 2 check surface); peer coordination patterns
**Type:** missing-feature
**Status:** proposed
**Template impact:**
- `profiles/solo/ci-check.sh` / `profiles/team/ci-check.sh` — add cross-ref Tier 2 checks
- `scripts/compute-registry.sh` — bash 3.2 compatibility
- NEW: `profiles/federation-node/` or equivalent federation profile
- `practices/session-lifecycle.md` — add explicit session-end freeze check
- `practices/cross-repo-coordination.md` (NEW) — contract manifest + peer introspection
**From:** filedag (`~/dev/filedag`), mid-Wave-2-prep audit 2026-04-21

## What Happened

Running a deep consistency audit on filedag before starting Wave 2 execution, I found that REBAR Tier 2's `ci-check.sh` passed green while:

- **18+ critical files were untracked** (5 Phase 12 contracts, the federation rubric `product/FEDERATION-STORIES-DRAFT.md`, 11 audit artifacts, 4 top-level plan docs, 1 agent fanout plan)
- **Registry impl-ref counts were stale** by factors of 2×–40× (hand-audited Phase 1 2026-04-17; code moved; no automation re-derives them)
- **Cold-start docs contradicted each other** (TODO.md: "Phase 15 completed 2026-04-18"; QUICKCONTEXT.md: "Phase 15 🔵 pending" — same facts, opposite claims)
- **METRICS numbers were stale** (`go_source_files = 56` / actual 132; `db_migrations = 13` / actual 15)

Full report: `../filedag/docs/audits/2026-04-21-post-wave1-drift-audit.md`.

The Tier 2 check surface is structurally clean — headers present, references syntactically valid, freshness within window — but all four drift classes evaded it because Tier 2 doesn't cross-validate between layers.

## What Was Expected

Tier 2 adoption advertises "Adopted — +contract-headers, freshness, registry (small team)" as the entry for teams serious about contract discipline. In practice, Tier 2 catches:

- Missing contract headers (structural)
- Broken contract references (syntactic)
- Stale freshness stamps (temporal)
- Missing README badge (structural)

But it does NOT catch:

- **Untracked references.** A doc can cite `product/FEDERATION-STORIES-DRAFT.md` as load-bearing; ci-check green if the file exists in working tree. New clone fails to resolve.
- **Registry↔code drift.** `compute-registry.sh` exists in peer REBAR but adoption is ad-hoc; filedag deferred it due to macOS bash 3.2.
- **Cross-doc consistency.** No check that "Phase N" statuses agree across TODO + QUICKCONTEXT + retrospectives.
- **METRICS↔reality.** METRICS fields are hand-maintained numbers with no backing "this is how you computed it" link.

Adoption path assumed these would be caught; they weren't.

## Suggestion

Six concrete adds, roughly in increasing implementation cost:

### 1. Cross-ref checker (S1)

New script `scripts/check-doc-refs.sh`:

- Walks every `*.md` file in the repo.
- Regex-extracts internal path references: backticks, markdown links, fenced-code paths.
- Verifies each target is tracked (`git ls-files`).
- Whitelists external URLs, generated files, explicit `<ignore-ref>` inline markers.
- Fails if any ref resolves to untracked.

Integrate into `ci-check.sh` Tier 2 by default. Would have prevented 100% of filedag's F1 finding.

### 2. bash 3.2-compatible compute-registry.sh

Current `compute-registry.sh` uses `local -n` namerefs (bash 4.3+). macOS ships bash 3.2. Either:

- Port to explicit-array semantics (preferred; no user-environment requirement), OR
- Ship a `check-bash-version.sh` with clear "install bash 4 via brew" guidance.

Blocking adoption on user's OS bash version is a Tier 2 friction point.

### 3. Cross-doc phase-status consistency

`scripts/check-phase-consistency.sh`: parse `Phase N` + status tokens from all tracked `*.md`; fail on divergence across files.

Easy to implement, catches a drift class that accumulates silently.

### 4. Session-end freeze check (practice + script)

`practices/session-lifecycle.md` already documents the intent; add an enforcement script `scripts/session-end.sh`:

- `git status --porcelain` on critical paths (`architecture/`, `docs/`, `product/`, `agents/`)
- Any untracked? Exit non-zero with the list.
- Prompt: "intentional handoff state (y) or commit needed (n)?"

Wire into peer REBAR's session-lifecycle.md as the recommended close-out.

### 5. Federation profile + contract manifest schema

New REBAR profile `federation-node` (proposed, not yet declared by any repo). Key additions over Tier 2:

- Publishes `.rebar/contracts.yaml` manifest per repo (schema defined by REBAR).
- Declares `depends_on_peer:` for every cross-repo contract citation.
- `scripts/check-peer-contracts.sh` walks `../<peer>/.rebar/contracts.yaml` for each peer; verifies cited versions exist.

Example manifest shape in `../filedag/docs/DRIFT-PREVENTION.md` §S7.

This is the federation-grade deliverable. Without it, filedag citing `blindpipe@I2-BLINDPIPE.2.0` is an unverified claim. With it, mismatches fail fast at ceremony-time.

### 6. Bidirectional feedback protocol

Current flow is one-way: consumer files in `~/dev/rebar/feedback/`; REBAR maintainer processes; consumer has no signal back except commit log + INVENTORY.md.

Propose: in the same feedback file, maintainer appends `## Response (YYYY-MM-DD)` section when processed. Consumer detects via polling/manual review. Closes the loop explicitly.

For federation-grade: each peer repo exposes `feedback/<peer>-INBOX.md` + `feedback/<peer>-OUTBOX.md` as structured channels; the equivalent of a federated inbox per peer-pair.

## Impact of shipping these

Based on the filedag audit, for a repo with 25 contracts + 10+ docs + 200 source files, these checks would prevent ~6 of 7 drift classes before they reach cold-start. Hours of human reconciliation per audit cycle → ~15 minutes ceremony time per session.

Cross-repo piece (S5) is the load-bearing one for federation: when filedag + blindpipe + TALOS + OpenDockit start coordinating on shared assertions, version drift between their contract citations will be silent failure unless there's mechanical verification.

## Related artifacts

- `../filedag/docs/audits/2026-04-21-post-wave1-drift-audit.md` — full audit findings
- `../filedag/docs/DRIFT-PREVENTION.md` — infrastructure design (co-submitted)
- `../filedag/docs/REBAR-ADOPTION-PLAN.md` — prior adoption plan; needs update to reference these new checks
- `feedback/2026-04-18-filedag-deep-audit-insights.md` — prior deep-audit round; this is the follow-up from actually executing what that one surfaced
