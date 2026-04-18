# Rebar Site Map & Learning Progression

**Visual guide to rebar's progressive disclosure architecture**

```
📍 START HERE: README.md (Gateway)
├── 🚀 Try It (5 minutes) ──────────┐
├── ❤️ Love It (1 hour) ───────────┐ │
└── 🎯 Master It (ongoing) ────────┐ │ │
                                  │ │ │
                                  V V V
┌─────────────────────────────────────────────────────────┐
│                 🚀 TRY IT LAYER                         │
│                   (Essential)                           │
├─────────────────────────────────────────────────────────┤
│ QUICKSTART.md           5-min solo dev setup            │
│ CONTRACT-QUICKSTART.md  Write your first contract       │
│ Cold Start Quad         Essential session files         │
│ ├─ QUICKCONTEXT.md     What's true right now           │
│ ├─ TODO.md             What needs doing                │
│ ├─ AGENTS.md           How we work                     │
│ └─ CLAUDE.md           Claude Code config              │
└─────────────────────────────────────────────────────────┘
                                  │
                                  V
┌─────────────────────────────────────────────────────────┐
│                 ❤️ LOVE IT LAYER                        │
│                  (Advanced)                             │
├─────────────────────────────────────────────────────────┤
│ FEATURE-DEVELOPMENT.md  Complete BDD→Contract→Code flow │
│ AGENTS-QUICKSTART.md    Role agents vs subagent temps  │
│ CONTRACT-QUICKSTART.md  5-minute contract writing       │
│ Testing Cascade T0-T2   Unit → Integration → Security  │
│ Quality Enforcement     Basic contract refs + TODOs    │
└─────────────────────────────────────────────────────────┘
                                  │
                                  V
┌─────────────────────────────────────────────────────────┐
│                🎯 MASTER IT LAYER                       │
│                   (Expert)                              │
├─────────────────────────────────────────────────────────┤
│ Problem-Based Navigation                                │
│ ├─ CASE-STUDIES.md      Real-world solutions            │
│ ├─ practices/           Specialized workflows           │
│ └─ feedback/            Battle-tested patterns          │
│                                                         │
│ Session & Coordination                                  │
│ ├─ Session lifecycle (start/checkpoint/end)             │
│ ├─ Multi-agent orchestration                           │
│ ├─ Worktree collaboration + merge strategy              │
│ ├─ Red team protocol                                    │
│ ├─ Cross-repo patterns                                  │
│ └─ Enterprise scaling                                   │
│                                                         │
│ Specialized Practices                                   │
│ ├─ Visual fidelity methodology                          │
│ ├─ Seam contracts (integration points)                  │
│ └─ rebar context CLI (context shepherd)                 │
│                                                         │
│ Deep Philosophy                                         │
│ ├─ DESIGN.md            Complete methodology            │
│ ├─ conventions.md       Standards & naming              │
│ └─ architecture/        Contract system reference       │
└─────────────────────────────────────────────────────────┘
```

---

## User Journey Flows

### 🆕 **New Team Member** (First Session)
```
README.md → QUICKSTART.md → First Contract → Success!
     ↓
QUICKCONTEXT.md → TODO.md → AGENTS.md → Start Working
     ↓
(When questions arise) → Ask role agents or check CASE-STUDIES.md
```

### 👩‍💻 **Feature Developer** (Daily Workflow)
```
AGENTS.md (workflow) → FEATURE-DEVELOPMENT.md (methodology)
     ↓
BDD Scenario → Contract Design → Agent Coordination → Implementation
     ↓
Quality Cascade (T0-T2) → Integration → Ship
```

### 🏗️ **Architect/Tech Lead** (Strategic Decisions)
```
DESIGN.md (philosophy) → CASE-STUDIES.md (patterns)
     ↓
Scaling Decision: profiles/ → Tier Selection → Implementation Plan
     ↓
Advanced Patterns: practices/ → Specialized Implementation
```

### 🤖 **Agent Coordinator** (Swarm Management)
```
AGENTS-QUICKSTART.md (ecosystem) → Role vs Template Decision
     ↓
practices/multi-agent-orchestration.md → Fan-out Planning
     ↓
Execution → practices/worktree-collaboration.md → Integration
```

---

## Content Labeling System

### 🟢 **Essential** (Try It)
*Required for basic rebar adoption*
- README.md
- QUICKSTART.md
- CONTRACT-QUICKSTART.md
- Cold Start Quad templates
- Basic contract headers (CONTRACT: comments)

### 🟡 **Advanced** (Love It)
*Valuable for productive development*
- FEATURE-DEVELOPMENT.md
- AGENTS-QUICKSTART.md
- Testing cascade T0-T2
- Role-based agent coordination
- Quality enforcement automation

### 🔴 **Expert** (Master It)
*Specialized for complex scenarios*
- Multi-agent orchestration
- Cross-repo coordination
- Enterprise scaling patterns
- Advanced contract versioning
- Swarm collective learning

---

## Quick Reference Cards

### **I need to..."**

