# Red Team Protocol

**Referenced from AGENTS.md. Read before running adversarial quality reviews.**

---

## When to Use

- After a major architectural change (new subsystem, rewrite, migration)
- Before a release or deploy to production
- When you suspect hidden quality issues
- After a marathon session that touched many files
- When adopting a new dependency or integration

---

## How It Works

A red team review uses **multiple adversarial personas**, each examining
the codebase from a different angle. The personas are complementary — each
finds classes of bugs the others miss.

**Two modes:**

| Mode | How | When |
|------|-----|------|
| **Single-agent** | One agent wears all 5 hats, produces a unified report | Quick review, smaller scope |
| **Fan-out** | One agent per persona, each produces independent findings | Deep review, large scope |

For single-agent mode, use `agents/subagent-prompts/red-team.md`.
For fan-out, assign each persona to a separate agent with the relevant
section of the template.

---

## The Five Personas

### 1. Adversarial User

**Mindset:** "I'm going to try every wrong input, every edge case, every
abuse scenario. I want to break this."

**Looks for:**
- What happens with empty inputs, null values, extreme lengths?
- What happens when I do things out of order?
- What happens when I spam-click, double-submit, or navigate away mid-operation?
- What happens with Unicode, RTL text, emoji, special characters?
- What can I access that I shouldn't be able to?

**Output:** Bug reports with exact reproduction steps.

### 2. Performance Engineer

**Mindset:** "I'm going to find every O(n^2), every memory leak, every
unnecessary re-render, every bundle bloat."

**Looks for:**
- Nested loops over large collections (O(n^2) or worse)
- Unbounded growth: caches without eviction, arrays that only grow, event
  listeners that only add
- Unnecessary re-renders (React: missing memoization, unstable references)
- Large bundle imports (importing all of lodash for one function)
- Database queries in loops (N+1 problem)
- Synchronous I/O on the main thread

**Output:** Performance findings with estimated impact and suggested fix.

### 3. Security Analyst

**Mindset:** "I'm an attacker. What's the cheapest path to exfiltration,
escalation, or denial of service?"

**Looks for:**
- Input injection: SQL, XSS, command injection, path traversal
- Auth/authz gaps: missing checks, client-only validation, token handling
- Crypto misuse: weak algorithms, nonce reuse, missing key zeroization
- Data exposure: secrets in logs, debug endpoints in prod, verbose errors
- Dependency risks: known CVEs, unmaintained packages, supply chain

**Output:** Security findings with severity (Critical/High/Medium/Low),
attack vector, blast radius, and remediation.

For a deeper security-focused review, use the standalone
`agents/subagent-prompts/security-surface-scan.md` template.

### 4. Fidelity Analyst

**Mindset:** "Does this system preserve data accurately across every
transformation? Is there any lossy conversion, silent truncation, or
format deviation?"

**Looks for:**
- Data loss in format conversions (import/export, serialization)
- Silent truncation (strings, numbers, precision)
- Encoding issues (UTF-8, binary data, base64)
- Type coercion surprises (JavaScript's `==`, Go's nil vs zero-value)
- Visual fidelity: does the rendered output match the source?

**Output:** Fidelity findings with before/after comparison and severity.

For visual output projects, see also `practices/visual-fidelity.md`.

### 5. API/Contract Reviewer

**Mindset:** "Does this code do what its contract says? Are the interfaces
clean, the error handling consistent, the types correct?"

**Looks for:**
- Contract violations: behavior doesn't match specification
- Error handling: inconsistent error types, swallowed errors, missing cases
- Type safety: `any` casts, unchecked type assertions, unsafe conversions
- Interface design: leaky abstractions, God objects, unclear boundaries
- Missing contracts: behavior exists but no contract covers it

**Output:** Contract findings with severity, file:line, and suggested fix.

This persona is closely related to the `architect` role agent and the
`agents/subagent-prompts/contract-audit.md` template.

---

## Running a Red Team

### Step 1: Define Scope

What are you reviewing? Be specific:
- A single component: `src/auth/`
- A feature area: "everything related to document signing"
- A recent change set: "all commits since v2.1.0"
- The full application: "everything"

### Step 2: Launch the Review

**Single-agent mode (quick):**
```
Agent(prompt: "Read agents/subagent-guidelines.md for behavioral rules.
              Read agents/subagent-prompts/red-team.md for your task.
              Parameters: TARGET=src/auth/ OUTPUT=agents/results/red-team-auth.json")
```

**Fan-out mode (deep):**
Launch 3-5 agents, each with one persona and the relevant scope.
Use worktree isolation if any persona might suggest code changes.

### Step 3: Triage the Report

1. **Group findings by root cause**, not by persona. If the adversarial
   user and the security analyst both found the same input validation gap,
   that's one issue with two symptoms, not two issues.

2. **Prioritize:** Critical → High → Medium → Low

3. **Build the fix DAG:** Which fixes are independent? Which depend on
   others? Independent fixes can be parallelized.

### Step 4: Fix and Verify

1. Fan out agents on independent fix clusters
2. Run the relevant test suite after each fix
3. Re-run the specific red team checks that found the issues
4. Document any issues deferred to backlog with severity and reasoning

---

## Integrating with Existing Templates

The red team protocol works alongside existing review templates:

| If you need... | Use... |
|----------------|--------|
| Full adversarial review | `red-team.md` (this protocol) |
| Deep security audit | `security-surface-scan.md` |
| Deep UX audit | `ux-review.md` |
| Contract conformance | `contract-audit.md` |
| Product alignment | `product-review.md` |
| Code quality | `code-review.md` |

The red team is the **breadth** pass — it finds issues across all
dimensions. The specialized templates are the **depth** pass — they go
deeper on a single dimension.

**Recommended workflow for releases:**
1. Red team review (breadth) → find issues across all dimensions
2. Fix critical/high issues
3. Specialized reviews (depth) on areas where the red team found clusters
4. Fix remaining issues
5. Final regression test

---

## Anti-Patterns

### 1. "We'll red-team it after launch"

After launch, you're fixing bugs under pressure. Red team before launch,
when you can fix things calmly.

### 2. "The security persona found nothing, so we're secure"

One persona's "nothing found" means nothing when the scope was limited.
Security review is complementary to, not a substitute for, a dedicated
security surface scan.

### 3. "We fixed all 18 issues, ship it"

The fix for issue #7 might have introduced issue #19. Always re-run the
test suite after the fix campaign, and consider a mini red-team on the
fix commits themselves.
