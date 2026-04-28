# TODO — rebar

**Last synced:** 2026-04-28

The canonical, vote-shaped backlog lives in [`feedback/INVENTORY.md`](feedback/INVENTORY.md).
This file holds short-horizon work that's actively in flight or imminent.

---

## P0 — In Flight

- [ ] **Max-compliance dogfooding** — add structural files rebar lacked for
      itself (AGENTS.md ✓, QUICKCONTEXT.md ✓, TODO.md ✓, METRICS, .rebar-version ✓,
      3 component contracts, pre-commit hook). Target: `rebar audit` 9-10/10.

## P1 — Imminent

- [ ] Triage `feedback/2026-04-24-process-gates-G-through-L.md` (untracked, dropped by Will 2026-04-24).
- [ ] Wave 1 doc-only items (5 items, ~1 day) — see `INVENTORY.md` §Queued.
- [ ] Wave 2 script + template surgery (2 items, ~1 day) — see `INVENTORY.md` §Queued.
- [ ] **Next big push: opportunistic auto-federation** — see [`feedback/2026-04-28-auto-federation-experiment.md`](feedback/2026-04-28-auto-federation-experiment.md). 5 candidate experiments (auto-flush cron, consumer-hello on first declaration, auto-PR for minor pin bumps, auto-discover via git remotes, age-out stale outbox entries). Goal: test "without trying too hard" — repos auto-link + auto-notify when both sides have the conventions wired, but every fallback intact. Doctrine anchor: [`practices/federation.md`](practices/federation.md) Principle 4 ("automation tries, doesn't require").

## P2 — Maintainer Queue

See `feedback/INVENTORY.md` §🧰 Maintainer Queue:
- A2 fontkit AGENT.md typo (foreign repo)
- A3 example questions in MCP tool inputSchema
- Per-feedback disposition for the three 2026-04-22→04-24 feedback files
- Session-start repo-state check (1 vote — promote when 2nd adopter hits it)

## Discoveries

<!-- The Steward parses this section. Each entry: BUG / DISCOVERY / DRIFT / DISPUTE
     scoped to a contract ID. Open issues only — close by removing the entry. -->

_None currently._

---

## Recently complete

(Move items here when shipped. Trim to last ~10 to stay concise.)

- ✓ 2026-04-25: Max-compliance push (Tier 3, contracts, hook) — see commit log
- ✓ 2026-04-25: TDFLite added to MCP swarm — `ff7fef3` (TDFLite), `5800647` (rebar)
- ✓ 2026-04-25: rebar audit + MCP server depth-2 recursion — `cdb2c45`, `5800647`
- ✓ 2026-04-25: Rebar CLI rebuilt with Go 1.25 — `949fada`
- ✓ 2026-04-24/25: Multi-persona red-team review (10 commits) — see `c610fcf...a06732b`
- ✓ 2026-04-24: Stale `.claude/worktrees/` untracked (476 files, 75K LOC) — `a06732b`
- ✓ 2026-04-22: ASK MCP — caller-facing role preambles — `2f52983`
- ✓ 2026-04-22: ASK MCP — first-paragraph extraction for tool descriptions — `bc936cf`
- ✓ 2026-04-22: ASK MCP — ignore JSON-RPC notifications — `0db9073`
- ✓ 2026-04-20: Wave 2.5 — MCP activation as first-class Claude Code tool — `d9e68fc`
