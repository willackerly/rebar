# Worktree Collaboration

**Referenced from AGENTS.md. Read when coordinating parallel agents or resolving merge conflicts.**

---

## Worktree Isolation Rules

- **Use worktrees for:** implementation work that modifies files, speculative
  approaches, any change that might conflict with parallel agents.
- **Use main-thread sub-agents for:** read-only research, validation (tests,
  typechecks, lint), synthesizing information from multiple files.
- **Never use worktrees for:** changes to a single shared file (merge will
  conflict), changes requiring real-time coordination, changes with unclear
  scope (agents will expand into each other's territory).

---

## Cherry-Pick Conflict Resolution

Conflicts between worktree agents are expected, not exceptional:

1. Understand which version is the superset — don't blindly take "theirs" or "ours"
2. Merge manually with understanding of both agents' intent
3. Run T2 (package-level tests) immediately after resolution
4. When a fix involves a common pattern across multiple files, assign all affected files to the same agent

---

## Post-Merge Integration

Plan post-merge integration as an explicit step, not an afterthought:

- Fan-out plans should include a "post-merge wiring" section listing which
  cross-file connections need to be made after all worktrees merge
- Budget ~30% of agent time for fix-up, not 0%
- Agents creating new files are safest (no existing state to conflict with)
- Agents modifying existing files need diff-against-main review
- Agents writing tests for existing code have ~50% wrong-assumption rate —
  always run on main before committing

---

## Freshness Markers

Status-bearing sections in docs should include a freshness timestamp:

```markdown
<!-- freshness: YYYY-MM-DD -->
```

Agents should check this date and treat claims in sections >2 weeks stale with skepticism.

---

## Role Flows

Who does what in the contract-driven development process:

| Role | Responsibility | Runs | Owns |
|------|---------------|------|------|
| **Developer** | Write code, inner-loop tests | T0-T2 | Source files within contracts |
| **Eng Lead** | QA + coordination | T3-T5, steward, fan-out | TODO, QUICKCONTEXT, deploys |
| **Architect** | Contract ownership | Contract audit, reviews | architecture/, methodology |
| **Product** | Requirements | BDD scenarios, backlog | product/, priorities |
| **Steward** (auto) | Project health scan | Full quality scan | architecture/.state/, report |

### The QA Flow

QA is fully automated. No separate QA role. The Eng Lead drives this:

1. **Run steward:** `scripts/steward.sh` produces health report
2. **Review action items:** Per-role items in `STEWARD_REPORT.md`
3. **Fan out:** Assign BUGs and implementations to developers
4. **Escalate:** DISPUTEs -> architect, DISCOVERYs -> product
5. **Verify:** Re-run steward after fixes land

### The Discovery Flow

How discoveries move through the system:

1. **Anyone reports:** Add entry to `TODO.md` Discoveries section with type tag
2. **Steward picks up:** Next `steward.sh` run includes it in the report
3. **Role-based routing:** BUG -> dev, DISCOVERY -> architect, DRIFT -> architect+dev, DISPUTE -> architect+product
4. **Resolution:** Fix code, write contract, or update contract — then check off the discovery
5. **Verification:** Steward confirms resolution on next run
