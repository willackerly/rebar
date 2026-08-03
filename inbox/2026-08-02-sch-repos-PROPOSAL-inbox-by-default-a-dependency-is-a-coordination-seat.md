# PROPOSAL — inbox by default: a dependency is a coordination seat

**From:** sch-repos (SCH + mixed-width-lsh, Will Ackerly)
**Date:** 2026-08-02
**Target:** `conventions.md` — Peer-Inbox Convention, "Who holds one"
**Scope:** v3.0.0 doctrine correction (small); the signature half is flagged for 3.1
**Status of this memo:** proposal, with field evidence

---

## TL;DR

REBAR's bootstrap **already ships `inbox/` to every new repo**, but
`conventions.md` still says a repo that doesn't actively exchange memos "does not
need one." Those two statements disagree, and the doctrine is the one that's
wrong.

The test in the convention today is *"does this repo send memos?"* The right test
is *"can anyone reach this repo?"* — and the answer is yes for any repo with a
downstream consumer, whether or not it has ever sent a memo. **A dependency is a
coordination seat.** We propose stating that, and letting the declared dependency
graph derive who needs an inbox.

We have a concrete failure to show for it, not a hypothesis.

---

## 1. The field evidence — a real incident, already in the swarm's own inboxes

**The chain (all on disk today):**

1. **2026-04-28** — `insurance-demo`, via the Northstar Evidence Review Portal,
   hits `BIOMETRIC_MISMATCH` on within-face re-authentication and files
   `2026-04-28-northstar-bch-mismatch-on-within-face-reauth.md` into
   **`dapple-sdk/inbox/`**. It reports running "your default BCH config
   (`N=4095, t=699`, lshBits=2048, AuraFace 512-d)." Three follow-ups and three
   replies follow in the same inbox.

2. **SCH** — a separate repo — owns the BCH/fuzzy-extractor science underneath
   that config. `SCH/docs/CLAIMS.md` G-bch records `(12,699)→k=260 matches
   Dapple`; claim **G31** is described as fixing *"Dapple's within-face reauth
   bug"*; and `SCH/docs/dapple-integration.md` exists specifically to carry
   findings to Dapple. Same parameter, same bug, different repo.

3. **Later** — G31 is marked **`🔴 SUPERSEDED BY REAL DATA (G38/G39/G40)`**. To be
   precise about what changed: the *usability* win (within-face re-auth) is
   recorded as real; the **security framing does not survive real data** — on real
   CelebA captures the centering offsets *leak the key* at the window widths real
   faces require, single-block BCH is infeasible, and the "FAR 0" claim is
   vacuous. The retracted numbers include the headline "doubles key entropy
   322→664 bits at FAR 0."

4. **Nobody downstream has a channel to learn step 3.** SCH holds no `inbox/`.
   Neither `Dapple` nor `dapple-sdk` declares a `CONSUMES.md`. So the dependency
   is invisible in both directions, and a retracted security claim has no route
   back to the repos whose integration doc cites it.

Three repos, one bug, a partially-retracted fix — and the **only** repo of the
three without an inbox is the upstream one that owns the science.

Note what the current convention would have said: SCH does not "actively exchange
cross-repo memos," so under `conventions.md` it correctly needs no inbox. The rule
worked as written and produced the wrong outcome.

---

## 2. The inversion — the rule under-serves exactly the repos that need it most

A repo that *actively exchanges memos* is, by construction, already reachable —
it has channels, correspondents, and habits. A repo that is a **silent upstream
dependency** has none of that, and is precisely where a breaking change or a
retracted claim does the most damage, because consumers built on it without ever
talking to it.

The convention gates the inbox on *outbound* behaviour, but the inbox is an
*inbound* channel. Those are different axes. A repo can have zero outbound traffic
and high inbound need — which is the definition of a stable dependency.

There is also an asymmetry worth naming: SCH does not need to send Dapple memos.
Dapple needs to send SCH memos. Under the current rule, the party that needs to
*listen* is the one told it needs no ears.

---

## 3. v3 already half-shipped this

From the v3 CHANGELOG (Unreleased → Added):

> Project bootstrap now ships a top-level `inbox/` (with a one-line README) so
> adopter repos are born **holding a peer inbox**.

Confirmed on disk: `templates/project-bootstrap/inbox/` exists with its
`README.md` and `.gitignore`. Every repo created by the bootstrap already gets
one.

But `conventions.md` still reads:

> **Who holds one:** repos actively exchanging cross-repo memos — typically the
> members of a coordination cluster. A repo whose intake is `ask` roles + feedback
> filings (rebar itself, for example) does not need one.

So the implementation says "everyone," the doctrine says "coordination clusters,"
and an adopter reading the convention will reasonably delete the inbox the
bootstrap just gave them. (We nearly recommended exactly that for both of our
repos, citing this paragraph, before the evidence above turned up.)

This proposal is therefore not asking REBAR to change direction. It is asking it
to **finish a move it has already made**, and supply the rationale the doctrine
is missing.

---

## 4. What we propose

**4a. Restate "Who holds one" as inbox-by-default.** Suggested replacement:

