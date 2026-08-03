# Feedback: Should office180 PPTV become the default means for producing diagram documentation?

**Date:** 2026-07-30
**Source:** maintainer question (Will) — evaluate `office180-md-office-converter` PPTV as the swarm-wide diagram default
**Type:** research / proposal
**Status:** research — **Finding 1 closed same-day** by office180 `agent/pptv-diagram-atom`
(merge 555b352, `0.1.0-alpha.3`); see §"Update 2026-07-30" below. Remaining open
questions are now 3 and 5 only.
**Template impact:** would add `practices/diagram-documentation.md` (new practice, currently none exists) and a `rebar:practice/diagram-documentation` abstract ref
**From:** rebar session 2026-07-30 (Claude Code, willackerly)

## Finding 0: rebar has no diagram convention to replace

Before evaluating a replacement, the baseline: this repo has **zero** mermaid
blocks, **zero** `.svg` or `.png` assets, and no `diagram` guidance in
`conventions.md`, `practices/`, or `templates/`. Every visual in rebar docs is
ASCII inside a ` ```text ` fence.

That changes the question. This is not "swap mermaid for PPTV" — it is
"establish a diagram practice where none exists, and decide whether PPTV is it."
The zero-dependency ASCII floor is the incumbent, and it is genuinely hard to
beat on cost: 0 packages, 0 runtimes, renders in a terminal, a GitHub diff, and
an agent's context window alike.

## Finding 1: the disqualifier is a unit mismatch, not immaturity

PPTV 0.1 semantically loads exactly one artifact form: a self-contained
`.pptv.html` **whole deck**. Standalone `.pptv.svg` — one diagram sitting beside
one markdown doc, which is what "diagram documentation" denotes — is
recognition/inventory only.

This is not an inference. `PPTV-PROFILE.md` states standalone `.pptv.svg` and
external manifests "are recognized/inventoried but not semantically loaded,"
`PPTV-DESIGN-INDEX.md` §4.2 repeats it, and the authoring skill
(`.agents/skills/pptv-authoring/SKILL.md`) instructs agents:

> Standalone `*.pptv.svg` is recognized, but the current semantic edit, resolve,
> editor-pack, and PPTX pipeline accepts self-contained HTML only.

So the default path for "add a diagram to `ARCHITECTURE.md`" does not exist in
0.1. Making PPTV the default today would mean authoring a one-slide deck per
diagram — a deck envelope, a manifest, a theme block, and a fixed viewer runtime
per figure. That is the wrong unit of work for a figure in a doc.

Three constraints compound it, each of which is a deliberate *feature* at deck
scale and a tax at figure scale:

1. **Fixed canvas** (§4.13) — one exact `0 0 1600 900` viewBox, deck-wide
   Widescreen. Doc figures are arbitrary aspect; a sequence diagram and a
   4-box component sketch do not share a slide canvas.
2. **No reflow** (§4.14) — explicit hard lines, explicit frame geometry, no
   autofit or shrink-to-fit. For an authored deck this is what makes PowerPoint
   output faithful. For a diagram that tracks a changing design, every label
   edit becomes a manual re-layout, and C8 text-fit is preflight *evidence*, not
   a fixer — it deliberately does not mutate source.
3. **Toolchain** — Node ≥20 + a pnpm workspace + `@office180/pptv@0.1.0-alpha.2`.
   The swarm is polyglot (Python, Go, TypeScript, shell). A default that only
   some repos can run is not a default.

## Finding 2: maturity argues for a gate, not a veto

C4/C5 are verified authorities. C6/C7/C8 are implemented and contracted but
`in-progress`: C6 retains browser-parity and fixture gates, C7 is a
deterministic **strict primitive-only** canary, C8 needs browser/native
calibration. `PPTV-DESIGN-INDEX.md` §8 lists nine remaining promotion steps and
says the full profile "must not be declared a stable standard based on prose
alone."

office180 is holding itself to exactly the discipline `practices/spike-first-contracts.md`
asks for. rebar declaring an alpha the swarm-wide default would undercut that
discipline from the outside — and would be rebar telling adopters to depend on a
surface its owner has not promoted.

## Finding 3: the federation direction is backwards for a blanket default

office180's `CONSUMES.md` has **no entries** — it is a consumer repo, not a
rebar-contract owner. A swarm-wide PPTV default would make rebar's own doc
convention depend on office180's release cadence, and would be rebar's *first*
consumed contract (`office180/C4-PPTV-SOURCE.1.0` or similar).

Per CHARTER §1.6 that dependency is legal and declarable — but it is a real
inversion worth doing on purpose rather than by default: the framework's core
documentation practice would pin to an alpha owned by one adopter.

## Finding 4: there is a lane where PPTV should be the default today

Everything above is about the wrong lane. Where the destination is **PowerPoint**,
PPTV is the strongest option in the swarm and nothing else is close:

- Deterministic text source → editable PPTX, validated against ISO/ECMA
  schemas, independent OpenDocKit reopen, and native PowerPoint open/render
  without repair (C7).
- Provenance that matches rebar's ground-truth ethos: SHA-256 over exact bytes,
  stable IDs, source maps, retained BOM/newline spelling — versus an opaque
  binary a human dragged around in the PowerPoint UI.
- An agent path (§4.10: `outline` → patch by stable ID → `validate`) that beats
  regenerate-the-whole-source once a diagram is large enough that reading it
  costs real context — which is deck scale, exactly where PPTV already works.

`practices/visual-fidelity.md` already assumes deck deliverables exist
(`cisa-advisory-15-slides.pptx`). That lane has no source-of-truth story today.
PPTV is it.

## Proposal (not implemented — awaiting answers)

Add `practices/diagram-documentation.md` that selects by **destination**, and
name PPTV as one lane's default rather than the default:

| Destination | Default | Rationale |
|---|---|---|
| In-repo doc figure (`architecture/`, `practices/`, README) | ASCII in a ` ```text ` fence | Zero dependency, diffable, renders in terminal + GitHub + agent context. Incumbent by default, not by decision. |
| In-repo figure too complex for ASCII | **open question 2** | Today there is no answer; this is the real gap. |
| Deliverable destined for PowerPoint | **PPTV** (`.pptv.html` + `pptx-canary`) | Only deterministic editable-PPTX path in the swarm; provenance fits ground-truth model. |
| Rendered fidelity target | existing `practices/visual-fidelity.md` | Already covered. |

