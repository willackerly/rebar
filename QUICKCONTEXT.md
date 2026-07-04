# QUICKCONTEXT — rebar (main)

last-synced: 2026-07-04
**Branch:** main
**Tier:** 3 (ENFORCED) — rebar dogfoods its own methodology
**In-flight branch:** `v3.0.0-beta` (tagged 2026-07-04) — see "Active Branches" below

---

## Current State

- **rebar v2.0.0** released (tag `v2.0.0`); **v3.0.0-beta built and
  tagged 2026-07-04** on the `v3.0.0-beta` branch (alpha line retired)
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
| `v3.0.0-beta` | v3 built + tagged 2026-07-04 (7 clusters; alpha line retired) | `git checkout v3.0.0-beta && cat docs/v3-beta-plan.md` |

The beta is BUILT and TAGGED (v3.0.0-beta, 2026-07-04): seven clusters
including the peer-inbox paradigm and Claude Skills packaging, verified
by a 71-agent adversarial review. The v3.0.0-alpha/v3.0.1-alpha branches
are retired. See `docs/v3-beta-plan.md` on the beta branch.

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

- **v3.0.0-beta tagged** — next: offer the beta to the first federated
  adopters (TDFLite, filedag, fontkit) and, when validated, merge the
  beta branch into main as the v3.0.0 release.

## What's Next

1. **Work happens on the beta branch:** `git checkout v3.0.0-beta &&
   cat QUICKCONTEXT.md` — main stays the v2.x stable line until the
   beta merges.
2. Other active feedback awaiting triage on main:
   - `feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md`
   - `feedback/2026-04-22-testing-rigor-six-moments.md` (multi-proposal
     disposition pending)
   - `feedback/2026-04-24-contract-discipline-and-jtbd-framing.md`
     (implemented in v3.0.0-beta Cluster 5)
   - `feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md`
     (semantic patterns still need self-audit-prompt addition)
3. Auto-federation experiments queued — `feedback/2026-04-28-auto-
   federation-experiment.md` has 7 maintainer-decision questions
   pending. Pairs with the beta's `scripts/inbox-watch.sh` for v3.1.

## Known issues / non-blockers

- Auto-federation experiments are research-only until the 7 open
  questions are answered (test partner repo, bot identity, fatigue
  thresholds, etc.).
- v3-beta compliance weighting demotes badges for adopters with all-stub
  contracts (intentional — that's the point of the maturity tagging
  feature).

## Verification (run before trusting this file)

```bash
git log --oneline -10                     # what's actually on main
git branch -a                             # see v3.0.0-beta + remotes
rebar audit                               # 9-10/10 expected on main
rebar audit --all ~/dev                   # cross-repo health
scripts/ci-check.sh                       # 13/13 expected
git checkout v3.0.0-beta && cat docs/v3-beta-plan.md  # beta plan + decision log
```

## Cross-device handoff (2026-07-04)

Both `main` and `v3.0.0-beta` (branch + tag) are pushed to `origin`. To
resume:

```bash
git fetch origin
git checkout v3.0.0-beta              # for v3 work
cat QUICKCONTEXT.md                   # beta-specific state
cat docs/v3-beta-plan.md              # canonical plan + decision log

# OR
git checkout main                     # for v2.x stable work
cat QUICKCONTEXT.md                   # this file
```

Will's auto-memory at `~/.claude/projects/-Users-will-dev-rebar/memory/`
does NOT travel between devices. All load-bearing context (plans,
conventions, doctrines) lives in-repo per the
`feedback-prefer-in-repo-persistence` working-style preference.
