# Feedback Directory

A lightweight way for agents (and humans) to leave feedback about the
rebar without needing to create a PR.

## How It Works

1. Create a new file in this directory using the template format below
2. Name it: `YYYY-MM-DD-short-description.md`
3. Commit it to your project (or to a fork of rebar)
4. Template maintainers review this directory periodically

## Feedback Template

```markdown
# Feedback: [short title]

**Date:** YYYY-MM-DD
**Source:** [which template or file this is about]
**Type:** improvement | bug | confusion | missing-feature | anti-pattern
**Status:** proposed | in-progress | implemented | wontfix
**Template impact:** [which templates/practices need updating, if any]
**From:** [agent session / human / project name]

## What Happened
[What you were doing when you noticed the issue]

## What Was Expected
[What the template should have guided you to do]

## Suggestion
[Concrete improvement — not just "make it better"]
```

## Processing Feedback

When feedback items are triaged:
1. Update the `**Status:**` field to reflect the decision
2. Record the disposition in [INVENTORY.md](INVENTORY.md)
3. Move the source file to `feedback/processed/` unless pending
   implementation (in which case leave it in `feedback/` root until the
   work lands, then move)

**Dispositions:**
- `implemented` — action complete; move to `processed/`
- `in-progress` — accepted, implementation pending (Wave 1/2); stays in `feedback/`
- `deferred` — watchlisted in INVENTORY.md; move to `processed/`
- `wontfix` / `redirected` — move to `processed/` with reason in INVENTORY.md

Periodic review: check [INVENTORY.md](INVENTORY.md) for Watchlist items
with multiple votes — those are candidates to promote to Queued. Scan
`feedback/*.md` (root) for active proposals. Use `ask steward "which
feedback items are still open?"` if automated scanning is configured.

## Inventory and vote accumulation

[INVENTORY.md](INVENTORY.md) is the single index of all feedback
dispositions. Its purpose: when a new project raises a proposal that's
already been deferred, increment the vote count rather than filing a
duplicate discussion. When an item reaches **2+ independent project votes**
OR **1 measured pain point** it's a candidate to promote from Watchlist
to Queued.

## Why This Exists

Agents using these templates discover gaps, anti-patterns, and missing
guidance in the course of their work. Without a feedback mechanism, those
discoveries are lost when the conversation ends. This directory captures
them for template maintainers to act on.

**Agents:** When you notice a template is missing guidance, has an
anti-pattern not listed, or could be improved — write a feedback file here
instead of trying to fix the template yourself. The maintainers will decide
what to incorporate.

---

## Feature Requests via `ask_rebar_featurerequest`

Most rebar adopters call `ask_rebar_*` over MCP but have no git write
access to this repo. The `featurerequest` ASK role is a **gated MCP intake
channel** that turns "yes, that's a clear gap" into a durable, structured,
provenance-stamped artifact in this directory.

### How it works

```
caller → ask_rebar_featurerequest "<scenario + missing capability>"
         ↓
         agent reads CHARTER.md (§3 acceptance gates)
         ↓
         scores against four-path triage:
           (a) in-scope per §1 + novel → file FR-YYYY-MM-DD-<slug>.md
           (b) duplicate of existing entry → increment vote in INVENTORY
           (c) already implemented → return pointer, no file
           (d) out-of-scope per §2 → return §rejection rationale, no file
         ↓
         agent replies with disposition + (if filed) the FR ID
         ↓
         maintainer reviews FR-*.md as untracked files on next visit,
         commits in batch, and updates INVENTORY.md disposition
```

### Hard gates (CHARTER §3)

A request files an FR only when **all four** are true:

1. **In-scope** per CHARTER §1 (one of the IS-positives applies)
2. **Not blocked** by CHARTER §2 (none of the IS-NOT lines applies)
3. **Concrete use case** — real scenario, not hypothetical
4. **Novel** — no existing FR / Watchlist / Implemented entry covers it

If any gate fails, the agent does NOT file. It returns the precise
disqualification reason instead. This is what stops feature-request
sprawl from realtime ask traffic.

### What the agent can and cannot write

- **Can:** create new `feedback/FR-*.md` files; increment vote counts on
  matching `INVENTORY.md` Watchlist rows
- **Cannot:** `git commit`, modify existing FRs, edit CHARTER.md, edit
  AGENT.md files, edit any source code

The deliberate no-commit policy means new FRs land as untracked files for
batch review. Auto-commit would be a much heavier trust delegation.

### Engaging beyond a typed ask

`ask_rebar_featurerequest` is a thin contribution pipe for **clear,
typed, missing-feature asks**. For open-ended discussion, design-shaped
proposals, or counterproposals to CHARTER, the right path is **clone
rebar locally and open a PR** — see CHARTER §4. Most substantive
engagement should land via direct repo work, not intake.

### FR lifecycle

| Stage | Location |
|-------|----------|
| Filed by agent | `feedback/FR-YYYY-MM-DD-<slug>.md` (untracked) |
| Maintainer commits | `feedback/FR-*.md` (tracked, status: proposed) |
| Triaged | INVENTORY.md updated; FR status field updated |
| Implemented or deferred | source FR moved to `feedback/processed/` |

See [`FR-TEMPLATE.md`](FR-TEMPLATE.md) for the required-fields shape.
