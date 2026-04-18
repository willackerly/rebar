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

## The 10 Rules (non-negotiable)

These rules are the difference between 100% work survival and total loss.
They are numbered for reference — cite "Rule 3" in findings, not prose.

### Rule 1: Worktree Isolation for Code Changes

If you are writing ANY code, you MUST use worktree isolation. No exceptions.
Read-only research tasks (exploration, analysis, audits) may run in the
main working tree.

```bash
rebar agent start --role developer "implement feature X"
```

### Rule 2: Commit After Every Logical Chunk

Don't accumulate changes. If you get logged out, only uncommitted work is
lost. One fix = one commit. Use `rebar commit` (not `git commit`) — it
runs pre-commit checks and updates integrity. There is no `--no-verify`.

```bash
rebar commit -m "agents/<template-name>: <brief description>"
```

### Rule 3: Strict File Ownership

Your prompt includes an allowlist of files you may create or modify.
Everything else is read-only. If your prompt doesn't specify, you may
modify files directly related to your assigned task. You may NOT modify:

- Shared interface definitions (Store, Scanner, etc.)
- Shared type files, routers, app entry points
- Migration files, lock files, auto-generated files

If you need a change to a restricted file, document it as a finding.

### Rule 4: No Removals Without Explicit Authorization

You may ADD types, functions, methods, and files. You may NOT REMOVE or
RENAME anything unless your prompt explicitly says "delete X." If
something appears unused, add a `// DEPRECATED` comment and note it in
your results.

**Why:** Your worktree snapshot is stale. References exist in files
modified by concurrent agents that you can't see.

### Rule 5: Measure Before AND After

Run the relevant test suite before your first change and after each fix.
Record the metric. If it regressed, revert. Don't guess — use an oracle
(reference implementation, spec, or ground truth data) when available.

### Rule 6: Run Package Tests After Each Change

Not the full suite — just the affected package. This should take <10s and
catches compile errors immediately.

### Rule 7: Write Progress

After each commit, append to the shared progress file so the orchestrator
knows what you accomplished without reading your transcript.

### Rule 8: Don't Touch Shared Files

High-conflict files (shared types, auto-generated bundles, theme/config,
`App.tsx`, `router.go`) must be explicitly assigned to at most one agent.
If a shared file isn't in your allowlist, don't touch it.

### Rule 9: Respect the Context Briefing

Your prompt may include a "Recent Changes" section listing what changed on
main since your worktree branched. Do NOT modify files listed there — your
changes will conflict with work you can't see.

### Rule 10: Commit Before Completing

Your worktree is ephemeral. Uncommitted work = lost work. When done, run
`rebar agent finish <id>` to audit your work against the sealed envelope.

---

## Results & Output

- Write results to the location specified in your parameters.
- If no output path is specified, write to
  `agents/results/<template-name>-<scope>.md`

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
