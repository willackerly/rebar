# AMENDMENT — the status-line design in §6 of today's proposal is withdrawn; field measurement refuted it

**From:** sch-repos (SCH + mixed-width-lsh, Will Ackerly)
**Date:** 2026-08-02
**Amends:** `inbox/2026-08-02-sch-repos-PROPOSAL-inbox-by-default-a-dependency-is-a-coordination-seat.md` §6
**Deposited as a new memo** per the Peer-Inbox Convention's append-only rule — the original stands unedited.

---

## What changes

**§1–§5 of the original memo (inbox-by-default) are UNCHANGED and still proposed.** Nothing below
affects them.

**§6 flagged a forthcoming cross-format "artifact status line" proposal with an `as-of` date and an
optional signature line.** We ran an adversarial review against our own live repos before writing it.
It did not survive. This memo withdraws the design and reports the measurement, because the negative
result is more useful to REBAR than the proposal would have been — it bears directly on
`scripts/check-freshness.sh`, an existing v3 mechanism.

---

## The measurement that killed it

We proposed a human-declared `as-of` date. Our repos have already run that experiment three times:

| Mechanism | Instances | Times updated after creation |
|---|---|---|
| REBAR `<!-- freshness: YYYY-MM-DD -->` | 12 | **0** |
| Hand-written `**Last updated:**` headers | 18 | **0** (every value June 2026 or earlier) |
| `mixed-width-lsh/AGENTS.md` freshness line | 1 | **0** — one commit, the day it was created |

Zero bumps across **540 commits and seven weeks**, in a repo where 100 markdown files were touched
after 2026-06-14 and 51 in July alone. Same operator, same agent swarm, three independent
implementations of "a hand-maintained date at the top of a file." Median time-to-first-bump: never.

This is field evidence against `check-freshness.sh`'s core assumption, not just against our proposal.
The check is opt-in by construction (`[ -z "$date_str" ] && continue`), so a marker that is never
bumped produces either a permanent warning that gets tuned out, or silence. In our repos it produced
silence for seven weeks while the docs went stale exactly as predicted.

**Why our SPDX precedent was wrong.** `SPDX-License-Identifier` spread everywhere because a license
is **immutable for the life of the file** — that is precisely why it does not rot. Citing it as
precedent for a *mutable* field is a category error, and we made it.

**The better precedent, if a date is kept at all:** RFC 9116's `Expires` is a **future** date, set
once at authoring time, that **self-invalidates without anyone touching it**. `as-of` degrades to
*silently wrong*; `expires` degrades to *loudly invalid*. If `check-freshness.sh` is ever revisited,
that inversion looks like the durable form.

## The finding that matters more than any marker

Our flagship stale document, `SCH/docs/STATUS.md`, **already has both halves of what we were about to
propose**: a visible `**Last updated:** 2026-06-06` on line 3, and a rendered blockquote on line 5
directing readers to `RESUME-HERE.md`. It still misleads every reader who opens it.

Because its H1 reads **"SCH — Project Status & Map (Single Source of Truth)"**, and readers trust the
title over the banner.

No marker line fixes that. Editing the title does, costs ten minutes, and requires no convention at
all. We flag it because REBAR's `DESIGN.md` §5 lists "freshness timestamps" as a first-class
anti-drift mechanism — and our field data says the drift vector was the **heading**, which no REBAR
mechanism currently addresses.

## What we would still propose, much reduced

One key. No dates, no vocabulary, no validator beyond "does the target path exist."

```markdown
> [!WARNING]
> **NOT CURRENT (2026-06-13)** — current state lives in [`synthesis-research/STATE-OF-PLAY.md`](…).
> `rebar:see-instead=synthesis-research/STATE-OF-PLAY.md`
```

A **location** pointer cannot rot: it has no expiry and stays true as its target evolves. Writing one
is strictly monotone — added once, never bumped. And multiple occurrences are harmless, which fixes a
failure our stricter grammar would have shipped: an anchored single-marker regex cannot distinguish a
real declaration from a document *quoting* one, so `conventions.md` itself, any ADR, and the spec
document would all have hard-errored.

**One principle inversion we caught, worth recording regardless of disposition.** Our draft made a
pointer mandatory whenever `currency ≠ current`. That makes *marking down* cost work (find a valid
path, survive a dangling-link check) while *marking up* is free and unverifiable — the exact reverse
of "marking down is free; marking up is a claim." Any future currency field should be checked against
that inversion.

## The signature line: withdrawn from v1

Two reasons, in order of severity.

1. **It would certify an attribution that is already false.** In `mixed-width-lsh`, 418 of 540 commits
   since 2026-06-01 carry Claude co-authorship, while `git log --format='%an'` returns exactly one
   name, 540/540. A `signer=will@…` line adds cryptographic weight to an existing misattribution,
   which is worse than adding nothing.
2. **It would fork REBAR's PKI.** We had proposed `ssh-keygen -Y`, not knowing `rebar sign` /
   `verify` / `key` already ship with an Ed25519 role-scoped trust store. Two PKIs in one methodology
   is the "two vocabularies" sin one layer down. (Honest counterpoint: `rebar sign` has no field
   users — no `.rebar/` manifest exists anywhere on disk, including in rebar itself.)

**One piece survives, and it solves a problem REBAR has today.** Following REUSE's `.license`
precedent, a **sidecar** — `PREREG-wave3.md.rebar` carrying `covers=` + `sha256=` + a signature —
detects post-hoc edits **without ever modifying the protected file**. We tested it end to end: signed
a copy of a real prereg, appended one line to the body, got a hash mismatch; the prereg itself was
never touched.

That matters to v3 directly. `check-freshness.sh` currently flags **five immutable
`SCH/experiments/PREREG-*.md` files as STALE**, and will keep doing so at every session start
forever. The only in-vocabulary remedies are to *lie* (bump the date) or to *edit a prereg*. A prereg
is not stale — it is **frozen**, and its date is its authoring date. The sidecar is how a
never-edit-this-file artifact carries status at all, and `freshness: archived` is the token it should
map onto rather than compete with.

## Routing correction

We had the process wrong. Per CHARTER §4, design-shaped proposals that reshape multiple files go via
**clone-and-PR, not intake**; an inbox memo is the notification only. And this is **v3.1 scope, not a
beta blocker** — adding a v3 feature now would restart graduation criterion 1's two-week clock and
contradict decision D1 ("the beta tag is earned, not declared").

Our two feedback filings today took the correct route (b) and remain v3.0.0 scope, since criterion 3
requires disposition of every v3-feature item raised in the window:
- `feedback/2026-08-02-sch-repos-check-compliance-maturity-degenerates-at-one-contract.md`
- `feedback/2026-08-02-sch-repos-canonical-status-grep-diverges-from-parsers-of-record.md`

## Disposition requested

1. **No action needed on §6** — it is withdrawn. Recorded so nobody builds on it.
2. Treat the **`as-of` measurement** as field feedback on `check-freshness.sh`'s assumption; the
   `expires`-style inversion is offered as a direction, not a proposal.
3. The **prereg/frozen collision** is a live v3 defect in our repos today and needs *some* answer,
   even if not ours — five files will warn forever with no honest remedy available.
4. §1–§5 (inbox-by-default) stand as originally filed.

**Practical note:** `/Users/will/dev/rebar/inbox/.inbox-watch.lock` is absent, so no watcher is
currently armed on rebar's own inbox and these memos may sit unread. Same gap `_atlas` raised
2026-07-30 — and somewhat on the nose given the original memo's subject.
