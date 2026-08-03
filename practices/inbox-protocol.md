# Inbox Protocol — the memo grammar that makes async coordination work

> Practice status: **proposed**, with field evidence. Distilled from the
> TDFLite ↔ nanotdf2 exchange of 2026-08-02/03, in which two seats negotiated the
> ownership handover of a wire format — nine memos, two days, zero synchronous
> meetings, three format changes and one retired doctrine — entirely through
> `inbox/` deposits.
> **Companion practices:** `inbox-watch.md` (how you learn a memo arrived),
> `federation.md` (the five principles), `conventions.md` → Peer-Inbox Convention
> (who holds an inbox at all).

## What this document is for

The neighbouring docs cover notification, principles, and eligibility. **None of
them says what goes inside a memo.** That gap is where the value actually lives:
the discipline works because the memos have a grammar, not because the directory
exists.

The reader of your memo is frequently **not the agent who will act on it** — it
may be a fresh session after a context clear, on a different machine, weeks later.
Write for that reader.

---

## 1. The naming grammar

```
YYYY-MM-DD-<sender>-<TYPE>-<kebab-case-description>.md
```

`<sender>` is the **originating seat, never the recipient** — the directory
already says who received it.

**Descriptions are long and load-bearing on purpose.** `ls inbox/` must read as a
changelog of the conversation: you should be able to reconstruct the thread
without opening a file. This is not stylistic. In the field case, a cold-starting
agent recovered a two-day negotiation — offer, withdrawal, correction, resolution
— from filenames alone, and knew which memos it could skip.

Good: `2026-08-03-nanotdf2-CORRECTION-our-closure-number-was-wrong-by-2x-here-is-the-real-inventory.md`
Bad: `2026-08-03-nanotdf2-update.md`

**The date is the sender's LOCAL deposit date — never UTC.** This looks like
pedantry and is not: agents routinely read timestamps from tooling that reports
UTC (`gh` API `created_at`, CI run times), and for a machine west of Greenwich
those cross midnight hours before the local date changes. The first field case
produced exactly this — memos named `2026-08-03-…` deposited on the evening of
the 2nd, spotted by the receiving seat. The changelog property in the paragraph
above depends on filename dates sorting as **arrival order**; a UTC date silently
breaks that for part of every day.

**Do not rename a memo after depositing it** to correct the date. Filenames are
cited — in decision logs, in replies' `Answers:` headers, in commit messages —
and a rename breaks those references to fix a cosmetic error. Get it right on the
way out; live with it afterwards.

## 2. The type vocabulary

The type tells the reader what they are obliged to do.

| type | means | obliges |
|---|---|---|
| `ASK` | we need something from you | a reply; usually ends in numbered questions |
| `ANSWER` | a reply to a specific ASK, citing it by filename | nothing further |
| `FINDING` | something we discovered that changes your picture | you to update your model |
| `CORRECTION` | something we previously told you was **wrong** | you to un-believe a specific claim |
| `RECEIPT` | a thing landed and is green | nothing — it closes a loop |
| `ACK` | acknowledged, no action needed | nothing |
| `FYI` | context you may want | nothing |
| `DIRECTION` | a ruling from the human principal | compliance, not debate |
| `HANDOFF` | ownership or custody is moving | the named preconditions |
| `FREEZE` | stop work; a takeover or migration is imminent | an explicit *confirmed frozen* reply |

`CORRECTION` earning its own type is deliberate: see §4.

## 3. The shape

Every memo opens with a claim and a header block:

```markdown
# <Title — a claim, not a topic>

**Status:** <TYPE> for <recipient> · YYYY-MM-DD · from the <seat> seat
**Answers:** <filename it replies to, if any>
**Labels:** [M] measured · [V] verified from source · [S] primary standard ·
            [R] reported · [U] unverified
```

Then **§0 — the one-paragraph version**, which must be sufficient on its own. A
busy peer reads §0 and the closing state table; everything between is for the
implementer.

Close with a **state table** — the current disposition of every open item. This
is what makes crossed memos survivable (§5).

**Titles are claims.** "CBOR is net-new here, measured at +0.418 MB" tells the
reader whether to keep reading. "Notes on dependencies" does not.

## 4. Confidence labels — the load-bearing invention

Every non-obvious claim carries one:

- **[M]** measured in-tree — you ran it and read the number
- **[V]** verified from source — you read the code or the API response
- **[S]** from a primary standard — RFC, NIST publication, vendor spec
- **[R]** reported — a human or peer told you
- **[U]** unverified — you believe it, you have not checked it

The obvious benefit is that a reader can weight your claims. **The larger benefit
is that labels make self-correction cheap and socially free**, and that is what
actually moved the field case:

- A seat labelled a dependency-size figure `[M]`, re-read its own method, found it
  had measured a two-symbol program the linker had mostly eliminated, and sent a
  `CORRECTION` — a **2.3× error**, caught by the author, before anyone built on it.
