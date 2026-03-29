# Subagent Guidelines

Shared behavioral contract for all subagent invocations. The orchestrating
agent should instruct every subagent to read this file before starting work.

<!-- Customize this for your project. These are sensible defaults drawn from
     OpenDocKit learnings and Claude Skills best practices. -->

---

## Context Loading Order

When you start, read files in this order:

1. **This file** (`agents/subagent-guidelines.md`) — you're here
2. **Your assigned template** from `agents/subagent-prompts/<template>.md`
3. **QUICKCONTEXT.md** — project orientation (if your task touches project code)
4. **Any additional files** specified in your parameters

Do not skip steps. Do not start working before reading your template.

## Isolation & Branching (MANDATORY)

- **If you are writing ANY code, you MUST use worktree isolation.** This is
  non-negotiable. No exceptions.
- **Use `rebar agent start` to create your worktree.** This is the canonical
  way to begin a coding task — it creates the worktree, snapshots integrity
  state, and enforces role-based file permissions automatically.
  ```bash
  rebar agent start --role developer "implement feature X"
  ```
- Read-only research tasks (exploration, analysis, audits) may run in the
  main working tree.
- Work in your worktree branch. Do not modify the main working tree.

## Results & Checkpointing

- Write results to the location specified in your parameters.
- If no output path is specified, write to
  `agents/results/<template-name>-<scope>.md`
- **You MUST commit before completing.** Your worktree is ephemeral —
  uncommitted work is lost. No commit = no work happened.
- **Use `rebar commit` instead of `git commit`.** This ensures pre-commit
  checks run (no bypass possible), the integrity manifest is updated, and
  ratchets are enforced. There is no `--no-verify` flag.
  ```bash
  rebar commit -m "agents/<template-name>: <brief description>"
  ```
- When done, run `rebar agent finish <id>` to audit your work against the
  sealed envelope (role permissions, ratchets, integrity).

## Architectural Change Detection

If your work reveals or requires any of the following, **do not silently
proceed**. Document the finding in `agents/findings/<date>-<short-title>.md`:

- Changes to shared interfaces or contracts (shared types, API boundaries)
- New external dependencies or framework introductions
- Crypto algorithm or key management implications
- Wire protocol, API, or storage model changes
- Security concerns (plaintext leaks, missing validation, nonce reuse)

### Finding Format

```markdown
# Finding: <short title>

**Discovered by:** <template-name>, <scope/shard>
**Date:** <YYYY-MM-DD>
**Severity:** info | needs-discussion | blocking

## What
<describe what you found>

## Why It Matters
<architectural / security / contract implications>

## Suggested Action
<what should happen next>
```

## Scope Discipline

- Do your assigned work. Nothing more.
- Do not "helpfully" expand scope into adjacent files or features.
- If your task depends on something outside your scope, document it as a
  finding — do not attempt to fix it yourself.

## Communication

- Do not ask clarifying questions — work with what you have.
- If ambiguity exists, document it in your results.
- If a task is impossible with the given parameters, write a clear error to
  your results file explaining why, and complete. Do not hang.

## Quality

- Follow the project's coding style (see `CLAUDE.md`).
- Run relevant tests for code you've changed before committing.
- Do not introduce `TODO:` comments — either fix it or document it as a
  finding.
- Do not add unnecessary comments, docstrings, or type annotations to code
  you didn't change.

## Enforcement

The `rebar` CLI enforces structural integrity. Key rules:

- **`rebar commit` is the only commit path.** It runs pre-commit checks, updates
  integrity hashes, and checks ratchets. There is no `--no-verify` flag.
  If checks fail, fix the issue — do not attempt to use raw `git commit`.
- **Role-based file permissions are enforced.** If you're a `developer` role,
  you can write to `src/` but not to `tests/`, `scripts/`, or `architecture/`.
  If you need to modify files outside your role, document it as a finding.
- **Assertion counts cannot decrease.** The ratchet system ensures test
  assertions only increase. If you need to remove assertions, explain why
  in your commit message.
- **`rebar verify` detects tampering.** Any modification to protected files
  outside the CLI is detectable. Do not edit enforcement scripts, contracts,
  or test files outside the `rebar` workflow.

## Error Handling

- If you encounter an error partway through, write partial results with a
  clear `"status": "partial"` indicator and describe what failed.
- Do not retry indefinitely. Three attempts max, then document the failure.
- Include error messages and stack traces in your results — they help the
  orchestrator diagnose issues.

## Level of Effort (LOE)

Each template declares a recommended LOE level. The orchestrating agent uses
this to select the right model and effort level for invocation.

| LOE | Model | Claude Code Flag | Typical Duration |
|-----|-------|-----------------|-----------------|
| **Max** | `opus` | `--model opus` | 2-10 min |
| **High** | `opus` or `sonnet` | `--model opus` (preferred) | 1-5 min |
| **Medium** | `sonnet` | `--model sonnet` | 30s-2 min |
| **Low** | `haiku` or local | `--model haiku` | 5-30s |

### Invoking with LOE

When the orchestrating agent delegates, it should match the model to the LOE:

```
# Max LOE — use opus for deep reasoning
Agent(prompt: "...", model: "opus")

# Medium LOE — sonnet is sufficient
Agent(prompt: "...", model: "sonnet")

# Low LOE — haiku or local model for simple tasks
Agent(prompt: "...", model: "haiku")
```

### When to Override

The template LOE is a default. Override when:
- The codebase is unusually large/complex → bump up
- The task scope is narrow (single file) → bump down
- Time pressure requires faster results → bump down with tradeoff noted
