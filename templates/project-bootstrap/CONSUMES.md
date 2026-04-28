# CONSUMES — external contract dependencies

<!-- last-synced: 2026-04-28 -->
<!-- format: rebar-consumes-v1 -->

This file declares which **external contracts** (owned by other rebar
repos) this project depends on. It exists because:

- Owners can scan their consumers (`scripts/scan-consumers.sh`) and notify
  them of revisions via the existing `ask_<consumer>_featurerequest` gate
- Consumers can detect upstream drift (`rebar contract drift-check`) and
  decide when to upgrade
- Forking is the default state; this file makes divergence *visible*
  rather than implicit

**This file is opt-in.** Only fill it out if your project actually
depends on contracts from other rebar repos. Single-repo projects don't
need a `CONSUMES.md`.

---

## Format

Each consumed contract is a top-level `## <owner_repo>/<contract_id>.<version>`
section with required and optional fields:

```markdown
## rebar/C1-AGENTS.2.0
- **owner_repo:** rebar
- **contract_id:** C1-AGENTS
- **version_pinned:** 2.0.0
- **pin_date:** 2026-04-28
- **rationale:** orchestration layer for our worktree fan-out
- **notify_on_change:** true   # OPTIONAL hint to owner
```

### Required fields

| Field | Format | Purpose |
|-------|--------|---------|
| `owner_repo` | bare repo name (e.g., `rebar`, `blindpipe`) | which rebar-adopting repo owns this contract |
| `contract_id` | `<prefix><N>-<NAME>` (e.g., `C1-AGENTS`) | the contract identifier in the owner's `architecture/` |
| `version_pinned` | semver `MAJOR.MINOR.PATCH` | the version your code is implemented against; mandatory semver per CHARTER §1.6 |
| `pin_date` | `YYYY-MM-DD` | when you pinned this version (helps owner age out stale pins) |
| `rationale` | one-line prose | why your project depends on this contract — surfaces value to drift-check output |

### Optional fields

| Field | Default | Purpose |
|-------|---------|---------|
| `notify_on_change` | (owner's call) | hint to owner: `true` = "I want notifications," `false` = "I'm a stable-pin acceptor, skip me," absent = "owner decides." Owner's `flush-notifications.sh` may filter by this field. |
| `extension_contracts` | `[]` | list of local contract IDs in this project that augment the consumed contract (e.g., `C2-AGENTS-MYAUDIT.1.0`). Helps owner spot patterns worth upstreaming. |
| `notes` | (none) | free-form prose — known incompatibilities, planned upgrade, etc. |

### Composition over inheritance

If your project needs more than the owner provides, **write your own
contract** that documents how it relates to the upstream. Don't try to
encode "extends" semantics in this file — composition is more honest
and forkable. See CHARTER §1.6.

Example:
```markdown
## rebar/C1-AGENTS.2.0
- **owner_repo:** rebar
- **contract_id:** C1-AGENTS
- **version_pinned:** 2.0.0
- **pin_date:** 2026-04-28
- **rationale:** baseline agent role definitions
- **extension_contracts:**
  - C2-AGENTS-MYAUDIT.1.0   # local audit-trail extensions
  - C2-AGENTS-MYRBAC.1.0    # local RBAC extensions
- **notes:** Extensions add behavior not present in upstream; they are
  fully owned by this project. May propose upstream-absorption via
  `rebar contract upstream` if patterns generalize.
```

---

## Tooling that uses this file

- **`rebar contract drift-check`** — compares your `version_pinned` to the
  owner's current version (read from owner's `architecture/CONTRACT-<id>.<v>.md`)
  and flags deltas. CI-friendly exit codes:
  - `0` if all pins match upstream current
  - `1` if any pin is N+ minor versions behind
  - `2` if any pinned contract was removed upstream

- **`rebar contract upstream <local-extension>`** — opens an FR in the owner
  repo via the existing `ask_<owner>_featurerequest` gate, proposing
  your local extension be considered for upstreaming.

- **Owner-side `scripts/scan-consumers.sh <my-contract>`** — greps known
  sibling repos for `CONSUMES.md` declarations citing the contract.

- **Owner-side `scripts/flush-notifications.sh`** — when owner has bumped
  a contract version, files notification FRs in each consumer who
  declared `notify_on_change: true` (or all, per owner's policy).

---

## Adoption

Adding a `CONSUMES.md` is **opportunistic**: declare your dependencies
when you hit a real cross-repo coordination need (you want notifications
of upstream changes, or you want your local extension considered for
upstreaming). Once you add this file, `rebar contract drift-check`
becomes a required CI check per `scripts/check-compliance.sh` —
self-selecting adopters get gated when they're ready.

If you have no cross-repo dependencies, **delete this file** or leave it
unedited. The presence of the template alone doesn't trigger any
checks; only when you add real `## <owner>/<contract>` entries does it
become load-bearing.

---

## Example — fully-filled

```markdown
# CONSUMES — external contract dependencies

## rebar/C1-AGENTS.2.0
- **owner_repo:** rebar
- **contract_id:** C1-AGENTS
- **version_pinned:** 2.0.0
- **pin_date:** 2026-04-28
- **rationale:** baseline orchestration of architect/product/englead agents
- **notify_on_change:** true

## blindpipe/C2-ZK-VERIFY.1.3
- **owner_repo:** blindpipe
- **contract_id:** C2-ZK-VERIFY
- **version_pinned:** 1.3.0
- **pin_date:** 2026-04-15
- **rationale:** ZK proof verification on receipt validation path
- **notify_on_change:** true
- **extension_contracts:**
  - C3-ZK-VERIFY-AUDIT.1.0  # adds tamper-evidence logging

## fontkit/C1-GLYPH-REGISTRY.4.0
- **owner_repo:** fontkit
- **contract_id:** C1-GLYPH-REGISTRY
- **version_pinned:** 4.0.0
- **pin_date:** 2025-11-22
- **rationale:** glyph lookup for our PDF rendering layer
- **notify_on_change:** false
- **notes:** Stable consumer; we will pin to 4.x indefinitely. Skip
  notifications for additive bumps.
```
