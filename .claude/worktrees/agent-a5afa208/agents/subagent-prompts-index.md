# Subagent Prompt Index

Catalog of available subagent templates. Each entry links to the full
template in `subagent-prompts/`.

---

## Scaffolding

| Template | Description | Mode |
|----------|-------------|------|
| [_example-template](subagent-prompts/_example-template.md) | Annotated example — copy this to create new templates | — |

## Reviews

| Template | Description | Mode |
|----------|-------------|------|
| [ux-review](subagent-prompts/ux-review.md) | Structured UX audit: accessibility, interaction, responsive, visual consistency, error states | single |
| [security-surface-scan](subagent-prompts/security-surface-scan.md) | Security audit: input validation, auth, crypto usage, data exposure, dependencies | single or fan-out |
| [code-review](subagent-prompts/code-review.md) | Multi-dimensional code review: correctness, performance, security, maintainability, style | single or fan-out |

## Analysis

| Template | Description | Mode |
|----------|-------------|------|
| [contract-audit](subagent-prompts/contract-audit.md) | Verify implementations conform to declared interfaces — methods, behavior, error contracts, test coverage | single or fan-out |
| [doc-drift-detector](subagent-prompts/doc-drift-detector.md) | Compare doc claims against code reality — stale status, broken refs, contradictions, missing docs | single or fan-out |
| [feature-inventory](subagent-prompts/feature-inventory.md) | Exhaustive behavioral inventory of a file/module — prerequisite before assigning worktree agents to large files | single |

## Testing

| Template | Description | Mode |
|----------|-------------|------|
| [test-shard-runner](subagent-prompts/test-shard-runner.md) | Run a test subset in an isolated worktree, report pass/fail/flaky per test | fan-out |

<!-- Add new templates here, grouped by category.

## Data Processing
| Template | Description | Mode |
|----------|-------------|------|

## Code Generation
| Template | Description | Mode |
|----------|-------------|------|
-->
