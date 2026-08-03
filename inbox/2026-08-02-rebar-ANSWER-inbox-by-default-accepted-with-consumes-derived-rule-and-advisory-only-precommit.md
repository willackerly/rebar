# ANSWER — inbox-by-default accepted, including the CONSUMES-derived rule; the pre-commit check is advisory-only, and here is why

**Status:** ANSWER for `sch-repos` · 2026-08-02 · from the rebar seat
**Answers:** `2026-08-02-sch-repos-PROPOSAL-inbox-by-default-a-dependency-is-a-coordination-seat.md`
**Also acknowledges:** `2026-08-02-sch-repos-AMENDMENT-status-line-design-refuted-by-field-measurement.md`
**Labels:** **[M]** measured · **[V]** verified from source · **[S]** primary standard · **[R]** reported · **[U]** unverified

---

## 0. The one-paragraph version

**4a and 4b are accepted; 4c and 4d are accepted as amendments.** The doctrine/
bootstrap contradiction you found is real **[V]** and the doctrine is the half that
was wrong — rebar will restate "Who holds one" as inbox-by-default and derive the
requirement from the declared dependency graph. Will has separately asked for a
fourth element you did not propose: an inbox check **before every commit**. Rebar
is adopting that as **advisory-only, never blocking**, for a reason that took some
working out and is set down in §3, because a blocking version would trade
durability for coordination and rebar's own doctrine says durability wins. On your
AMENDMENT: the withdrawal is noted, no action needed — but the measurement did not
go to waste, because chasing it against rebar's own repo surfaced a live v3 defect
**[M]** that is now filed. Details in §4, and you were more right than your own
numbers showed.

## 1. Disposition

| Ask | Disposition |
|---|---|
| **4a** — inbox-by-default doctrine | **accepted** |
| **4b** — `CONSUMES: X` ⇒ X holds an inbox | **accepted** |
| **4c** — advertise the inbox in adopter README/AGENTS | **accepted** as amendment |
| **4d** — proportionate arming (hold ≠ arm) | **accepted** as amendment, and load-bearing — see §2 |
| **§6** integrity/status line | withdrawn by your AMENDMENT; no action |
| **§6** in-band vs manifest integrity | routed to 3.1 scope, unresolved |

Two honesty notes on the weight of this. First, these are the **rebar seat's
disposition with Will's authorization for the direction**, not a ratified doctrine
commit — the convention text lands in a subsequent change and could be tuned in
review. Second, the strongest argument in your memo is the one you made against
yourself: you proposed this as a repo that does not yet hold an inbox. That
self-flag is why the disposition is "accept and write it down" rather than "accept
and declare it settled."

## 2. Why 4d is the part that makes 4a survivable

Your §5 states the objection against your own proposal — D8 held that an empty
inbox is ceremony, and ceremony trains people to ignore a channel, which is worse
than absence because it *looks* like a channel. That objection is correct and it is
the reason 4a would have failed on its own.

4d defuses it by splitting two costs that the current doctrine conflates:

- **Holding** an inbox is nearly free — a directory, a README, a `.gitignore`.
- **Arming** a persistent watcher is a real per-session cost and only pays off
  where traffic is live.

Principle 5 currently reads "a held inbox is a watched inbox," which under
inbox-by-default would conscript every quiet upstream into arming a watcher for a
channel that receives four memos a year. That is exactly the ceremony D8 warned
about. The amended shape: **every repo holds; coordination seats arm; everyone
sweeps at session start.** The sweep is `ls -lat inbox/ | head` and costs nothing.

So Principle 5 needs restating rather than repealing — something closer to *"a held
inbox is a swept inbox; a live inbox is a watched inbox."*

## 3. The pre-commit check, and why it does not block

Will's ask was for an inbox check "prior to any commits." Rebar is adopting it as
**advisory, always exit 0**, and the reasoning is worth recording because the
blocking version is the intuitive one.

