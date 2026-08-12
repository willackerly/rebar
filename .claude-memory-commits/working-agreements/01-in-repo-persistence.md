# prefer in-repo persistence over assistant-memory

**In one line:** Will strongly prefers landing doctrines, principles, and decisions as files in the repo (CHARTER, practices/, feedback/) rather than saving to assistant-memory. Memory is for working-style cues; doctrines belong in repo.

*Migrated from assistant-memory 2026-08-12.*

---
When session-emergent doctrines or principles need persistence, default
to writing them as files in the rebar repo (typically
`practices/<topic>.md`, `feedback/<date>-<topic>.md`, or extending
`CHARTER.md` / `DESIGN.md`) — NOT to assistant-memory.

**Why:** Assistant-memory is invisible to other contributors, agents,
and external readers of the repo. Doctrines that govern how rebar
operates need to be readable by anyone who clones the repo, including
adopters who never see your assistant session. In-repo persistence is
also durable across machine moves, tool changes, and session
boundaries that memory can't span.

**How to apply:**
- When the user says something doctrinal ("this should be the rule"),
  propose persisting it as a `practices/*.md` file or charter
  amendment, not as memory.
- Save to memory only for working-style cues that wouldn't make sense
  to commit (e.g., "user prefers terse responses," "user asks for
  red-team after each design proposal").
- If unsure, ASK whether to persist in repo or memory. Will will
  almost always say "in repo" for substantive items.
- Specific anchor from 2026-04-28: when offered to save the federation
  4-principle doctrine to memory, Will responded "persist those as
  principles in the repo itself." That became `practices/federation.md`.

**Edge cases:**
- Cross-conversation working-style preferences (THIS memory) → memory
- Decisions with project-wide impact (e.g., "no extends: field ever")
  → repo (CHARTER, practices/, feedback/processed/)
- Failure-mode patterns the assistant should avoid → memory
- Architectural rules that bind future PRs → repo
