# Feedback: Opportunistic Auto-Federation — Next Big Push

**Date:** 2026-04-28
**Source:** maintainer-direct, post-Cluster-5 federation pass landing
**Type:** missing-feature | proposal
**Status:** proposed (research-only — no code lands until reviewed)
**Template impact:** scripts/, cli/cmd/contract.go, .github/workflows/ templates, possibly `agents/<role>/AGENT.md` updates
**From:** maintainer-direct ("I do think automating federation as much as possible is good though... automation TRIES on both sides but doesn't REQUIRE")

## What Happened

The v2.2.0 federation pass shipped (5 commits, 2026-04-28 eve) with
the *manual-loop* primitives in place: CONSUMES.md, version-bump
detector, outbox, scan-consumers, flush-notifications, drift-check,
upstream proposal. All work, all green.

Maintainer flagged the next layer: **opportunistic automation**. In a
"wonderful world," when both sides have the conventions wired, repos
auto-link and auto-notify each other end-to-end without manual
intervention. The discipline (CHARTER §1.6 + §2.10) holds either way
— automation is best-effort, never required (Principle 4 in
`practices/federation.md`).

## What Was Expected

Each step of the federation loop has an arrow that *can* be
auto-fired:

```
owner bumps  →  notif queued  →  notif flushed  →  consumer FR  →
   drift-check warns  →  pin bumped  →  drift-check clean
```

Today most arrows are manual. The proposal: auto-fire as many as we
can while keeping every fallback intact. **Test that "without trying
too hard" — repos auto-link and auto-notify, but nothing breaks if
they don't.**

## Suggestion — opportunistic auto-federation, 5 candidate experiments

Each experiment has a specific "tries vs. requires" boundary. None
of them block manual fallback; all surface their effects per
Principle 4's visibility corollary.

### Experiment A — auto-flush via cron / GitHub Actions

**What:** A scheduled GitHub Actions workflow (or local cron) that
runs `scripts/flush-notifications.sh` daily/weekly. Owner doesn't
need to remember; pending notifications dispatch on a cadence.

**Try:** workflow runs, dispatches FRs, marks outbox sent.
**Don't require:** owner can disable the workflow, run flush
manually, or ignore the outbox indefinitely.
**Visibility:** workflow run history; `rebar status` shows pending
queue regardless.

**Risk:** notification fatigue if owner bumps frequently. Mitigation:
default cadence weekly + severity coalescence at flush time.

### Experiment B — auto-detect new consumers (consumer-hello)

**What:** When a consumer adds a new entry to `CONSUMES.md` and
commits, a post-commit hook fires `ask_<owner>_featurerequest "Hello
— we just declared consumption of <contract>"`. Owner gets a
"new consumer" FR for visibility.

**Try:** post-commit hook detects new H2 sections in CONSUMES.md and
dispatches once per new section.
**Don't require:** consumer can skip the hook; owner can ignore the
FR; nothing breaks.
**Visibility:** owner sees the FR like any other typed ask.

**Risk:** spam if consumer churns CONSUMES.md (adds, removes, re-adds).
Mitigation: dedupe by `(consumer, owner, contract)` tuple; only fire
on first-add per tuple.

### Experiment C — auto-PR for minor-bump pin updates

**What:** When `rebar contract drift-check` detects minor-behind, a
GitHub Actions workflow opens a PR bumping `version_pinned` in the
consumer's `CONSUMES.md`. Maintainer reviews + merges.

**Try:** workflow detects minor-behind status, generates PR, requests
review.
**Don't require:** maintainer can ignore the PR forever; drift-check
keeps reporting minor-behind until reviewed.
**Visibility:** the PR itself; `rebar contract drift-check` output.

**Risk:** noisy if upstream releases frequent minor versions.
Mitigation: deduplicate to one PR per (contract, current-version);
update existing PR rather than creating new ones; only auto-PR for
*minor* (additive) — never *major* (breaking).

### Experiment D — auto-discover sibling repos via git remotes

**What:** `scripts/scan-consumers.sh` could opportunistically expand
its search beyond `~/.config/ask/projects` by sniffing git remotes
of registered repos for hints (org name, sibling repo names) and
discovering more repos to scan.

**Try:** scan + add discovered repos with same-org match heuristic.
**Don't require:** explicit `REBAR_REPOS=...` always overrides;
scan-consumers without auto-discovery still works.
**Visibility:** `--verbose` flag shows discovered vs. explicit
sources; never silent.

**Risk:** scope creep — risks violating CHARTER §2.10 if discovery
becomes a centralized index. Mitigation: discovery is per-machine,
per-invocation; nothing persisted.

### Experiment E — auto-mark outbox entries "dropped" on stale

