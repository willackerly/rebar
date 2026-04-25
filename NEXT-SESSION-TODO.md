# Next Session — TODO

Concrete cleanup & implementation actions outstanding as of 2026-04-24.
Ordered by impact × effort. Check [feedback/INVENTORY.md](feedback/INVENTORY.md)
for the vote-accumulated watchlist and longer-horizon backlog.

---

## A. MCP activation — follow-ups from Wave 2.5

### A1. ~~Copywriting pass on agent descriptions~~ — DONE (centralized, 2026-04-24)
- [x] **Centralized version implemented** in commit `2f52983`. Added
      `ROLE_DESCRIPTIONS` dict in `bin/ask-mcp-server` with caller-facing copy
      for all 7 canonical roles. Each lead with what the role OWNS, "Best for:"
      question shapes, and where applicable a "Prefer over grep when..." hint.
      Agent-voice intros from AGENT.md prose are stripped iteratively. 32
      tools verified, average 285 chars per description, zero leading
      "You are..." copy.
- [ ] **Distributed version (still optional)** — if you want repo-specific
      signal beyond the centralized preamble, rewrite each AGENT.md Role
      section to be caller-facing. The centralized version covered the
      80% of value; distributed is the remaining 20% if/when you want it.

### A2. Fix fontkit AGENT.md copy/paste error
- [ ] Edit `~/dev/fontkit/agents/architect/AGENT.md` — currently says
      *"You are the architect agent for **rebar**"* instead of fontkit.
- 30 seconds, but in a foreign repo (outside rebar).

### A3. Optional: example questions in tool inputSchema
- [ ] Add `examples` array to the `question` parameter in tools/list output.
      Helps Claude formulate good queries.
- ~30 min; low priority; do after A1 if appetite exists.

---

## B. Wave 1 — doc-only, ~1 day total

From [feedback/INVENTORY.md §Queued](feedback/INVENTORY.md). All already decided, just need to land.

- [ ] **W1-1** Numeric drift principle → `DESIGN.md` §Anti-Drift
  (~15 lines, source: `feedback/digital-signer-feedback.md`)
- [ ] **W1-2** Single Source of Truth Table section → `AGENTS.template.md`
  (~25 lines, same source)
- [ ] **W1-3** Deploy-confirmation TTY guard pattern → `AGENTS.template.md`
  (~15 lines, same source)
- [ ] **W1-4** Zero-tolerance testing doctrine → `AGENTS.template.md`
  + 1-line reference in `DESIGN.md` (source: `feedback/zero-tolerance-testing-feedback.md`)
- [ ] **W1-5** CHANGELOG `### Migration` subsections per version →
  `CHANGELOG.md` (source: `feedback/versioning-and-upgrade-path-2026-03-20.md`)

**On completion:** move the 3 source feedback files from `feedback/` →
`feedback/processed/`. Update INVENTORY.md to move W1 items to Implemented.

---

## C. Wave 2 — script + template surgery, ~1 day total

- [ ] **W2-1** `O-` operational-contract prefix in
      `architecture/CONTRACT-TEMPLATE.md` + use filedag's `CONTRACT-O1-PIPELINE-DAEMON.1.0.md`
      and `CONTRACT-O2-API-GATEWAY.1.0.md` as reference examples (copy filedag's
      into `architecture/examples/` or similar).
- [ ] **W2-2** Extend `scripts/compute-registry.sh` to detect:
      - drift (computed lifecycle ≠ registry lifecycle)
      - shadow (code refs `CONTRACT:ID` not in registry)
      - ghost (registry row but no contract file)
      - zombie (0 code refs, deletion candidate)
      - unlisted (contract file exists, not in registry)
      Output in JSON + markdown modes matching steward.sh convention.

**On completion:** move `feedback/2026-04-18-filedag-deep-audit-insights.md` →
`feedback/processed/`.

---

## D. New incoming feedback — triage needed

- [ ] **Triage `feedback/2026-04-22-testing-rigor-six-moments.md`**
  (Will's commit `aabe09a`, 205 lines). Six proposals from Dapple SafeSign
  Phase 5.5 red-team audit about "tests pass but narrow coverage." Proposal 5
  (tag-to-CI enforcement) has a working prototype at
  `~/dev/pdf-signer-web/scripts/check-tag-ci-coverage.mjs` — likely the
  strongest Queue candidate. Per-proposal disposition needed:
  1. File-to-tier matrix
  2. Negative-control mandate for detection-style tests
  3. Test Fidelity Ladder as machine-checkable comment
  4. Drift-mode taxonomy for differential tests
  5. Tag-to-CI enforcement **(prototype attached)**
  6. Security-test commit-message template
  Will already added a "Testing rigor" Watchlist subsection to INVENTORY.md;
  this is the formal per-proposal triage to decide which to queue now.

- [ ] **Examine commit `befc4c4`** — "feedback: cross-reference checks +
  cross-repo federation coordination" — haven't inspected the content yet;
  decide disposition + update INVENTORY.md.

---

## E. Promotion candidate (opportunistic)

- [ ] **Session-start repo-state check** — currently 1 vote (FontKit
  adoption-day drift) in INVENTORY.md Watchlist (Session Lifecycle section).
  Promote if: a 2nd adopter reports the same drift surprise, OR while you're
  already editing `rebar status` / `practices/session-lifecycle.md` for
  another reason. Implementation sketch:
  - Extend `rebar status` to read `.rebar-version`, fetch latest rebar tag
    from the upstream (or a local cache), print a warning with a link to
    `CHANGELOG.md` migration sections if >1 minor version behind.
  - Add a "Pre-flight" subsection to `practices/session-lifecycle.md`.
  - ~1 hour.

---

## F. Housekeeping (low priority, pick up between other work)

- [ ] Working tree has untracked deletions from stale worktree cleanup:
      `.claude/worktrees/agent-a28b156a/` and `.claude/worktrees/agent-a3c77b08/`.
      Either `git clean` them or commit the deletions.
- [ ] `.DS_Store` modified in root — confirm it's in `.gitignore`
      (it's the kind of noise that should never land in a commit).
- [ ] `bin/__pycache__/` appears as untracked — add to `.gitignore` if not
      already.

---

## Reference

- [feedback/INVENTORY.md](feedback/INVENTORY.md) — vote accumulator + full watchlist
- [docs/MCP-SETUP.md](docs/MCP-SETUP.md) — MCP config guide for adopters
- [docs/MCP-IMPLEMENTATION.md](docs/MCP-IMPLEMENTATION.md) — protocol-level details
