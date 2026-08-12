# Two review lenses: smallest input, and relaxation-not-migration

**In one line:** Check every counting rule at n=1 and against a document quoting
itself; and when retiring a convention, never mass-rename.

*Both earned on 2026-08-02, when five defects were fixed in one pass and three
of them shared a shape.*

---

## Lens 1 — test the smallest input, and the self-referential one

Three of the five defects fixed that day were arithmetically correct and broke
only at the boundary. All three landed on **new and small adopters** — the
population least able to diagnose them and most likely to conclude the framework
is broken.

- **Check 9 at n=1** (sch-repos). A percentage over one contract quantizes to 0%
  or 100%, so the middle band was unreachable and the only honest downgrade
  jumped straight to FAIL plus a tier demotion. Declaring your single contract
  `draft` failed CI; leaving it `active` passed. **Dishonesty was the green
  path** — the exact inversion v3 exists to remove.
- **Check 8 vs. its own documentation.** A line-anchored `grep` counted the five
  worked examples inside `CONSUMES.md`'s ` ```markdown ` fence as real
  declarations, so **every freshly bootstrapped project failed compliance** for
  a federation violation it could not have committed.
- **`check-freshness.sh` and the second vocabulary.** It parsed `freshness:` but
  not `last-synced:`, so `QUICKCONTEXT.md` — the most-read file in any rebar
  repo — was invisible to it. Rebar's own drifted 29 days while cold-start
  reported all-green.

**Apply as questions, before shipping any counting or matching rule:**

1. What does this do at **n=0 and n=1**? Is every documented band reachable?
2. What happens when **the documentation quotes the pattern**? A regex that
   counts declarations must be fence-aware — docs are the *most* likely place
   the pattern appears, not an edge case.
3. Is there a **second spelling** in the wild? Two vocabularies with one parser
   converts partial coverage into false confidence.
4. Does the **honest** answer pass? If declaring the truth fails the check and
   overstating passes, the check is inverted regardless of its arithmetic.

sch-repos named lens-2 in design review the same day rebar discovered it had
already shipped it. Worth remembering that a maintainer reviewing their own
proposal caught what a shipped, tested checker did not.

## Lens 2 — relaxation, not migration

When a convention is retired, **existing artifacts keep their old form.**

Retiring contract prefix numbering (2026-08-02) could have triggered a
repo-wide rename to "clean up." That rename would have cost exactly the pain
that made numbering bad in the first place: every `CONTRACT:` header in source,
every companion file, every commit trailer, and the registry. **The same wound,
self-inflicted for tidiness.**

The rules that came out of it, now in `conventions.md`:

- Never renumber. Never mass-rename.
- New artifacts use the new form; that is the entire change.
- Mixed repos are expected and permanently fine.
- Rebar's own `S1-STEWARD` / `S2-ASK-CLI` / `S3-MCP-SERVER` keep their names
  forever, as the worked example of the rule.

**Apply:** when proposing to retire a convention, state in the same breath what
happens to existing artifacts. If the answer is "rename them all," the proposal
is not done yet. And check whether the tooling ever enforced the old convention
at all — for numbering, it never had (see
`working-agreements/04-verify-by-executing-not-by-reading.md`).
