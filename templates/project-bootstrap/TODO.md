# TODO

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: 2026-03-21 -->
<!-- last-synced: 2026-03-21 — date this file was verified against code -->

Active tasks only. Scan in 10 seconds, not 5 minutes.
Priorities live in QUICKCONTEXT.md "What's Next" — that is the single source of truth.

---

## Open Items

- [ ] Define first contract for core component
- [ ] Implement basic project structure
- [ ] Set up development environment and dependencies
- [ ] Write first set of unit tests following contract specifications
- [ ] Configure CI pipeline with rebar quality gates

## Known Issues & Blockers

<!-- One canonical entry per issue. Cross-reference, don't duplicate.
     If an issue also appears in QUICKCONTEXT, point there: "See QUICKCONTEXT §X" -->

_None currently._

### Gotchas

- **Rebar learning curve** — Team members new to contract-driven development may need time to adapt.
  Fix: Start with CONTRACT-QUICKSTART.md and FEATURE-DEVELOPMENT.md guides

## Discoveries

<!-- The Steward (scripts/steward.sh) parses this section to track contract health.
     Format: checkbox, type tag, contract ref, description. -->

- [ ] **DISCOVERY** `none` — Project setup complete, ready for first contract implementation

## Code Debt

<!-- Items tracked via `TRACKED-TASK:` comments in source code. -->

_None currently._

---

<details>
<summary><strong>Completed</strong> (click to expand)</summary>

<!-- Move completed items here. Archive items older than 2 weeks.
     For full history, see git log. -->

- [x] Rebar project bootstrap setup — completed 2026-03-21
- [x] Agent configuration and ASK CLI setup — completed 2026-03-21

</details>

---

<!-- MAINTENANCE NOTES:

ADDING ITEMS:
1. Every `TODO:` comment in code MUST have a corresponding entry here
2. Convert `TODO:` → `TRACKED-TASK:` in code after adding here
3. Include enough context that a cold-start agent understands the task

COMPLETING ITEMS:
1. Check the box [x] and add completion date
2. Move to the collapsed Completed section
3. Remove the corresponding `TRACKED-TASK:` comment from code

KEEPING IT SHORT:
- This file should have <50 lines of open items
- Completed items go in the collapsed section (or just git log)
- Archive completed items older than 2 weeks
- Priorities are NOT tracked here — they're in QUICKCONTEXT.md "What's Next"

DRIFT RISK:
This file is HIGH drift risk. Agents complete tasks but forget to update here.
The `last-synced` date tells you when this was last verified against actual
code state. If it's >1 week old, verify against git log before starting work.
-->
