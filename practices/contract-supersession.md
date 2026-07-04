# Contract Supersession — versioning, migration, retirement

**Status:** load-bearing doctrine — consolidates rules previously scattered
across [`conventions.md`](../conventions.md) ("Modified Contract" checklist),
[`architecture/README.md`](../architecture/README.md) (§Versioning), and the
[CONTRACT-TEMPLATE.md](../architecture/CONTRACT-TEMPLATE.md) header comments
**Source:** [`feedback/processed/2026-04-24-contract-discipline-and-jtbd-framing.md`](../feedback/processed/2026-04-24-contract-discipline-and-jtbd-framing.md) §C
(the retirement-lag drift class)

A contract version is never edited into a new meaning — it is **superseded**
by a new version file, and the old file is explicitly marked. This practice
covers the whole arc: deciding the bump, writing the marker lines, migrating
implementing-file headers, updating the registry, and retiring the
predecessor on a deadline instead of letting it rot.

---

## When a change is a supersession

From [`architecture/README.md`](../architecture/README.md):

| Change | Version bump | Autonomy |
|--------|-------------|----------|
| Doc fix (no behavior change) | None | Full |
| New optional method/field | Minor (1.0 → 1.1) | Full |
| Changed signature, removed method, renamed output field | Major (1.1 → 2.0) | **Plan mode** |
| New contract | New ID + 1.0 | **Plan mode** |

Any version bump — minor or major — is a supersession: a new file
`CONTRACT-{ID}-{NAME}.{NEW}.md` is created and the old file stays on disk,
marked. Two things are **not** supersession:

- **Companion-file edits.** Tribal knowledge changes freely with no bump
  (see "When to companion-file" below).
- **Renumbering.** If two different contract IDs collide on a prefix number
  (I3-LLM-CLIENT vs I3-SCANNER), the fix is a *new ID* for the newer
  contract, not a version bump — versions belong to one continuous contract
  identity. `scripts/check-prefix-uniqueness.sh` catches collisions.

## The marker lines

Supersession is declared **in the contract files themselves**, not in the
registry narrative. This pushes the decision to the contract-version author,
where it belongs, and makes the state greppable without tooling.

In the **old** file, immediately below the H1 title:

```markdown
<!-- SUPERSEDED BY: CONTRACT-{ID}-{NAME}.{NEW} -->
```

In the **new** file's header block:

```markdown
SUPERSEDES: CONTRACT-{ID}-{NAME}.{OLD}
```

And in the commit message, per [`conventions.md`](../conventions.md) commit
format:

```
contract(C3-CRYPTO-BRIDGE): bump to 2.0 — add key rotation interface

CONTRACT: C3-CRYPTO-BRIDGE.2.0
SUPERSEDES: C3-CRYPTO-BRIDGE.1.0
```

The exact token `SUPERSEDED BY:` matters: enforcement scripts key on it.
`scripts/check-jtbd-presence.sh` exempts marked files from current template
requirements — an *unmarked* old version will be held to the latest
standards and fail the gate. That asymmetry is deliberate: forgetting the
marker has a cost, so old versions get marked.

## The supersession procedure

1. **Copy the old file to the new filename** with the bumped version:
   `CONTRACT-S1-STEWARD.1.0.md` → `CONTRACT-S1-STEWARD.2.0.md`. Never
   rename away the old file — it stays for history.
2. **Edit the new file:** update the `**Version:**` line, add the
   `SUPERSEDES:` line, make the actual contract changes, and append a row
   to the Change History table describing the change *and the migration*.
3. **Write the Retirement / supersession plan section** in the new file.
   Required, not optional, for any contract that supersedes another:
   - **Predecessor:** the old contract ID + version
   - **Retirement criterion:** mechanical and checkable — the canonical
     form is `grep -rn "CONTRACT:{ID}.{OLD}"` returns zero
   - **Migration deadline:** a concrete date or a named phase boundary
   - **Migration owner:** who is on the hook for the cutover
4. **Mark the old file** with the `SUPERSEDED BY:` comment **and flip its
   declared maturity to `**Status:** superseded`** — the two travel
   together (`conventions.md` §Declared Maturity). The Status flip is what
   removes the old version from `check-compliance.sh`'s badge weighting;
   the comment alone exempts it only from the JTBD gate.
5. **Migrate implementing-file headers.** Run
   `grep -rn "CONTRACT:{ID}.{OLD}"` across source (excluding
   `.claude/worktrees/`, `node_modules/`, `vendor/`, `.git/` — see the
   script-exclusion patterns in [`conventions.md`](../conventions.md)) and
   update every header to the new version. For a minor bump this is
   mechanical; for a major bump each site is a code change that must
   actually adopt the new behavior — updating the header without updating
   the code is drift with a fresh coat of paint.
