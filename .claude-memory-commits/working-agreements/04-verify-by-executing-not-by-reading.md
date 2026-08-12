# Verify by executing, not by reading prose

**In one line:** Before reporting that something works, is broken, or is
required — run it. Documentation, contracts, and your own prior conclusions are
hypotheses, not evidence.

*Emerged across sessions 2026-07-30 → 2026-08-02. The single highest-yield habit
of that arc.*

---

## The agreement

When a claim can be checked by running something, run it. Then report what
happened, with the command and the output — not what the docs said would happen.

Label claims by how you know them, per `practices/inbox-protocol.md`: `[M]`
measured, `[V]` read from source, `[R]` reported by someone else, `[U]`
unverified. The friction of writing `[M]` next to something you did not actually
run is the mechanism; keep it uncomfortable.

## Why — four times it changed the answer

1. **office180 PPTV, first review (2026-07-30).** The design docs read as though
   the diagram workflow existed. Running the CLI showed standalone `.pptv.svg`
   was recognition-only, which was the actual blocker. Reading would have
   produced a confident, wrong recommendation.

2. **office180 PPTV, second review (2026-08-02).** Prose again claimed the gap
   was closed. This time running it confirmed the claim *and* showed the
   fail-closed aspect behavior was better than described. Same method, opposite
   result — which is the point: the method is not skepticism, it is measurement.

3. **Contract numbering (2026-08-02).** Everyone "knew" the tooling required
   prefix numbers. A five-minute probe — add `CONTRACT-NAMING-PROBE.1.0.md`, run
   the enforcement chain, delete it — proved every script already accepted
   unnumbered IDs. **The requirement existed only in documentation.** That single
   probe turned a feared migration into a doc change.

4. **Graduation criterion 4 (2026-08-02).** A `grep` for `verified` returned
   hits in three repos and looked like a failure. Reading the scripts showed all
   three were v2 repos whose steward emits *and* parses the old vocabulary
   consistently — no mismatch anywhere. **The grep was the wrong instrument**;
   it could not distinguish a script emitting a token from one parsing it.

## How to apply

- **Probe, then delete.** To test whether a rule is enforced, create the minimal
  artifact that would violate it, run the checks, remove it, and confirm the
  repo is back to its prior state. Cheap, decisive, leaves nothing behind.
- **Prefer the real binary over a simulation.** Testing the arms of a shell
  conditional by hand-executing the arithmetic is not the same as running the
  script; run the script.
- **When a tool disagrees with a doc, the tool is right** — and the doc is now a
  finding worth filing.
- **State the caveat when you verified against uncommitted state.** On
  2026-08-02 the PPTV verification ran against a `dist/` built from an
  uncommitted tree; saying so mattered, because the claim did not carry to HEAD.
- **A negative result is a result.** "I ran it and it did not reproduce" is
  worth reporting, and is how the criterion-4 false alarm was closed.

## Anti-patterns

- Reporting a capability from a `README`, a contract, or a `QUICKCONTEXT` claim
  without exercising it. Rebar's own QUICKCONTEXT was **29 days** out of date
  while asserting current state.
- Using `grep` to answer a question about *semantics* (does anything parse this
  value?) rather than *presence* (does this string appear?).
- Re-deriving a conclusion you already measured this session. Measure once,
  then cite yourself.
