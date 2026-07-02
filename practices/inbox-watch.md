# Peer-Inbox Watching — live notification for federation memos

> Practice status: proven in the field (go-tak-server ↔ tak-tdf ↔ TDFLite-tak, 2026-07-02 —
> same-day memo turnaround across three repos during a coordination-heavy release day).

## The problem

Federated peer repos coordinate by depositing dated markdown memos into each other's `inbox/`
directories (the discipline: append-only, one file per memo, `YYYY-MM-DD-<from>-<topic>.md`).
The discipline works — but an agent session only *sees* a deposit when it happens to look. During
a coordination phase (a release, a cross-repo arc, a ratification round), "happens to look" means
either wasteful re-checking or memos sitting unread for hours while the peer waits.

## The mechanism

A session-scoped background monitor that polls the inbox and emits one line per new deposit. Each
emitted line lands in the agent's conversation as a notification — the agent gets pinged with the
filename mid-task and can process the memo immediately.

```bash
cd <repo>/inbox
prev=$(ls -1 | sort)
while true; do
  sleep 30
  cur=$(ls -1 | sort)
  new=$(comm -13 <(echo "$prev") <(echo "$cur"))
  if [ -n "$new" ]; then echo "NEW INBOX DEPOSIT: $new"; fi
  prev=$cur
done
```

Registered through the harness's background-monitor facility (Claude Code: the `Monitor` tool) in
**persistent** mode — it runs for the life of the session, across turns, not tied to any one task.
Any harness that turns a long-running command's stdout lines into agent notifications can host it.

## Design choices (and why)

- **Poll, don't fswatch.** 30s cadence on a local directory is effectively free, and peer memos
  arrive on human-ish timescales. `fswatch`/`inotifywait` work too, but the poll loop has zero
  dependencies and identical behavior on macOS/Linux — the substrate rule (CHARTER: plain text,
  bash, grep) applies to watchers as much as to state files.
- **Filename diff, not content hash.** The inbox convention is append-only: peers deposit new dated
  files, they never edit old ones. New-name detection is therefore the *complete* signal, and
  `comm -13` on sorted listings is the whole implementation. If your inbox convention allows edits,
  this practice does not fit it — fix the convention first (append-only is what makes inboxes
  auditable).
- **One line per event, no digests.** Each deposit is an independent notification; batching would
  reintroduce the latency the watcher exists to remove.
- **`sort` both sides.** `comm` requires sorted input; unsorted listings silently mis-diff.

## Honest limitations

- **Session-scoped.** The watcher dies with the session. Re-arm it at session start during
  coordination phases (add it to the session-start ritual next to `git log --oneline -10`), and
  keep the manual fallback in the ritual regardless: `ls -lat inbox/ | head`. For durable,
  session-independent watching, use a cron/launchd job or a repo hook — but weigh that against
  Principle 4 (automation tries, doesn't require): a coordination *phase* rarely justifies
  permanent infrastructure.
- **Silence is ambiguous.** A quiet watcher means "no deposits" only while the watcher is alive.
  If the monitor facility reports the watch died (timeout, session restart), re-arm before trusting
  the silence.
- **Watches arrivals, not departures.** Your outbound memos sitting unanswered in a peer's inbox
  are not covered — that's the peer's watcher's job. (Symmetry is the point: each repo watches its
  own inbox.)

## Variations

- **Multi-inbox:** prefix the emitted line per directory and run one loop over several inboxes
  (`for d in $DIRS; ...`) when a session coordinates more than one repo pair.
- **First-line preview:** append `&& head -1 "inbox/$new"` to the emit for a one-line memo preview
  in the notification — useful when triaging which deposit to read first.
- **Tighter cadence:** drop to 5–10s only when a peer session is known to be mid-exchange with you
  (live back-and-forth); return to 30s after.

## Relationship to other practices

- **federation.md** — this is a Principle-4-compliant convenience: the discipline (inbox memos)
  works with zero automation; the watcher only removes read latency. Nothing may *require* it.
- **federation-outbox.md** — the outbox flush dispatches notifications to consumers; this practice
  is the receiving side's ear. Together they close the loop without any shared infrastructure.
- **session-lifecycle.md** — arm during session start for coordination phases; it dies at session
  end by design.
