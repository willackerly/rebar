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

When feedback items are implemented:
1. Update the `**Status:**` field to `implemented`
2. Move the file to `feedback/processed/`
3. Note which templates/practices were updated in the `**Template impact:**` field

Periodic review: scan `feedback/*.md` for `Status: proposed` to find
unactioned items. Use `ask steward "which feedback items are still open?"`
if automated scanning is configured.

## Why This Exists

Agents using these templates discover gaps, anti-patterns, and missing
guidance in the course of their work. Without a feedback mechanism, those
discoveries are lost when the conversation ends. This directory captures
them for template maintainers to act on.

**Agents:** When you notice a template is missing guidance, has an
anti-pattern not listed, or could be improved — write a feedback file here
instead of trying to fix the template yourself. The maintainers will decide
what to incorporate.
