---
name: rebar-feedback
description: Use when you hit a gap, anti-pattern, bug, or missing guidance in rebar's templates, practices, or scripts during real work — or when the user says "file feedback" / "write this up for rebar." Files a durable feedback item instead of editing templates directly.
---

# rebar-feedback — file a feedback item correctly

Canonical reference: `rebar:feedback/README` (template, processing rules,
disposition lifecycle) — resolve with `scripts/rebar-doc.sh
rebar:feedback/README --cat` (or `rebar doc` if the CLI is installed).
Reference it; do not restate it.

## First: where does this filing go?

- **Working in the rebar repo itself** (it has `feedback/`): file locally,
  steps below.
- **Working in an adopter repo** (no `feedback/` directory): the feedback is
  *about rebar*, so it belongs upstream — use `ask rebar featurerequest
  "<the situation>"` (MCP/ASK), or write the file into your rebar checkout's
  `feedback/` directory. Do not create a `feedback/` dir in the adopter repo
  for rebar-bound items.

## Steps (rebar repo)

1. **Read the canonical reference** (`rebar:feedback/README`, above) and skim
   one recent `feedback/*.md` file for the house voice before writing anything.
2. **Check the gate:** the situation must be real, not hypothetical — a
   specific scenario you actually hit (`rebar:charter` §3, gate 3: concrete use
   case). "Would be cool if…" does not qualify; do not file.
3. **Create the file:** `feedback/YYYY-MM-DD-<slug>.md` (today's date, short
   kebab-case slug).
4. **Fill the required header fields:** `Date`, `Source`, `Type`, `Status`
   (start at `proposed`), `Template impact`, `From` (agent/model, project,
   date — provenance matters).
5. **Body:** What Happened / What Was Expected / Suggestion. Be concrete.
   Point at files with one-line pointers rather than restating their content.

Do not edit `feedback/INVENTORY.md` or move files into `feedback/processed/`
— triage and disposition are maintainer-owned (see "Processing Feedback" in
`rebar:feedback/README`).
