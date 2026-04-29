# QUICKCONTEXT — rebar (main)

last-synced: 2026-04-29
**Branch:** main
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology
**In-flight branch:** `v3.0.0-alpha` — see "Active Branches" below

---

## Current State

- **rebar v2.0.0** released (tag `v2.0.0`); **v3.0.0-alpha** branch
  cut 2026-04-29 for the next major bump
- **8 rebar-adopted repos** in the MCP swarm: rebar, TALOS, blindpipe,
  filedag, fontkit, office180, pdf-signer-web, TDFLite
- **37 ASK agents** discoverable via the rebar-ask MCP server
- **3 contracts** for rebar's load-bearing components (S1-STEWARD,
  S2-ASK-CLI, S3-MCP-SERVER)
- **15+ enforcement scripts** in `/scripts/` (synced into
  `templates/project-bootstrap/scripts/`)
- **CHARTER §1.6 + §2.10** federation amendments shipped 2026-04-28;
  full federation tooling (CONSUMES.md, drift-check, outbox) live

## Active Branches

| Branch | Purpose | Where to look |
|--------|---------|---------------|
| `main` | v2.x stable, federated, dogfooded at Tier 3 | this file |
| `v3.0.0-alpha` | major bump bundling 5 ripe concepts | `git checkout v3.0.0-alpha && cat docs/v3-alpha-plan.md` |

The alpha is in flight. Five clusters: maturity tagging (headline) +
SessionStart hook + TEST_FIDELITY/UAKS + FANOUT_PATTERN + contract
discipline. Tag after Cluster 4. See `docs/v3-alpha-plan.md` on the
alpha branch for the canonical state-of-work.

## Recent Ship (2026-04-28 → 2026-04-29)

| Commit | Topic |
|--------|-------|
| `b08c98d` | Filed 2026-04-26 SessionStart-hook + 2026-04-28 fanout-playbook feedback |
| `b65def2` | gitignore agents/*/.progress runtime artifact |
| `d80cb4b` | Federation 4 principles + auto-federation queued as next push |
| `2cbce75` | Federation Cluster 5 — compliance + docs |
| `5d2fbac` | Federation Cluster 4 — consumer-side commands |
| `c0f50bf` | Federation Cluster 3 — owner-side tooling |
| `614b353` | Federation Cluster 2 — CONSUMES.md format + bootstrap |
| `e79c56b` | Federation Cluster 1 — CHARTER §1.6 + §2.10 |
| `470335d` | Federation design proposal filed |
| `c517d25` | Usability RT Cluster E — adoption hygiene polish |
| `aca68d5` | Usability RT Cluster D — error-path polish |
| `8e8675c` | Usability RT Cluster B — case-insensitive resolution |
| `b8894f6` | Usability RT Cluster A — cold-start completeness |
| `b09f9fb` | Usability RT Cluster C — claude flag regressions fixed |
| `6df6b3d` | CHARTER.md + `ask featurerequest` gated MCP intake |

## In Progress

- **v3.0.0-alpha branch** (cut 2026-04-29) — five-cluster scope
  bundling the headline maturity-tagging feature with SessionStart
  hook, TEST_FIDELITY+UAKS, FANOUT_PATTERN, and contract discipline
  followups. See branch's QUICKCONTEXT + `docs/v3-alpha-plan.md`.

## What's Next

1. **Resume on alpha branch:** `git checkout v3.0.0-alpha`, start
   Cluster 1 (maturity tagging).
2. Other active feedback awaiting triage on main:
   - `feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md`
   - `feedback/2026-04-22-testing-rigor-six-moments.md` (multi-proposal
     disposition pending)
   - `feedback/2026-04-24-contract-discipline-and-jtbd-framing.md`
     (becomes alpha Cluster 5)
   - `feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md`
     (semantic patterns still need self-audit-prompt addition)
3. Auto-federation experiments queued — `feedback/2026-04-28-auto-
   federation-experiment.md` has 7 maintainer-decision questions
   pending. v3.0.x or v3.1 push after alpha tag.

## Known issues / non-blockers

- Auto-federation experiments are research-only until the 7 open
  questions are answered (test partner repo, bot identity, fatigue
  thresholds, etc.).
- v3-alpha breaks compliance scoring for adopters with all-stub
  contracts (intentional — that's the point of the maturity tagging
  feature).

## Verification (run before trusting this file)

```bash
git log --oneline -10                     # what's actually on main
git branch -a                             # see v3.0.0-alpha + remotes
rebar audit                               # 9-10/10 expected on main
rebar audit --all ~/dev                   # cross-repo health
scripts/ci-check.sh                       # 13/13 expected
git checkout v3.0.0-alpha && cat docs/v3-alpha-plan.md  # alpha plan
```

## Cross-device handoff (2026-04-29)

Both `main` and `v3.0.0-alpha` are pushed to `origin` so the work can
resume on a different device. To resume:

```bash
git fetch origin
git checkout v3.0.0-alpha             # for alpha work
cat QUICKCONTEXT.md                   # alpha-specific state
cat docs/v3-alpha-plan.md             # canonical task list

# OR
git checkout main                     # for v2.x stable work
cat QUICKCONTEXT.md                   # this file
```

Will's auto-memory at `~/.claude/projects/-Users-will-dev-rebar/memory/`
does NOT travel between devices. All load-bearing context (plans,
conventions, doctrines) lives in-repo per the
`feedback-prefer-in-repo-persistence` working-style preference.
