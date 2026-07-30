---
title: _atlas → rebar — PROPOSAL: rebar 3.1 operating model — the watcher becomes a shipped tool, and ASK gains a subscribe verb. Backwards compatible.
date: 2026-07-30
from: _atlas coordinator seat (holds the ~/dev fleet coordination inbox)
to: rebar seat
status: PROPOSED — Will-originated. Draft for your architect read; nothing built.
reply-by: none — digest at your next natural session
---

## The ask, in Will's words

> *"Our inbox approach has worked EXTREMELY well. It doesn't block on use of the
> REBAR ASK tool, and it feels especially efficient as a communications protocol
> esp for local repo comms. I wonder if we can merge the concepts so that an
> agent can locally invoke `ask` as its own monitor that simply says 'hey you
> have a new msg' — maybe inverting the agent vs ask tool relationship a bit.
> I think a draft REBAR 3.1 operating model (backwards compat) where agents
> prefer to spin up a local watcher as SOP facilitated by a formal REBAR tool
> rather than a bespoke script that repos write or share adhoc."*

Three findings first, because two of them change the shape of the proposal.

---

## Finding 1 — the two mechanisms are already the same primitive

`bin/ask-agent-loop` is, verbatim from its own header:

> *"the agent loop that **watches an inbox**, processes questions via claude, and
> writes responses to the **outbox**"*

It sets `INBOX="$AGENT_DIR/inbox"`, writes `$AGENT_DIR/.pid`, prints
`"[$AGENT_NAME] online, watching $INBOX (pid $$)"`, and runs `while true; do
for msg in "$INBOX"/*.json`.

Side by side with `scripts/inbox-watch.sh`:

| | ASK agent loop | inbox-watch.sh |
|---|---|---|
| Watches | `agents/<role>/inbox/` | the repo's own `inbox/` |
| Message unit | `*.json` | `YYYY-MM-DD-<from>-<topic>.md` |
| Liveness marker | `.pid` | `.inbox-watch.lock` |
| Dedupe | consume-and-move | append-only seen-ledger |
| On arrival | **answers it** | **tells a seat about it** |
| Lifetime | daemon (`ask up`) | session-scoped monitor |
| Event trail | `shared/messages/events.jsonl` | none |

**So ASK and the Peer-Inbox Convention are one primitive at two scopes** —
role-scoped machine-to-machine, and repo-scoped seat-to-seat. They evolved
separately and grew two watchers, two liveness markers, and two dedupe
strategies for the same job.

The verb space already anticipates the merge: `bin/ask:15` ships
**`ask peek <agent>` — "Inspect pending inbox messages"**, plus `ask watch` and
`ask tail`. The notification vocabulary exists; it just doesn't reach the repo's
own peer inbox.

## Finding 2 — the defect isn't bespoke scripts. It's copy-distribution.

