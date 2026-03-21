# Profile: Department / Multi-Team

For 10+ developers across multiple teams with cross-repo dependencies.
Full enforcement, shared agent memory, contract catalog.

## Recommended Tier: 3 (Enforced)

Set in `.rebarrc`:
```
tier = 3
```

Enforces everything: contract headers, refs, TODOs, freshness, registry, ground truth, strict steward.

## What to Copy (Everything + Cross-Repo Tooling)

Everything from [small-team.md](small-team.md), plus:

| Addition | Purpose | Effort |
|----------|---------|--------|
| **Contract catalog repo** | Cross-repo contract discovery | 1-2 days |
| **CI-triggered catalog collection** | Keep catalog fresh automatically | 1 hour per repo |
| **Cross-repo namespacing** | `CONTRACT:blindpipe/C1-BLOBSTORE.2.1` | Convention only |
| **AI-native frontmatter** | Machine-readable contract metadata | Per contract |
| **Shared ASK memory** | Un-gitignore `memory.log.md` for team knowledge | 5 minutes |
| **Breaking change detection** | Catalog script that diffs version bumps | 1 hour |

## The Contract Catalog

The keystone addition at this tier. A shared git repo that aggregates steward reports:

```
contract-catalog/
├── reports/
│   ├── blindpipe.json
│   ├── office180.json
│   └── opendockit.json
├── index.md           ← auto-generated: all contracts by domain
├── deps.md            ← auto-generated: dependency graph
├── changes.md         ← auto-generated: version bumps since last run
└── scripts/
    ├── collect.sh
    └── build-index.sh
```

**Collection is CI-triggered, not manual.** Each repo's CI pushes its steward-report.json to the catalog after every merge to main. The catalog is always fresh.

## Cross-Repo Contract References

Use namespace prefixes for cross-repo dependencies:

```
CONTRACT:C1-APP-REGISTRY.1.0           ← local (no prefix = this repo)
CONTRACT:blindpipe/C1-BLOBSTORE.2.1    ← cross-repo (namespace = repo name)
```

`grep -rn "CONTRACT:blindpipe/"` finds all cross-repo dependencies instantly.

## Shared Agent Memory

Un-gitignore `agents/*/memory.log.md` so agent knowledge accumulates across the team:

- `memory.log.md` is append-only (merge-conflict-safe: both sides appended different entries)
- `memory.md` is generated via `ask compact <agent>` (never a conflict source)
- Run `ask compact <agent>` periodically to keep memory manageable

## Breaking Change Detection

When a contract version bumps, the catalog knows who depends on it:

```bash
# In catalog CI: detect version bumps since last collection
# BREAKING: blindpipe/C1-BLOBSTORE bumped 2.1 → 3.0
#   Affected: office180 (14 refs), opendockit (3 refs)
```

This is a script in the catalog repo, not a service. ~50 lines of bash + jq.

## Onboarding Funnel

New hires follow this path:

```
1. Org-level README (in catalog repo)     → 5 min
2. Catalog index.md (contracts by domain)  → 10 min
3. Pick your team's repo → Cold Start Quad → 30 min
4. ask architect "what should I know?"     → 5 min
```

## Practice Files

All `practices/` files are relevant at this tier. Teams should customize them for their specific patterns.

## Security-Tier Contracts (Optional)

For repos with security-critical contracts, add tiers to CODEOWNERS:

```
# .github/CODEOWNERS
architecture/CONTRACT-S2-AUTH*    @security-team @tech-leads
architecture/CONTRACT-*           @architecture-team
```

## When to Level Up

The scalability assessment describes Tier 4 (enterprise) for 50+ devs / 40+ repos.
At that scale, the catalog becomes a service, you add a formal breaking change workflow
(RFC → review → deprecation → migration → retire), and governance layers emerge.
See `feedback/scalability-assessment-2026-03-20.md` for the full roadmap.

## Setup Time

~2 hours for catalog setup + CI integration. Each additional repo: ~30 minutes.
