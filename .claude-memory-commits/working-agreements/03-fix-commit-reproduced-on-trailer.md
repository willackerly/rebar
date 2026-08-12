# fix:/regression: commits MUST include "Reproduced on:" trailer

**In one line:** rebar's Gate G enforced via scripts/check-fix-commit.sh requires every fix:/regression: commit to have a "Reproduced on:" line in the body. ci-check fails otherwise. Include from the start, not as amend afterthought.

*Migrated from assistant-memory 2026-08-12.*

---
**Rule:** Any commit prefixed `fix:` / `fix(scope):` / `regression:` /
`regression(scope):` MUST contain a `Reproduced on:` line in the body
referencing a SHA, deploy URL, log excerpt, or screenshot path.
Enforced by `scripts/check-fix-commit.sh` (Gate G), which is part of
`scripts/ci-check.sh`.

**Why:** Source —
`feedback/processed/2026-04-24-process-gates-G-through-L.md`.
Without it, the fix is for a phantom — the user-reported symptom hasn't
been observed independently before fixing began. Causes spray-fix
churn and retrospective uncertainty about what actually fixed what.

**How to apply:**
- Include `Reproduced on:` in the FIRST commit message you draft for
  any `fix:` or `regression:` commit. Don't wait for ci-check to
  complain.
- Form: "Reproduced on: <SHA> / <deploy URL> / <log excerpt path> /
  <screenshot path>". For local repros, include the command run + a
  brief verbatim of the output.
- Tripped on 2026-04-28 with `fix(ask): claude flag regressions` —
  had to amend the commit message to add the trailer. Avoidable
  with this rule applied at draft time.
- Applies to BOTH commit subject prefixes (`fix:` and `regression:`)
  and their scoped forms (`fix(scope):`, `regression(scope):`).
- Does NOT apply to `feat:`, `docs:`, `chore:`, `refactor:`, `test:`,
  `ci:`, or `style:` prefixes.

**Common reproducer formats that satisfy the gate:**
- `Reproduced on: local — \`rebar ask featurerequest "test"\` (verbatim error in body)`
- `Reproduced on: SHA abcd1234 (commit just before this one)`
- `Reproduced on: see feedback/2026-04-28-usability-red-team.md §C3`
- `Reproduced on: deploy URL https://staging.example.com/foo`

The check is regex-based: `^Reproduced[[:space:]]on:[[:space:]]*\S` in
the commit body — keep the prefix exact.

**Cousin rule (Gate I):** commits mentioning `--skip-*` / `--no-verify`
/ `--force` / `SKIP_*=` need a `Bypass tickets:` line, enforced by
`scripts/check-bypass-flags.sh`. Same shape.
