# Session Lifecycle

**Referenced from AGENTS.md. Read when starting, checkpointing, or ending a session.**

---

## Why This Matters

The Cold Start Quad handles session start brilliantly. But sessions also
need checkpoints and endings. Without a structured lifecycle, marathon
sessions leave QUICKCONTEXT stale, TODO listing completed items as open,
worktrees abandoned, and the next session spending 20-30 minutes on
context archaeology.

**The meta-principle:** Agents reliably forget any protocol that requires
manual action after the exciting work is done. The session lifecycle makes
the right thing the expected thing by defining clear triggers and checklists.

```
Session Lifecycle:
  START      → Cold Start Quad + staleness verification
  CHECKPOINT → every 10 commits or 2 hours
  END        → structured wrapup + handoff
```

---

## Session Start (Enhanced Cold Start)

### 1. Read the Cold Start Quad (existing — 5 minutes)

| Order | File | Purpose |
|-------|------|---------|
| 1 | `README.md` | What is this project? |
| 2 | `QUICKCONTEXT.md` | What's true right now? |
| 3 | `TODO.md` | What needs doing? |
| 4 | `AGENTS.md` | How do we work together? |

Then: `CLAUDE.md` for Claude-specific configuration.

### 2. Verify Freshness (new — 2 minutes)

**Do not trust QUICKCONTEXT blindly.** Cross-reference against reality:

```bash
# What actually happened recently?
git log --since='7 days' --oneline | head -20

# Compare against QUICKCONTEXT claims — flag discrepancies
# Check: test counts, feature status, "What's Next" ordering

# Any abandoned worktrees from previous sessions?
git worktree list

# Any untracked TODOs in code?
grep -rn "TODO:" --include="*.ts" --include="*.go" --include="*.py" \
  src/ lib/ internal/ packages/ 2>/dev/null | grep -v "TRACKED-TASK" | head -10
```

If the `last-synced` date in QUICKCONTEXT is >1 week old, treat **all**
claims as suspect and verify against `git log` before acting.

**Automation:** Run `scripts/refresh-context.sh` if available — it does
all of the above in one command.

### 3. Establish Test Baseline

**This step prevents the #1 time-waster:** inheriting a repo that claims
"0 failures" in QUICKCONTEXT but actually has 63. Run the tests NOW.

```bash
# Run the project test suite to know what's passing NOW
scripts/refresh-context.sh --test-baseline   # Automated (recommended)

# Or manually:
pnpm test          # Node.js
go test ./...      # Go
pytest             # Python
```

Record the baseline. Any new failures at session end are yours to own or
document. **File counts are necessary but not sufficient** — 116 test
files with 63 failures looks identical to 116 test files with 0 failures
in ground truth checks.

---

## Session Checkpoint

### When to Checkpoint

Checkpoint when ANY of these triggers fire:

| Trigger | Why |
|---------|-----|
| **10 commits** since last checkpoint | Enough work to cause significant drift |
| **2 hours** of continuous work | Context quality degrades after sustained effort |
| **Sprint/milestone boundary** | Natural handoff point |
| **Context compaction** | If your context is being compressed, it's time |
| **Before fan-out** | Ensure docs are current before launching parallel agents |

### What to Do

1. **Update QUICKCONTEXT.md** — at minimum: timestamp + what shipped since
   last update. Don't skip this. It takes 2 minutes and saves the next
   session 20.

2. **Commit work-in-progress** — uncommitted work is lost work. Even if
   it's not ready, commit with a `wip:` prefix.

3. **Save memory** (if using Claude Code) — any decisions, discoveries,
   or architectural insights that should survive context compaction.

4. **Check context quality** — if you notice these signals, it's time to
   break and restart fresh:
   - Re-reading files you read 30+ tool calls ago
   - Repeating searches you already did
   - Losing track of which agents are running or what they're doing
   - Confusing details between different parts of the codebase

### Marathon Sessions (>4 hours)

Marathon sessions need more discipline, not less:

- **Context compaction = mandatory break.** When your context is being
  compressed, commit everything, update QUICKCONTEXT, and start a fresh
  session. The fresh session reads the Cold Start Quad and picks up cleanly.