**What:** Outbox entries that have been `pending` for longer than N
days (default 30) auto-mark as `dropped` to prevent forever-pending
queue growth.

**Try:** `flush-notifications.sh --age-out=30d` skips ancient
pending entries with a "dropped (stale)" reason.
**Don't require:** owner can override; entries can be revived to
`pending` manually.
**Visibility:** flush output shows aged-out entries; rebar status
distinguishes pending from aged-out counts.

**Risk:** consumer never gets notified about a real bump. Mitigation:
visibility (owner sees the aged-out report at flush time and can
decide).

---

## Cross-cutting design constraints

All experiments must satisfy `practices/federation.md` Principle 4:

1. **Try, don't require** — every automated action has a manual
   fallback that fully replaces it
2. **Surface failure** — automation that silently fails is worse than
   no automation; if a step doesn't fire, downstream effects must
   make it visible
3. **No new infrastructure** — Principle 1 still binds; experiments
   that need a daemon or central server are out
4. **Composition over coupling** — experiments that make repos depend
   on each other's automation succeeding are wrong (each repo is
   sovereign per CHARTER §1.6)

---

## Test plan — "without trying too hard"

Maintainer wrote: *"i want to test that without trying too hard,
repos link w/ each other and notify each other."*

Suggested staged test:

### Phase 1 — observe the manual loop in production

Two adopters: rebar-self (host) + one external adopter (e.g., TDFLite
or filedag). Add a real CONSUMES.md to the consumer; bump a contract
in rebar; manually run flush-notifications. Verify the FR lands and
the consumer's drift-check reports the bump. **No automation
introduced yet — establish baseline.**

### Phase 2 — wire one auto-fire arrow at a time

In order of risk:

1. Experiment B (consumer-hello) — lowest risk, opt-in for consumers
2. Experiment A (auto-flush cron) — owner-side, easy to disable
3. Experiment E (age-out) — pure outbox hygiene
4. Experiment C (auto-PR pin updates) — highest leverage, needs care
5. Experiment D (auto-discover) — only if 4+ adopters demand it

### Phase 3 — measure the loop

For each experiment that lands, capture:
- Did it fire when expected?
- Did the manual fallback still work when disabled?
- Did failure surfaces actually surface (visibility check)?
- Notification fatigue: how many FRs per consumer per week?

If any experiment violates "tries, doesn't require" or fails to
surface, roll it back. The loop must stay fundamentally manual-
capable.

---

## Open questions for maintainer

1. **Test partner repo** — TDFLite, filedag, or a fresh tmp-repo? A
   real adopter gives the most signal but also the most blast radius
   if something goes wrong.
2. **Phase 1 trigger** — wait for an organic contract bump, or
   manufacture one for testing? Manufactured is more controllable;
   organic is more honest.
3. **Auto-PR bot identity** — when Experiment C opens PRs, what
   account? `rebar-bot`? Maintainer's own? GitHub Actions default?
4. **Notification fatigue threshold** — what's "too many FRs per
   week" for a consumer before we coalesce? 1? 3? 10?
5. **Cron cadence default for auto-flush** — daily, weekly, or
   per-bump?
6. **Should `CONSUMES.md` auto-open PRs back to the upstream?** —
   when consumer first adds an entry, an opportunistic FR ("here's
   our new use case") might be more valuable than waiting for the
   maintainer to ask. Or it might be spam.
7. **CHARTER amendment needed?** — the existing §1.6 + §2.10 already
   admit "automation tries, doesn't require"; the experiments above
   fit. But if we land Experiment C (auto-PR), we should consider
   whether it deserves its own §1.6.1 sub-section, or stays as
   tooling under §1.6.

## Provenance notes

- Maintainer's exact framing: *"automation TRIES on both sides but
  doesn't REQUIRE.... so if they match up in a wonderful world,
  automated updates do occur. maybe make that as a clear TODO for
  next big push, i want to test that without trying too hard, repos
  link w/ each other and notify each other."*
- Principle 4 in `practices/federation.md` was added concurrently
  (same commit) to anchor the doctrine before the experiments land.
- This file is research-only; landing follows the propose-then-ship
  cycle that worked for the v2.1.0 UX pass and the v2.2.0 federation
  pass. Maintainer reviews + decides which experiments to attempt
  + in what order; implementation lands as subsequent commits keyed
  off the answers.

---

## Disposition (maintainer-filled)

- [ ] Accepted as proposed — proceed with Phase 1 baseline
- [ ] Accepted with subset selected — see notes
- [ ] Deferred — see notes
- [ ] Rejected — see notes

**Triaged:** YYYY-MM-DD by [maintainer]
**Notes:** [especially: which experiments to run, in what order, with what test partner; answers to the 7 open questions above]
