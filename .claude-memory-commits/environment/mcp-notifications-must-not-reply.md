# Custom MCP servers must not reply to JSON-RPC notifications

**In one line:** When building/debugging stdio MCP servers, replying to notifications (requests with no "id" field) with id=null responses causes Claude Code's Zod validator to drop the connection silently — tools appear "connected" but none surface in sessions

*Migrated from assistant-memory 2026-08-12.*

---
JSON-RPC 2.0 §4.1: a request without an `"id"` member is a **notification**. Servers MUST NOT reply. MCP uses notifications constantly — `notifications/initialized` is sent right after `initialize` response, and others fire on cancellation, progress, etc.

**The failure mode when servers reply anyway:**

1. Server sends `{"jsonrpc":"2.0","id":null,"error":{...}}` (or a result with id=null)
2. Claude Code's Zod schema on the client side rejects id=null (expects string or number)
3. Claude Code drops the stdio transport after ~0s uptime with `stdio transport error: ZodError`
4. `claude mcp list` still shows ✓ Connected (the initial handshake succeeded)
5. But no tools ever surface in the session — `/mcp` inside session shows empty, no `mcp__<server>__*` tools in tool list

**Why:** Connection = stdio pipe established + initialize exchange. Tools come later, but Claude Code cancels the connection after the invalid notification reply, before it ever asks tools/list.

**How to diagnose:**

```bash
cd <project-where-your-server-registered>
claude --debug-file /tmp/x.log --print "brief test"
grep -i "ZodError\|connection dropped\|stdio transport error" /tmp/x.log
```

If you see `connection dropped after 0s uptime` or ZodError detail mentioning `"id"` with `"received null"` — this is the bug, not session staleness, not config format.

**How to fix (Python MCP server pattern):**

```python
def handle_request(self, request):
    is_notification = "id" not in request
    if is_notification:
        return None  # swallow, no reply

    # ... handle actual requests ...

# stdio loop:
for line in sys.stdin:
    request = json.loads(line)
    response = server.handle_request(request)
    if response is None:  # notification — don't reply
        continue
    print(json.dumps(response))
    sys.stdout.flush()
```

**Also:** on JSON parse error, don't reply with id=null — just log to stderr. You can't tell if the malformed input was a notification, and guessing wrong is worse than silence.

**How to apply:** Any time a custom MCP server is "connected per CLI but invisible in session", suspect this before blaming config/restarts. Run the `claude --debug-file` trick; it reveals the actual failure within seconds. Rebar's `bin/ask-mcp-server` shipped with this bug until 2026-04-22 commit `0db9073`.
