# ASK CLI routing rules + the MCP -32603 misdiagnosis trap

**In one line:** When ASK_SERVER is set, only `ask <repo>:<role>` routes remote — `ask <role>` always runs locally. The MCP server already strips ASK_SERVER from subprocess env, so -32603 errors are NOT env-var related; investigate the actual stderr.

*Migrated from assistant-memory 2026-08-12.*

---
**Routing rule (`bin/ask`):** `_is_remote_query()` returns true ONLY when both
conditions hold: `ASK_SERVER` is non-empty AND the agent spec contains `:`.
This means:
- `ask architect "..."` (no colon) → always local, regardless of `ASK_SERVER`
- `ask filedag:architect "..."` (with colon) + `ASK_SERVER` set → routes to server
- `ask filedag:architect "..."` + `ASK_SERVER` unset/empty → fails (no local cross-repo)
- `(cd ~/dev/<repo> && ask <role> "...")` → always works, no server needed

**Reliable fallback when the ASK server is down:** subshell-cd into the target
repo and run `ask <role>` without the qualified prefix. Bypasses remote routing
entirely.

**MCP -32603 misdiagnosis trap (Apr 2026):** TDFLite session's debugger
hypothesized that `ASK_SERVER=192.168.0.181:7232` inherited via `~/.zshrc` was
causing MCP tool/call -32603 errors. **That hypothesis is wrong.**
`bin/ask-mcp-server` (around line 482) explicitly strips `ASK_SERVER=""` from
the subprocess env before spawning `ask`. The MCP path always routes locally.

The reason this misdiagnosis keeps happening: pre-2026-04-25 the error result
was `result.stderr.strip() or "Ask command failed"` — when stderr was empty
(timeout, exit-by-signal), users got the bare unhelpful fallback and
defaulted to env vars as the explanation. Post-2026-04-25, full stderr +
exit code + cmd + cwd are forwarded; future -32603s name their actual cause.

**Why:** burned ~30 min in the TDFLite Tier-3 push session diagnosing this; the
real root causes for -32603 in practice are: `claude` CLI not on the MCP
server's PATH, agent file mismatch, or LLM endpoint failure. Not env vars.

**How to apply:** When a -32603 lands, read `error.data` for the actual
stderr first. Don't reach for env-var theories until you've read what the
subprocess actually said.