| Task | File | Time |
|------|------|------|
| Get started immediately | [QUICKSTART.md](QUICKSTART.md) | 5 min |
| Write my first contract | [CONTRACT-QUICKSTART.md](CONTRACT-QUICKSTART.md) | 5 min |
| See the full workflow | [FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md) | 1 hour |
| Use agents effectively | [AGENTS-QUICKSTART.md](AGENTS-QUICKSTART.md) | 15 min |
| Find real-world solutions | [CASE-STUDIES.md](CASE-STUDIES.md) | As needed |
| Scale beyond solo dev | [profiles/](profiles/) + [CASE-STUDIES.md](CASE-STUDIES.md) | 30 min |

### **I'm stuck with..."**

| Problem | Solution | File |
|---------|----------|------|
| Numbers drift from reality | Ground truth enforcement | [Human-based Digital Signer case](feedback/digital-signer-feedback.md) |
| Mature codebase adoption | Selective adoption strategy | [blindpipe case](feedback/blindpipe-adoption-2026-03-19.md) |
| Agent coordination failures | Swarm orchestration protocols | [OpenDocKit case](feedback/2026-03-18-opendockit-fidelity-session.md) |
| Merge conflicts | Worktree collaboration | [practices/worktree-collaboration.md](practices/worktree-collaboration.md) |
| Quality gates | Testing cascade + enforcement | [FEATURE-DEVELOPMENT.md](FEATURE-DEVELOPMENT.md) |
| Scaling decisions | Tier progression patterns | [Scalability Assessment](feedback/scalability-assessment-2026-03-20.md) |

### **Common Commands**

```bash
# Getting started
ask who                          # List available agents
ask steward summary              # Health check
scripts/check-compliance.sh      # Verify setup

# Daily development
ask architect "<question>"       # Design guidance
ask product "<question>"         # User perspective
claude --prompt agents/subagent-prompts/code-review.md

# Quality gates
scripts/ci-check.sh             # Full quality scan
ask steward "health report"     # Contract status
```

---

## Troubleshooting Guide

### 🚨 **"I'm overwhelmed by all the files"**
**→ Start with just README.md and QUICKSTART.md**
- Ignore everything else initially
- Follow the 5-minute solo dev setup
- Write one contract, link it to code
- Everything else can wait

### 🚨 **"Agents aren't giving good answers"**
**→ Check the Cold Start Quad**
- Make sure QUICKCONTEXT.md is current
- Verify TODO.md has active work
- Check that AGENTS.md reflects your workflow
- Use `ask reset <agent>` to clear session

### 🚨 **"Contract system seems complex"**
**→ Use CONTRACT-QUICKSTART.md**
- Start with the 2-minute concept
- Use the quick template
- Write behavioral specs, not just interfaces
- Link with `CONTRACT:` comments in code

### 🚨 **"Multi-agent coordination is confusing"**
**→ Start with AGENTS-QUICKSTART.md**
- Understand role agents vs subagent templates
- Use decision tree: questions → role agents, work → templates
- See FEATURE-DEVELOPMENT.md for coordination example

### 🚨 **"Setup doesn't work for my project"**
**→ Check adoption patterns**
- New project: SETUP.md
- Mature project: [blindpipe case](feedback/blindpipe-adoption-2026-03-19.md)
- Specific project type: [profiles/](profiles/)

### 🚨 **"Quality enforcement is too strict/loose"**
**→ Adjust your tier**
- Edit `.rebarrc` to change tier (1=Partial, 2=Adopted, 3=Enforced)
- See [scalability patterns](feedback/scalability-assessment-2026-03-20.md)
- Check [zero-tolerance testing](feedback/zero-tolerance-testing-feedback.md) for extremes

### 🚨 **"Can't find what I need"**
**→ Use problem-based navigation**
- [CASE-STUDIES.md](CASE-STUDIES.md) for "someone solved this before"
- [practices/](practices/) for specialized workflows
- `ask steward` for contract health issues
- `ask architect` for design questions

---

## Navigation Breadcrumbs

*These appear at the top of key files to guide progression*

### QUICKSTART.md
```markdown
📍 **You are here:** Try It (5 min) → [Love It (1 hour)](FEATURE-DEVELOPMENT.md) → [Master It](CASE-STUDIES.md)
**Next step:** Write your first contract with [CONTRACT-QUICKSTART.md](CONTRACT-QUICKSTART.md)
```

### FEATURE-DEVELOPMENT.md
```markdown
📍 **You are here:** [Try It](QUICKSTART.md) → Love It (1 hour) → [Master It](CASE-STUDIES.md)
**Prerequisites:** [QUICKSTART.md](QUICKSTART.md) complete
**Next step:** [Agent coordination](AGENTS-QUICKSTART.md) or [real-world patterns](CASE-STUDIES.md)
```

### CASE-STUDIES.md
```markdown
📍 **You are here:** [Try It](QUICKSTART.md) → [Love It](FEATURE-DEVELOPMENT.md) → Master It
**Prerequisites:** Basic rebar experience
**Deep dive:** [DESIGN.md](DESIGN.md) for complete philosophy
```

---

This site map ensures that anyone can find their appropriate entry point and progression path through rebar's progressive disclosure architecture, from 5-minute trial to expert-level coordination patterns.