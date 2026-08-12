# Claude Code MCP config requires session restart

**In one line:** Editing ~/.claude.json mcpServers or .mcp.json does not hot-reload; must restart Claude Code for new MCP servers to appear in tool list

*Migrated from assistant-memory 2026-08-12.*

---
When adding or modifying MCP servers (user-scope in `~/.claude.json` top-level `mcpServers`, project-scope via `.mcp.json` at repo root, or local-scope via `claude mcp add`), Claude Code only picks up the change at **session launch**. Running sessions keep their frozen tool list.

**Why:** MCP servers are initialized once during session startup — there's no watcher on the config files.

**How to apply:** After any MCP config change, tell the user explicitly: "exit this Claude Code session and start a fresh one to see the new tools." Don't just say "reload" — that's ambiguous. Also point them at diagnostic commands:
- `claude mcp list` — outside-session check; works immediately after config change
- `claude mcp get <name>` — shows scope + connection status
- `/mcp` inside a session — live state for that session

**Signal that the user hit this:** they edit/add a config, start a tool, and ask "why don't I see X MCP tool?" — if `claude mcp list` shows it connected, the answer is almost always "your session was started before the edit."

**BUT** a restart won't help if the server itself is broken at the protocol level. Always rule out the *other* failure mode too: the server connects successfully but drops the transport during protocol exchange. Diagnostic: `cd <project> && claude --debug-file /tmp/x.log --print "test"` and `grep -i "ZodError\|connection dropped\|stdio transport error" /tmp/x.log`. If you see ZodError or "connection dropped after 0s uptime", the server is sending something Claude Code's Zod validator rejects — NOT a session-staleness issue.
