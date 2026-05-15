# REBAR Enhancement Proposal — Architectural Fidelity Propagation

> **From:** filedag (Will, Federation Architect, via Claude)
> **To:** REBAR methodology maintainers (Will, same person — but worth capturing as a reviewable proposal)
> **Date:** 2026-04-19
> **Type:** Enhancement proposal + live evidence
> **Pairs with:** `~/dev/rebar/feedback/2026-04-18-filedag-deep-audit-insights.md` (previous batch of REBAR insights from the same repo)

## The event that prompted this

On 2026-04-19 Will issued a single architectural directive for filedag's ABAC Phase 1:

> Follow TDFLite for EntitlementSource. 100% explicit ceilings, always login, long persistent cookies fine. TDF wrap/unwrap for web: blindpipe. For services and crawlers: OpenTDF Go SDK. Perhaps a Go blindpipe to complement the JS — your call. BlindPipe stubs/harness fine. Keep in mind multi-lang blindpipe concept.

Capturing that one directive with fidelity required coordinated updates to **eight** files in the filedag repo + four memory entries + one peer-repo ask:

| Artifact | What got updated |
|---|---|
| `~/.claude/.../memory/MEMORY.md` + 4 new memory files | Persist the directive across sessions |
| `CLAUDE.md` | Peer repos section + cold-start list |
| `QUICKCONTEXT.md` | Current state + architectural directive summary |
| `TODO.md` | P0 stack — ABAC Phase 1 now references the plan doc |
| `docs/ABAC-PHASE-1-PLAN.md` | Revision 2 with five directives applied |
| `architecture/CONTRACT-P2-ABAC.3.0.md` | Amendment A (implementation scope scrub) |
| `architecture/CONTRACT-I2-BLINDPIPE.2.0.md` | Amendment A (multi-lang blindpipe concept) |
| `architecture/CONTRACT-D1-DATA-MODEL.1.0.md` | Amendment A (migration 15 + identity sources + memberships NOT in filedag) |
| `~/dev/rebar/feedback/2026-04-19-tdflite-entitlement-consumption-ask.md` | External ask to TDFLite architect |

That's high manual coordination. I did it deliberately this time, but nothing in REBAR **forces** it to happen. A less disciplined agent or a hurried session would miss half these files, and the architectural claim would fragment — exactly the kind of "same fact reported by multiple sources with different values" pattern that filedag's own SoT discipline is fighting in code.

REBAR's existing tooling covers **structural** fidelity (contract headers, registry lifecycle, doc freshness) but not **semantic** propagation. Here's what would close that gap.

## Proposed enhancements (four, picked roughly by leverage-per-effort)

### 1. ADR pattern — `rebar/decisions/NNNN-title.md`

**The gap:** Directives like Will's 2026-04-19 statement have no canonical home. They live scattered in commit messages, memory entries, contract amendments, and plan docs. The origin of any particular architectural shift is hard to trace backward.

**The proposal:** REBAR formalizes Architecture Decision Records as a practice. Each REBAR-compliant repo maintains `decisions/` with numbered entries:

```
rebar/decisions/0001-adopt-rebar-tier-2.md
filedag/decisions/0024-abac-phase-1-scrubbed-for-external-authority.md
                                    (2026-04-19, Will's directive)
```

