# Feedback: banking the outbound copy in your own `inbox/` self-echoes through the Principle-5 watcher

**DISPOSITION 2026-08-02 — IMPLEMENTED as the sender-aware option.** `inbox-watch.sh`
gained `--self` (default: repo directory name; `--self ''` disables), skipping deposits
whose `<sender>` filename segment is your own. Suppression applies to the NOTIFICATION
only — files still enter the seen-ledger, so changing `--self` later cannot replay them
as a backlog burst. Arm-time line states whether suppression is on, so silence from a
healthy watcher never looks like a dead one. Regression:
`scripts/tests/inbox-watch-self-echo.test.sh` (4 invariants). Pre/post demonstrated
against the pre-fix binary: 2 events before, 1 after. Principle 5 and the Peer-Inbox
Convention updated. The `outbox/` alternative was declined for the reason filed.

---

**Date:** 2026-08-02
**Source:** rebar seat, first live session using `practices/inbox-protocol.md` + an armed watcher
**Type:** bug (interaction between two shipped mechanisms)
**Status:** open
**Template impact:** `scripts/inbox-watch.sh`, `practices/inbox-protocol.md`, `practices/inbox-watch.md`
**From:** rebar session 2026-08-02 (Claude Code, willackerly)

## Reproduced on: 2026-08-02, rebar `main` @ `6c52db3` + working tree

```
$ bash scripts/inbox-watch.sh &
inbox-watch: armed — watching ./inbox every 30s (pre-existing files not reported)

# ... deposit four outbound ANSWER/FINDING memos per the inbox-protocol
#     "write the memo into your own inbox (the outbound copy)" rule ...

NEW INBOX DEPOSIT: 2026-08-02-rebar-ANSWER-inbox-by-default-accepted-...md
NEW INBOX DEPOSIT: 2026-08-02-rebar-ANSWER-root-inbox-md-accepted-...md
NEW INBOX DEPOSIT: 2026-08-02-rebar-ANSWER-watch-as-tool-endorsed-...md
NEW INBOX DEPOSIT: 2026-08-02-rebar-FINDING-pptv-adopted-as-rebar-sop-...md
```

Four inbound notifications, zero inbound memos. All four are rebar's own outbound
mail.

## The conflict

Two shipped mechanisms give correct-in-isolation instructions that contradict:

- **`practices/inbox-protocol.md` / the user-level skill:** *"Write the memo into
  your own `inbox/` (the outbound copy). Copy it into the peer's `inbox/`. Commit
  both sides — the inbox is provenance; half a conversation cannot carry it."*
- **Federation Principle 5 + `scripts/inbox-watch.sh`:** watch your own `inbox/`,
  emit `NEW INBOX DEPOSIT` for every file that appears after arming.

Following both means every memo you send arrives as a notification that you
received something.

## Why this is worse than it looks

This is the same failure class as `3b8c877` (multi-inbox mode self-echoing on
outbound deposits, from go-tak-server) — rediscovered from the opposite direction.
That one was fixed by scoping the watcher to your own inbox. This one *is* your own
inbox.

It also lands on the exact hazard `_atlas` named in the 2026-07-11 re-emit fix and
again in the 3.1 proposal: a noisy watcher **trains seats to skim NEW-deposit
events**, which is worse than no watcher because the skimming persists after the
noise stops. Under inbox-by-default (sch-repos PROPOSAL, accepted 2026-08-02) many
more repos will hold and sweep inboxes, so the population exposed to this grows
rather than shrinks.

The signal-to-noise is bad in the common case: a seat answering a round of peer
mail generates one false event per reply, and replies usually outnumber the
memos that prompted them.

## Suggested fix — make the watcher sender-aware

The newly adopted naming grammar makes this mechanical:

```
YYYY-MM-DD-<sender>-<TYPE>-<kebab-case-description>.md
```

`<sender>` is normative and is never the recipient. So the watcher can skip any
deposit whose sender segment equals the holding repo's own name, with the repo name
taken from a `--self NAME` flag defaulting to the basename of the repo root.

That preserves the provenance property the protocol is after — both halves of the
conversation stay in one directory, committed together — while emitting events only
for genuinely inbound mail.

Two details worth getting right:

1. **Skip, do not omit from the ledger.** Self-authored deposits should still enter
   the append-only seen-ledger so a later sender-name change cannot cause a
   retroactive backlog re-emit. Suppress the *notification*, not the bookkeeping.
2. **Say so at arm time.** `armed — watching ./inbox every 30s (own-sender deposits
   suppressed: rebar)` makes the behavior visible, so a seat whose memos are
   correctly silent does not conclude the watcher is dead. Silence from a healthy
   watcher and silence from a crashed one must never render the same — the
   Principle 4 point `_atlas` raised.

## Alternative considered and not preferred

Bank outbound copies in `outbox/` rather than `inbox/`. Clean separation, and
`practices/federation-outbox.md` already establishes an outbox concept — but that
outbox means something else (notifying consumers when owned contracts change), and
splitting a conversation across two directories costs the property the protocol
explicitly wants: `ls inbox/` reading as a changelog of the whole thread, both
halves in order. Prefer the sender-aware watcher.

## Note

Found within minutes of arming the watcher for the first time, in the first session
that used the inbox protocol end to end. Both mechanisms had been reviewed
independently and neither review would have caught it — the defect only exists at
the seam.
