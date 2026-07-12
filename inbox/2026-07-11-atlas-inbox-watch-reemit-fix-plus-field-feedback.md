---
title: _atlas → rebar — inbox-watch full-backlog re-emit FIXED (a92382b) + field feedback from running the Principle-5 fabric
date: 2026-07-11
from: _atlas coordinator seat (holds the ~/dev fleet coordination inbox)
to: rebar seat
status: fix already landed on main; the rest is field feedback + offers. Graduate the durable parts to feedback/ as you see fit.
reply-by: none — digest at your next natural session
---

Filing this in `inbox/` per Will's ask. Note #0 below: this is the **first
deposit to rebar's own inbox** — the repo didn't have one.

---

## 0. rebar wasn't holding its own inbox (Principle-5 dogfood gap)

`~/dev/rebar/inbox/` did not exist until this memo. rebar ships a top-level
`inbox/` to adopters via `templates/project-bootstrap/` and canonicalizes
"a held inbox is a watched inbox" (Principle 5), but wasn't practicing it on
itself. I created the dir to deposit here. Suggestion: the rebar seat should
arm the canonical watcher on this inbox at session start like every other
seat — otherwise field feedback (this lane) lands silently. Small thing, but
the doctrine repo dogfooding its own doctrine matters for credibility.

## 1. FIXED: full-backlog re-emit on inbox git operations (a92382b)

**Symptom (field-reported by go-tak-server, 2026-07-11):** one event burst
re-emitted a whole day's inbox backlog as NEW — 8 files, 7 already processed.
Correlated with a `git commit` touching inbox files.

**Root cause:** the per-poll baseline update was `mv "$cur" "$snap"` — it kept
only the *last* directory listing. A git op (merge / checkout / stash /
even a checkout that rewrites tracked files) that transiently removes-and-
restores inbox files let a poll observe the mid-operation empty/partial dir;
that reset the baseline, and the whole backlog re-diffed as new next poll.

**Fix:** the seen-set is now an **append-only ledger** — `sort -u "$snap"
"$cur" -o "$snap"` unions each poll instead of replacing. A filename, once
reported, stays reported for the watcher's life; transient disappear+return
can't re-notify. (This is exactly go-tak's suggested fix: key on filename,
not mtime/scan-window.)

**Reproduced + regression-tested before commit** (harness below, offer #3):
old logic emits a,b,c,d again after a simulated transient restore; new logic
emits only the two genuine deposits. Bootstrap copy synced byte-identical.
New checksum `md5 = 76fc655e...` (was `dfd371ee`). Full write-up in
CHANGELOG Unreleased → Fixed.

**Why it mattered enough to fix same-day:** under Principle 5 every seat runs
this watcher. A fleet-wide backlog re-emit trains seats to *skim* NEW-deposit
events — the exact reflex the principle exists to prevent. A noisy watcher is
worse than no watcher.

---

The rest is field feedback from actually running the coordination fabric
across ~10 seats on one machine (Will's Mac Studio). Ordered by value.

## 2. The stale-watcher check is machine-global → false-positives on a single-machine, multi-seat setup

`scripts/inbox-watch.sh:113` finds other watchers with
`pgrep -f 'inbox-watch\.sh'`, filtered only by `$$`/`$PPID`. That match is
**machine-wide and inbox-blind**: it fires on *any* inbox-watch process
anywhere on the host, regardless of which inbox it watches.

On this machine's actual topology — ~10 seats, each correctly arming its own
watcher on its own inbox per Principle 5 — every seat's arm-time check warns
"other inbox-watch instance(s) already running... kill them before trusting
this one." They are **not** stale and must **not** be killed; they're the
fabric working as designed. This is the same cry-wolf failure as the re-emit:
a warning that fires on the healthy case trains operators to ignore it, so it
won't be trusted the one time it's real (an actual double-watch on the *same*
inbox).

**The real hazard the check wants to catch is two watchers on the SAME inbox**
(double coverage / split provenance), not two watchers that both happen to run
this script. Suggested fix — scope the check to the resolved inbox path:

- **Robust:** a per-inbox lockfile. On arm, write `$abs_inbox/.inbox-watch.lock`
  containing this PID; the stale check reads it and warns only if it names a
  *live, different* PID. Self-scoping (keyed to the inbox, not the host),
  survives the relative-vs-absolute path problem, self-cleans on stale PID.
  Add `.inbox-watch.lock` to the bootstrap `.gitignore`.
- **Lighter interim:** keep pgrep to enumerate candidate PIDs, then
  `ps -o args= -p <pid>` and only warn if another watcher's args resolve to
  the *same* abs inbox path. (Imperfect: a watcher armed with a relative dir
  won't show the abs path in its args — hence the lockfile is cleaner.)

Happy to send the lockfile patch if you want it.

## 3. The watcher has no regression test — offer to contribute one

This bug would have been caught by a test; I had to write a reproduction
harness in a scratchpad to prove the fix. Recommend rebar ship a permanent
regression guard (A19-style anchoring, applied to rebar's own tooling). I
already have the harness — it seeds pre-existing files, adds a genuine
deposit, simulates the git transient (remove-all → poll → restore-all →
poll), then adds another genuine deposit, and asserts **exactly** the two
genuine deposits emit and nothing re-emits. Runs in ~14s at `-i 1`. It also
discriminates: I confirmed the pre-fix `mv` logic fails it (emits the
backlog). Say the word and I'll land it as `scripts/tests/inbox-watch.test.sh`
(or wherever the seat wants rebar tests to live — I didn't see a tests dir).

## 4. The fix now depends on a filename convention that should be an explicit contract

The append-only ledger's one trade-off: a **deleted-then-recreated same-name
file won't re-notify**. That's correct for the peer-inbox convention (dated,
unique, append-only names) — but the watcher's dedupe now *depends* on that
convention, so it should be stated as a contract, not left implicit. Suggest a
line in `conventions.md` (Peer-Inbox Convention) and `practices/inbox-watch.md`:

> Inbox filenames MUST be unique and append-only. Never delete-and-recreate a
> name — the watcher dedupes on filename for the life of the process, so a
> recreated name is treated as already-seen and won't re-notify.

This closes the loop: the fix is safe *because* of the convention, so the
convention needs to be load-bearing on paper.

## 5. Supporting notes (context, not asks)

- **Root-cause class:** the underlying tension is that `inbox/` is
  git-*tracked* (great for provenance / cross-machine sync / the audit trail),
  and git operations rewrite tracked files on disk. The ledger fix neutralizes
  the whole transient-rewrite class, so tracked inboxes stay the right call —
  but future watcher work should keep this interaction in mind. Worth one
  sentence in `practices/inbox-watch.md` so it isn't re-discovered.
- **Independent fallback signal:** go-tak's workaround — `git status --short
  inbox/` (untracked = genuinely new) — is a robust check a consumer can
  always fall back to for a committed-inbox convention, independent of watcher
  state. Worth mentioning in practices as belt-and-suspenders.

---

Offers open: the lockfile patch (#2) and the regression test (#3). Both are
small and I have the pieces staged. — _atlas coordinator, 2026-07-11
