# REBAR Feedback: "Assume Nothing" Documentation Audit

> **Date:** 2026-04-05
> **Project:** filedag
> **Author:** Claude (session with Will)
> **Scope:** Full audit of Cold Start Quad accuracy, contract lifecycle integrity, and Tier 2 structural compliance

---

## Executive Summary

An "assume nothing" audit of filedag's documentation revealed that **REBAR was structurally in place but operationally hollow**. The Cold Start Quad existed, contracts existed, enforcement scripts existed — but quantitative claims were stale, contract lifecycles were fabricated, and half the Tier 2 requirements were missing. A new session starting from the Cold Start Quad would have been working from a false picture.

**Root cause:** REBAR enforces *structural presence* (files exist, headers present, timestamps fresh) but not *content accuracy*. You can pass every check while your QUICKCONTEXT claims 149 tests when there are 271.

---

## 1. Quantitative Claims Drift Without Ground Truth Enforcement

**Finding:** QUICKCONTEXT.md had 7 stale quantitative claims out of 12 checked:

| Claim | Stated | Actual | Drift |
|-------|--------|--------|-------|
| Playwright e2e tests | 149+ | 271 | +82% |
| Vitest unit tests | 30 | 38 | +27% |
| Go test packages | 18 | 20 | +11% |
| React components | 47 | 46 exported | -2% (also wrong counting basis) |
| CLI commands | 21 | 23 | +10% |
| Contract count | 15 | 14 | -7% |
| Contract lifecycle | "7 VERIFIED, 8 DRAFT" | 4 VERIFIED, 2 TESTING, 3 ACTIVE, 5 DRAFT | Completely wrong |

**Impact:** An agent reading QUICKCONTEXT to plan work would misjudge test coverage, feature completeness, and contract health. The contract lifecycle claim was the worst — "7 VERIFIED" suggests most contracts have passing tests. The reality: most are DRAFT (missing required doc sections).

**Root cause:** No `METRICS` file existed, no `check-ground-truth.sh` existed. Freshness check only verifies timestamp, not content. Numbers were written once and never re-verified.

**REBAR recommendation:** The `METRICS` file + `check-ground-truth.sh` should be **Tier 2 requirements, not Tier 3**. Structural compliance without content accuracy is security theater. Alternatively, the freshness check should compare key counts against code reality.

---

## 2. Contract Lifecycle Was Declared, Not Computed

**Finding:** AGENTS.md correctly states lifecycle is "computed, never declared." But CONTRACT-REGISTRY.md manually declared 7 contracts as VERIFIED when:
- 5 of those 7 were missing required sections (Interfaces, Behavior, Error Contracts)
- By REBAR's own rules, missing sections = DRAFT regardless of implementation status
- The actual computed lifecycle was: 4 VERIFIED, 2 TESTING, 3 ACTIVE, 5 DRAFT

**Impact:** The registry was aspirational, not descriptive. An agent checking contract health before modifying code would see "VERIFIED" and skip due diligence.

**Root cause:** REBAR defines the lifecycle computation rules in prose but provides no automated tool to compute them. The `steward.sh` script is mentioned in documentation but doesn't exist in most projects. Without automation, lifecycle is just another field to maintain by hand — and hand-maintained metadata always drifts.

**REBAR recommendation:** Provide a reference `compute-lifecycle.sh` that:
1. Checks each contract file for required sections (Purpose, Interfaces, Behavior, Error Contracts, Test Requirements, Implementing Files)
2. Greps for `CONTRACT:{id}` in source to verify implementations exist
3. Checks for test files in implementing packages
4. Outputs computed lifecycle per contract
5. Optionally updates CONTRACT-REGISTRY.md

---

## 3. P0 Bugs Listed as Open Were Already Fixed

**Finding:** TODO.md listed 5 P0 bugs ("users see failure"). Code review showed 4 of 5 had been fixed:
- People view: working (user confirmed)
- Shortcuts pane: code fix in place
- Scroll reset: scroll preservation implemented via ref + requestAnimationFrame
- Content filters in DAG views: client-side filtering added

**Impact:** The TODO painted a picture of a broken product when most issues were resolved. A new agent session would waste time investigating fixed bugs.

**Root cause:** REBAR's session lifecycle protocol (v2.0.0) requires TODO reconciliation at session end. But this protocol was not in AGENTS.md and no automation enforces it. The TODOs were written in one session and never reconciled against subsequent commits.

**REBAR recommendation:** Session-end TODO reconciliation should be more prominent. Consider a `check-todo-staleness.sh` that cross-references TODO items against recent git log to flag potentially-resolved items (e.g., "P0: Scroll resets" + commit message "fix: scroll preservation in GridView" = likely resolved).

---

## 4. Half of Tier 2 Structural Requirements Were Missing

**Finding:** Despite claiming Tier 2 compliance, filedag was missing:

