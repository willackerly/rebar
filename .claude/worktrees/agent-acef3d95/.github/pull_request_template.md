## Summary

<!-- What does this PR do? 1-3 bullet points. -->

## Contracts

<!-- Which contracts does this PR touch? -->

- **Implements:** CONTRACT:___ (new implementation)
- **Modifies behavior of:** CONTRACT:___ (check contract still holds)
- **New contract:** architecture/CONTRACT-___  (requires plan mode review)
- **No contract impact** (refactor, docs, tests only)

## Checklist

- [ ] Every new/modified source file has a `CONTRACT:` header comment
- [ ] Contract references point to existing files (`./scripts/check-contract-refs.sh`)
- [ ] No untracked `TODO:` comments (`./scripts/check-todos.sh`)
- [ ] Tests pass at T2+ (package-level or higher)
- [ ] Docs updated if behavior changed (QUICKCONTEXT, TODO, relevant READMEs)

## Test Plan

<!-- How was this tested? Which cascade tier? -->

- [ ] T1 — targeted test(s): ___
- [ ] T2 — package suite: ___
- [ ] T3+ — cross-package / full suite
