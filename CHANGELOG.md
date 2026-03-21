# Changelog

All notable changes to rebar. Versioned with [semver](https://semver.org/).

**Versioning policy:**
- **Major** — breaking changes to contract format, agent role structure, CLI interface, or enforcement script API
- **Minor** — new agents, templates, practices, profiles, scripts
- **Patch** — doc fixes, script bug fixes, template clarifications

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
