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