- The same seat had called a repo-visibility flag an access blocker and put it at
  the top of a handoff. Re-checking who owned the repo retracted it.

Both errors had the same shape: **checking the easy signal instead of the real
one.** The label is what makes that shape visible, because writing `[M]` next to a
number you did not actually measure is an uncomfortable act.

**Corollary — never revise a memo in place.** A memo is an append-only artifact
once deposited. If it was wrong, send a `CORRECTION` that cites the filename and
names the specific claim being withdrawn. Silent edits destroy the provenance the
inbox exists to create.

## 5. The norms that did the real work

**Answer the question you were asked, *then* correct the premise.** Reversing this
order reads as evasion, and the asker often needs their original answer anyway.

**Never write into a peer's tree without a heads-up** — including creating a
directory. In the field case one seat needed to reply to a peer with no `inbox/`;
it asked rather than creating one, and the peer adopted the rule explicitly on the
strength of the restraint.

**Crossed memos are normal — design for them.** Async means both seats write
simultaneously. Twice in two days, an `ANSWER` crossed the memo it was answering
and materially changed it. The mitigations are the closing state table, citing the
filename you are answering, and treating "our memos crossed" as ordinary rather
than as a failure.

**Argue against your own offer when you come to think it is wrong.** The field
case's strongest single memo was a seat withdrawing its own proposal and telling
the counterparty *"do not accept out of politeness, we now think it is the worse
design."* Doctrine: a proposal is not a position to defend.

**Correct the model, not just the fact.** When a peer plans around an assumption
about *your* environment that is not true, say so explicitly. One seat assumed a
red CI check would appear in the other's repo automatically; correcting the
assumption was worth more than answering the question that rested on it.

**Bank both sides in git.** Commit inbound memos *and* copies of your outbound
ones. The inbox is the provenance record of how a decision was reached, and half a
conversation cannot carry that.

## 6. When a memo beats a synchronous ask — and when it does not

Agent harnesses offer an interactive "ask the human" affordance. It is not a
substitute, and neither is a substitute for the other.

| | inbox memo | synchronous ask |
|---|---|---|
| durability | survives context clears, compaction, machine moves | gone after the turn |
| audience | any future agent, any seat, third-party reviewers | the one human present |
| provenance | greppable artifact, citable by filename | none |
| blocking | nobody waits | someone waits |
| confidence discipline | labels are structural | none |
| latency | hours | seconds |

**The rule: a decision that changes what someone builds gets a memo. A single
blocking choice from a human who is present gets an ask.** A memo can contain an
ask; an ask cannot contain provenance.

The failure mode worth naming: resolving a design question in a synchronous
exchange and never writing it down. Six weeks later the decision is real, load
bearing, and unattributable — and the next agent relitigates it.

## 7. Anti-patterns

- ❌ **Memo as chat** — five short deposits where one structured memo belongs.
- ❌ **Topic titles** — "re: the KAS thing".
- ❌ **Unlabelled claims** — a number with no `[M]`/`[U]` is a rumour.
- ❌ **Silent revision** — editing a deposited memo instead of sending a `CORRECTION`.
- ❌ **Peer-inbox surveillance** — watch your own inbox only (`inbox-watch.md`);
  watching a peer's echoes your own outbound deposits back at you.
- ❌ **A `FREEZE` without a confirmation handshake** — a takeover that races a
  peer's working tree destroys context nobody can reconstruct. Require an explicit
  *confirmed frozen* reply, and check `git status` on the other side first.

## 8. Adoption checklist

1. `inbox/` exists and is committed (see the Peer-Inbox Convention).
2. `inbox/README.md` states the naming grammar, the type table, and the labels —
   so a memo moves between repos unchanged.
3. **The protocol is surfaced at the repo ROOT**, not only here. See §9.
4. The watcher is armed at session start (`inbox-watch.md`).
5. Outbound copies are banked alongside inbound.
6. The first memo of any thread carries §0 and a state table.

## 9. Keep it at the root — cold-start reads root, not `practices/`

**Recommended convention:** every repo holding an inbox carries a short root-level
`INBOX.md` — the naming grammar, the type table, the labels, and who the peers
are — pointing here for the doctrine.

The argument is about *when* the doc is read. A practice guide is read when
someone goes looking for it; a cold-starting agent reads what is at the root and
what the root docs point to. Coordination is not a specialised topic you consult
occasionally — for a repo with peers it is load-bearing from the first minute of a
session, on the same tier as the build command and the test command. Filing it
only under `practices/` puts it a level below where it is actually needed, and the
cost of getting it wrong is asymmetric: a memo written in the wrong shape, or an
inbox that goes unwatched for a day, is a coordination failure the other seat
absorbs.

Keep the root copy **short and local** — conventions plus the peer map, not the
reasoning. The reasoning lives here and stays portable across adopters; the root
file tells this repo's next agent what to do in ninety seconds.
