# propose-then-ship cycle for substantial changes

**In one line:** For multi-commit features or new doctrines, Will prefers a propose-then-ship cycle — file a research-only feedback artifact first, let him triage open questions, then implement in subsequent commits keyed off the answers. Worked well 3x on 2026-04-28.

*Migrated from assistant-memory 2026-08-12.*

---
For non-trivial multi-commit work, the preferred workflow is:

1. **Propose** — write a research-only artifact at
   `feedback/YYYY-MM-DD-<topic>.md` containing: design (concrete
   enough to react to), red-team / failure modes, deliverables list
   with line estimates, open questions for the maintainer.
2. **Triage** — Will reviews + answers the open questions inline (or
   in conversation). May modify scope.
3. **Ship** — implementation lands in subsequent commits, themed by
   cluster, each commit-message keyed off the resolved questions.
4. **Close** — move feedback file to `feedback/processed/` (or update
   disposition in-place), update INVENTORY.md.

**Why:** Lets Will absorb a substantial design (~300+ lines of
proposal) without the overhead of immediately reviewing code changes.
Surfaces tradeoffs when they're cheap to flip, not after 500 lines
have landed. Clusters become natural commit boundaries. Worked 3x on
2026-04-28: ASK featurerequest landing, v2.1.0 UX pass, v2.2.0
federation pass.

**How to apply:**
- For changes >~150 lines or that touch CHARTER / DESIGN / new
  practices: PROPOSE first, ship second. Don't write code until
  questions are answered.
- For 1-3 commit fixes / small features: skip the cycle, ship
  directly with a clear commit message.
- The proposal should be substantive (~250-400 lines) — Will reads
  carefully and engages with specifics. Don't write thin proposals.
- Always include explicit "open questions for maintainer" with the
  assistant's own lean. Will often "defers to you" on questions
  beyond his immediate scope.
- After Will answers questions ("ship as is otherwise" or similar),
  proceed to implementation without re-asking permission per cluster.
  Will trusts the cycle; don't add friction.

**Anti-patterns to avoid:**
- "Want me to start with X?" + "100% go for it" + asking again two
  paragraphs later — once green-lit, ship.
- Writing implementation without the proposal artifact first for
  substantial work — Will will pause work to reset to propose-first.
- Not capturing the open-questions resolutions in the
  feedback file's Disposition section before closing.
