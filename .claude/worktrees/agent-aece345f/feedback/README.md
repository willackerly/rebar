# Feedback Directory

A lightweight way for agents (and humans) to leave feedback about the
agent-templates without needing to create a PR.

## How It Works

1. Create a new file in this directory using the template format below
2. Name it: `YYYY-MM-DD-short-description.md`
3. Commit it to your project (or to a fork of agent-templates)
4. Template maintainers review this directory periodically

## Feedback Template

```markdown
# Feedback: [short title]

**Date:** YYYY-MM-DD
**Source:** [which template or file this is about]
**Type:** improvement | bug | confusion | missing-feature | anti-pattern
**From:** [agent session / human / project name]

## What Happened
[What you were doing when you noticed the issue]

## What Was Expected
[What the template should have guided you to do]

## Suggestion
[Concrete improvement — not just "make it better"]
```

## Why This Exists

Agents using these templates discover gaps, anti-patterns, and missing
guidance in the course of their work. Without a feedback mechanism, those
discoveries are lost when the conversation ends. This directory captures
them for template maintainers to act on.

**Agents:** When you notice a template is missing guidance, has an
anti-pattern not listed, or could be improved — write a feedback file here
instead of trying to fix the template yourself. The maintainers will decide
what to incorporate.
