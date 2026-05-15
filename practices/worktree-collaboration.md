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

## Fan-Out Merge Strategy

### File Conflict Zones — Identify BEFORE Launching

Before any fan-out, categorize files by how many agents will touch them:

| Zone | Examples | Risk | Mitigation |
|------|----------|------|------------|
| **HOT** | Shared interfaces, routers, main configs, `App.tsx` | Every agent touches these | Freeze during fan-out, or assign to ONE agent |
| **WARM** | Specific handlers, components | 2-3 agents touch these | Explicitly assign ownership |
| **COLD** | New files, tests | Only one agent creates these | Safe to parallelize |

### Merge Ordering

The order you merge agents matters enormously:

1. **First:** Merge the agent with the MOST interface changes (it
   establishes the base that all subsequent merges resolve against)
2. **Next:** Merge agents that only CREATE new files (zero conflict risk)
3. **Last:** Merge agents that modify WARM/HOT files (they resolve
   against the fully-established base)

**For interface changes** (adding methods to Store, Scanner, etc.):
- The agent adding the interface method goes FIRST
- All other agents' mock stubs then merge trivially
- NEVER let two agents add different methods to the same interface
  simultaneously — combine them into one agent or sequence them

**For CSS/style changes:**
- APPEND-ONLY is safe to parallelize (each agent adds to the end)
- MODIFY requires sequencing (only one agent modifies existing rules)

### Shared Mock Consolidation Rule

If an interface has **>3 mock implementations** across test files,
consolidate into a shared mock package BEFORE launching parallel agents
that will modify the interface. The cost of creating the shared mock
(30 minutes) is always less than resolving 6-way mock conflicts across
a fan-out (measured: 20% of total merge resolution time in a 62-commit
sprint).

---

## Cherry-Pick Conflict Resolution

Conflicts between worktree agents are expected, not exceptional:

1. Understand which version is the superset — don't blindly take "theirs" or "ours"
2. Merge manually with understanding of both agents' intent
3. Run T2 (package-level tests) immediately after resolution
4. When a fix involves a common pattern across multiple files, assign all affected files to the same agent

### Cherry-Pick Best Practices

- **Keep HEAD (main) as the base** — it has all prior merges
- **From the cherry-pick, add ONLY what's NEW** — don't replace HEAD's code
- **For interface additions:** keep HEAD's methods + add the new one
- **For mock stores:** keep HEAD's stubs + add new method stubs
- **After resolution:** build + test before continuing to next cherry-pick

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

## Worktree Lifecycle Checklist

Track worktree state throughout the session:

1. **Before fan-out:** `git worktree list` — verify clean state (no
   leftovers from previous sessions)
2. **During fan-out:** track which agent is in which worktree and what
   it's doing (agent IDs are random hashes — document the mapping)
3. **After each agent completes:** verify commit exists in the worktree
   branch (`git log --oneline <branch> -3`)
4. **If agent wrote to main instead of worktree:** `git status`
   immediately — the work may need to be stashed or committed before
   other agents merge
5. **After all agents complete:** `git worktree prune` — clean up
6. **Session end:** `git worktree list` must show only the main worktree

**Include in session-end checklist:**
```bash
# Verify no abandoned worktrees
git worktree list
# Should show only one entry (the main working directory)
# If extras exist, either merge their work or prune them
git worktree prune
```

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
