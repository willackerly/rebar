# E2E Testing

**Referenced from AGENTS.md. Read when setting up or debugging E2E test infrastructure.**

---

## Test Fidelity Ladder

Not all E2E tests are equal. A test that calls the API directly and one
that clicks through the UI like a human are testing fundamentally different
things. Declare the fidelity level for each user journey so there's no
ambiguity about what's actually covered.

| Level | Name | What It Tests | When to Use |
|-------|------|---------------|-------------|
| **L1** | API contract | Request/response shapes, status codes | Every endpoint, always |
| **L2** | UI happy path | Click through the primary flow via browser | Every user-facing journey |
| **L3** | Human emulation | Every click, every screen state, every persona | Critical journeys (auth, signing, payment) |
| **L4** | Visual baseline | Pixel-level screenshot comparison | Journeys where layout/styling is part of the contract |

**The key rule:** API helpers may set up preconditions at any level, but
**the journey under test must be driven at the declared fidelity level.**

If the compose wizard is the journey, L2+ means clicking through all 4
steps. Using `createEnvelope()` via API is only acceptable when the wizard
is a *precondition* for testing something else (e.g., the signing flow).

**Declare fidelity per journey in your spec or test plan:**
```markdown
### Journey: Create and Send Envelope
Test fidelity: L3 (human emulation)
Personas: Alice (power user), Bob (first-time)
Visual baseline: L4 for compose wizard review step
```

### When to Use Visual Baselines (L4)

- **Use `toHaveScreenshot()`** for screen states that are part of the
  product contract (login page, signing page, completed state). Layout
  regressions in these states are real bugs.
- **Skip it** for highly dynamic content (dashboards with live data, lists
  with variable items). Baseline maintenance cost exceeds bug-detection value.
- **Always** capture baselines on a fixed viewport size and with
  deterministic data (seeded test accounts, fixed timestamps).
- Update baselines explicitly (`--update-snapshots`) — never auto-accept.
- Review baseline diffs in PRs the same way you review code diffs.

### Persona Coverage Enforcement

When a spec defines personas, each persona must have corresponding tests
or an explicit deferral with reasoning:

```markdown
### Pre-Merge Checklist: Persona Coverage
- [x] Alice (power user): tested in auth-alice.spec.ts
- [x] Bob (first-time signer): tested in auth-bob.spec.ts
- [ ] Carol (upgrade path): deferred — requires migration API not yet built
- [x] Eve (impersonator): tested in auth-adversarial.spec.ts
```

"Explicitly deferred with reason" is acceptable. "Silently absent" is not.

### Notification Parity Testing

When a system sends notifications through multiple channels (email +
in-app, email + SMS, push + in-app), add a parity assertion:

```typescript
// Both channels reference the same event
const email = await waitForEmail(recipient, 'Document Ready');
const inApp = await api.getNotifications(recipient);
const match = inApp.find(n => n.type === 'document_ready');

expect(match).toBeDefined();
expect(match.documentId).toBe(email.metadata.documentId);
```

This is straightforward when mock transports exist (mock-resend,
mock-sendgrid). The pattern catches silent failures where one channel
works but the other doesn't.

---

## Managed Test Stack

When E2E tests require multiple servers (API, frontend, mock services), use a **managed test stack** approach for reliability. The core problems with Playwright's built-in `webServer` are: sequential startup (slow), no PID tracking (orphans), no hard timeouts (hangs), and opaque failures (no logs).

### Architecture: `test-stack.sh`

Create a single shell script that manages the full server lifecycle:

```bash
#!/usr/bin/env bash
# scripts/test-stack.sh — Managed test server stack
# Usage:
#   ./scripts/test-stack.sh run <tier> [playwright-args...]   # start -> test -> stop
#   ./scripts/test-stack.sh start <tier>                      # start servers only
#   ./scripts/test-stack.sh stop                              # kill everything
#   ./scripts/test-stack.sh status                            # show running servers
```

**Key design principles:**

1. **PID tracking** — Write each server's PID to a temp file (`/tmp/<project>-test-stack.pids`). Read it back for reliable cleanup.

2. **EXIT/INT/TERM trap** — `trap 'cleanup' EXIT INT TERM` ensures servers die on Ctrl+C, crash, or normal exit. Never leave orphans.

3. **Parallel startup** — Start all servers with `&`, then health-check each. Much faster than Playwright's sequential `webServer`.

4. **Per-server health checks with dead-process detection** — Poll the health URL, but also check if the PID is still alive. If the process died, show last N lines of its log file immediately instead of waiting for timeout:
    ```bash
    wait_for_health() {
      local name="$1" url="$2" timeout_s="${3:-20}"
      local start_time=$(date +%s)
      while true; do
        local elapsed=$(( $(date +%s) - start_time ))
        [ "$elapsed" -ge "$timeout_s" ] && { show_log "$name"; return 1; }
        curl -sf "$url" >/dev/null 2>&1 && return 0
        # Fail fast if process died
        local pid=$(grep "^${name}=" "$PIDFILE" | cut -d= -f2)
        kill -0 "$pid" 2>/dev/null || { show_log "$name"; return 1; }
        sleep 0.5
      done
    }
    ```