Plus a **named promotion gate** so this is revisitable in one file rather than
re-litigated: flip the in-repo-figure default to PPTV when (a) standalone
`.pptv.svg` gains semantic load/resolve/compile, (b) a non-16:9 or
author-declared canvas profile exists, and (c) the package leaves alpha. Those
map onto office180's own §8 promotion list, so the gate costs office180 nothing
it was not already planning.

If this direction is accepted, the federation-clean form is a `CONSUMES.md`
entry in rebar pinning the PPTV contract, with `notify_on_change: true` — which
also becomes the first live exercise of `rebar contract drift-check` against a
real entry rather than a documented-but-empty file.

## Open questions (maintainer decision required)

1. **Scope of "diagram documentation."** Does the question mean in-repo doc
   figures, PowerPoint deliverables, or both? The recommendation splits precisely
   on this, and the two lanes have opposite answers.

2. **The complex-in-repo-figure gap.** ASCII does not scale past ~6 boxes and
   rebar currently has no answer above that. Is the intended fix (a) mermaid
   fences — free, renders on GitHub and in Claude artifacts natively, no
   toolchain, but no PowerPoint path and no stable IDs; (b) wait for PPTV
   standalone SVG; or (c) accept the gap and keep prose?

3. **Alpha dependency appetite.** Is rebar willing to make `0.1.0-alpha.2` a
   documented default for adopters, or should the default wait for 1.0?
   (`practices/spike-first-contracts.md` implies the latter; a deliberate
   exception is fine if stated.)

4. **Polyglot floor.** Should a rebar *default* ever require a Node ≥20
   toolchain, given adopters in Python, Go, and shell? Or must defaults stay
   runtime-free, with tool-specific choices living in per-repo conventions?

