# Working agreements

How the maintainer wants an agent to operate in this repo. Short by design —
read all of it at session start, after the cold-start quad.

| # | Agreement | Use when |
|---|-----------|----------|
| [01](01-in-repo-persistence.md) | Persist doctrine in the repo, not in assistant-memory | Something doctrinal emerges and needs a home |
| [02](02-propose-then-ship.md) | Propose-then-ship for substantial work | A change is >~150 lines or touches CHARTER / DESIGN / new practices |
| [03](03-fix-commit-reproduced-on-trailer.md) | `fix:`/`regression:` commits carry `Reproduced on:` | Drafting any fix commit — include it from the start, not as an amend |
| [04](04-verify-by-executing-not-by-reading.md) | Verify by executing, not by reading prose | Before reporting anything works, is broken, or is required |
| [05](05-advisory-over-blocking.md) | New checks report and exit 0 | Adding any check or gate |
| [06](06-smallest-input-and-relaxation-lenses.md) | Test at n=1 and self-reference; retire without migrating | Shipping a counting rule, or retiring a convention |

01–03 predate 2026-08; 04–06 were earned in the 2026-07-30 → 2026-08-02 arc.