| Requirement | Status |
|-------------|--------|
| `.rebar-version` file | Missing |
| `METRICS` ground-truth file | Missing |
| Pre-commit hook installed | Not installed (only .sample files) |
| `check-compliance.sh` | Missing |
| `check-ground-truth.sh` | Missing |
| `pre-commit.sh` | Missing |
| README rebar badge | Missing |
| QUICKCONTEXT "What's Next" section | Missing |
| TODO.md Discoveries section | Missing |
| AGENTS.md ASK CLI mention | Missing |
| AGENTS.md session lifecycle | Missing |
| CONTRACT-GAPS.md | Missing |

**Impact:** The `.rebarrc` said `tier = 2` and `ci-check.sh` ran Tier 2 checks — but those checks only covered contract headers and freshness. The broader Tier 2 requirements (badge, METRICS, session lifecycle, discoveries) were simply not enforced.

**Root cause:** The `ci-check.sh` script was created to run the checks that existed. Nobody audited it against the full Tier 2 requirement list. The requirements grew (v2.0.0 additions) but the enforcement didn't keep up.

**REBAR recommendation:** `check-compliance.sh` should be the canonical "does this project meet its declared tier?" checker, and it should be **provided by REBAR** (not written per-project). When REBAR version bumps, projects should copy the updated script. Currently each project writes its own compliance checks, which inevitably diverge from the spec.

---

## 5. Consistency Across Documents Was Not Cross-Checked

**Finding:** The same facts appeared in multiple documents with different values:

| Fact | README | QUICKCONTEXT | AGENTS.md | CONTRACT-REGISTRY |
|------|--------|-------------|-----------|-------------------|
| Component count | (not stated) | "47" | "41" | (not stated) |
| Contract count | "15" | "15" | (not stated) | 14 listed |
| Similar endpoint | `{hash}` | (not stated) | (not stated) | (not stated) |
| Similar endpoint (actual) | `{path}` | — | — | — |

**Impact:** Different documents tell different stories. An agent reading AGENTS.md thinks there are 41 components; reading QUICKCONTEXT thinks 47. Neither is correct.

**Root cause:** REBAR's Cold Start Quad assigns each file a different *purpose* (identity, state, tasks, norms) but doesn't address cross-document consistency. No script checks that the same fact appears consistently across files.

**REBAR recommendation:** Key quantitative facts should live in exactly one place (METRICS file) and be referenced, not duplicated. Prose documents should say "see METRICS" rather than embedding counts that will drift. The ground truth script becomes the single verifier.

---

## 6. Seam Contracts Not Used Despite Active Go↔TypeScript Boundary

**Finding:** filedag has a clear language seam: Go backend defines REST response types, TypeScript frontend consumes them via `client.ts`. There are no seam contracts documenting the type mapping. The v2.0.0 seam contract template exists in REBAR but was never adopted.

**Impact:** Type mismatches between Go structs and TypeScript interfaces are a common bug source (the `similar/{hash}` vs `similar/{path}` discrepancy is an example). A seam contract would make this explicit.

**REBAR recommendation:** When a project has multiple language runtimes communicating over a wire protocol, the Tier 2 compliance check should flag the absence of seam contracts. Not as a blocking error, but as a discovery in CONTRACT-GAPS.md.

---

## Fixes Applied

1. **QUICKCONTEXT.md** — Updated all 7 stale numbers, honest phase description, added "What's Next" section
2. **TODO.md** — Moved 4 fixed P0 bugs to completed, added Discoveries section (5 entries), collapsed completed items
3. **README.md** — Added rebar badge, fixed `similar/{path}` documentation
4. **AGENTS.md** — Added session lifecycle protocol, ASK CLI section, fixed "41 components" → "46"
5. **CONTRACT-REGISTRY.md** — Recomputed all 14 lifecycle statuses, added methodology note
6. **CONTRACT-GAPS.md** — Created with uncontracted code, missing seam contracts, section gaps
7. **METRICS** — Created with 18 verified ground-truth values
8. **.rebar-version** — Created (v2.0.0)
9. **scripts/check-compliance.sh** — Created (badge, version, AGENTS sections, cold start quad)
10. **scripts/check-ground-truth.sh** — Created (METRICS vs code reality)
11. **scripts/pre-commit.sh** — Created (tier-aware hook runner)
12. **scripts/ci-check.sh** — Updated to include compliance + ground truth checks
13. **Pre-commit hook** — Installed via symlink

---

## Meta-Observation

REBAR's value is highest when it prevents exactly this kind of drift. But the methodology currently optimizes for *initial adoption* (write the quad, add headers, create contracts) more than *ongoing maintenance* (keep numbers accurate, reconcile TODOs, recompute lifecycles). The enforcement scripts catch structural violations but not semantic ones.

The fix is straightforward: promote `METRICS` + `check-ground-truth.sh` to Tier 2, provide `compute-lifecycle.sh` as a REBAR-maintained script, and add a "consistency" section to the freshness check that compares key facts across documents. These are all automatable.

The deeper lesson: a methodology that can be "in place" while its core documents are wrong is a methodology that optimizes for ceremony over truth. REBAR should make it **harder to lie** — not just harder to forget.
