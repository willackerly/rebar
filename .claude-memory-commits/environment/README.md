# Environment gotchas

Hard-won diagnosis of the toolchain *around* rebar — MCP wiring and ASK routing.
None of it is discoverable from the code, and each cost real hours.

Each file leads with the **symptom**, because that is what you will have when
you arrive.

| Symptom | File |
|---------|------|
| Edited `.mcp.json` / `~/.claude.json` and nothing changed | [mcp-session-reload.md](mcp-session-reload.md) |
| Server "connected" per CLI but invisible in-session | [mcp-notifications-must-not-reply.md](mcp-notifications-must-not-reply.md) |
| `ask` hits the wrong repo, or an opaque `-32603` | [ask-routing-and-error-diagnosis.md](ask-routing-and-error-diagnosis.md) |

Not covered by `TROUBLESHOOTING.md` (verified 2026-08-12) — that guide covers
rebar's own scripts and conventions, not the MCP/ASK layer.