6. **Regenerate the registry:** `./scripts/compute-registry.sh`. The
   registry is computed from the files on disk — never hand-edit it. Both
   versions will be listed; that's correct while migration is in flight.
7. **Update the companion file** (if one exists) to reflect the new
   version. The companion filename has no version — one companion serves
   all versions of an ID.
8. **Commit** with `contract(...)` type and the `SUPERSEDES:` footer.
9. **Cross-repo consumers:** if the contract has external consumers
   (federation), the bump is detected by `scripts/check-version-bump.sh`
   and queued to the notification outbox — flush on your schedule. See
   [`architecture/README.md`](../architecture/README.md) §Cross-Repo
   Contract Federation and [Federation](federation.md).

## Retirement deadlines — why they're required

The observed drift class (filedag's C9-ABAC): a contract marked superseded
"weeks ago" that still had live impl refs and no deadline. Superseded-with-
live-refs is a *transition* state; without a deadline it silently becomes a
*permanent* state, and every reader of the codebase now has to hold two
contract versions in their head indefinitely.

The rubric:

- **Deadline:** a real date or a named phase boundary ("end of DP-5"),
  never "eventually" or "when convenient."
- **Criterion:** a single grep whose empty output proves retirement. If
  you can't write the grep, the criterion isn't mechanical enough.
- **Owner:** a name. Unowned migrations don't finish.
- **On the deadline:** either the grep returns zero (done — the old file
  stays as history, needing nothing further) or the deadline is
  consciously re-negotiated *in the Retirement section* with a new date.
  Silent lapse is the failure mode this practice exists to kill.

Multi-version coexistence (filedag ran P2-ABAC 2.0 / 3.0 / 3.1
simultaneously) is legitimate during migration — the plan and deadline are
what distinguish "migrating" from "rotting."

## When to companion-file

Every contract MAY have one companion: `CONTRACT-{ID}-{NAME}.impl.md` — no
version in the filename, one per contract ID across all versions
(see [`conventions.md`](../conventions.md) §Companion Files). Use it during
supersession for:

- **Migration guides** — step-by-step "porting from 1.x to 2.0" notes,
  worked examples, gotchas discovered mid-migration. This is tribal
  knowledge: it helps the cutover but doesn't define behavior.
- **Historical context** — why 1.0's approach was abandoned, what was
  tried in between.

What must **never** move to the companion: behavioral deltas. What changed
between versions belongs in the new contract's body and Change History
table — companions carry no behavioral authority and don't bump versions.
Rule of thumb: if a consumer could implement the new version wrongly by
not reading it, it's contract content; if it just saves them time, it's
companion content.

## What never happens

- **Old version files are never deleted.** They're history; disk is cheap
  and provenance isn't.
- **The registry is never hand-edited** to reflect a supersession — it's
  regenerated.
- **A contract file's meaning is never changed in place** past a doc fix.
  If behavior changes, the version bumps and the old file gets marked.
- **A superseded contract never keeps collecting new implementing files.**
  New code targets the latest version, always.

## Enforcement

| Concern | Gate |
|---------|------|
| Latest version carries required JTBD sections; `SUPERSEDED BY:`-marked files exempt | `scripts/check-jtbd-presence.sh` |
| Prefix number claimed by two different IDs | `scripts/check-prefix-uniqueness.sh` |
| Registry stale after a bump | `scripts/compute-registry.sh --check` |
| Version bump with external consumers → notification queued | `scripts/check-version-bump.sh` (post-commit) |
| Consumer pins drifting behind upstream bumps | `rebar contract drift-check` (via CONSUMES.md, see [Federation](federation.md)) |

Not yet mechanized: flagging contracts whose migration deadline has passed
while the retirement grep still returns hits. Until a checker exists, the
Retirement section's deadline is enforced by review — put it on the phase
checklist that the deadline names.

## See also

- [`conventions.md`](../conventions.md) — Modified Contract review checklist, commit format, companion-file rules
- [`architecture/README.md`](../architecture/README.md) — versioning table, federation discipline
- [`architecture/CONTRACT-TEMPLATE.md`](../architecture/CONTRACT-TEMPLATE.md) — Retirement / supersession plan section template
- [Spike-First Contracts](spike-first-contracts.md) — drafting new versions during an architectural spike
- [Federation](federation.md) — cross-repo consumers of superseded contracts
