# Feedback: `check-freshness.sh` parses one marker vocabulary; QUICKCONTEXT uses the other and is invisible to it

**Date:** 2026-08-02
**Source:** rebar self-audit, triggered by the sch-repos AMENDMENT memo of 2026-08-02
**Type:** bug (v3.0.0 scope — `check-freshness.sh` is a shipped v3 mechanism)
**Status:** open
**Template impact:** `scripts/check-freshness.sh`, `templates/project-bootstrap/QUICKCONTEXT.md`,
`templates/component-templates/QUICKCONTEXT.template.md`
**From:** rebar session 2026-08-02 (Claude Code, willackerly)

## The defect

`scripts/check-freshness.sh:44` extracts dates with exactly one pattern:

```sh
date_str=$(grep -o 'freshness: [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}' "$file" ...)
```

Rebar's docs carry **two** marker vocabularies. `STEWARD_REPORT.md` uses
`<!-- freshness: -->`. `QUICKCONTEXT.md` uses `<!-- last-synced: -->`. Only the
first is parsed, so the checker reports:

```
Checked 1 docs with freshness markers, 1 stale (>14 days).
```

One. In a repo where `QUICKCONTEXT.md`, `TROUBLESHOOTING.md`,
`docs/learnings-from-opendockit.md`, `templates/project-bootstrap/QUICKCONTEXT.md`,
`templates/project-bootstrap/METRICS.md`, `templates/project-bootstrap/CONSUMES.md`,
and several component templates all carry a currency marker of some kind.

## Why this one matters more than the count suggests

`QUICKCONTEXT.md` is the first file of the cold-start quad and the most-read
document in any rebar repo. It is the one doc whose staleness directly
mis-grounds an agent at session start — which is the entire failure mode
`check-freshness.sh` exists to prevent.

Measured 2026-08-02: `QUICKCONTEXT.md` reads `last-synced: 2026-07-04` against a
HEAD of `2026-08-02` — **29 days of drift**, through the whole inbox-watch
hardening arc and the v3 annealing window. Cold-start reported all-green on
freshness that entire time, because the file it most needed to flag was the one it
could not see.

This is the two-vocabularies failure mode that S1-STEWARD 2.0 fixed for maturity
vs. lifecycle, reappearing one layer down in the currency markers.

## Field evidence from a peer, and rebar's own counter-measurement

The sch-repos AMENDMENT
(`inbox/2026-08-02-sch-repos-AMENDMENT-status-line-design-refuted-by-field-measurement.md`)
measured their repos and found **0 bumps across 12 `freshness:` markers**, 540
commits, seven weeks — and argued this is evidence against `check-freshness.sh`'s
core assumption that humans maintain a hand-written date.

Rebar's own numbers partly refute the absolute claim and confirm the useful one:

| File | Commits touching it | Distinct marker values |
|---|---:|---:|
| `QUICKCONTEXT.md` | 6 | 3 |
| `TROUBLESHOOTING.md` | 5 | 2 |
| `templates/project-bootstrap/QUICKCONTEXT.md` | 3 | 2 |
| `docs/learnings-from-opendockit.md` | 2 | 2 |

Rebar *does* bump markers — roughly half the commits that touch a marked file
update its date. So "nobody ever bumps" does not generalize. But "markers lag
reality even when maintained" does, and 29 days of undetected drift on the
cold-start doc is the sharper end of that.

The honest reading: the marker convention is not worthless, but a checker that
silently covers one of two vocabularies converts partial maintenance into false
confidence — which is worse than not checking, per Principle 4.

## Suggested fix

1. **Parse both vocabularies.** Accept `last-synced:` alongside `freshness:`,
   preferring `freshness:` when a file carries both. One line of `grep -oE`.
2. **Report the denominator honestly.** "Checked N docs" should count files with
   *any* recognized marker, and cold-start should say how many markable docs carry
   no marker at all — silence about unmarked files is how a repo drifts to one
   checked doc without anyone noticing.
3. **Pick one vocabulary in the templates** and migrate the other, so adopters
   born after the fix have a single spelling. `last-synced:` is the better-named of
   the two (it says what the date means); `freshness:` is the one the checker and
   the `freshness: archived` opt-out already speak. Either is fine — divergence is
   the bug, not the choice.

## Adjacent, from the same peer memo — not proposed here

sch-repos raised two items that belong to whoever picks up this fix:

- **The `expires` inversion.** RFC 9116's `Expires` is a future date set once that
  self-invalidates without anyone touching it. `as-of` degrades to *silently
  wrong*; `expires` degrades to *loudly invalid*. Worth considering if the marker
  design is ever revisited.
- **Frozen artifacts have no honest token.** Five `SCH/experiments/PREREG-*.md`
  files are flagged STALE forever; a preregistration is not stale, it is frozen,
  and its date is its authoring date. The existing `freshness: archived` opt-out
  is the right landing spot — but adopters do not appear to know it exists, which
  is a documentation gap rather than a mechanism gap.
