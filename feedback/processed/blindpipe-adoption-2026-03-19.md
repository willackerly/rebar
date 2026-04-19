# Feedback: blindpipe adoption of rebar (2026-03-19)

**Project:** blindpipe — zero-knowledge collaborative office suite (Go + TypeScript, crypto-critical)
**Adopter context:** Mature codebase with existing Cold Start Quad, contract system (`pkg/contracts/`), and nascent `agents/` dir

---

## What worked well

### 1. Selective adoption over wholesale bootstrap

blindpipe already had mature versions of the Cold Start Quad (CLAUDE.md, AGENTS.md, QUICKCONTEXT.md, TODO.md). Replacing them with templates would have been a regression. The right move was keeping blindpipe's docs and **merging rebar's gaps in** — the contract header convention, enforcement scripts, METRICS ground truth, and Steward system were the highest-value additions.

**Lesson for rebar:** Consider documenting an "adoption for mature projects" path alongside the bootstrap path. The current SETUP.md assumes starting from scratch.

### 2. Enforcement scripts are the real value

The templates are nice, but what made the biggest immediate impact was the *enforcement automation*: `check-contract-headers.sh`, `check-ground-truth.sh`, `steward.sh`. These are what keep docs honest. blindpipe had `check-todos.sh` and `check-doc-freshness.sh` but lacked the contract enforcement family.

### 3. Role-based agent structure provides natural knowledge partitioning

Setting up architect/product/englead/steward/merger directories — even before using them heavily — immediately clarified "who owns what" in a way that a flat docs/ tree doesn't. When a new agent session starts, it can claim a role and load only that role's context.

### 4. Subagent templates filled a real gap

blindpipe had identified 6 high-value template ideas but only built 1 example. Rebar's 8 battle-tested templates were immediately usable. The `security-surface-scan.md` template is especially valuable for a crypto-critical project.

---

## Key insight: ASK CLI as context preservation, not just convenience

The initial assessment was that ASK was duplicative with Claude Code's Agent tool. This was wrong. The critical difference:

**ASK agents maintain persistent sessions.** The first question sends full role context; follow-ups resume the session without re-sending everything. This means:

- **10 questions to the architect costs ~1x context load, not 10x** — massive token savings
- **Your own context window stays clean** — the expert's knowledge lives in their session, not yours
- **The architect accumulates understanding** — it remembers what it learned 5 questions ago
- **Works outside Claude Code** — any terminal, any team member, any AI tool

This is fundamentally different from Claude Code subagents, which are ephemeral (fresh context per invocation) and whose results get injected back into the parent's context window.

**Recommendation for rebar:** Lead with this "context preservation" framing in the ASK docs. The current README explains the mechanics but undersells the *why*. "ASK saves tokens and keeps your context clean" is a stronger pitch than "ASK queries role-based agents."

---

## What needed adaptation

### 1. Dual contract systems

blindpipe already had `pkg/contracts/` (Go interfaces) and `docs/specifications/` (prose specs). Adding `architecture/CONTRACT-*.md` creates a third representation. The resolution was to make architecture/ contracts *reference* the existing specs and Go interfaces rather than replace them. Over time, architecture/ becomes the authoritative lifecycle tracker.

**Suggestion:** Rebar could document this pattern — "adopting contracts when you already have interfaces/specs" — as a common scenario.

### 2. check-todos.sh was strictly improved

Rebar's version scans 7 file types (.go, .ts, .tsx, .js, .jsx, .py, .rs, .rego) across more directories. blindpipe's original only scanned .go and .rego. The upgrade was a strict improvement, but it surfaced untracked TODOs in TypeScript that needed a cleanup pass. Worth warning about in adoption docs.

### 3. Blindpipe-specific subagent guidelines

The merged `subagent-guidelines.md` needed blindpipe-specific additions to "Architectural Change Detection": ABAC attribute schema changes, P2P protocol changes, TDF format changes, and a full Crypto Rules section. These are domain-specific and can't be templated generically.

---

## Role discipline pattern (new — consider for rebar)

We embedded a "role discipline" pattern that rebar doesn't currently have:

**Every Claude Code session should assert a role at startup.** Based on the user's request, the agent claims architect/englead/product/steward/merger. If work spans roles:

1. **Quick factual question?** → `ask <role> "question"` — persistent session, no context pollution
2. **Substantial cross-role work?** → Spin up a separate Claude Code instance or subagent
3. **Need to coordinate?** → Write to the other role's inbox

This prevents the common failure mode where a single agent session tries to be everything — architect, implementer, tester, product owner — and burns context on all of it. Role discipline + ASK means each role has clean, focused context.

**Suggestion:** Consider adding a "Role Discipline" section to AGENTS.template.md. It pairs naturally with the agent role definitions and the ASK CLI.

---

## Adoption checklist (what we actually did)

1. Copied enforcement scripts (8 new + 1 upgraded check-todos.sh)
2. Created `architecture/` skeleton (template, registry with 12 planned contracts, .state/)
3. Created `METRICS` ground truth file (7 verified metrics)
4. Set up 5 role-based agents with AGENT.md + commands/
5. Copied and adapted 8 subagent templates
6. Merged subagent-guidelines.md (rebar base + blindpipe crypto/ABAC additions)
7. Copied reference docs (DESIGN.md, conventions.md, crypto-library profile)
8. Installed ASK CLI (`bin/ask`, `bin/ask-agent-loop`)
9. Updated existing docs (CLAUDE.md, AGENTS.md, TODO.md, .gitignore)
10. Embedded role discipline pattern in AGENTS.md
11. Did NOT replace Cold Start Quad docs (blindpipe's were more mature)
12. Did NOT install ASK CLI initially (corrected after recognizing persistent session value)

Total: 60+ files, ~5000 lines. One commit for infrastructure, one for ASK + role discipline.