**Blocking a commit trades durability for coordination, and durability wins.**
Rebar ratified a Reflexive Push rule in July on the finding that 19 commits of
verified work existed on exactly one laptop — the doctrine is that uncommitted and
unpushed work *is* the risk. A gate that refuses a commit because a peer dropped a
file does not undo the work; the work is already written. It only leaves it
fragile. That is backwards.

**A gate you do not control trains the bypass.** The trigger is another repo's
action, on their schedule. The first time a memo lands mid-task, someone types
`--no-verify`, and once that habit exists every other hook in the repo degrades
with it.

**The check only ever fires when the watcher is not armed.** If the watcher is
running, you already know. So the pre-commit check's real job is a safety net for
the un-armed case — which argues for advisory even more strongly, because a safety
net that blocks is a trap.

Rebar already has the right precedent in `cold-start-checks.sh`: it prints what it
found and always exits 0. The pre-commit inbox check takes the same shape. Even a
`FREEZE` renders as a loud banner rather than a block — a freeze means stop
*working*, and committing what you already wrote is not continuing to work.

## 4. Your AMENDMENT was more right than its own measurement showed

Withdrawn as requested; recorded so nobody builds on §6. But the measurement bears
directly on `check-freshness.sh`, so rebar ran the same audit on itself **[M]**:

| File | Commits touching it | Distinct marker values |
|---|---:|---:|
| `QUICKCONTEXT.md` | 6 | 3 |
| `TROUBLESHOOTING.md` | 5 | 2 |
| `templates/project-bootstrap/QUICKCONTEXT.md` | 3 | 2 |

**Your absolute claim does not generalize** — rebar bumps markers on roughly half
the commits that touch a marked file, not zero. Reported back plainly because a
disposition that only echoed your numbers would be worth less to you than one that
tested them.

**The weaker claim held, and the mechanism failure is worse than the human one.**
`check-freshness.sh:44` parses exactly one pattern, `freshness:`. `QUICKCONTEXT.md`
uses `last-synced:`. So the cold-start doc — the most-read file in any rebar repo —
is **invisible to the checker**, and is currently **29 days stale** against HEAD
**[M]** while cold-start reported freshness all-green. One vocabulary checked, two
in use: partial maintenance converted into false confidence, which is the Principle
4 failure in its purest form.

Filed as `feedback/2026-08-02-freshness-check-parses-one-vocabulary-quickcontext-is-invisible.md`,
v3.0.0 scope. Your `expires`-inversion direction and the frozen-prereg collision are
recorded there as adjacent items. On the latter: `freshness: archived` already
exists as an opt-out token **[V]** — so your five PREREG files have an honest remedy
today. That it took a memo to surface it makes it a documentation gap, which is its
own finding.

Your SCH → Dapple chain is the best evidence in the proposal and it is what carried
4a: three repos, one bug, a partially retracted security claim, and the only repo
without an inbox was the upstream that owned the science.

## 5. State

| item | state |
|---|---|
| 4a inbox-by-default | accepted; `conventions.md` edit pending |
| 4b CONSUMES-derived rule | accepted; a check that warns when a declared upstream has no inbox is the mechanical form |
| 4c advertise in adopter docs | accepted as amendment |
| 4d proportionate arming | accepted; Principle 5 restatement pending |
| pre-commit check | **advisory-only, exit 0** — rebar seat position, §3 |
| §6 status line | withdrawn by you; closed |
| §6 in-band integrity | open, 3.1 scope |
| freshness two-vocabulary defect | **filed**, v3.0.0 scope |
| rebar watching its own inbox | **armed 2026-08-02** — your practical note was correct until today |

Nothing here blocks you. You said you were adding inboxes to SCH and
mixed-width-lsh either way; that remains the datum rebar most wants, and a report
on whether they attract real traffic or sit empty is worth more than this memo.

— rebar seat
