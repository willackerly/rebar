# Feedback: "Commit any uncommitted work" isn't enough — make PUSHES reflexive (durability rule + steward unpushed-commit warning)

**Date:** 2026-07-02
**Source:** tak-tdf integration-arc kickoff session — cold-start review found 19 unpushed commits on `main`
**Type:** improvement
**Status:** proposed
**Template impact:** `templates/` CLAUDE.md + AGENTS.md ("Ending a Session" checklist), steward health surface (`ask steward summary` / refresh-context freshness check)
**From:** tak-tdf coordination session (Claude Code, willackerly)

## What happened

A cold-start review of tak-tdf (a rebar Tier-3 repo) found **19 unpushed commits on `main`** — the
entire IVM perf arc (an O(N²)→O(1) fix measured at ~970×, plus the prod-incident hardening that arc
shipped) existed only on one laptop. The repo's QUICKCONTEXT even *documented* this as fine:
"prod deploys from dist, not origin, so unpushed is fine."

That reasoning conflates **deploy safety** with **durability**. Deploying from dist means unpushed
work is *invisible* to prod — not *safe*. One disk failure loses a week of verified,
shipped-to-prod engineering history, plus the docs/receipts that make rebar's ground-truth model
work in the first place.

Commit discipline held perfectly — the template's session-end checklist says "commit any
uncommitted work." Push discipline had no rule to hold to, so it silently didn't exist.

## Suggestion

1. **Codify a Reflexive Push rule in the templates.** `git push` is part of the commit ritual, not
   a separate decision. "Ending a Session" step 3 becomes: clean up, commit, **push**. (tak-tdf
   adopted this wording in its CLAUDE.md on 2026-07-02, Will-ratified — happy to be the reference.)
2. **Steward/health surface:** `git rev-list --count @{u}..HEAD` is one line — surface
   "N unpushed commits on <branch>" in `ask steward summary` / the freshness check, and WARN above
   a small threshold (e.g. 5) or when the oldest unpushed commit is older than ~48h.
3. **Cold-start verify step:** the existing "VERIFY `git log` vs QUICKCONTEXT" step should also flag
   ahead-of-origin state — a stale origin makes *origin* the liar rather than the docs, and every
   consumer that isn't the laptop (CI, collaborators, a future recovery, a fresh clone) reads origin.

## Why it fits rebar

Rebar's thesis is trustable status + ground truth. An origin 19 commits behind is a status surface
silently lying to everyone but one machine. The fix is the cheapest possible kind: one sentence of
template, one line of steward check, pure-upside default.