Each ADR has a fixed shape (adapted from Nygard's classic + REBAR-specific):

```markdown
---
number: NNNN
status: proposed | accepted | superseded-by:NNNN | reversed-by:NNNN
date: YYYY-MM-DD
author: name
consulted: [roles/people]
tags: [contract-names, phase, primitive-ids]
---

# Title — one-line decision

## Context (the forces)
What was pushing in multiple directions before this decision?

## Decision
The choice made, in ~2 paragraphs.

## Consequences
What we now can / can't do. Which contracts this affects (link them).
Which memory entries pin this. Which plans execute on it.

## Cross-refs (the propagation list)
- Contracts amended: [list]
- Memory entries: [list]
- Plan docs: [list]
- External asks: [list]
```

**The REBAR enforcement:** a `check-adr-propagation.sh` script walks each ADR's cross-refs and verifies they actually reference the ADR by number. Missing backlink → failure.

**Why this matters:** an ADR is a single source of truth for a directive. Memory entries, contracts, and plans all point back to it. Propagation becomes checkable.

### 2. Contract impact DAG + amendment lint

**The gap:** Today if I amend `CONTRACT-P2-ABAC.3.0.md`, nothing automatically asks "does `CONTRACT-I2-BLINDPIPE.2.0.md` need review?" There's a web of relationships between contracts that exists only in human memory.

**The proposal:** each contract declares its `depends_on` and `consumed_by` relationships in the frontmatter:

```yaml
---
id: P2-ABAC.3.0
depends_on:
  - I2-BLINDPIPE.2.0  # we call their TDF wrap interface
  - P4-DELEGATION.0.1  # we consume their verified chain
  - D1-DATA-MODEL.1.0  # our access_attributes table shape
consumed_by:
  - S1-API.2.0  # API handlers use our middleware
  - C5-WEB-UI.1.0  # frontend reads our response headers
---
```

**The REBAR enforcement:** `scripts/check-contract-graph.sh` computes the DAG and, when run in "impact" mode against a changed contract, lists the transitively-dependent contracts that need review. A `check-amendment-coherence.sh` additionally asserts that every *amended* contract on a given date has amendments in all other contracts with `consumed_by` relationships that the amendment would affect — or an explicit `no-impact-on: [...]` statement in the ADR.

**Why this matters:** stops the pattern of "I changed ABAC but forgot to update Blindpipe" silently. Makes cross-contract coherence a check, not a hope.

### 3. Amendment discipline lint

**The gap:** Amendments I wrote today are formatted inconsistently ("Amendment A (2026-04-19)", "## Revision 2", etc.). No lint catches:

- Undated amendments
- Amendments that contradict the original section without marking the original as superseded
- Amendments that introduce terms not defined elsewhere in the contract or explicitly referenced from the ADR
- Amendments stacked so deep the contract is unreadable (Amendment A, B, C, D without a rev-bump)

**The proposal:** a REBAR-provided amendment template:

```markdown
## Amendment <LETTER> (<YYYY-MM-DD>) — <one-line rationale>

**Origin:** ADR <NNNN>
**Affected sections of this contract:** <list>
**Breaking change?** <yes/no>
**What this changes** (vs the original contract body):
- ...
**What stays the same:**
- ...
**Implementation impact:** <link to plan doc / checklist>
```

Amendments that accumulate to >2 per contract trigger a "consider version bump" warning (`P2-ABAC.3.1` vs amending 3.0 further). Amendments marked `breaking: yes` without a superseding version number trigger a hard fail.

**Why this matters:** amendments today are a loose convention. Making them structured means the history of a contract is readable forward AND a tool can flag when to revision-bump.

### 4. Cold-start coherence check

**The gap:** Cold-start for filedag reads: README → QUICKCONTEXT → TODO → AGENTS → CLAUDE → END-STATE → stories. Nothing automatically verifies that claims in each are consistent with the others. Today I found QUICKCONTEXT claiming "v4 manifest" while END-STATE-ARCHITECTURE was at "v4" — matched — but nothing checks this reliably.

**The proposal:** `scripts/check-cold-start-coherence.sh` extracts factual claims from each cold-start doc (via structured frontmatter or conventional headings) and asserts agreement. Example claims to cross-check:

- "Current phase:" in QUICKCONTEXT vs last section of TODO
- "Contracts: N" count in QUICKCONTEXT vs actual count in `architecture/`
- "End-state version" referenced in multiple docs matches the file
- Core principles count (7) matches README — and is referenced by the same count everywhere else
- Peer repos list in CLAUDE.md is consistent with peer repos mentioned in end-state architecture

**The REBAR enforcement:** runs as part of `make check`. A fail means the cold-start sequence is internally inconsistent — so the next agent is about to be told conflicting things.

**Why this matters:** a cold start that contradicts itself is the most expensive kind of error — the agent doesn't know which doc to trust and asks the user. Catching it at `make check` is 10× cheaper.

## Sequencing

If I were picking one to adopt first: **#1 (ADR pattern)**. Everything else composes on top of it. An ADR is the single source of truth that amendments, memory entries, plan docs, and cross-contract impact analysis all reference. Without ADRs, the other three enhancements don't have a natural anchor.

Order of adoption I'd recommend:

1. **ADR pattern** (#1) — foundation. Make writing an ADR as easy as `rebar adr new "title"` and the template generates.
2. **Cold-start coherence** (#4) — highest immediate value per effort. Can be implemented with bash + grep against known headings. Catches drift fast.
3. **Contract impact DAG** (#2) — more work (needs contract frontmatter migration) but huge payoff for cross-contract coherence. Start with `depends_on` as a warning-only check; tighten over time.
4. **Amendment discipline lint** (#3) — last. Requires #1 to exist (ADR references). Nice-to-have on top.

## How these compose with existing REBAR tooling

| Existing tool | What it catches | How enhancements extend |
|---|---|---|
| `check-contract-refs.sh` | Valid contract ID refs | ADR cross-refs become a new class of refs to check |
| `check-contract-headers.sh` | Source files have contract headers | Contracts gain frontmatter requirement |
| `check-freshness.sh` | Doc stale warnings | Coherence check enforces stronger claim matching, not just dates |
| `check-todos.sh` | Untracked TODOs in code | Unreferenced ADRs could surface similarly |
| `check-compliance.sh` | Structural fidelity | Amendment discipline extends this to contract-internal structure |

None of these enhancements *replace* existing tools; they layer.

## Live evidence from today

If #1 (ADR) had existed, I would have opened `filedag/decisions/0024-abac-phase-1-external-authority-scrub.md` once, and from there generated:
- The memory entries (via a `rebar adr propagate --memory` subcommand)
- The contract amendment stubs in affected contracts (via `rebar adr propagate --contracts`)
- A checklist of QUICKCONTEXT / TODO / CLAUDE.md sections needing update

I still would have WRITTEN the content by hand, but the coordination overhead — the "did I update everywhere this needs to go?" question Will explicitly asked — would be automated.

## Ask

- Adopt #1 for the REBAR meta-repo itself first (eat the dogfood)
- Once #1 works in rebar, promote to a filedag trial (low-risk starter repo since filedag is Will's most-active)
- Report back from filedag after 3-5 ADRs on friction / gaps
- Then the next three enhancements become REBAR v2.1.0 features

---

**Document status:** proposal, not specification. Treat as the opening move of a design conversation.
