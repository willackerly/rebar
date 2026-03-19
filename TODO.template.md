# TODO

<!-- FRESHNESS: Update this date every time you modify this file -->
<!-- freshness: YYYY-MM-DD -->
<!-- last-synced: YYYY-MM-DD — date this file was verified against code -->

Consolidated task tracking for the project. All `TRACKED-TASK:` comments in
code reference sections of this file.

**Two-Tag System:** See AGENTS.md "TODO Tracking" for full rules. Short version:
- `TODO:` in code = untracked = **blocks commit**
- `TRACKED-TASK:` in code = tracked here = commit allowed

---

## P0 — Immediate (do next)

<!-- Critical path items. No more than 3-5 at a time.
     If this section has >5 items, re-prioritize. -->

- [ ] [task description]
  <!-- Source: where this came from (user request, bug report, agent finding) -->

## P1 — Soon (this sprint / this week)

<!-- Important but not blocking. Queue for after P0 is clear. -->

- [ ] [task description]

## P2 — Backlog (when time permits)

<!-- Nice-to-have improvements, tech debt, optimization.
     Review monthly — promote or remove stale items. -->

- [ ] [task description]

## Known Issues & Blockers

<!-- Known Issues go ABOVE Code Debt intentionally.
     Gotchas affect every agent's session (e.g., "DATABASE_URL breaks tests").
     Code debt is background maintenance. Agents need to see gotchas first. -->

<!-- Things that PREVENT work or CAUSE CONFUSION. This replaces a separate
     KNOWN_ISSUES.md — keeping everything in one file reduces drift.
     Include the fix/workaround, not just the problem. -->

### Active Blockers

<!-- Things blocking work. Include who/what is blocked and workarounds.
     Remove promptly when resolved (move to Completed). -->

_None currently._

### Gotchas

<!-- Counter-intuitive behaviors, misleading names, undocumented dependencies.
     Keep until the underlying cause is fixed. -->

<!-- Example:
- **Playwright HTML reporter blocks in non-interactive mode**
  Fix: Always use `reporter: [['html', { open: 'never' }], ['list']]`
-->

### Workarounds In Place

<!-- Temporary fixes in the codebase. MUST link to a task above.
     Format: what → where → why → remove when -->

<!-- Example:
- **Blob URL for web workers** — `src/worker-loader.ts:15`
  Why: CDN serves .mjs with wrong MIME type
  Remove when: CDN config updated (see P1 task above)
-->

## Code Debt

<!-- Items tracked via `TRACKED-TASK:` comments in source code.
     Each entry should reference the file and line where the comment lives.
     Periodically verify these still exist in code (comments may be
     removed when the debt is paid). -->

<!-- Example:
- [ ] Handle edge case for concurrent session timeout
  - `internal/relay/relay.go:142` — TRACKED-TASK comment
  - Context: race condition when two peers timeout simultaneously
-->

## Completed

<!-- Recently completed items. Keep for 1-2 weeks so agents can see what's
     been done, then archive.
     Format: [x] task — completed YYYY-MM-DD, commit/PR reference -->

<!-- Example:
- [x] Add AES-256-GCM encryption to blob store — completed 2026-03-10, commit abc1234
-->

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
1. `grep -rn "TRACKED-TASK:" src/ packages/ internal/` — verify each
   still references a live entry in this file
2. `grep -rn "TODO:" src/ packages/ internal/` — should return 0 results
   (any untracked TODOs need to be tracked or fixed)
3. Review P2 backlog — promote urgent items, remove stale ones
4. Archive Completed items older than 2 weeks
5. Update the freshness and last-synced dates

DRIFT RISK:
This file is HIGH drift risk. Agents complete tasks but forget to update
here. The `last-synced` date tells you when this was last verified against
actual code state. If it's >1 week old, treat priorities with skepticism
and verify against git log before starting work.
-->
