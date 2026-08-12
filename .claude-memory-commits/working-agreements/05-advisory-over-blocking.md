# Advisory over blocking

**In one line:** New checks in rebar report and exit 0. A gate that blocks must
earn it, and a gate triggered by someone else's action never earns it.

*Ruled 2026-08-02, deciding how the pre-commit inbox check should behave.*

---

## The agreement

When adding a check, the default is: **print what you found, always exit 0.**
`scripts/cold-start-checks.sh` is the house pattern — it is explicitly advisory
and its header says so.

Blocking is reserved for checks that (a) guard an invariant the repo genuinely
cannot tolerate violating, and (b) are triggered by something the committer
controls.

## Why

**Blocking a commit trades durability for coordination, and durability wins.**
Rebar ratified the Reflexive Push rule after finding 19 commits of verified work
on exactly one laptop. Uncommitted work *is* the risk. A gate that refuses a
commit does not undo the work — the work is already written — it only leaves it
fragile. That is backwards.

**A gate you do not control trains the bypass.** The pre-commit inbox check fires
on *a peer's* deposit, on their schedule. The first time a memo lands mid-task,
someone types `--no-verify`, and once that habit exists every other hook in the
repo degrades with it. Rebar has a whole gate (`check-bypass-flags.sh`) devoted
to noticing this, which is itself evidence of the pressure.

**The check usually only fires when the other mechanism failed.** If the inbox
watcher is armed you already know a memo arrived; the pre-commit check therefore
only matters when the watcher is *not* running. Its real job is a safety net —
and a safety net that blocks is a trap.

**Officialness invites false confidence.** From `_atlas`, 2026-07-30, and worth
quoting whenever a check gets promoted to a tool: *"the more official the
watcher looks, the more tempting it is for some future gate to assume it ran."*
Principle 4 — never manufacture false confidence.

## How to apply

- New check → advisory, exit 0, until there is evidence a block is needed.
- If a check must block, say in its header **what invariant** justifies it and
  **who** can act on a failure.
- Even severe signals can be loud without blocking. A `FREEZE` memo renders as a
  banner, not a refusal: a freeze means stop *working*, and committing what is
  already written is not continuing to work.
- **Distinguish silence-because-clean from silence-because-dead.** An advisory
  check that prints nothing is indistinguishable from one that did not run. Say
  `watching: false` plainly; state suppression at arm time. Silence is only a
  real answer from something you just called.
- Report the honest denominator. "Checked 2 docs" reads as "the repo is fresh"
  unless you also say 160 files carry no marker at all.

## Precedent

`cold-start-checks.sh` (advisory by construction), `check-freshness.sh`
(advisory below Tier 2), the 2026-08-02 pre-commit inbox ruling, and the
`--self` watcher suppression that reports its own state at arm time.
