# Feedback: Rebar needs versioning, upgrade paths, and compliance markers

**Date:** 2026-03-20
**Source:** Core framework (README, SETUP, adoption flow)
**Type:** missing-feature
**From:** FontKit adoption session — comparing adopted rebar vs latest repo

## What Happened

FontKit was scaffolded from rebar on 2026-03-19. One day later, the rebar
repo had already evolved (scalability assessment, AI-native contracts,
adoption badges, project profiles). But FontKit has no way to know:

1. **Which version it adopted** — no marker, no tag, no `.rebar-version`
2. **What it's missing** — no changelog, no diff tooling
3. **How to upgrade** — no migration guide between versions

The rebar repo itself has no git tags. "v1.0" is mentioned in a commit
message (`de49484`) but was never formally tagged. Version identification
requires reading commit history and inferring from feature completeness.

This means every rebar adopter is running an opaque snapshot with no
upgrade path. As the framework evolves, adopters silently fall behind.

## What Was Expected

A clear version contract between rebar and its adopters:

- Rebar declares its version (semver tags)
- Adopters declare which version they adopted
- A mechanism exists to detect drift and guide upgrades

## Suggestion

### 1. Semantic Versioning with Git Tags

Tag the repo with semver releases:

```
v1.0.0  — original release (de49484)
v1.1.0  — merger agent, subagent LOE, scout rule
v1.2.0  — scalability tiers, AI-native contracts, project profiles
```

**Versioning policy:**
- **Major** — breaking changes to contract format, agent role structure,
  CLI interface, or enforcement script API
- **Minor** — new agents, templates, practices, profiles, scripts
- **Patch** — doc fixes, script bug fixes, template clarifications

### 2. Version Declaration in Adopting Repos

Adopters declare their rebar version. Two complementary mechanisms:

**Machine-readable file** (for tooling):
```
# .rebar-version
v1.2.0
```

**Human-readable badge** (for orientation):
```markdown
<!-- Top of README.md -->
> Built with [rebar](https://github.com/willackerly/rebar) v1.2.0 | ADOPTED
```

The steward can validate this:
- `check-rebar-version.sh` reads `.rebar-version`
- Compares against latest tag (via `git ls-remote` or a pinned URL)
- Reports as INFO if behind, WARN if more than one minor behind

### 3. CHANGELOG.md

Standard changelog in the rebar repo with per-version entries:

```markdown
## v1.2.0 (2026-03-20)

### Added
- Project profiles: web-app, api-service, crypto-library, cli-tool,
  solo-dev, small-team, department (profiles/)
- Scalability assessment with tier 1-4 adoption curve (feedback/)
- AI-native contracts: cross-repo namespacing, MCP tool schemas (feedback/)
- Adoption badges: partial / adopted / enforced

### Changed
- _rebar-config.sh now supports tiered enforcement levels

### Migration
- Copy `profiles/` directory into your project
- No breaking changes — all additions are optional
```

The "Migration" section per version is the key innovation. It tells
adopters exactly what to do: which files to copy, which to merge, which
to delete.

### 4. Upgrade Command (Future)

A `rebar upgrade` command that:
1. Reads `.rebar-version` from the project
2. Diffs project's rebar files against the target version
3. Reports: new files to copy, changed files to review, removed files
4. Optionally applies non-conflicting updates automatically

This could be a bash script initially (`bin/rebar-upgrade`) and evolve
into part of the ASK CLI later.

### 5. Adoption Level Validation

The README badge claims an adoption level (PARTIAL / ADOPTED / ENFORCED).
The steward should validate this claim against reality:

| Level | Requirements |
|-------|-------------|
| PARTIAL | Cold Start Quad exists, ≥1 contract, contract headers in source |
| ADOPTED | + steward scanning, enforcement scripts, CI integration |
| ENFORCED | + pre-commit hooks, all checks passing, zero tolerance testing |

If the README claims ENFORCED but pre-commit hooks aren't installed,
the steward flags it as DRIFT.

## Impact

Without versioning:
- Every adopter is a fork with no upstream tracking
- Framework improvements don't propagate
- Bug fixes in scripts (like the pipefail bug we found in
  `check-freshness.sh` today) stay siloed in individual projects
- The "adoption" story is really "copy once, diverge forever"

With versioning:
- Adopters know where they stand
- Upgrades are deliberate and guided
- Bug fixes flow upstream (fix in rebar, adopters pull on next upgrade)
- The framework becomes a living dependency, not a dead scaffold
