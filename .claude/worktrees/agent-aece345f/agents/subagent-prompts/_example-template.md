# Template: Example Template

<!-- ANNOTATED EXAMPLE — Copy this file as a starting point for new templates.
     Prefix with _ so it sorts first in the directory listing. -->

> One-line description of what this template does and when to use it.
> This line helps the orchestrator decide whether this template fits the task.

## Metadata

<!-- Inspired by Claude Skills frontmatter. Not parsed by tooling (yet),
     but useful for the orchestrator and for the index. -->

| Field | Value |
|-------|-------|
| **Category** | analysis / generation / testing / review / data-processing |
| **Mode** | single-invocation / fan-out / either |
| **Isolation** | worktree (modifies files) / none (read-only) |
| **Estimated tokens** | ~5K-15K per invocation |

## Parameters

<!-- Every runtime parameter the orchestrator must supply.
     Use SCREAMING_SNAKE for parameter names so they're easy to spot. -->

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `TARGET` | yes | File or directory to operate on | `src/components/` |
| `SCOPE` | no | Narrow the focus | `accessibility` |
| `OUTPUT` | no | Where to write results (default: `agents/results/`) | `agents/results/example-01.json` |

## Task

<!-- Clear, imperative description of what the subagent should do.
     Be declarative (what + why), not procedural (step 1, step 2).
     Let the agent decide HOW to accomplish the task. -->

You are [doing X] on [TARGET].

Your goal is to [produce Y] that [satisfies Z].

## Context Files

<!-- Point to existing project files for domain knowledge.
     Don't inline information that lives elsewhere — reference it.
     This prevents the template from drifting from the source of truth. -->

Read these before starting:
- `QUICKCONTEXT.md` — project orientation
- `docs/design-system.md` — component standards (if reviewing UI)
- (add task-specific files here)

## Output Format

<!-- Specify exactly what the results file should look like.
     The orchestrator needs to parse this — be precise. -->

```json
{
  "template": "example-template",
  "scope": "<TARGET value>",
  "status": "complete | partial | failed",
  "summary": "One-line summary of findings",
  "items": [
    {
      "location": "file.ts:42",
      "severity": "P0 | P1 | P2 | P3",
      "category": "what dimension this falls under",
      "finding": "what you found",
      "suggestion": "what to do about it"
    }
  ],
  "errors": []
}
```

## Success Criteria

<!-- How does the orchestrator know this invocation succeeded?
     Make these testable — not subjective. -->

- `OUTPUT` file exists and is valid JSON
- `status` is `complete`
- Every file in `TARGET` was examined (none silently skipped)

## Anti-Patterns

<!-- If agents consistently make a specific mistake on this task, say so.
     These are the "no, not like that" corrections encoded as guardrails. -->

- Do NOT [common mistake agents make on this task]
- Do NOT [another common mistake]
- If [edge case], then [correct behavior]

## Notes

<!-- Gotchas, edge cases, tips. Optional. -->

- This template is an example — replace everything above with your actual task
- Keep templates under 300 lines. Move reference material to separate files.