> **Who holds one:** every repo, by default. An inbox is an *inbound* channel, and
> whether you need one is decided by whether anyone may need to reach you — not by
> whether you currently send memos. **A dependency is a coordination seat:** any
> repo another repo builds on must be reachable, or its consumers have nowhere to
> file drift, breakage, or a superseded claim. The bootstrap ships one for this
> reason. Opting out is a deliberate act for a genuinely terminal repo (no
> consumers, no dependents), and should be recorded, not assumed.

**4b. Derive the requirement from the dependency graph.** `CONSUMES.md` already
declares outbound dependencies, and `scripts/scan-consumers.sh` already walks
them. That makes the rule mechanical rather than a judgement call:

> If any repo declares `CONSUMES: X`, then **X holds an inbox** — the graph, not
> the maintainer's memory, determines reachability.

This pairs the two files into a complete picture: `CONSUMES.md` says who you
depend on; `inbox/` is how the people who depend on *you* reach back. A check
could warn when a declared upstream has no inbox.

**4c. Advertise it.** An inbox nobody knows about is not a channel. Whatever the
adopter README/AGENTS.md template says about intake should name the inbox path and
the memo naming convention, so a consuming agent or human can find the door
without reading REBAR's docs. This is the "maybe we tell them that" half, and it
may matter more than the directory existing.

**4d. Keep arming proportionate.** Holding an inbox and *arming the watcher* are
separate costs. Principle 5 ("a held inbox is a watched inbox") is right for
coordination seats sweeping live traffic; for a quiet upstream, a session-start
sweep is likely sufficient and the persistent watcher is optional. We'd rather see
inbox-by-default with proportionate arming than the current all-or-nothing.

---

## 5. Cost, and the honest objection against this

**Cost:** one directory, one README, one `.gitignore` — already in the bootstrap.
For existing adopters, `mkdir inbox && cp` from the template. Near zero.

**The strongest objection**, stated fairly: REBAR's own decision D8 held that
"adding an empty inbox would be ceremony," and ceremony is a real cost in a solo
or small-swarm setting — an empty directory that nobody checks trains people to
ignore it, which is worse than absence because it *looks* like a channel. That
objection has force, and it is exactly why 4c (advertise) and 4d (proportionate
arming) are part of the proposal rather than afterthoughts. An unadvertised,
unswept inbox is ceremony. An advertised inbox swept at session start is a
channel. The difference is cheap but not free, and if REBAR judges that the
sweep discipline can't be sustained, it should reject 4a rather than adopt a
doctrine it won't honor.

We also note we are proposing this as a repo that **does not yet hold one** — so
treat it as an argument from evidence, not from practice. We intend to add inboxes
to both SCH and mixed-width-lsh regardless of this memo's disposition, and can
report back on whether they attract real traffic or sit empty. That is probably
the datum REBAR actually wants before changing doctrine.

---

## 6. Flagged for 3.1, not proposed here — self-describing integrity

A separate thread from the same work, recorded so it isn't lost. We are drafting a
proposed cross-format **artifact status line** (a generalization of v3's `Status:`
from architecture contracts to any artifact, with an `as-of` date and a
supersession pointer); that will arrive as its own memo. One design question inside
it is out of our depth to settle unilaterally and belongs to REBAR:

**Integrity that travels with the artifact.** `rebar sign` today writes signatures
into a **side manifest** keyed by file hash. That is a good design — it sidesteps
the self-reference problem an in-band signature has entirely. But the integrity
**does not travel**: export the PDF, email the memo, attach it to a filing, and the
manifest is left behind. For durable assets — reports, papers, preregistrations,
anything whose integrity should survive leaving the repo — an **in-band** second
line is the only form that travels.

These are complements, not competitors: manifest for repo-internal integrity,
in-band for artifacts that leave. We'd suggest REBAR own that strategy in 3.1
rather than have adopters improvise, and we'd flag a cheap intermediate worth
considering: **a bare content hash on line 2 as SOP**, no PKI at all. For the
dominant real threat here — a status headline drifting out of sync with the
content it describes — a hash is sufficient, because the adversary is entropy, not
a forger. Signatures earn their key-management cost only where attribution and
tamper-evidence are actually needed. Splitting those two cases may let the cheap
half ship immediately while the signature design takes its time.

(We also filed two v3 feedback items today from the same audit:
`feedback/2026-08-02-sch-repos-check-compliance-maturity-degenerates-at-one-contract.md`
and
`feedback/2026-08-02-sch-repos-canonical-status-grep-diverges-from-parsers-of-record.md`.)

---

## 7. Disposition requested

1. Accept/reject **4a** (inbox-by-default doctrine) — the substantive ask.
2. Accept/reject **4b** (`CONSUMES` ⇒ upstream holds an inbox) — mechanical, and
   makes 4a checkable rather than aspirational.
3. Note **4c/4d** as amendments to the convention text if 4a lands.
4. Route the §6 integrity question to 3.1 scope.

No reply needed on any timescale that constrains us; we're proceeding with inboxes
in both repos either way and will report what actually arrives.
