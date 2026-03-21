# Feedback from Dapple SafeSign (pdf-signer-web)

**Project type:** Web app — monorepo (React/Vite client, Express API, shared packages, vendored PDF library, biometric identity surface)
**Scale:** 586 unit tests, 80 E2E specs, 15 Playwright configs, 14 API route modules, ~50k LOC
**Agent usage:** Heavy — multiple sessions per day, max autonomy, parallel subagents, worktree isolation
**Date:** 2026-03-18

---

## The Central Finding

The templates have excellent **policy** but incomplete **enforcement for quantitative claims**. The existing enforcement scripts (`check-freshness.sh`, `check-todos.sh`, `check-contract-refs.sh`) catch structural problems — stale dates, untracked TODOs, broken links. But they don't catch the most common failure mode we experienced: **numeric claims drifting from reality while everything else looks green.**

Our docs said "126 tests pass" for weeks while the actual count grew to 586. Freshness dates were current (agents touched the files), TODO tracking was clean, contract refs were valid. Every check passed. The numbers were just wrong — across 5 root documents simultaneously.

This feedback proposes a single cohesive addition to the template system: a **ground truth enforcement layer** that makes quantitative claims machine-verifiable. Everything below serves that idea.

---

## Proposed Changes (Implementation-Ready)

### Change 1: `scripts/check-ground-truth.sh` (new file, ~60 lines)

A scaffold script that projects customize with their own metrics. Outputs key=value pairs, then compares against documented claims.

The script has two responsibilities:
1. **Compute** — Extract metrics from the codebase (test counts, config counts, version numbers, endpoint counts)
2. **Verify** — Compare computed values against claims in documentation, fail on mismatch

```bash
#!/usr/bin/env bash
# check-ground-truth.sh — Verify documented metrics match codebase reality.
#
# This script computes project metrics from code and compares them against
# claims in documentation. Catches "silent success" drift where everything
# works but docs describe a different reality.
#
# CUSTOMIZATION REQUIRED: Define your project's metrics in compute_metrics()
# and your verification rules in verify_claims().
#
# Usage: ./scripts/check-ground-truth.sh
# Exit: 0 = all claims match, 1 = drift detected

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
exit_code=0

# ── CUSTOMIZE: Define your project's metrics ──
compute_metrics() {
  # Examples for a TypeScript monorepo. Replace with your project's metrics.

  # Test counts
  # UNIT_TEST_FILES=$(find packages/ src/ -name '*.test.ts' -o -name '*.test.tsx' \
  #   | grep -v node_modules | grep -v e2e | wc -l | tr -d ' ')
  # E2E_SPEC_FILES=$(find tests/e2e/ -name '*.spec.ts' | wc -l | tr -d ' ')

  # Infrastructure counts
  # PLAYWRIGHT_CONFIGS=$(find . -maxdepth 2 -name 'playwright*.config.ts' | wc -l | tr -d ' ')
  # API_ROUTE_MODULES=$(find packages/api/src/routes/ -name '*.ts' \
  #   -not -name 'index.ts' -not -name '*.test.ts' | wc -l | tr -d ' ')

  # Version references
  # MAIN_PACKAGE_VERSION=$(grep '"version"' package.json | head -1 | grep -o '[0-9]*\.[0-9]*\.[0-9]*')

  echo "No metrics defined. Customize compute_metrics() in this script."
  echo "See comments for examples."
}

# ── CUSTOMIZE: Define verification rules ──
verify_claims() {
  # Compare computed metrics against documented claims.
  # Use check_claim() helper: check_claim <file> <pattern> <expected_value> <label>

  # Examples:
  # check_claim "QUICKCONTEXT.md" "unit tests" "$UNIT_TEST_FILES" "unit test file count"
  # check_claim "CLAUDE.md" "Vitest tests" "$UNIT_TEST_FILES" "unit test count in CLAUDE.md"
  # check_stale_ref "docs/" "beta\.9" "pdfbox-ts version (expected: $MAIN_PACKAGE_VERSION)"

  echo "No verification rules defined. Customize verify_claims() in this script."
}

# ── Helpers (don't modify) ──
check_claim() {
  local file="$1" pattern="$2" expected="$3" label="$4"
  if [ ! -f "$file" ]; then
    echo -e "${YELLOW}SKIP${NC}: $file not found — $label"
    return 0
  fi
  # Check if the expected value appears near the pattern
  if grep -q "$pattern" "$file" 2>/dev/null; then
    if grep "$pattern" "$file" | grep -q "$expected" 2>/dev/null; then
      echo -e "${GREEN}OK${NC}: $label ($expected)"
    else
      echo -e "${RED}DRIFT${NC}: $label — expected $expected in $file near '$pattern'"
      grep -n "$pattern" "$file" | head -3
      exit_code=1
    fi
  fi
}

check_stale_ref() {
  local dir="$1" pattern="$2" label="$3"
  local count
  count=$(grep -rn "$pattern" "$dir" --include='*.md' 2>/dev/null \
    | grep -v 'archive/' | grep -v 'CHANGELOG' | wc -l | tr -d ' ')
  if [ "$count" -gt 0 ]; then
    echo -e "${RED}DRIFT${NC}: $count stale references — $label"
    grep -rn "$pattern" "$dir" --include='*.md' 2>/dev/null \
      | grep -v 'archive/' | grep -v 'CHANGELOG' | head -5
    exit_code=1
  else
    echo -e "${GREEN}OK${NC}: no stale references — $label"
  fi
}

echo "=== Ground Truth Verification ==="
echo ""
compute_metrics
echo ""
verify_claims
echo ""

if [ $exit_code -eq 0 ]; then
  echo -e "${GREEN}✓ All documented claims match codebase reality${NC}"
else
  echo -e "${RED}✗ Documentation drift detected — update docs to match reality${NC}"
  echo "  Run ./scripts/check-ground-truth.sh to see which claims are stale"
fi

exit $exit_code
```