5. **Federation posture.** Accept rebar's first `CONSUMES.md` entry pointing at
   an adopter's contract? This inverts the usual owner→consumer direction and is
   worth an explicit yes/no rather than arriving as a side effect.

6. **Who owns the practice?** If PPTV becomes any kind of default, does
   `practices/diagram-documentation.md` live in rebar (framework doctrine,
   pinning an external contract) or does rebar link out to office180's design
   packet via a `rebar:` abstract ref (looser coupling, weaker doctrine)?

## Update 2026-07-30 — Finding 1 closed, gate now 1/3 open

office180 shipped `agent/pptv-diagram-atom` (merge 555b352, 26 commits,
`@office180/pptv@0.1.0-alpha.3`) the same day. Verified by executing the CLI, not
by reading the docs — full evidence in the memo deposited at
`office180-md-office-converter/inbox/2026-07-30-rebar-pptv-diagram-atom-review-and-followup.md`.

**Closed:**

- **Finding 1 (unit mismatch)** — standalone `.pptv.svg` is now a first-class
  semantic atom: load, outline, validate, resolve, text-fit, direct-text patch,
  editor-pack. Verified against a fresh 1200×800 atom.
- **Canvas constraint** — atoms take any finite-positive `viewBox`; only deck
  slides stay pinned to `0 0 1600 900`.
- **Finding 2 (maturity)** — C4/C5/C6 promoted to 1.1 `verified`; C7/C8 honestly
  `in-progress`. 222 tests pass, `ci-check` 14/14, cold-start 4/4.
- **Q4 (polyglot floor)** — substantially weakened. An atom is inert, ordinary SVG
  with zero embedded script, so the *read* path (GitHub blob view, markdown
  `![](x.pptv.svg)`, any browser) needs no toolchain at all. Node ≥20 is required
  only to edit semantically or audit text fit.

Also worth recording for the practice, whenever it is written: a sha-preconditioned
`set-text` patch produced a **one-line git diff**. Minimal reviewable diffs are the
property that makes a vector format legitimate as a version-controlled doc artifact,
and most vector formats fail it by reserializing.

**Still open:** Q3 (alpha appetite) and Q5 (federation posture). Q1 and Q2 are
answered by the two-lane split below; Q6 is downstream of Q5.

**New, from the fix:** the two lanes are now *disconnected*. `pptx-canary` on an
atom fail-closes (`PPTV-DOCUMENT-KIND`, exit 1 — correct), and hydration runs one
way only (deck → atom via `extract`); atom → deck composition is unimplemented in
office180's `TODO.md`. So "author a figure beside the docs, later put it in a
customer deck" has no path. The atom's arbitrary canvas is exactly what blocks
promotion into a fixed-`1600 900` deck. That question is now the live input from
office180 — it decides whether rebar's practice describes one workflow with two
output targets or two workflows sharing a vocabulary.

**Net:** the gate proposed above was (a) standalone SVG semantics, (b) a non-16:9
canvas profile, (c) non-alpha. (a) and (b) are met; only (c) blocks. That is a
version-string question rather than a capability question — a materially better
position than when this memo was filed.

## Evidence

- `/Users/will/dev/office180-md-office-converter/PPTV-DESIGN-INDEX.md` §§4.2,
  4.13, 4.14, 8, 9
- `/Users/will/dev/office180-md-office-converter/PPTV-PROFILE.md`
  ("Implemented boundary and version vocabulary")
- `/Users/will/dev/office180-md-office-converter/.agents/skills/pptv-authoring/SKILL.md`
  ("Choose the supported container")
- `/Users/will/dev/office180-md-office-converter/packages/pptv/package.json`
  (`0.1.0-alpha.2`, `engines.node >=20`)
- `/Users/will/dev/office180-md-office-converter/CONSUMES.md` (no entries)
- `/Users/will/dev/office180-md-office-converter/METRICS.md`
  (8 contracts, 121 TS test cases, 2 schemas)
- rebar baseline: no mermaid blocks, no `.svg`/`.png` assets, no `diagram`
  guidance in `conventions.md` / `practices/` / `templates/`
