# TODO

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: 2026-03-21 -->
<!-- last-synced: 2026-03-21 — date this file was verified against code -->

Consolidated task tracking for the project. All `TRACKED-TASK:` comments in
code reference sections of this file.

---

## P0 — Immediate (do next)

- [ ] Define first contract for core component
  <!-- Source: project setup, need to establish contract-driven development -->

- [ ] Implement basic project structure
  <!-- Source: initial setup, need working foundation -->

- [ ] Set up development environment and dependencies
  <!-- Source: project bootstrap -->

## P1 — Soon (this sprint / this week)

- [ ] Write first set of unit tests following contract specifications
  <!-- Source: quality foundation needed for reliable development -->

- [ ] Configure CI pipeline with rebar quality gates
  <!-- Source: automated quality enforcement -->

- [ ] Create user stories or BDD scenarios for core features
  <!-- Source: need product requirements clarity -->

## P2 — Backlog (when time permits)

- [ ] Upgrade to Tier 2 rebar enforcement (headers + freshness)
  <!-- Source: as team grows and quality requirements increase -->

- [ ] Implement integration testing strategy
  <!-- Source: beyond unit tests for system reliability -->

- [ ] Set up production deployment pipeline
  <!-- Source: prepare for production deployment -->

## Known Issues & Blockers

### Active Blockers

_None currently._

### Gotchas

- **Rebar learning curve** — Team members new to contract-driven development may need time to adapt
  Fix: Start with CONTRACT-QUICKSTART.md and FEATURE-DEVELOPMENT.md guides

### Workarounds In Place

_None currently._

## Discoveries

<!-- The Steward (scripts/steward.sh) parses this section to track contract health.
     Format: checkbox, type tag, contract ref, description. -->

- [ ] **DISCOVERY** `none` — Project setup complete, ready for first contract implementation

## Code Debt

<!-- Items tracked via `TRACKED-TASK:` comments in source code. -->

_None currently. Add items here as development progresses._

## Completed

<!-- Recently completed items. Keep for 1-2 weeks for agent reference. -->

- [x] Rebar project bootstrap setup — completed 2026-03-21
- [x] Agent configuration and ASK CLI setup — completed 2026-03-21
- [x] Initial project structure creation — completed 2026-03-21

---

<!-- MAINTENANCE NOTES:

ADDING ITEMS:
1. Every `TODO:` comment in code MUST have a corresponding entry here
2. Convert `TODO:` → `TRACKED-TASK:` in code after adding here
3. Include enough context that a cold-start agent understands the task

COMPLETING ITEMS:
1. Check the box [x] and add completion date + commit reference
2. Remove the corresponding `TRACKED-TASK:` comment from code
3. Move to Completed section (don't just delete)

PERIODIC SCRUB (weekly):
1. `grep -rn "TRACKED-TASK:" src/` — verify each still references a live entry
2. `grep -rn "TODO:" src/` — should return 0 results (untracked TODOs block commit)
3. Review P2 backlog — promote urgent items, remove stale ones
4. Archive Completed items older than 2 weeks
5. Update freshness and last-synced dates

DRIFT RISK:
This file is HIGH drift risk. Agents complete tasks but forget to update here.
The `last-synced` date tells you when this was last verified against actual
code state. If it's >1 week old, treat priorities with skepticism and verify
against git log before starting work.
-->