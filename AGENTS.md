# AGENTS.md — working on rebar itself

> **rebar v2.0.0** | **Tier 3: ENFORCED** — this repo dogfoods its own methodology.

## Read Before Coding

**Cold Start Quad** (every session, every agent, no exceptions):
1. `README.md` — universal orientation
2. `QUICKCONTEXT.md` — current state (verify against `git log --oneline -10`)
3. `TODO.md` — task list + pointer to INVENTORY.md (canonical planning surface)
4. `AGENTS.md` (this file) — norms, contracts, collaboration

**Reference:**
- `DESIGN.md` — the philosophy (contracts are the operating system)
- `architecture/CONTRACT-REGISTRY.md` — contract index
- `architecture/CONTRACT-*.md` — current contracts (S1-STEWARD, S2-ASK-CLI, S3-MCP-SERVER)
- `practices/` — orchestration, session lifecycle, red team, fidelity, e2e
- `feedback/INVENTORY.md` — vote-shaped accumulator + Maintainer Queue

## Core Tenets

1. **Eat your own dog food.** Every change to rebar's methodology must hold up
   when applied to the rebar repo itself. If the cookie-cutter template would
   produce a structure rebar doesn't follow, fix the template AND rebar.
2. **Mechanical over manual.** When you find yourself doing the same drift fix
   by hand, ship a script that catches it next time (`check-doc-refs.sh`,
   `check-decay-patterns.sh`, `sync-bootstrap.sh` are recent examples).
3. **One canonical planning surface.** `feedback/INVENTORY.md` is the index.
   Don't recreate parallel planning docs at root — append to INVENTORY.

## Agent Autonomy

**Maximum autonomy granted.** Act decisively. Ship code. Don't ask permission
for routine work.

| Situation | Autonomy |
|-----------|----------|
| Editing scripts, docs, templates within existing patterns | Full |
| Adding a new check script, subagent prompt, or practice doc | Full |
| Renaming a public-facing concept or breaking a CLI surface | **Plan mode** |
| Modifying a contract (breaking) | **Plan mode** |
| Modifying a contract (non-breaking minor bump) | Full |
| Removing or deprecating a contract | **Plan mode** |
| Touching `bin/ask-mcp-server` protocol behavior | **Plan mode** (Claude Code clients depend on it) |

Rule of thumb: if your change is reversible and follows existing patterns,
just do it. If it establishes new patterns or affects MCP-connected clients,
plan first.

## Session Lifecycle

| Stage | Trigger | Key Actions |
|-------|---------|-------------|
| **Start** | New session | Cold Start Quad + verify QUICKCONTEXT vs `git log` |
| **Checkpoint** | Every 10 commits or 2 hours | Update QUICKCONTEXT, commit WIP, sanity-check |
| **End** | Closing | Update QUICKCONTEXT.md What's Next, scrub TODO, commit |

See `practices/session-lifecycle.md` for the full protocol.

## Contract-Driven Development (applied to rebar itself)

The rebar source repo has contracts for its own load-bearing components:

| Contract | Component | Source location |
|----------|-----------|-----------------|
| `CONTRACT:S1-STEWARD.1.0` | Quality scanner (steward.sh + companion infra) | `scripts/steward.sh`, `agents/steward/` |
| `CONTRACT:S2-ASK-CLI.1.0` | Persistent role-agent CLI | `bin/ask`, `bin/ask-server`, `bin/ask-agent-loop` |
| `CONTRACT:S3-MCP-SERVER.1.0` | MCP bridge exposing ASK as Claude-Code-native tools | `bin/ask-mcp-server` |

**Rules:**
1. Don't modify these without checking the contract first.
2. Behavior changes that break the contract → plan mode.
3. After modification: `grep -rn "CONTRACT:S1-STEWARD"` (or whichever) to find
   all impl + test files; update if interface shifts.

## Testing Cascade

| Tier | Target | Speed | Command |
|------|--------|-------|---------|
| **T0** | Bash syntax | <2s | `for f in scripts/*.sh; do bash -n "$f"; done` |
| **T1** | Single Go test file | <10s | `cd cli && go test ./cmd/<file>_test.go` |
| **T2** | All Go tests | <30s | `cd cli && go test ./...` |
| **T3** | Full ci-check (rebar enforces itself) | <30s | `scripts/ci-check.sh` |
| **T4** | MCP smoke test | <10s | `echo '{"jsonrpc":"2.0",...}' | bin/ask-mcp-server --stdio --repos-dir ~/dev` |
| **T5** | Cross-repo audit | <30s | `rebar audit --all ~/dev` |

**Rules:**
- Iterate at T1 (Go) or T0 (bash). Promote on success.
- Never skip tiers in the inner loop.
- The Scout Rule applies — no skipped tests, no flakes, no walking past red.

## TODO Tracking

**Two-tag system:**
- `TODO:` — untracked work (blocks commit)
- `TRACKED-TASK:` — already in TODO.md or INVENTORY.md (commit allowed)

Run `scripts/check-todos.sh` before every commit. Pre-commit hook handles this.

## Multi-Agent Orchestration

For parallel agent campaigns on rebar itself (uncommon but possible — e.g.,
fanout to write multiple practice docs), see
`practices/multi-agent-orchestration.md`. Worktree isolation, commit-per-chunk,
and the 10 rules apply.

## Documentation Maintenance

After any code change, walk the doc tree and update:
- README.md if it changed how rebar describes itself
- QUICKCONTEXT.md What's Next + last-synced
- INVENTORY.md if a feedback item was implemented or rejected
- METRICS file if counts shifted (run `scripts/check-ground-truth.sh`)
- Practice docs if a new pattern was codified

## Commit & PR Guidelines

Conventional prefixes: `feat`, `fix`, `chore`, `docs`, `feedback`. Keep
summary under 70 chars. Use HEREDOC for multi-line bodies.

**Co-Authored-By trailer:** add when an agent (Claude or otherwise) shipped
the work, with the exact model name + window:
```
Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```
