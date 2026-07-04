# Claude Code Configuration

**Claude-specific settings and context for this project.**

---

## Project Context

### What This Project Does
[Brief description of the project's purpose and main functionality]

### Technology Stack
- **Language:** [Primary programming language]
- **Framework:** [Web framework, if applicable]
- **Database:** [Database system, if applicable]
- **Dependencies:** [Key libraries or tools]

### Key Architecture Patterns
- **Contract-driven development** — behavioral specifications before implementation
- **Progressive testing cascade** — T0 (unit) → T1 (integration) → T2+ (security, system)
- **Role-based agent coordination** — architect, product, englead, steward perspectives

---

## Development Workflow

### Starting a Session

**Health checks run automatically.** A `SessionStart` hook (configured in
`.claude/settings.json`) runs `scripts/cold-start-checks.sh` before your
first turn. Its output appears wrapped in
`<rebar-cold-start>...</rebar-cold-start>` tags — treat that block as
ground truth from the harness, not prose to interpret. It reports one
pass/fail line per enforcement check (`check-contract-refs`,
`check-todos`, `check-freshness`, `check-ground-truth`) plus a maturity
count of contract `Status:` fields. The hook always exits 0: failures
are made visible, never blocking — read them before trusting any doc
claims.

**Self-check:** if no `<rebar-cold-start>` block appeared at the top of
your first turn, the hook is not installed (or this harness doesn't
support hooks). Fall back to running the checks manually.

**Manual fallback (harnesses without hooks, or to re-check mid-session):**
```bash
scripts/cold-start-checks.sh       # same summary block the hook injects
scripts/check-contract-refs.sh     # or run the quad individually for full detail
scripts/check-todos.sh
scripts/check-freshness.sh
scripts/check-ground-truth.sh
```

**Still on you** — the hook checks health; it doesn't orient you:
1. **Read the Cold Start Quad:**
   - `README.md` — project overview
   - `QUICKCONTEXT.md` — current state
   - **VERIFY:** `git log --since='7 days' --oneline | head -20` — cross-reference
     against QUICKCONTEXT claims. If the `last-synced` date is >1 week old,
     treat ALL claims as suspect.
   - `TODO.md` — active work (open items only)
   - `AGENTS.md` — coordination guidelines

2. **Check working-tree state:**
   ```bash
   git status
   git worktree list              # Check for abandoned worktrees
   scripts/refresh-context.sh     # Context refresh helper (if available)
   ask steward summary
   ```

### Ending a Session
1. **Update QUICKCONTEXT.md** with current project state
2. **Update TODO.md** — mark completed items, add newly discovered items
3. **Clean up:** `git worktree prune`, commit any uncommitted work
4. **Write a wrapup** — see `practices/session-lifecycle.md` for the template
5. **Verify:** does QUICKCONTEXT match `git log --oneline -10`?

### Adding a Feature
1. **Define success** — BDD scenario or user story
2. **Design contract** — behavioral specification in `architecture/`
3. **Get coordination:**
   ```bash
   ask architect "system design questions"
   ask product "user experience questions"
   ask englead "implementation planning"
   ```
4. **Implement** following the contract specification
5. **Test** following the T0-T1 cascade
6. **Quality gates** before committing

### Making Changes
- **Before modifying existing code:** Check its contract in `architecture/`
- **When adding new components:** Write contract first
- **Before breaking changes:** Coordinate with relevant agents

---

## Quality Standards

### Contract Compliance
- Every source file has `CONTRACT:` header
- All `CONTRACT:` refs point to valid files
- Behavioral specifications match implementation

### Testing Requirements
- **T0 (Unit):** Test contract behavior directly
- **T1 (Integration):** Test component interactions
- **T2+ (Advanced):** Add as system complexity grows

### Documentation
- Keep `QUICKCONTEXT.md` current with project state
- Track all tasks in `TODO.md` (no untracked `TODO:` comments)
- Update contract specifications when behavior changes

---

## Agent Coordination

### Role-Based Queries
```bash
# System design and technical architecture
ask architect "Should we add caching? What are the trade-offs?"

# User experience and product requirements
ask product "Does this design meet user needs? Any missing scenarios?"

# Implementation planning and delivery
ask englead "Timeline estimate? Any delivery risks?"

# Quality and compliance
ask steward "Health report? Any contract violations?"
```

### Multi-Agent Patterns
- **Design review:** architect + product + englead input before implementation
- **Quality gates:** steward scanning + test cascade before merge
- **Post-implementation:** merger coordination for integration

---

## Code Patterns

### Contract Headers
```[language]
// filename.ext
// CONTRACT:C1-COMPONENT.1.0
package/module declaration
```

### TODO Tracking
```[language]
// TRACKED-TASK:TODO.md#feature-name Specific task description
// NOT: TODO: vague comment (this blocks commit)
```

### Error Handling
Follow contract specifications for error types and behaviors.

---

## Project-Specific Context

### Domain Knowledge
[Key business logic, domain concepts, or technical constraints that Claude should understand when working on this project]

### Integration Points
[External services, APIs, or systems this project connects to]

### Deployment Context
[How code gets deployed, environment differences, rollback procedures]

### Team Context
- **Rebar tier:** 1 (Partial) — basic contract + TODO enforcement
- **Team size:** [Solo/Small/Large team context]
- **Quality requirements:** [Security, performance, compliance needs]

---

## File Ignore Patterns

When working on this project, generally avoid modifying:
- `scripts/` — rebar enforcement scripts (unless specifically updating rebar)
- `architecture/.state/` — computed steward output
- Configuration files unless specifically requested

Focus on:
- Source code implementation
- Test files
- Documentation updates
- Contract specifications

---

## Success Indicators

### Good Session Outcomes
- Clear understanding of current project state
- Changes follow contract specifications
- Quality gates pass before committing
- Documentation stays current
- Agent coordination leads to better decisions

### Watch Out For
- Untracked `TODO:` comments in code
- Contract references that don't match files
- Breaking changes without coordination
- Stale documentation or metrics
- Working in isolation when coordination would help

---

**Remember:** This project uses rebar's contract-driven development and agent coordination patterns. When in doubt, read contracts, ask role-based agents for guidance, and maintain quality gates throughout development.