5. **Hard execution timeouts** — Wrap the Playwright command in `gtimeout` (GNU coreutils on macOS) or a background watchdog:
    ```bash
    # macOS: brew install coreutils -> provides gtimeout
    gtimeout --signal=TERM --kill-after=5 90 npx playwright test --config ...
    # Exit code 124 = timeout
    ```

6. **Server logs to files** — Redirect all server output to `/tmp/<project>-test-logs/<server>.log`. On failure, the health check shows the last 15 lines.

7. **Orphan sweep** — After killing tracked PIDs, also kill anything on known test ports. Catches processes that escaped PID tracking:
    ```bash
    for port in 8820 8821 8822 8823; do
      pids=$(lsof -ti :"$port" 2>/dev/null || true)
      [ -n "$pids" ] && echo "$pids" | xargs kill 2>/dev/null || true
    done
    ```

8. **Playwright config dual-mode** — When `__TEST_STACK_RUNNING=1` is set, skip `webServer[]` entirely:
    ```typescript
    const stackRunning = !!process.env.__TEST_STACK_RUNNING;
    export default defineConfig({
      ...(stackRunning ? {} : { webServer: [...] }),
    });
    ```

---

## Fixed Port Ranges

Use deterministic port ranges so cleanup is simple and tiers don't conflict:

```typescript
// tests/e2e/utils/find-ports.ts
const PORT_BASES: Record<string, number> = {
  'unit':       8810,  // fast browser-only tests
  'core':       8820,  // golden-path (API + Web + services)
  'integration': 8830, // full integration suite
  'e2e':        8840,  // complete E2E
};
```

---

## Playwright Configuration Gotchas

These are hard-won lessons. Follow them exactly:

1. **HTML reporter: `open: 'never'`** — Without this, Playwright starts an HTTP server after test failure and blocks forever waiting for Ctrl+C. This is the #1 cause of "test hangs for 10 minutes" in non-interactive contexts:
    ```typescript
    reporter: [
      ['html', { outputFolder: 'playwright-report', open: 'never' }],
      ['list'],  // streaming progress to terminal
    ],
    ```

2. **Zero retries locally** — `retries: 0` in dev. Retries hide bugs and triple execution time. Only enable in CI:
    ```typescript
    retries: process.env.CI ? 1 : 0,
    ```

3. **`globalSetup` vs `webServer` ordering** — Playwright starts `webServer` entries BEFORE running `globalSetup`. If your globalSetup kills servers on test ports, it kills the servers Playwright just launched. Use globalSetup only for non-destructive initialization.

4. **Per-step evaluate timeouts** — A single `page.evaluate()` can hang for the full test timeout (30-90s) with zero output. Wrap in a race:
    ```typescript
    async function evalWithTimeout<T>(
      page: Page, fn: () => Promise<T>, label: string, ms = 10_000
    ): Promise<T> {
      return Promise.race([
        page.evaluate(fn),
        new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error(`${label} hung for ${ms}ms`)), ms)
        ),
      ]);
    }
    ```

5. **`strictPort` for Vite servers** — Without `--strictPort`, Vite silently picks the next available port if the configured one is busy. Your tests then connect to the wrong server or an old orphan:
    ```bash
    npx vite --port 8821 --strictPort  # fails if 8821 is taken
    ```

---

## Environment Variable Hygiene for Tests

**dotenv auto-loading is a footgun.** Many frameworks (Express, Next.js, etc.) auto-load `.env` files at startup via `dotenv.config()`. This means environment variables you set in the test runner can be silently overridden by values in `.env`. Common symptoms:

- Tests fail with auth errors because `.env` has `DATABASE_URL` set, which changes the auth middleware behavior
- Tests hit production services because `.env` has production URLs
- Tests fail with "already in use" because `.env` sets a port that conflicts with the test port

**Defenses:**
1. Keep `.env` minimal — comment out anything not needed for local dev
2. In test startup scripts, explicitly `unset` dangerous env vars before launching servers
3. Use a separate `.env.test` and configure your framework to load it in test mode
4. Never put secrets in `.env` files committed to git (use `.env.local` or `.env.example`)

---

## Tier Timeouts

| Tier | Servers | Timeout | Rationale |
|------|---------|---------|-----------|
| Unit/component | 1 (web only) | 30s | Fast, isolated tests |
| Core/golden | 2-4 | 90s | Critical path, should be fast |
| Integration | 2-4 | 120s | More complex flows |
| Full E2E | All | 300s | Complete system tests |

---

## Dev Server Memory for Long Test Runs

When running E2E suites with many specs (50+), dev servers can leak
memory and get OOM-killed mid-run. Common with Vite, Webpack HMR, and
Next.js dev mode.

**Pattern:** Use production-like servers for E2E, not dev servers.

| Server Type | Memory | Stability |
|------------|--------|-----------|
| Dev server (`vite dev`) | Growing (~500MB+) | OOM-killed during long runs |
| Preview/static (`vite preview`) | Constant (~50MB) | Stable indefinitely |

For Vite: `vite build --mode test && vite preview --port $PORT` gives a
static server with constant memory. The 10-15s build time pays for itself
by eliminating OOM kills. Adjust health check timeouts (60s vs 20s) to
account for the build step.