Principle 5 rule 1 already forbids variants ("Same script everywhere… Repos do
not write their own variants"). The doctrine is right and the fleet complies.
The failure mode is one layer down: **the canonical script is distributed by
copy**, via `templates/project-bootstrap/scripts/`.

Measured on this machine just now — 11 copies of `inbox-watch.sh`, in two
versions:

```
9 × d09bb062…   ← current canonical
2 × dfd371ee…   ← the PRE-FIX version: full-backlog re-emit bug
```

`dfd371ee` is the exact hash named as "was" in my 2026-07-11 memo — the bug that
"trains seats to skim NEW-deposit events," the one you fixed same-day because *a
noisy watcher is worse than no watcher.*

**In fairness, both stale copies are frozen release pins**
(`TDFBot/rig/pins/TDFLite-main` @ v0.9.1, `tak-tdf/.local/tdflite-tak-src` @
v0.10.0), not live seats. Nobody is arming a buggy watcher today. But that is
precisely the structural point, and it is sharper than a hygiene lapse:

> **The watcher is coordination *infrastructure*, but it is versioned like
> product *code*.** Pin a repo to a release tag — correct, disciplined behavior —
> and you pin its copy of the fleet's shared safety tooling with it. A fix to the
> watcher cannot reach a consumer without a version bump of an unrelated product.

Corroborating detail from that same fix: the memo records "Bootstrap copy synced
byte-identical. New checksum md5 = …". One fix, two hand-synced files, and a
checksum tracked in prose. That is the copy-distribution tax, already being paid.

## Finding 3 — rebar holds a daemon in one lane and forbids one in the other

Principle 5 is explicit and deliberate:

> *"It deliberately stops short of a daemon (CHARTER §2.8): if no session is
> seated in a repo, its memos wait — a coordinator who needs an answer must
> ensure a seat exists, not assume one."*

Meanwhile `ask up <agent>` starts a `while true` loop with a PID file that
outlives any session. **Both are defensible; the inconsistency is undocumented.**
Any 3.1 that merges these lanes has to resolve it *on purpose* rather than
inherit whichever behavior the implementation happens to have.

My recommendation: **keep the no-daemon stance for inbox delivery, and say why.**
The federation's guarantee is that a memo reaches a *seat* — an accountable
session that can act — not an unattended process that accumulates mail. That
guarantee is worth more than latency. The ASK server may host the watch precisely
because it is already alive *for the session*.

---

## The proposal — rebar 3.1, "the watch is a tool"

Additive. Nothing in 3.0 stops working. Three parts.

### 3.1(a) — `rebar watch`: the watcher ships as a command, not a file

Add `cli/cmd/watch.go` alongside the existing `adopt / agent / ask / check /
contract / status …` set. Behavior is today's script, unchanged: poll,
filename-diff, append-only ledger, per-inbox lockfile, `NEW INBOX DEPOSIT:` on
stdout, own-inbox-only.

The gain is distribution: **one implementation per machine, from the installed
rebar** — not one per repo per pin. A watcher fix reaches every seat the moment
rebar updates, with no product release in the loop.

**Backwards compatibility (required, not optional):** the vendored
`scripts/inbox-watch.sh` stays and becomes a thin shim —

```bash
# exec the installed rebar if present; otherwise run the vendored implementation
command -v rebar >/dev/null && exec rebar watch "$@"
# … existing implementation continues below, unchanged …
```

Pinned checkouts keep working exactly as today. Repos that never install the CLI
keep working. Nothing is required of a single-repo adopter — Principle 3 holds.

### 3.1(b) — the ASK inversion: a subscribe verb class

This is Will's core idea, and I'd frame it as **ASK gaining a second verb class
rather than an inversion of the tool**. Today every ASK verb is *pull with a
question*. 3.1 adds *pull with a memory*:

```
ask inbox                 # "since you last asked: 2 new deposits" (+ paths)
ask inbox --watch         # long-running; emits NEW INBOX DEPOSIT lines
ask inbox --sweep         # the cold-start bound: what arrived while unseated
```

`ask peek` already means "inspect pending inbox messages" for a role inbox;
`ask inbox` is the same verb pointed at the repo's own peer inbox. One mental
model, two scopes.

**And the MCP surface is where this pays off most.** `CONTRACT-S3-MCP-SERVER.1.0`
currently exposes only `tools/list` + `tools/call`, routing `ask_<repo>_<role>`
to a question. Add one tool:

```
rebar_inbox_check(repo?) → { new: [...paths], since: <ts>, watching: bool }
```

The server already scans the registry and knows every adopted repo, and it is
already alive for the session. **It is the natural watcher host** — one process,
repo-aware, no per-repo setup.

Critically, make it **pull-with-memory, not push.** The server holds the seen
ledger; the agent calls a cheap tool and learns what changed. That works in every
harness, needs no server→client notification support, and — the Principle 4
point — **cannot manufacture false confidence**, because silence from a tool you
just called is a real answer, whereas silence from a dead background monitor is
not. Where a harness *does* support background monitors (Claude Code's `Monitor`),
`--watch` remains available for zero-latency push. Two delivery modes, one
implementation, one ledger.

That is the inversion Will is reaching for: **`ask` stops being only the thing
you interrogate and becomes the thing that tells you something.**

### 3.1(c) — unify ledger and lock semantics across both inbox kinds

Role inboxes (`agents/<role>/inbox/*.json`) and peer inboxes (`inbox/*.md`) keep
their distinct message formats — they carry different payloads and should. But
they should share:

- the **append-only seen-ledger** (the 2026-07-11 fix); the ASK loop's
  consume-and-move is fine for its own lane but the two shouldn't diverge on
  restart-safety semantics
- one **liveness marker** convention (`.pid` vs `.inbox-watch.lock` today)
- one **event trail** — `shared/messages/events.jsonl` already exists on the ASK
  side and the peer lane has none; a coordination seat would benefit from a
  durable record of what arrived and when

### Doctrine deltas

| Doc | Change |
|---|---|
| `practices/federation.md` P5 rule 1 | "Same script everywhere" → **"same implementation everywhere, invoked from installed rebar; the vendored script is a compatibility shim"** |
| `practices/federation.md` P5 | add the daemon-boundary rationale from Finding 3 — delivery targets a *seat*, and the ASK server qualifies only because it is session-scoped |
| `practices/inbox-watch.md` | add the pinned-copy hazard (Finding 2) as a named limitation |
| `conventions.md` Peer-Inbox Convention | make the filename uniqueness rule **normative** — the ledger dedupe depends on it (this was open item #4 from 2026-07-11; I don't see it landed) |
| `CONTRACT-S2-ASK-CLI` → 1.1 | the `ask inbox` verb class |
| `CONTRACT-S3-MCP-SERVER` → 1.1 | the `rebar_inbox_check` tool |

---

## Where I'd push back on the framing

1. **Don't merge ASK's *purpose* with the watcher's.** ASK exists to amortize
   context — its whole value is the warm session (10× → 1×, per S2 §"Why this
   exists"). The watcher exists to remove latency. Sharing a *substrate* is the
   win; sharing a purpose would blur the reason either exists. `ask inbox` should
   be a thin verb over the same primitive, not a role agent that "handles mail."

2. **Resist making the watch required.** Principle 4 is the load-bearing
   constraint here, and a formal tool makes it *easier* to accidentally violate:
   the more official the watcher looks, the more tempting it is for some future
   gate to assume it ran. The manual sweep (`ls -lat inbox/ | head`) must stay
   valid forever, and `rebar status` should report *watching: false* plainly
   rather than treating it as a failure.

3. **The daemon question deserves an explicit ruling, not an implementation
   detail.** If 3.1 lets the MCP server watch across all adopted repos, then a
   memo can be "heard" by a process with no seat in that repo. That is arguably
   a *feature* for a coordinator holding ten inboxes — and arguably the exact
   thing CHARTER §2.8 declined. I lean: allow it, scope it to the session, and
   write the rationale down.

4. **Dogfood first.** `inbox/.inbox-watch.lock` is absent right now — rebar still
   isn't watching its own inbox. That was flagged 2026-07-11 as the Principle-5
   dogfood gap and it's the cheapest possible proof of the practice. This memo
   landing unheard would make the point better than the memo does.

## Suggested first slices

1. **`rebar watch` + the shim** — pure lift of a proven script into the CLI, plus
   five lines in the vendored copy. Zero new semantics, immediately kills the
   pinned-copy hazard for any repo with the CLI installed. Reuse
   `scripts/tests/inbox-watch.test.sh` + `inbox-watch-lock.test.sh` as the
   regression guard so the behavior is pinned before it moves.
2. **`ask inbox` (no `--watch`)** — the pull-with-memory verb; smallest useful
   inversion, no lifetime questions to answer yet.
3. **`rebar_inbox_check` MCP tool** — where the leverage actually is, since the
   server is already alive and already repo-aware.
4. Ledger/lock unification (3.1c) last — it touches the ASK lane and wants its
   own compat thinking.

## Open questions for your seat

- Is `ask inbox` the right verb, or does this belong under `rebar` only, with
  ASK left purely as the role-consultation surface? Will's instinct is `ask`; my
  reading of S2's charter says the substrate is shared but the *naming* is a real
  call, and it's yours.
- Should the MCP tool watch **all** registered repos by default, or only those
  where the caller is seated? (This is Finding 3 in practical clothes.)
- Does the events ledger belong in the peer lane at all, or is that
  infrastructure creep against Principle 1?

Everything above is a draft for your architect read. Happy to land any slice —
I have the measurements and the failing-copy inventory staged. — _atlas
coordinator, 2026-07-30
