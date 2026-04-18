# Changelog

All notable changes to rebar. Versioned with [semver](https://semver.org/).

**Versioning policy:**
- **Major** — breaking changes to contract format, agent role structure, CLI interface, or enforcement script API
- **Minor** — new agents, templates, practices, profiles, scripts
- **Patch** — doc fixes, script bug fixes, template clarifications

---

## v2.0.0 (2026-04-01)

### Added
- **Session lifecycle protocol** — `practices/session-lifecycle.md`: start/checkpoint/end framework with wrapup template, marathon session guidance, architect review checkpoints
- **Red team protocol** — `practices/red-team-protocol.md`: 5-persona adversarial review strategy (adversarial user, performance, security, fidelity, API/contract)
- **Visual fidelity methodology** — `practices/visual-fidelity.md`: ground truth, RMSE measurement, oracle pattern, human emulator tests, regression prevention
- **Red team subagent template** — `agents/subagent-prompts/red-team.md`: multi-persona template with structured JSON output and fix DAG
- **Product review subagent template** — `agents/subagent-prompts/product-review.md`: BDD alignment, persona fit, flow completeness, scope assessment
- **Seam contracts** — new contract type for integration points across language/protocol boundaries; `architecture/CONTRACT-SEAM-TEMPLATE.md`
- **`rebar context` CLI command** — context shepherd that cats role-relevant files in reading order (`rebar context`, `rebar context architect`, `rebar context session-start`, etc.)
- **Context refresh script** — `templates/project-bootstrap/scripts/refresh-context.sh`: automated QUICKCONTEXT staleness checker
- **Feedback status tracking** — `feedback/README.md` now includes `Status:` and `Template impact:` fields

### Changed
- **Cold Start Quad enhanced** — staleness verification step added between QUICKCONTEXT and TODO reads
- **AGENTS.template.md** — session lifecycle reference, priority tracking rule (QUICKCONTEXT is single source of truth), issue dedup rule
- **CLAUDE.template.md** — session-end protocol added, `refresh-context.sh` in health check
- **TODO.template.md** — forward-looking only (<50 lines open items), completed items in collapsed `<details>` section
- **QUICKCONTEXT.template.md** — "What's Next" section as canonical priority list
- **Worktree collaboration** — fan-out merge ordering strategy (HOT/WARM/COLD), shared mock consolidation rule, worktree lifecycle checklist, cherry-pick best practices
- **Multi-agent orchestration** — file-level conflict matrix and interface-change sequencing in pre-launch audit
- **UX review template** — interaction stability (human emulator) dimension added
- **Security surface scan template** — red team mode (adversarial mindset) section added
- **DESIGN.md** — seam contracts in §3 (The Contract System), session lifecycle in §5 (The Information Environment)
- **README.md** — battle-tested results updated with OpenDocKit (15+ agents, 8K tests) and filedag (40+ agents, 62 commits in 48hrs)

### Why v2.0

Three field reports from production deployments (Dapple SafeSign, OpenDocKit, filedag) converged on the same meta-insight: rebar's structural protocols work; its behavioral protocols don't. v2.0 addresses the session lifecycle gap, adds adversarial review and visual fidelity practices, introduces seam contracts for integration points, and adds the `rebar context` CLI command. The core contract system is unchanged — this is a methodology evolution, not a rewrite.

### Migration from v1.2.0
1. Copy new practice files: `practices/session-lifecycle.md`, `practices/red-team-protocol.md`, `practices/visual-fidelity.md`
2. Copy new subagent templates: `agents/subagent-prompts/red-team.md`, `agents/subagent-prompts/product-review.md`
3. Copy `architecture/CONTRACT-SEAM-TEMPLATE.md`
4. Copy `templates/project-bootstrap/scripts/refresh-context.sh` to your project's `scripts/`
5. Re-diff your AGENTS.md against the new template — session lifecycle and priority tracking sections are new
6. Re-diff your CLAUDE.md — session-end protocol and staleness verification are new
7. Consider shortening your TODO.md — move completed items to a collapsed section
8. Add a "What's Next" section to QUICKCONTEXT.md as your canonical priority list
9. Rebuild rebar CLI: `cd cli && go build -o ../bin/rebar .`
10. Update `.rebar-version` to `v2.0.0`

---

## v1.2.0 (2026-03-20)

### Added
- **Scalability overhaul** — AGENTS.template.md slimmed from 917 to 382 lines
- **`practices/` directory** — multi-agent orchestration, E2E testing, deployment patterns, worktree collaboration extracted as reference guides
- **Computed registry** — `scripts/compute-registry.sh` generates CONTRACT-REGISTRY.md from contract files on disk (replaces manual maintenance)
- **Memory compaction** — `ask compact <agent>` summarizes old memory entries, auto-triggers at 50KB threshold
- **Script versioning** — all scripts have `# rebar-scripts: YYYY.MM.DD` headers; steward detects stale copies
- **Tier-aware enforcement** — `.rebarrc` configures tier (1=partial, 2=adopted, 3=enforced); scripts skip inapplicable checks
- **Team-size profiles** — `profiles/solo-dev.md`, `profiles/small-team.md`, `profiles/department.md`
- **Conventions minimum viable** — Tier 1 section at top of conventions.md
- **Rebar version tracking** — `.rebar-version` file + README badge for adopting repos
- **Adoption level validation** — steward checks README badge against actual compliance
- **CHANGELOG.md** — you're reading it

### Changed
- `AGENTS.template.md` — mandatory foundations only; advanced practices moved to `practices/`
- `architecture/CONTRACT-REGISTRY.template.md` — now documents the computed format
- `scripts/check-registry.sh` — deprecated in favor of `compute-registry.sh`
- `scripts/steward.sh` — uses `compute-registry.sh --check`, adds compliance validation
- `scripts/ci-check.sh` — uses `compute-registry.sh --check`
- `conventions.md` — added minimum viable section for Tier 1
- `SETUP.md` — added tier selection, team-size profiles, practices/ copy step
- All profile files — updated section references for practices/

### Migration from v1.1.0
1. Copy `practices/` directory into your project
2. Copy `scripts/compute-registry.sh` and `scripts/_rebar-config.sh`
3. Create `.rebarrc` from `.rebarrc.template` (set your tier)
4. Create `.rebar-version` with `v1.2.0`
5. Add rebar badge to top of your README.md (see README.template.md)
6. Re-diff your AGENTS.md against the new AGENTS.template.md — moved sections are now in `practices/`
7. Run `scripts/compute-registry.sh` to generate your registry
8. Optional: update all scripts from rebar (check `# rebar-scripts:` dates)

---

## v1.1.0 (2026-03-19)

### Added
- Merger agent — branch integration + conflict resolution (actor agent)
- Subagent LOE levels in prompt index
- Scout Rule — zero tolerance for skipped or failing tests
- Adoption badges — PARTIAL / ADOPTED / ENFORCED tiers
- AI-native contracts feedback — cross-repo namespacing (`CONTRACT:namespace/ID`)
- Blindpipe adoption feedback — ASK as context preservation, role discipline pattern

### Changed
- `ask init` now creates merger agent directory
- `AGENTS.template.md` — added Scout Rule section, merge coordinator flow

### Migration from v1.0.0
1. Copy `agents/merger/` directory
2. Merge Scout Rule section into your AGENTS.md (§Testing Expectations)
3. Optional: add adoption badge to README.md

---

## v1.0.0 (2026-03-17)

### Added
- Cold Start Quad templates — README, QUICKCONTEXT, TODO, AGENTS
- CLAUDE.md template with full Claude Code configuration
- Contract system — CONTRACT-TEMPLATE, CONTRACT-REGISTRY, naming conventions
- Methodology.md — full philosophy document
- Conventions.md — branch naming, commits, headers, discovery taxonomy
- 5 enforcement scripts — contract-headers, contract-refs, TODOs, freshness, ground-truth
- Steward — automated project health scanner with JSON + markdown output
- CI check runner and pre-commit hook
- ASK CLI — role-based agent queries with persistent sessions
- 8 subagent templates — code review, contract audit, security scan, UX review, doc drift, feature inventory, test shard, merge coordinator
- 4 project profiles — web-app, api-service, crypto-library, cli-tool
- SETUP.md — step-by-step adoption guide
- METRICS template with ground truth verification
- Learnings from OpenDocKit — 37KB of battle-tested patterns

### Migration
This is the initial release. Follow SETUP.md.
