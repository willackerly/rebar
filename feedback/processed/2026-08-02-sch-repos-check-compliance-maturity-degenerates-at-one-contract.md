# Check 9 maturity weighting degenerates at exactly one live contract — honest downgrade jumps straight to FAIL + demotion (from sch-repos, 2026-08-02)

**DISPOSITION 2026-08-02 — IMPLEMENTED as filed.** Check 9 now guards the demotion arm
at `live_total -lt 2`, printing an advisory NOTE instead of weighting a badge on a
population where the percentage can only be 0% or 100%. Threshold is `-lt 2` per the
filing's own reasoning: n=1 is precisely the degenerate case, and from n=2 the NOTE band
is reachable. 100%-stub still fails at every n>=2, so anti-laundering is intact. The
gaming residual (delete contracts to drop under the threshold) is accepted for the
reasons given — self-defeating, visible in the diff, and the resulting state is what the
advisory already prints. Excellent filing: the arithmetic table made this a
ten-minute fix.

---

**Script:** `scripts/check-compliance.sh` — Check 9, declared-maturity badge
weighting (v3.0.0-beta, Cluster 1).

**Bug:** the weighting is a percentage over the live-contract population, so at
`live_total = 1` the documented middle band is **mathematically unreachable**.
Only 0.0% and 100.0% are attainable, and the transition between them is a
one-tier demotion plus `exit 1`. A repo with a single contract therefore cannot
declare that contract `draft` or `stub` without failing CI — which directly
punishes the convention's own stated principle, *"marking down is free; marking
up is a claim"* (`conventions.md`, Declared Maturity).

Arithmetic, from the three product comparisons at lines ~358–366:

| live_total | stub_draft | `sd*100 < lt*33` | `sd*100 <= lt*66` | Outcome |
|---|---|---|---|---|
| 1 | 0 | `0 < 33` ✓ | — | OK |
| 1 | 1 | `100 < 33` ✗ | `100 <= 66` ✗ | **FAIL — demote, exit 1** |
| 2 | 1 | `100 < 66` ✗ | `100 <= 132` ✓ | NOTE — IN PROGRESS |
| 3 | 1 | `100 < 99` ✗ | `100 <= 198` ✓ | NOTE — IN PROGRESS |
| 2 | 2 | ✗ | `200 <= 132` ✗ | FAIL (correct) |

`n = 1` is uniquely degenerate. From `n = 2` the NOTE band is reachable and the
formula behaves as documented; 100%-stub still fails at every `n`, so
anti-laundering is intact above the degenerate point.

**Trigger observed:** `mixed-width-lsh` (Tier 2, adopting v3) has exactly one
live contract, `architecture/CONTRACT-R1-COMPACTINDEX.1.0.md`. It currently
declares `**Status:** active`, but the contract has drifted from its
implementation — it specifies `encode()` where `mwlsh/retrieval.py` has `add()`,
and declares two `ValueError` error contracts in a file that contains no `raise`
at all. Under "marking up is a claim" the honest value today is `draft`. Making
that honest declaration takes the repo from `OK: Rebar compliance verified`
(exit 0) to `FAIL: 100.0% of live contracts are stub-or-draft (>66%)`, `Declared:
Tier 2: ADOPTED — Effective: Tier 1: PARTIAL`, exit 1.

So the check, as written, makes dishonesty the green path for a
one-contract repo. That is the exact inversion v3 exists to remove.

**Fix suggestion:** guard only the demotion arm at small populations, leaving the
OK and NOTE arms untouched. Inside the `status_total > 0 && live_total > 0`
branch, before the tier comparisons:

```bash
if [ "$live_total" -lt 2 ]; then
  echo "NOTE: only $live_total live contract(s) — maturity is advisory at this population"
  echo "  (a percentage over n=1 quantizes to 0% or 100%; badge not weighted)"
elif [ $((stub_draft * 100)) -lt $((live_total * 33)) ]; then
  ...
```

`-lt 2` is the minimal correct threshold (it is precisely the degenerate case).
`-lt 3` is defensible if you would rather the advisory cover the whole
small-population regime, where a single contract's status swings the badge by
33–50 points; that is a judgement call about how much signal `n = 2` carries, and
we have no field evidence either way.

**Residual risk, stated honestly:** any small-`n` exemption is in principle
gameable by *deleting* contracts to drop under the threshold. We think that is
acceptable — shrinking your governance surface to escape a badge penalty is
self-defeating, visible in the diff, and leaves the repo with ≤1 contract, which
is itself the honest signal the advisory prints. Worth a line in `conventions.md`
either way.

**Severity:** medium. It does not affect repos with mature contract populations,
but it lands squarely on **new and small adopters** — i.e. REBAR's own adoption
on-ramp, and the population v3's honesty vocabulary most needs to reach. It also
means a v3 migration in a one-contract repo produces a red badge for doing the
right thing, which is a bad first experience of the feature.

**Scope note:** this is a real P1 against a v3 feature, so per the v3-beta plan's
graduation criterion 3 it needs a disposition before the beta graduates. Relatedly
— and we flag it so it is not miscounted — `mixed-width-lsh` and `SCH` both
already fail `compute-registry.sh --check` ("Registry Consistency", exit 1) on
**v2**, before any v3 migration. That red is pre-existing, not v3 breakage.

**Verification:** the arithmetic above was read directly from
`scripts/check-compliance.sh` and confirmed against the documented thresholds in
`conventions.md`. The `mixed-width-lsh` contract/implementation drift was
confirmed by reading `architecture/CONTRACT-R1-COMPACTINDEX.1.0.md` against
`mwlsh/retrieval.py`. No rebar or adopter file was modified in producing this
filing.
