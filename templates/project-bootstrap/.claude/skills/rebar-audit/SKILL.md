---
name: rebar-audit
description: Use when asked about a rebar repo's health, compliance, tier, or contract status — and before claiming any tier, badge, or "green" state in docs or commits. Runs the three status surfaces and reads them without conflating the two maturity vocabularies.
---

# rebar-audit — run and READ the status surfaces

## Steps

1. **`rebar audit`** — REBAR CLI compliance audit; sectioned score out of 10
   (rebar itself expects 9–10/10).
2. **`scripts/ci-check.sh`** — the full enforcement-script suite; the exit
   code is the verdict, the output names each failing check.
3. **`scripts/steward.sh --summary`** — one-liner: contract counts by
   computed lifecycle, open discoveries, enforcement passing. A full
   `scripts/steward.sh` run writes `STEWARD_REPORT.md` +
   `architecture/.state/steward-report.json`.

## The two vocabularies (do not conflate)

Computed lifecycle (`draft`/`active`/`testing`/`impl-present`) is derived by
the steward from file presence, while declared maturity
(`stub`/`draft`/`in-progress`/`active`/`verified`) is an honest human/agent
declaration of how real the artifact is. Canonical definitions:
`conventions.md`.

- **Computed** shows up in steward output: the `--summary` counts, the
  `STEWARD_REPORT.md` table, the `.state` JSON. `impl-present` means
  implementation files exist — it does NOT mean proven.
- **Declared** shows up as `Status:` lines in artifact headers
  (`architecture/CONTRACT-*.md`) and is weighted into the compliance badge
  by `scripts/check-compliance.sh`. Only this vocabulary may say `verified`
  (active + passing tests/scenarios proving it).

Report what the surfaces actually printed, not what you expected.
