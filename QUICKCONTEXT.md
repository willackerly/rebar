# QUICKCONTEXT — rebar (v3.0.0-alpha branch)

last-synced: 2026-04-29
**Branch:** v3.0.0-alpha (off main @ b08c98d)
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology
**Plan:** [`docs/v3-alpha-plan.md`](docs/v3-alpha-plan.md) — canonical state-of-work for this branch

---

## Why this branch exists

v3.0.0-alpha bundles five concepts that were ripe in `feedback/` and
are best taken for a spin together. The headline feature is **maturity
tagging** — a small fixed vocabulary (stub / draft / in-progress /
active / verified) declared per-artifact so the compliance badge
reflects reality instead of just artifact existence. Major bump because
it's a breaking change for adopters; alpha because we want real-world
failure to refine, not pre-engineer.

Authorized in conversation 2026-04-29 by Will: *"lets branch to a major
version bump alpha and fold in the very best concepts we have
throughout."*

## Five clusters (see `docs/v3-alpha-plan.md` for detail)

| # | Cluster | Status |
|---|---------|--------|
| 1 | Maturity tagging + compliance honesty | pending — start here |
| 2 | SessionStart hook for cold-start enforcement | pending |
| 3 | TEST_FIDELITY.md + UAKS tier + closed-loop demo gate | pending |
| 4 | agents/FANOUT_PATTERN.md | pending |
| 5 | Contract discipline followups | post-tag refinement |

**Tag `v3.0.0-alpha`** after Cluster 4 lands. Cluster 5 is post-tag.

## Cluster 5 (cold-start UX) was dropped — already shipped

The original draft plan had a six-cluster scope. Cluster 5 of the
original was "cold-start UX completeness" (C1/C3/C4/M10/L3/L4 from the
2026-04-28 usability red team). Caught during impl: it had already
shipped on main 2026-04-28 in commits `b09f9fb` (regression fixes) and
`b8894f6` (auto-run ask init, welcome nudges, score annotation).
Re-numbered the remaining work and dropped to five clusters. See commit
`6a333f3` for the post-mortem.

## What's deferred (with rationale)

- **Auto-federation experiments** (`feedback/2026-04-28-auto-federation-experiment.md`)
  — 7 open questions need maintainer answers. v3.0.x or v3.1 once
  experiments inform design.
- **Interaction-class fix protocol** (`feedback/2026-04-20-...`) —
  doesn't share v3-alpha narrative.
- **Usability RT Cluster E** (tab completion, log formatting) —
  post-tag polish.

## Verification commands (run before trusting this file)

```bash
git log --oneline -10                # what's actually on the branch
git status                           # clean? working tree state
rebar audit                          # 9-10/10 expected
git log main..HEAD --oneline         # alpha-specific commits since branch point
```

## What's Next (when picking this back up)

1. **Start Cluster 1** — maturity tagging. Vocabulary in
   `conventions.md`; `Status:` field in `architecture/CONTRACT-TEMPLATE.md`;
   weight `scripts/check-compliance.sh` by stub-or-draft ratio.
2. Active task list lives in `docs/v3-alpha-plan.md` §Sequence.
3. Pre-commit hook is wired (T0 bash syntax + contract-refs + todos +
   ground-truth) — runs on every commit on this branch.

## Cross-device handoff notes

This branch was set up on Will's primary device 2026-04-29 and pushed
to origin so it can be picked up elsewhere. To resume:

```bash
git fetch origin && git checkout v3.0.0-alpha
cat docs/v3-alpha-plan.md  # full plan
cat QUICKCONTEXT.md        # this file
```

Will's auto-memory at `~/.claude/projects/-Users-will-dev-rebar/memory/`
does NOT travel between devices — all load-bearing context for this
branch is in-repo (this file + plan doc + commit messages).
