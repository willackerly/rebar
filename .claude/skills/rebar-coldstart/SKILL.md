---
name: rebar-coldstart
description: Use at the start of every session in a REBAR-managed repo, before the first substantive task — and any time you catch yourself working from stale context (post-compaction restart, missing cold-start hook output, QUICKCONTEXT claims that don't match git reality). Runs the session-start ritual that grounds you in current project state.
---

# rebar-coldstart — the session-start ritual

Canonical practice: `rebar:practice/session-lifecycle` (Session Start section)
— resolve with `scripts/rebar-doc.sh rebar:practice/session-lifecycle --cat`
(or `rebar doc` if the CLI is installed).
This skill is a pointer to the ritual, not a replacement for the practice doc.

## Steps

1. **Read the cold-start quad:** `QUICKCONTEXT.md`, `TODO.md`, `CLAUDE.md`
   (or `AGENTS.md` where that is the agent-facing doc), and recent reality —
   `git log --oneline -15`.
2. **Look for the hook output.** A `SessionStart` hook (configured in
   `.claude/settings.json`) runs `scripts/cold-start-checks.sh` and injects a
   `<rebar-cold-start>…</rebar-cold-start>` block into your first turn. If the
   block is present, read it — it is harness ground truth, not prose.
3. **No block? Run it yourself.** On harnesses without hooks (or before the
   hook is installed), run `scripts/cold-start-checks.sh` manually and read
   its output. Missing block = missing hook; say so.
4. **Cross-check freshness.** Compare QUICKCONTEXT claims against `git log`;
   if its last-synced date is >1 week old, treat all claims as suspect and
   verify before acting.
5. **Coordination seats only:** if this repo holds a peer `inbox/`, sweep it
   first (`ls -lat inbox/ | head`), then arm `scripts/inbox-watch.sh` as a
   persistent background monitor — see the `rebar-inbox-watch` skill.

Do not start substantive work until steps 1–4 are done.
