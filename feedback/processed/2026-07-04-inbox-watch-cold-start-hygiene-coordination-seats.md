# Feedback: Coordination Seats Should Arm a Live Inbox Watch at Cold Start — Convention Today, Hygiene Rule Tomorrow

**Date:** 2026-07-04
**Source:** tak-tdf multi-repo coordination session (side channel), relayed by Will to the rebar main line for durable filing. The coordinating session cannot file rebar feedback itself.
**Type:** missing-hygiene-rule / convention-not-enforcement
**Status:** implemented — v3.0.0-beta (2026-07-04); disposition in feedback/INVENTORY.md
**Template impact:** cold-start ritual sections in `QUICKCONTEXT`/`CLAUDE.template.md` for coordination-seat repos; probable interaction with `feedback/2026-04-26-sessionstart-hook-cold-start-enforcement.md` (settings.template.json hook block); possibly a new documented convention for the `inbox/` peer-mail pattern, which rebar does not codify anywhere today.
**From:** tak-tdf coordinator seat (Claude Code), 2026-07-04, during active three-way cross-repo development with go-tak-server and TDFLite-tak.

## What Happened

The tak cluster (tak-tdf / go-tak-server / TDFLite-tak, plus TDFLite-main, p2p-abac, TDFBot) runs an
append-only `inbox/` peer-mail convention: repos file dated memos into each other's `inbox/` dirs for
asynchronous cross-repo coordination (spec approvals, amendment ratifications, experiment results).
Traffic is real and dense — ~14 memos in the last 36 hours in tak-tdf's inbox alone.

During today's session the coordinator seat armed a **live inbox watch**: a persistent Monitor task
(survives until session end or an explicit stop) on a 30s poll that snapshots the three inbox dirs,
diffs against the previous snapshot, and emits one `NEW INBOX MEMO: <path>` event per new file into
the session. Verbatim from the coordinating session:

> Right now the monitor dies with the session and re-arming it is a line in QUICKCONTEXT's cold-start
> ritual — which means it's convention, not enforcement, and a session that skips the ritual goes deaf
> to peer traffic.

Known caveat, acceptable under the convention: the watch sees only new filenames, not edits to
existing memos — fine because the inbox convention is append-only.

## What Was Expected

A coordination seat in active cross-repo development should not be able to silently go deaf to peer
mail. If the seat's job includes reacting to inbound memos, arming the watch should be part of the
seat's enforced cold-start protocol — not a prose line an agent can narrow its way past. This is the
same disease documented in `2026-04-26-sessionstart-hook-cold-start-enforcement.md`: an instruction
that *describes* a check is not the same as a system that *runs* one.

## Suggestion

Bake "coordinator seats arm an inbox watch at cold start" into rebar hygiene, with two scoping
decisions made deliberately:

1. **Scope to multi-repo coordination seats, not every rebar project.** The reconciliation question
   against the standing ASK paradigm is real: `ask <repo> <role>` is pull-based, synchronous, and
   stateless; the inbox is push-based peer mail. They are complementary — asks for questions, inboxes
   for filings — so the hygiene rule belongs to seats that hold an inbox, not to the general profile.
2. **Keep it a Monitor-style event-driven watch, not another polling ask.** The value is events
   arriving *into* the session while other work proceeds. Folding it into the ask layer would change
   its character (and its failure mode) for no gain.

Concrete shape, in leverage order:

- **Codify the `inbox/` convention itself.** Rebar has no documented peer-mail convention despite six
  adopter repos running one (append-only, dated `YYYY-MM-DD-<from>-<slug>.md` memos, processed-on-read).
  A conventions entry or practices doc is the prerequisite for any hygiene rule referencing it.
- **Add the watch to the coordination seat's cold-start protocol** — a line in the seat's QUICKCONTEXT
  ritual at minimum, and per the SessionStart-hook feedback, preferably a hook-enforced or
  hook-reminded step so a session that skips the ritual gets told rather than going deaf.
- **Pair it with the cold-start sweep, don't replace it.** Today's session pattern was: cold-start
  sweep establishes "all traffic through HH:MM processed," then the live watch covers the session
  forward. Both halves are needed; the sweep bounds what the watch missed before it existed.

## Why It Matters

During active cross-repo development, inter-repo latency is the coordination cost. Today's session
had three repos ratifying an amendment (A1) and closing an empirical finding (sealed-shed) within a
single morning — turnaround measured in minutes because memos landed in-session as events. A seat
that cold-starts without the watch reverts to sweep-only: peer mail sits unseen until someone thinks
to re-sweep, and the other two repos block on a reply that isn't coming. The live-watch-vs-sweep-only
difference is exactly the difference the coordinating session observed: "live monitoring beats
sweep-only during active cross-repo development matches how today's session actually behaves."

## Adjacent / Related Work

- `feedback/2026-04-26-sessionstart-hook-cold-start-enforcement.md` — same family: convention →
  enforcement via SessionStart hook. This item is a coordination-seat-specific instance.
- The "reflexive-push" rule (filed upstream from a peer session; not yet codified on rebar main) —
  same spirit: making a coordination behavior structural rather than remembered.
- `feedback/2026-04-28-cross-repo-contract-federation.md` and CHARTER §1.6/§2.10 — the federation
  layer the inbox convention grew alongside; a codified inbox convention should state its relationship
  to the outbox tooling shipped there.
- `feedback/2026-06-19-trustable-status-and-cross-repo-ask-to-cut-rederivation-loe.md` §2 — the
  ask-vs-inbox complementarity noted here bears on that item's "make ask answer capability questions"
  push: neither channel substitutes for the other.

---

## Disposition (maintainer-filled)

- [x] **Accepted → implemented** — v3.0.0-beta, Cluster 6 (peer-inbox paradigm)
- [ ] Watchlisted
- [ ] Rejected (reason: ___)
- [ ] Redirected (filed elsewhere: ___)

**Triaged:** 2026-07-04 by the rebar main-line session (Claude), under Will's
authorization for the beta scope ("execute with autonomy, document decisions").

**Notes:**
- Shipped in Cluster 6: `scripts/inbox-watch.sh` (executable extraction of the
  watch loop — multi-inbox, `-i/--interval`, `--preview`, zero-dep bash 3.2);
  `practices/inbox-watch.md` now points at the script as the canonical
  implementation; `practices/session-lifecycle.md` Session Start gains step 4
  (coordination-seat hygiene: sweep held inboxes, then arm the watch; manual
  `ls -lat inbox/ | head` fallback retained).
- The prerequisite this item names — codifying the `inbox/` peer-mail
  convention itself — lands as a `conventions.md` entry (integration-owned),
  explicitly disambiguated from the ASK runtime's `agents/<role>/inbox/` queues.
- Both scoping suggestions adopted: hygiene rule scoped to coordination seats
  (not the general profile), and the watch stays a Monitor-style event stream
  (not folded into the ask layer).
- Hook-enforced arming (the SessionStart-hook pairing this item anticipates)
  is deliberately not wired in this release: the hook ships in Cluster 2 as a
  visible-drift reporter, and per plan D8 rebar itself holds no `inbox/`, so
  there is no seat here to enforce on. Adopter seats get the ritual (practice +
  skill nudge); enforcement can follow on real-world failure, per doctrine.
