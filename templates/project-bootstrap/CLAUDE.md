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
1. **Read the Cold Start Quad:**
   - `README.md` — project overview
   - `QUICKCONTEXT.md` — current state
   - `TODO.md` — active work
   - `AGENTS.md` — coordination guidelines

2. **Check project health:**
   ```bash
   git status
   scripts/check-contract-refs.sh
   ask steward summary
   ```

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