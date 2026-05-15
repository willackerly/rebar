# Subagent Prompt Index

Catalog of available subagent templates. Each entry links to the full
template in `subagent-prompts/`.

---

## Scaffolding

| Template | Description | LOE | Mode |
|----------|-------------|-----|------|
| [_example-template](subagent-prompts/_example-template.md) | Annotated example — copy this to create new templates | — | — |

## Reviews

| Template | Description | LOE | Mode |
|----------|-------------|-----|------|
| [ux-review](subagent-prompts/ux-review.md) | Structured UX audit: accessibility, interaction, responsive, visual consistency, error states, interaction stability | High | single |
| [security-surface-scan](subagent-prompts/security-surface-scan.md) | Security audit: input validation, auth, crypto usage, data exposure, dependencies, red team mode | Max | single or fan-out |
| [code-review](subagent-prompts/code-review.md) | Multi-dimensional code review: correctness, performance, security, maintainability, style | High | single or fan-out |
| [product-review](subagent-prompts/product-review.md) | Product alignment: BDD coverage, persona fit, user flow completeness, scope assessment | High | single |
| [red-team](subagent-prompts/red-team.md) | Multi-persona adversarial review: 5 personas stress-test a component simultaneously | Max | single or fan-out |

## Analysis

| Template | Description | LOE | Mode |
|----------|-------------|-----|------|
| [contract-audit](subagent-prompts/contract-audit.md) | Verify implementations conform to declared interfaces — methods, behavior, error contracts, test coverage | Max | single or fan-out |
| [cleanroom-audit](subagent-prompts/cleanroom-audit.md) | Spec-vs-implementation audit with zero context — finds bugs the author missed (proven 4-bug ROI) | High | single |
| [rebar-compliance-audit](subagent-prompts/rebar-compliance-audit.md) | Full REBAR v2.0.0 compliance assessment — structural, content accuracy, enforcement, session lifecycle | High | single or fan-out |
| [doc-drift-detector](subagent-prompts/doc-drift-detector.md) | Compare doc claims against code reality — stale status, broken refs, contradictions, missing docs | Medium | single or fan-out |
| [feature-inventory](subagent-prompts/feature-inventory.md) | Exhaustive behavioral inventory of a file/module — prerequisite before assigning worktree agents to large files | Medium | single |

## Coordination

| Template | Description | LOE | Mode |
|----------|-------------|-----|------|
| [merge-coordinator](subagent-prompts/merge-coordinator.md) | Post-worktree merge coordination — cherry-pick, conflict resolution, integration summary | Max | single |

## Testing

| Template | Description | LOE | Mode |
|----------|-------------|-----|------|
| [test-shard-runner](subagent-prompts/test-shard-runner.md) | Run a test subset in an isolated worktree, report pass/fail/flaky per test | Low | fan-out |

<!-- Add new templates here, grouped by category.

## Data Processing
| Template | Description | LOE | Mode |
|----------|-------------|-----|------|

## Code Generation
| Template | Description | LOE | Mode |
|----------|-------------|-----|------|
-->