**Why a scaffold, not a runnable script:** Ground truth metrics are inherently project-specific. A TypeScript monorepo counts `*.test.ts` files; a Go project counts `*_test.go` files; a Python project counts `test_*.py` files. The scaffold provides the verification framework and helpers; the project fills in the metrics.

**Integration with existing scripts:**

In `ci-check.sh`, add one entry to the check list:
```bash
run_check "Ground Truth" "./scripts/check-ground-truth.sh"
```

In `pre-commit.sh`, optionally add it (it's fast — just grep/find/wc):
```bash
# Optional: verify numeric claims haven't drifted
[ -x scripts/check-ground-truth.sh ] && ./scripts/check-ground-truth.sh
```

---

### Change 2: AGENTS.template.md — Two new sections (~60 lines)

Add after the existing "Documentation Maintenance Policy" section, before "Quality Gates."

#### Section A: Single Source of Truth Table

```markdown
#### Single Source of Truth for Metrics

Every quantitative claim in documentation must have ONE authoritative source.
All other documents cross-reference it. When the metric changes, update the
authoritative source; the ground truth script (`scripts/check-ground-truth.sh`)
catches stale cross-references.

<!-- Customize this table for your project. Examples: -->

| Metric | Authoritative Source | Verified By | Cross-Referenced In |
|--------|---------------------|-------------|---------------------|
| Unit test count | `pnpm test` output | `check-ground-truth.sh` | QUICKCONTEXT.md, CLAUDE.md |
| E2E spec count | `tests/e2e/*.spec.ts` | `check-ground-truth.sh` | QUICKCONTEXT.md, AGENTS.md |
| API endpoint count | Route handler files | `check-ground-truth.sh` | docs/specifications/ |
| Package version | `package.json` | `check-ground-truth.sh` | Architecture docs |

**Why this matters:** In our experience, numeric claims are the fastest-drifting
documentation content. Prose drifts over weeks (architecture descriptions,
design rationale). Numbers drift with every commit (test counts, endpoint
counts, config counts). An agent adding one test file silently invalidates
counts in 3-5 documents. Without a lookup table, agents update the 1-2
obvious docs and miss the rest.
```

#### Section B: Code Change → Doc Update Matrix (metrics extension)

Extend the existing "Documentation Maintenance Policy" table with metric-specific rows:

```markdown
#### Metric-Bearing Changes (High Drift Risk)

These code changes invalidate numeric claims in multiple documents.
Use the Single Source of Truth Table above to find all affected docs.

| Code Change | Docs to Update |
|-------------|----------------|
| Add/remove test file | All docs listing test counts (see truth table) |
| Add/remove E2E spec | All docs listing spec counts |
| Add/remove Playwright config | All docs listing config counts |
| Add/remove API route module | Corresponding spec file + route count docs |
| Change dependency version | All architecture docs referencing it |
| Change default algorithm/protocol | All architecture docs describing it |

**Anti-pattern:** Hardcoding the same number in 5 documents. Instead,
list the command that produces the number:
```
pnpm test    # unit tests (run check-ground-truth.sh for current count)
```
This way, stale counts are obviously stale — the reader knows to run the
command rather than trust the number.
```

---

### Change 3: DESIGN.md — Numeric drift principle (~15 lines)

Add to §5 "The Information Environment" under "Anti-Drift Mechanisms":

```markdown
### Numeric Claims: The Fastest Drift Vector

Quantitative documentation claims (test counts, endpoint counts, config
counts, version numbers) drift faster than any other content. Every commit
that adds a test file, creates an endpoint, or bumps a version silently
invalidates numbers in multiple documents.

**The failure mode is silent success.** Tests pass. The app works. CI is
green. But "126 tests" became "586 tests" over 3 weeks, and five documents
still say 126. No mechanism detects this because the metric being wrong
doesn't break anything — it just makes documentation fictional.

**Defenses:**
1. **Ground truth script** — `scripts/check-ground-truth.sh` computes
   metrics from code and compares against documented claims. Fails on drift.
2. **Single Source of Truth Table** — one authoritative source per metric,
   all other docs cross-reference. Agents know exactly which docs to update.
3. **Prefer commands over numbers** — Document `run X to see count` rather
   than hardcoding `count is N`. Stale commands are obviously stale.
4. **Cold start verification** — new sessions run the ground truth script
   before trusting QUICKCONTEXT.md claims.
```

---

### Change 4: TODO.template.md — Reorder sections

Move "Known Issues & Blockers" above "Code Debt":

```markdown
## P0 — Immediate
## P1 — Soon
## P2 — Backlog
## Known Issues & Blockers    ← moved UP (was after Code Debt)
## Code Debt
## Completed
```

**Rationale:** Known issues are "what will bite you" — they affect every agent's work. Code debt is "what we should fix eventually" — it's background context. During cold start, an agent scanning TODO.md top-to-bottom should hit gotchas before reaching debt items. The current order buries gotchas after a potentially long code debt list.

Add a note in the template comments:

```markdown
<!-- Known Issues go ABOVE Code Debt intentionally.
     Gotchas affect every agent's session (e.g., "DATABASE_URL in .env breaks tests").
     Code debt is background maintenance. Agents need to see gotchas first. -->
```

---

### Change 5: AGENTS.template.md — E2E memory management (~10 lines)

Add to the existing "E2E Test Server Management" section, after the "Hard Timeout Strategy" subsection:

```markdown
#### Dev Server Memory for Long Test Runs

When running E2E test suites with many specs (50+), development servers
can leak memory and get OOM-killed by the OS mid-run. This is particularly
common with Vite, Webpack HMR, and Next.js dev mode.

**Pattern:** Use production-like servers for E2E, not dev servers.

| Server Type | Memory | Stability | Example |
|------------|--------|-----------|---------|
| Dev server | Growing (~500MB+) | OOM-killed during long runs | `vite dev`, `next dev` |
| Preview/static | Constant (~50MB) | Stable indefinitely | `vite build && vite preview` |

For Vite projects: `vite build --mode test && vite preview --port $PORT`
gives you a static server with constant memory. The 10-15s build time is
recovered many times over by eliminating OOM kills and restarts.

Adjust the web health check timeout to allow for build time (60s vs 20s).
```

---

### Change 6: AGENTS.template.md — Deploy confirmation guard (~15 lines)

Add to the existing "Deployment Traps & Lessons" section:

```markdown
#### Production Deploy Confirmation

Deploy scripts that target production MUST require interactive confirmation.
Without this, agents with "maximum autonomy" can and will deploy to production
autonomously — and autonomy grants are for development workflow, not production
operations.

**Pattern:**
```bash
# In your production deploy script:
if [ -t 0 ]; then
  read -p "Deploy to PRODUCTION? Type 'yes' to confirm: " confirm
  [ "$confirm" = "yes" ] || { echo "Aborted."; exit 1; }
else
  echo "ERROR: Production deploy requires interactive terminal (TTY)."
  echo "This prevents automated/scripted deploys without human confirmation."
  exit 1
fi
```

The `-t 0` check ensures the script is running in an interactive terminal,
not piped or called from another script. This is a deliberate friction point
— the one place where we want to slow agents down.

**Document for your project:**
- Which deploy commands target production vs. staging
- Which commands have this guard and which don't
- How to bypass the guard for CI/CD pipelines (e.g., `DEPLOY_CONFIRMED=1`)
```

---

### Change 7: Profiles — Retrofit adoption path (~15 lines per profile)

Add a "Retrofitting an Existing Project" subsection to each profile. The content is profile-specific because the highest-leverage CONTRACT: adoption targets differ by project type.

**profiles/web-app.md:**
```markdown
### Retrofitting an Existing Project

Don't add CONTRACT: headers to every file at once. Start with the highest-leverage boundaries:

1. **API route modules** — Each gets a CONTRACT: header and a corresponding spec in `docs/specifications/api/`. This is the highest-leverage target because route modules define the external interface. One spec per route file, verified by the ground truth script.
2. **Shared type definitions** — The types that cross package boundaries (shared/types/, API request/response schemas). These are the IR — the stable interface between packages.
3. **Core library entry points** — The main exports of each package (index.ts, public API surface).

These 3 areas cover ~80% of the contract system's value. Internal modules, utility functions, and UI components can be tagged incrementally as you touch them.
```

**profiles/api-service.md:**
```markdown
### Retrofitting an Existing Project

1. **Route handlers / controllers** — Each gets a CONTRACT: header + spec. These ARE the API contract.
2. **Database schema / migrations** — The data model contract. Tag schema files and migration directories.
3. **Middleware chain** — Auth, validation, rate limiting. These define cross-cutting behavioral contracts.
```

**profiles/crypto-library.md:**
```markdown
### Retrofitting an Existing Project

1. **Algorithm implementations** — Each algorithm gets a CONTRACT: header. These are the core behavioral contracts.
2. **Public API surface** — Exported functions/classes that consumers use.
3. **Test vectors** — Tag test vector files with the contract they validate.
```

**profiles/cli-tool.md:**
```markdown
### Retrofitting an Existing Project

1. **Command handlers** — Each command/subcommand gets a CONTRACT: header.
2. **Configuration schema** — Config file format, env var handling, flag definitions.
3. **Output formatters** — The contract between internal data and user-visible output.
```

---

### Change 8: SETUP.md — Ground truth in verification step (~5 lines)

In the "Verify" section, add:

```markdown
### Verify

- [ ] All core files exist and have no placeholder content
- [ ] `./scripts/ci-check.sh` passes (or individual checks pass)
- [ ] `./scripts/check-ground-truth.sh` has at least one metric defined
- [ ] No untracked `TODO:` comments in source code
- [ ] Cold Start Quad is readable in <5 minutes
```

---

### Change 9: Cold Start methodology — Ground truth step (~5 lines)

In AGENTS.template.md "Cold Start Methodology", add after "Step 1: Verify Document Freshness":

```markdown
### Step 1b: Verify Numeric Claims

If the project has a ground truth script, run it and compare against
QUICKCONTEXT.md claims:

```bash
# Compare documented metrics against reality
./scripts/check-ground-truth.sh
```

If metrics have drifted, update the docs BEFORE proceeding with your task.
Stale numeric claims cascade — an agent that trusts "126 tests" when
there are 586 will make wrong assumptions about test coverage, package
scope, and the codebase's maturity.
```

---

## What NOT to Implement (and Why)

### Separate ACID-MAINTENANCE.template.md

The original feedback proposed a standalone template file. On reflection, this would be a 6th document in a system that already fights doc proliferation. The Single Source of Truth Table and Code→Doc matrix fit naturally in AGENTS.template.md alongside the existing doc maintenance policy. One file, one place to look.

### `check-api-spec-parity.sh` as a core enforcement script

API route-to-spec parity is valuable but too project-specific for the core scripts. The existing ground truth scaffold handles it: projects that need it define a metric (`API_ROUTE_MODULES` vs `API_SPEC_FILES`) and a verification rule (`check_claim`). The web-app and api-service profiles can recommend this pattern without requiring a dedicated script.

### KNOWN_ISSUES as a separate file

The original feedback suggested some projects might benefit from a standalone KNOWN_ISSUES.md. On reflection, the template's approach (merged into TODO.md) is correct — fewer files means less drift surface. The fix is reordering (Known Issues above Code Debt) and adding a cold start callout, not adding another file.

### Numeric claims via indirection ("see ground-truth script for count")

The original feedback suggested replacing exact numbers with script references. This is theoretically clean but practically hostile — agents and humans reading QUICKCONTEXT.md want to see "586 tests" at a glance, not "run a script." The better approach: hardcode the number AND have the ground truth script verify it. Stale numbers get caught by CI; readers still get instant context.

---

## Summary: Implementation Scope

| Change | File(s) | Lines | Category |
|--------|---------|-------|----------|
| Ground truth script | `scripts/check-ground-truth.sh` (new) | ~60 | Core enforcement |
| Single Source of Truth Table | `AGENTS.template.md` | ~25 | Policy |
| Code→Doc metric matrix | `AGENTS.template.md` | ~25 | Policy |
| Numeric drift principle | `DESIGN.md` | ~15 | Philosophy |
| Known Issues reorder | `TODO.template.md` | ~5 | Structure |
| E2E memory management | `AGENTS.template.md` | ~10 | Guidance |
| Deploy confirmation guard | `AGENTS.template.md` | ~15 | Guidance |
| Retrofit adoption paths | `profiles/*.md` (4 files) | ~60 | Adoption |
| Ground truth in verification | `SETUP.md` | ~5 | Adoption |
| Cold start ground truth step | `AGENTS.template.md` | ~10 | Methodology |
| ci-check.sh integration | `scripts/ci-check.sh` | ~3 | Wiring |
| **Total** | **9 files** | **~235 lines** | |

No new concepts. No new directories. No structural changes. Everything extends existing patterns. The ground truth script is the only new file; everything else is additions to existing templates.

---

## What's Working Great (Confirmed by Experience)

These patterns from the templates were independently discovered and validated in our project — we built equivalent systems before finding them codified here:

- **Cold Start Quad** — reading order prevents agents diving into code blind
- **Testing Cascade T0-T5** — without it, agents run T5 for every single-function change
- **E2E server lifecycle** (PID tracking, orphan sweep, hard timeouts) — identical failure modes
- **TODO two-tag system** — agents are prolific TODO generators, terrible TODO followers-up
- **Pre-launch audit** — we wasted significant agent time before learning this lesson
- **Feature inventory for worktree agents** — our equivalent of the W6 incident
- **Subagent templates** — single-invocation value is undersold; we use them constantly
- **Freshness markers** — simple but catches the obvious staleness
- **The learnings document** — reading it felt like reading our own incident reports
- **Deployment traps section** — we independently documented identical traps (MIME types, origin allowlists, env vars baked at build time)

The templates are battle-tested. The proposed changes fill the one gap that experience revealed: quantitative claims need machine enforcement, not just human policy.
