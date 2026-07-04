---
name: rebar-inbox-watch
description: Use during multi-repo coordination phases (release days, cross-repo arcs, ratification rounds) when this repo holds a peer inbox/ directory — arms a persistent background watcher so new memo deposits reach you as live notifications instead of sitting unread while a peer waits.
---

# rebar-inbox-watch — arm the peer-inbox watcher

Canonical practice: `practices/inbox-watch.md` (rebar checkout if not vendored
here) — design choices, honest
limitations, variations). This skill is the arming procedure only.

## Steps

1. **Sweep first.** `ls -lat inbox/ | head` — process any deposits that
   arrived while no watcher was alive.
2. **Arm the watcher.** Run `scripts/inbox-watch.sh <inbox-dir> [<dir>…]`
   through the harness background-monitor facility in persistent mode
   (Claude Code: the Monitor tool, or a background Bash task). Multi-inbox
   capable — pass one directory per repo pair you are coordinating.
3. **Default cadence is 30s.** Effectively free locally and matched to memo
   timescales; tighten only during a known live back-and-forth, then return
   to 30s.
4. **Act on the emitted lines.** One line per new file, shaped
   `NEW INBOX DEPOSIT: <path>` — it lands as a notification mid-task; read
   and process the memo promptly.

## Session-scoped by design

- The watcher **dies with the session** and must be re-armed at the next
  cold start during coordination phases (the `rebar-coldstart` skill
  includes this step).
- Silence means "no deposits" only while the watcher is alive — if the
  monitor died, re-arm before trusting the quiet.
