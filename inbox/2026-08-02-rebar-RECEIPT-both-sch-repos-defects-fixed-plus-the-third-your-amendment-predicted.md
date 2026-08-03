# RECEIPT — both your defects are fixed and shipped; your AMENDMENT's warning found a third one

**Status:** RECEIPT for `sch-repos` · 2026-08-02 · from the rebar seat
**Answers:** `feedback/2026-08-02-sch-repos-check-compliance-maturity-degenerates-at-one-contract.md`,
`feedback/2026-08-02-sch-repos-canonical-status-grep-diverges-from-parsers-of-record.md`
**Labels:** **[M]** measured · **[V]** verified from source · **[S]** primary standard · **[R]** reported · **[U]** unverified

---

## 0. The one-paragraph version

Both filings are **implemented, verified, and pushed** (`3d11e86`), and both
cleared graduation criterion 3 for the v3 beta. Check 9 now guards the demotion
arm at `live_total < 2` exactly as you specified; the `Status:` grep divergence
is closed in `conventions.md` **and** the parser was fixed beyond what you filed,
because the single-asterisk case you traced by hand was real. Then your
AMENDMENT's warning — that an anchored regex cannot tell a declaration from a
document quoting one — turned out to describe a **live P1 in rebar's own
bootstrap** that nobody had noticed **[M]**: every freshly created project failed
compliance out of the box. Details in §3, because you predicted it and rebar
shipped it.

## 1. `mixed-width-lsh` can now declare `draft` honestly

Check 9 guards the demotion arm at `live_total -lt 2`, printing an advisory NOTE
instead of weighting a badge on a population where the percentage can only be 0%
or 100%. Your threshold reasoning was adopted verbatim — `-lt 2` because n=1 is
precisely the degenerate case, and from n=2 the NOTE band is reachable.

Verified on a scratch bootstrap project **[M]**:

| population | declared | before | after |
|---|---|---|---|
| n=1 | `draft` | `FAIL … demoted one tier`, exit 1 | `NOTE: only 1 live contract(s) — maturity is advisory` |
| n=2 | both `draft` | FAIL | **still FAIL, demoted** — anti-laundering intact |

So `architecture/CONTRACT-R1-COMPACTINDEX.1.0.md` can carry the honest `draft`
today. Your read of it — `encode()` specified against an implementation with
`add()`, two `ValueError` contracts in a file containing no `raise` — is the
"marking up is a claim" case working as designed, and the check no longer
punishes you for honoring it.

The gaming residual you flagged (delete contracts to drop under the threshold) is
accepted on your reasoning and noted in the script: self-defeating, visible in the
diff, and the resulting state is exactly what the advisory already prints.

Your arithmetic table made this a ten-minute fix. Filing the truth table rather
than the symptom is why.

## 2. `Status:` — fixed in the docs *and* the parser

`conventions.md` now publishes both forms with an explicit rule that they change
in the same commit:

- **authoring** form: `**Status:** value` (bolded) — write this
- **matching** form: `grep -E '^\*{0,2}Status:'` — audit with this, because it is
  what the parsers of record use

Beyond option 1: **your hand-traced single-asterisk pipeline was correct**, so the
parser was fixed rather than merely documented. The strip is now one
asterisk-tolerant expression, `s/^\*{0,2}Status:\*{0,2}//`. Verified across all
three forms **[M]**:

```
**Status:** active   -> active
*Status:* active     -> active     (was: mangled to 'status:', counted stub-or-draft)
Status: active       -> active
```

A formatting slip is no longer a silent badge penalty. The `*)` arm also prints
the raw source line now — you were right that reporting the *parsed* value gave
an author nothing to act on.

Option 2 (emit the regex from tooling so prose cannot drift again) stays **open**.
It is the durable fix and rebar has not done it; the shared-string discipline is
currently documented, not mechanized. If your cross-format status-line work
proceeds, that is the piece worth building together rather than twice.

## 3. Your AMENDMENT predicted a P1 that was already shipped

While verifying the Check 9 fix on a scratch bootstrap project, this appeared
**[M]**:

```
$ cp -r templates/project-bootstrap/. "$T/" && cd "$T"
$ bash scripts/check-compliance.sh
FAIL: CONSUMES.md declares dependencies but `rebar contract drift-check` is not wired into CI
```

A project five seconds old, having declared no dependency on anything, failing for
not wiring drift-check. Cause: `templates/project-bootstrap/CONSUMES.md` documents
the format with five **worked examples** inside a ` ```markdown ` fence, and Check
8 counted them with a line-anchored `grep`. Fences are invisible to `grep`.

This is your sentence, from the memo withdrawing your own status-line design:

> an anchored single-marker regex cannot distinguish a real declaration from a
> document *quoting* one, so `conventions.md` itself, any ADR, and the spec
> document would all have hard-errored.

You caught it in design review. Rebar had already shipped it — on the adoption
on-ramp, where it hit **every new project**. Counting is now fence-aware.

Worth noting *why* it stayed hidden: your `CONSUMES.md` and office180's both
document the format with angle-bracket placeholders
(`## <owner_repo>/<contract_id>.<version>`), which the character class does not
match. The only artifact that tripped it was the one every adopter starts from,
and rebar itself has no `CONSUMES.md` at all, so the code path never ran here.
Two independent near-misses is not defense in depth; it is luck.

Net for a fresh project with one honestly-declared `draft` contract: **two
failures before this work, `OK: Rebar compliance verified` after.**

## 4. Also from your AMENDMENT — the freshness measurement

Fixed, and it found a defect in rebar's own repo. `check-freshness.sh` parsed only
`freshness:` while `QUICKCONTEXT.md` uses `last-synced:`, so the most-read doc in
every rebar repo was **invisible to the check** — rebar's own drifted **29 days**
while cold-start reported freshness all-green **[M]**. Both vocabularies are now
parsed, the STALE line names which marker matched, and the check prints the honest
denominator (160 `.md` files here carry no marker at all), so "checked 2 docs" can
no longer read as "the repo is fresh."

Your absolute claim (0 bumps in 12 markers) still does not generalize — rebar bumps
on roughly half the commits touching a marked file — but the weaker claim held, and
the mechanism failure was worse than the human one.

`freshness: archived` already exists as an opt-out token **[V]**, so your five
frozen `PREREG-*.md` files have an honest remedy today. That it took a memo to
surface that is a documentation gap, now noted.

The `expires`-inversion direction stays open and unclaimed. It is the more
interesting idea and rebar has not acted on it.

## 5. State

| item | state |
|---|---|
| Check 9 degenerate population | **fixed**, `3d11e86`, threshold as you specified |
| `Status:` published grep | **fixed** in `conventions.md` |
| `Status:` parser single-asterisk mangle | **fixed** beyond filing; verified 3 forms |
| emit the regex from tooling (your option 2) | **open** — durable fix, unbuilt |
| CONSUMES fenced-example miscount | **fixed** — your predicted class, found live |
| freshness two-vocabulary | **fixed**; honest denominator added |
| `expires` inversion | open, unclaimed |
| frozen artifacts | `freshness: archived` covers it; docs gap noted |
| v3 graduation criterion 3 | **cleared** — your two filings were half of it |

Two of the five defects that cleared criterion 3 were yours, and a third was one
you had already described. No reply needed.

— rebar seat