- **Mid-flight handoff:** If work is in progress across multiple worktree
  agents, document the state before breaking:
  ```
  Active agents:
  - worktree-agent-abc: DOCX headers fix (committed, needs cherry-pick)
  - worktree-agent-def: editing pipeline (in progress, ~70% done)
  - worktree-agent-ghi: completed, merged to main
  
  Next: cherry-pick abc, wait for def, then run full test suite
  ```

- **The 50-tool-call rule:** After ~50 tool calls, actively assess whether
  you're still making progress or just churning. If churning, checkpoint
  and restart.

---

## Session End

### The Wrapup Protocol

**Every session should produce a structured handoff.** This takes 5 minutes
and saves the next session 30 minutes of context archaeology.

#### Wrapup Template

```markdown
# Session Wrapup — [YYYY-MM-DD]

## Duration & Scope
[1-2 sentences: what was the session about, how long, how many commits]

## What Shipped (commits, not aspirations)
| Commit | Category | Description |
|--------|----------|-------------|
| abc123 | feat     | Added user authentication flow |
| def456 | fix      | Fixed race condition in session handler |

## Test State at Session End
```bash
# Run these first thing next session:
pnpm test                    # Expected: all passing
npx playwright test          # Expected: 2 known failures (see below)
`` `

## Known Failures (exact repro for each)
1. `npx playwright test -g "login-redirect"` — root cause: auth callback URL mismatch
2. `go test ./internal/store/...` — flaky on CI, passes locally (pre-existing)

## Decisions Made (that future sessions must respect)
- Auth tokens stored client-side only (zero-knowledge architecture)
- BCH recovery key encrypts signing key, does NOT replace it

## Next Session Entry Point
1. Run: `pnpm test` to verify baseline
2. Read: `docs/plans/AUTH_UX_REDESIGN.md` for context
3. Fix: `src/auth/callback.ts:47` — redirect URL uses wrong env var
```

#### Wrapup Checklist

- [ ] QUICKCONTEXT.md updated with current state (not aspirational)
- [ ] TODO.md updated (completed items checked, new items added)
- [ ] `git worktree list` — clean up abandoned worktrees
- [ ] `git status` — commit or stash any uncommitted changes
- [ ] Wrapup written to `docs/session-wrapups/` or saved to memory
- [ ] Test suite state documented (what passes, what fails, why)

### Where to Put the Wrapup

Options, in order of preference:

1. **`docs/session-wrapups/YYYY-MM-DD.md`** — lives in repo, next agent
   finds it automatically. Best for significant sessions.
2. **Claude Code memory** — survives across sessions on the same machine.
   Good for routine sessions.
3. **Inline in QUICKCONTEXT.md** — update the "Recently completed" section.
   Minimum viable wrapup.

---

## Architect Review Checkpoint

At sprint boundaries or every 10 commits, do a quick integration review:

1. **Walk the happy path** — open the app, perform the 5 most common user
   actions. Does each one complete without errors?

2. **Check the console** — are there JavaScript errors? 404s? WebSocket
   failures? Each console error is a broken contract between frontend and
   backend.

3. **Cross-reference APIs** — for each frontend component that calls an API:
   - Does the endpoint exist?
   - Does the response shape match the TypeScript type?
   - Is the data complete (no `undefined` where a value is expected)?

4. **Test the transitions** — the most fragile code is at the seams:
   - Navigation between views
   - Search → result click → detail display
   - Form submit → success/error handling
   - Real-time updates → UI refresh

5. **Write what you find** — each broken seam becomes a bug in TODO.md
   with severity and a description of expected vs actual behavior.

---

## Anti-Patterns

### 1. "I'll update QUICKCONTEXT at the end"

You won't. You'll be excited about the next feature, or tired, or your
context will get compacted. Update at checkpoints, not "at the end."

### 2. "The session is short, I don't need a wrapup"

Even a 1-hour session can make decisions that the next session needs to
know about. At minimum, update QUICKCONTEXT's "Recently completed" section.

### 3. "I'll remember what the agents were doing"

You won't. Agent IDs are random hashes. If you're running 5+ parallel
agents, document which agent is doing what BEFORE you forget.

### 4. "Tests were passing when I checked 2 hours ago"

Run them again. Other agents may have merged work that broke something.
The session-end test state is what matters, not the mid-session state.
