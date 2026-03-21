# Deep Review: Scalability Assessment — Merge Noise, Error Modes, and the Tier 2→3 Gap

**Reviewer context:** Read the full rebar system (methodology, all templates, all 10 feedback docs, ASK CLI implementation, steward.sh, all enforcement scripts, all agent definitions) before writing this.
**Date:** 2026-03-20

---

## Verdict

The scalability assessment is fundamentally sound. "Layers, not modes" is the right philosophy. Git-only through Tier 3 is a genuine advantage. The tiered adoption curve maps real organizational pressure points correctly.

But it's optimistic in three specific areas: merge noise on shared files, unaddressed error modes that compound at scale, and the Tier 2→3 transition gap. All fixable within the existing substrate. None require new infrastructure.

---

## I. Merge Noise: The File Hotspot Problem

The assessment identifies QUICKCONTEXT.md and TODO.md as merge conflict sources at Tier 2 and proposes computed METRICS + auto-generated QUICKCONTEXT sections. Good. But the hotspot problem is bigger than those two files.

### The Five Hotspots

| File | Why It's Hot | When It Breaks | Assessment Coverage |
|------|-------------|----------------|-------------------|
| **QUICKCONTEXT.md** | Every session updates "What's In Progress" | 2+ concurrent devs | Addressed (auto-generate) |
| **TODO.md** | Discoveries section is append-only by multiple agents | 2+ concurrent agents | Partially addressed |
| **AGENTS.md** (39KB) | Active Workstreams section changes constantly | 2+ devs on different features | **Not addressed** |
| **CONTRACT-REGISTRY.md** | Every new contract adds a row | Contract creation sprints | **Not addressed** |
| **agents/*/memory.md** | Tier 3 un-gitignore proposal = every dev's session appends | 2+ devs sharing memory | **Not addressed** |

### Why AGENTS.md Is the Biggest Risk

AGENTS.md is 39KB. Most of it is stable norms (autonomy rules, testing cascade, collaboration patterns). But the "Active Workstreams" section at the bottom changes constantly — it's where you track who's doing what right now. At Tier 2 (4-15 devs), this section is a merge conflict every time two people push.

The fix isn't to make AGENTS.md smaller. The stable norms are valuable. The fix is to **split the volatile section out**:

```
AGENTS.md                    ← stable norms (changes rarely, merge conflicts rare)
WORKSTREAMS.md               ← who's doing what now (changes constantly, conflicts expected)
```

This is the same pattern as the computed METRICS proposal — separate the stable from the volatile. AGENTS.md becomes a file you read but rarely edit. WORKSTREAMS.md becomes a file you edit but that's small enough that conflicts are trivial to resolve (or auto-generated from TODO.md's P0 section).

### Why CONTRACT-REGISTRY.md Should Be Computed

The registry is a table listing all `architecture/CONTRACT-*.md` files. But the set of contract files IS the registry. The Dapple SafeSign adoption feedback already found that check-registry.sh's grep-based verification is fragile (table format doesn't match expected filenames).

At Tier 2+, the registry should be generated:

```bash
# scripts/compute-registry.sh
# Scans architecture/CONTRACT-*.md, generates CONTRACT-REGISTRY.md
# No more manual table maintenance. No more merge conflicts on the registry.
# The filesystem IS the registry. This script just renders it.
```

This follows the same principle as computed METRICS: if a file can be derived from the source of truth (contract files on disk), don't maintain it manually. The adoption-day-learnings feedback already showed that the registry format and the enforcement script expectations diverge — generating the registry eliminates this entire class of bugs.

### The Shared Memory Problem

The Tier 3 proposal to un-gitignore `agents/architect/memory.md` is elegant in concept: zero-infrastructure shared knowledge. But in practice:

1. **Concurrent appends conflict.** Two developers asking the architect questions simultaneously both append to memory.md. Git can't auto-merge appends to the same region.

2. **Memory grows unboundedly.** The IMPLEMENTATION.md roadmap mentions memory summarization for ASK v1, but Tier 3 shared memory is proposed NOW, before that exists. Without summarization, shared memory becomes 50KB of noise within weeks.

3. **Memory quality varies.** Developer A's architect session learns something wrong. That wrong knowledge is now shared to Developer B via git pull. There's no review gate on memory — unlike code, it bypasses PR review.

**Proposed fix: memory as append-only log with computed summary.**

```
agents/architect/memory.log.md    ← append-only, git-tracked, one entry per interaction
agents/architect/memory.md        ← GENERATED from memory.log.md (summarized, pruned)
```

The log is the source of truth. It's append-only, so conflicts are always resolvable (both sides appended different entries — keep both). The summary is computed (like METRICS, like REGISTRY), so it's never a conflict source. The summarization script can be simple initially (keep last N entries, deduplicate) and get smarter over time.

This also gives you an audit trail: you can see WHAT the architect learned, WHEN, and from WHOM. If bad knowledge gets in, you can find and remove it.

---

## II. Error Modes the Assessment Doesn't Address

### Error Mode 1: Stale Catalog (Tier 3)

The contract catalog is described as a git repo where developers run `./scripts/collect.sh` manually. The assessment says "just git, no services."

But "manual collection" is exactly the pattern that causes documentation drift — the central lesson from the OpenDocKit learnings doc. The learnings doc says explicitly: *"Documentation updates are a secondary task that agents deprioritize when focused on their primary coding objective."* The same applies to humans running `collect.sh`.

**The catalog WILL be stale within a week of creation unless collection is automated.**

The fix doesn't require a service. It requires CI:

```yaml
# In each project's CI (e.g., .github/workflows/steward.yml)
- name: Run steward
  run: scripts/steward.sh

- name: Push report to catalog
  run: |
    cp architecture/.state/steward-report.json /tmp/report.json
    cd ../contract-catalog  # or clone it
    cp /tmp/report.json reports/${{ github.event.repository.name }}.json
    git add . && git commit -m "update ${{ github.event.repository.name }}" && git push
```

This is still "just git." No service. No dashboard. But it's automated — the catalog stays fresh because CI pushes on every merge to main. The scalability assessment should make CI-triggered collection the DEFAULT at Tier 3, not an optional upgrade for "50+ repos."

**Why this matters for adoption:** If the catalog is stale, developers lose trust in it. If they lose trust, they stop using it. If they stop using it, cross-repo coordination falls back to Slack and tribal knowledge — exactly what the catalog was supposed to replace. Freshness isn't a nice-to-have; it's the catalog's survival condition.

### Error Mode 2: Enforcement Script Divergence

Today, each project copies `scripts/` from rebar during adoption. Blindpipe copied 8 scripts. Dapple SafeSign copied them too. When rebar fixes a bug in `check-contract-headers.sh` (e.g., the CONTRACT-GAPS.md exclusion issue from adoption-day-learnings), every adopter has a stale copy.

This is the same drift problem that rebar's methodology exists to solve — but applied to rebar itself.

**The assessment doesn't address script distribution at all.** At Tier 1-2 (solo/small team), this is fine — you have 1-3 repos that you update manually. At Tier 3 (10-40 repos), script divergence becomes a real governance issue: repos enforcing different versions of the same rules.

**Three options, in order of simplicity:**

1. **Version header + check.** Every script gets a `# rebar-scripts: 2026.03.20` header. `steward.sh` compares against the latest known version and warns if stale. No distribution mechanism — just detection. Humans update manually, but they KNOW they need to.

2. **Git submodule.** `vendor/rebar/` as a submodule. `git submodule update --remote` pulls the latest scripts. Works with existing git infrastructure. The downside: submodules are universally disliked.

3. **Upstream remote.** Each project adds rebar as a git remote. `git fetch rebar && git checkout rebar/main -- scripts/` pulls the latest scripts. No submodule hassle, but requires discipline.

Recommendation: **option 1 (version header + detection) for Tier 2-3, option 3 (upstream remote) for Tier 4.** The key insight: you don't need perfect distribution, you need **stale detection**. `steward.sh` already runs per-repo — add a "rebar version" check to its report.

### Error Mode 3: Cross-Repo Breaking Changes Need Detection Before Tier 4

The assessment places the breaking change workflow at Tier 4 (50+ devs, 40+ repos). But the ai-native-contracts feedback already introduces cross-repo `CONTRACT:blindpipe/C1-BLOBSTORE.2.1` references at Tier 3. Once cross-repo references exist, breaking changes are a Tier 3 problem.

Consider: blindpipe bumps C1-BLOBSTORE from 2.1 to 3.0. Office 180 has `CONTRACT:blindpipe/C1-BLOBSTORE.2` in 14 files. No one notices until Office 180's steward fails — and the failure message is "contract file not found," not "breaking change upstream."

**The catalog already has the data to detect this.** If each repo's steward-report.json includes cross-repo dependencies (which the ai-native frontmatter enables), then a simple diff between consecutive catalog snapshots detects version bumps:

```bash
# scripts/catalog-breaking-change-check.sh
# Compares current reports/ against previous commit
# Finds version bumps → looks up dependents → prints impact

# Example output:
# BREAKING: blindpipe/C1-BLOBSTORE bumped 2.1 → 3.0
#   Affected: office180 (14 refs), opendockit (3 refs)
#   Action: open issues in affected repos
```

This is 50 lines of bash. No service. It runs in the catalog repo's CI (which already exists if you followed the "CI-triggered collection" recommendation above). It turns the catalog from a passive index into an active early warning system.

**Moving breaking change detection to Tier 3 means:** by the time you need the full Tier 4 governance workflow (RFC → review → deprecation window → migration → retire), you already have the detection infrastructure. The workflow is just process layered on top of working tooling.

---

## III. The Tier 2→3 Gap: Where the Assessment Is Most Optimistic

The assessment says Tier 2→3 costs "1-2 weeks" and the feel is "still just git." This understates the coordination challenge.

### What Actually Changes at Tier 3

At Tier 2, every developer has all repos checked out as siblings. Cross-repo search is `grep -rn "CONTRACT:" ../`. Cross-repo deps are `link:../sibling/packages/foo`. Everyone pushes to the same remote. The mental model is: "I can see everything."

At Tier 3, you can no longer assume that. New hires don't have all repos. CI runners don't have sibling directories. The architect agent on Developer A's machine doesn't know what Developer B's architect learned. The mental model shifts from "I can see everything" to "I need to look things up."

This shift is where the catalog becomes load-bearing. But the assessment describes the catalog as "just a git repo you grep" — which undersells what it needs to be. At Tier 3, the catalog needs to answer:

1. **"What contracts exist for auth?"** → search across all repos by tag/name
2. **"Who depends on our blobstore?"** → reverse dependency lookup
3. **"Is anyone else building offline sync?"** → duplicate detection
4. **"What changed since last week?"** → cross-repo changelog

Questions 1 and 2 are answerable from steward-report.json + ai-native frontmatter. Questions 3 and 4 require the catalog to maintain history (git log of the catalog repo itself). This is still "just git" — but the `build-index.sh` script needs to be smarter than "concatenate all reports."

### The Catalog's build-index.sh Is the Keystone Script

The assessment correctly identifies the catalog as the Tier 3 keystone. But it spends 3 paragraphs on the directory structure and 1 sentence on `build-index.sh`. The directory structure is trivial. The index generation is where all the value lives.

**What build-index.sh needs to produce:**

```
index.md          — all contracts, grouped by domain tag (not by repo)
deps.md           — dependency graph (who depends on what, including cross-repo)
changes.md        — version bumps since last generation (breaking change detection)
orphans.md        — contracts with no implementations (across all repos)
duplicates.md     — contracts with similar names/tags across repos (duplicate detection)
```

This is still bash + jq. No service. But it's 200-300 lines of bash, not the 20-line script the assessment implies. **Recommendation: build-index.sh should be one of the "near-term" items to add to the rebar repo, not left as an exercise for adopters.**

### The Onboarding Funnel at Tier 3

The assessment mentions "progressive onboarding" as a near-term addition but doesn't design it. At Tier 3, the onboarding problem is real: a new hire faces 10-40 repos, each with a Cold Start Quad. Where do they start?

The catalog's `index.md` IS the answer — if it's organized by domain rather than by repo. A new hire reads:

```
1. Org-level README (what we build, how repos relate)  → 5 min
2. Catalog index.md (all contracts by domain)           → 10 min
3. Pick your team's repo → Cold Start Quad              → 30 min
4. ask architect "what should I know about <domain>?"   → 5 min
```

Step 1 is a new document (org-level README in the catalog repo). Steps 2-4 already exist. The gap is step 1 + making sure index.md is organized for humans, not just agents.

---

## IV. Protecting Tier 1 Lightness as the System Grows

The assessment's most important promise: "a solo developer with 2 repos should feel zero overhead." This must be actively protected as features are added for higher tiers. Three specific risks:

### Risk 1: Template Bloat

The AGENTS.template.md is already 39KB. It includes sections for worktree orchestration, feature inventory protocol, pre-launch audits, subagent coordination — features that a solo developer will never use. When they scaffold a new repo, they get all of it.

**Fix: Template profiles should strip unused sections.**

The profiles/ directory already has web-app.md, api-service.md, etc. Extend these to specify which AGENTS.md sections to include:

```yaml
# profiles/solo.yaml (hypothetical)
agents_sections:
  - core_tenets
  - agent_autonomy
  - cold_start
  - testing_cascade
  - todo_tracking
  # EXCLUDED: worktree orchestration, feature inventory, pre-launch audit, subagent coordination
```

Or simpler: `ask init` prompts for team size and strips sections accordingly. A solo dev gets a 10KB AGENTS.md, not 39KB.

### Risk 2: Frontmatter Creep

The ai-native-contracts feedback proposes YAML frontmatter with `id`, `version`, `namespace`, `depends_on`, `implements`, `interface`, `mcp_tools`, `tags`. For a solo dev writing their first contract, this is intimidating. The current CONTRACT-TEMPLATE.md has clean, readable sections. Adding a YAML block at the top changes the feel from "document I want to write" to "schema I have to fill out."

**Fix: Frontmatter is optional at Tier 1-2, required at Tier 3+.**

The contract template should have two modes:

```markdown
<!-- MODE: Simple (Tier 1-2) — just write the sections below -->
<!-- MODE: AI-Native (Tier 3+) — add frontmatter for cross-repo tooling -->
```

Steward shouldn't enforce frontmatter unless a config flag is set (`REBAR_TIER=3` in a project-level config). The enforcement scripts already respect per-project configuration — extend this to tier-aware checks.

### Risk 3: Convention Overhead

Conventions.md defines branch naming (`<type>/CONTRACT-<id>-<description>`), commit format (conventional commits with CONTRACT: footer), header format (two tiers), discovery taxonomy (BUG/DISCOVERY/DRIFT/DISPUTE). A solo developer doing a quick fix on their personal project doesn't need most of this.

This isn't a structural problem — it's a documentation problem. The conventions doc should have a "minimum viable conventions" section at the top:

```markdown
## Minimum (Solo / Small Team)
- Every source file gets a CONTRACT: or Architecture: header
- Commits reference contracts when relevant
- TODO: in code = untracked, fix or promote before committing

## Full (Team / Department)
- Branch naming: <type>/CONTRACT-<id>-<description>
- Commit format: conventional commits with CONTRACT: footer
- Discovery taxonomy: BUG/DISCOVERY/DRIFT/DISPUTE
...
```

---

## V. Three Things the Assessment Gets Exactly Right

Worth calling out what doesn't need changing:

### 1. "Git is the only infrastructure through Tier 3"

This is the single most important design decision. Every competing approach (Backstage, Cortex, OpsLevel) requires a service from day one. Rebar doesn't. A developer can `git clone`, run `ask init`, and be productive in 30 minutes with zero infrastructure. This advantage must be fiercely protected.

### 2. The Maturity Model (L1-L5)

Teams choosing their own level is correct. Mandating a single level across an organization is how methodologies die — the most resistant team becomes the bottleneck, and leadership waters down the methodology to accommodate them. Self-assessment with social pressure ("the catalog shows each team's level") is the right governance model.

### 3. "The feel: still light"

The assessment repeatedly checks whether each tier addition preserves the "feel." This is the right instinct. Methodology adoption fails when it stops feeling helpful and starts feeling bureaucratic. The fact that the assessment measures success by feel — not by completeness or rigor — shows the right priorities.

---

## VI. Summary: What to Build

Prioritized by impact on the Tier 1→2→3 adoption path:

### Immediate (unlocks smoother Tier 2)

1. **Split AGENTS.md volatile section** → `WORKSTREAMS.md` (or auto-generate from TODO.md P0)
2. **Compute CONTRACT-REGISTRY.md** from filesystem (like computed METRICS)
3. **Add rebar-version header to scripts** + detection in steward.sh
4. **"Minimum viable conventions" section** in conventions.md

### Near-term (unlocks Tier 3)

5. **build-index.sh** — the real one, 200+ lines, with domain grouping, deps, changes, orphans
6. **CI-triggered catalog collection** (not manual collect.sh)
7. **Append-only memory log** with computed summary (prerequisite for shared memory)
8. **Breaking change detection script** in catalog repo (50 lines of bash + jq)
9. **Org-level README template** for the catalog repo (onboarding funnel step 1)

### Design decisions (affects template direction)

10. **Template profiles for team size** (solo gets 10KB AGENTS.md, not 39KB)
11. **Optional frontmatter** (Tier 1-2 = prose only, Tier 3+ = YAML block)
12. **Tier-aware steward checks** (don't enforce Tier 3 rules on Tier 1 projects)

---

## VII. The Deeper Question: What Makes Rebar Different from Every Other Methodology

Every methodology starts light and grows heavy. XP → Scrum → SAFe. REST → OpenAPI → API governance platforms. README → Backstage → developer portal teams.

Rebar's bet is that **the substrate stays the same** (files + git + bash) while the complexity moves into optional layers. This is a genuinely different bet from "start simple, add infrastructure later." It's "infrastructure is always git, complexity is always optional."

The risk isn't that the philosophy is wrong. The risk is that the implementation adds mandatory complexity accidentally — through template bloat, convention overhead, or enforcement scripts that assume Tier 3 when the project is Tier 1. The recommendations above are all aimed at this risk: keeping Tier 1 truly light while making Tier 3 actually work.

The feedback from blindpipe, Dapple SafeSign, and OpenDocKit validates that the core works. 17 contracts in 3 hours. 18 worktree agents with zero conflicts. 10x token savings from persistent ASK sessions. These aren't aspirational claims — they're measured results from real projects.

The scalability assessment is asking the right question: does this survive contact with 50 devs and 40 repos? The answer is yes, with the six specific adjustments above. The philosophy scales. The file-hotspot problem, the stale-catalog problem, and the enforcement-divergence problem are all solvable without adding infrastructure. They just need to be solved explicitly rather than left as "exercise for the reader" in the Tier 3 adoption path.

**The strongest signal from the real-world feedback:** blindpipe's adoption insight that enforcement scripts are the real value — not templates, not docs. The scripts are what keep the system honest. At scale, the scripts need to be versioned, distributed, and tier-aware. That's the operational work that makes "layers, not modes" actually work in practice.
