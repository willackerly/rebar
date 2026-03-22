# ASK Scoped Knowledge

**ASK** — a Unix-like CLI for querying role-based agents, each with
bounded context and memory. Implements the [ASK-SHELL.md](../ASK-SHELL.md)
proposal (v0: bash proof-of-concept).

## Quick Start

```bash
# Initialize directory structure (creates shared/, sets up agent dirs)
./bin/ask init

# List available agents
./bin/ask who

# Ask an agent a question (synchronous, one-shot)
./bin/ask architect "What contract governs authentication?"

# Check agent status
./bin/ask status architect

# View interaction history
./bin/ask log architect
```

## Installation

Add `bin/` to your PATH, or symlink:

```bash
ln -s "$(pwd)/bin/ask" /usr/local/bin/ask
```

### Dependencies

- **bash** (4.0+)
- **jq** — JSON processing (`brew install jq` / `apt install jq`)
- **claude** — Claude Code CLI (for LLM invocation)

## Commands

| Command | Description |
|---------|-------------|
| `ask <agent> "<question>"` | Send a question, get an answer |
| `ask who` | List available agents |
| `ask where <agent>` | Show agent directory path |
| `ask status <agent>` | Check if agent is running, inbox depth |
| `ask log <agent>` | View interaction history |
| `ask peek <agent>` | Inspect pending inbox messages |
| `ask tail` | Follow live event log |
| `ask up <agent>` | Start agent loop (background listener) |
| `ask init` | Initialize directory structure |
| `ask help` | Show help text |

## Two Modes of Operation

### Direct mode (default)

When you run `ask architect "question"`, the tool checks whether the agent
is running (has a live `.pid` file). If not, it invokes claude directly with
the agent's context and returns the answer synchronously. This is the common
case -- no need to start agents separately.

### Inbox/outbox mode

If you start an agent with `ask up architect`, it runs a loop watching
`agents/architect/inbox/` for question files. When you then run
`ask architect "question"`, the tool writes to the inbox and polls the
outbox for a response. This mode supports background agents that maintain
persistent state.

## Session Persistence

Agents maintain sessions across questions — the first question pays the
context cost, every follow-up resumes the session without re-sending
the full context.

```bash
# First question: full context sent, session established
ask architect "What contracts exist?"

# Follow-ups: resumes session, agent remembers everything
ask architect "Which one governs signing?"
ask architect "Show me the interface definition"

# Context getting full? Auto-cleared at 70% (configurable via ASK_CONTEXT_LIMIT)
# Or manually:
ask reset architect
```

Session IDs are stored in `agents/<name>/.session-id`. When context usage
hits the limit, the session auto-clears and the next question starts fresh
with full context re-sent.

## Composability

`ask` follows Unix conventions. Output is plain text, pipe-friendly:

```bash
# Filter answers
ask architect "what contracts exist?" | grep CONTRACT

# Save responses
ask product "current requirements?" > requirements.txt

# Piped input
echo "is offline access required?" | ask product

# Chain queries
ask product "what matters most?" | ask architect

# Batch questions
cat questions.txt | while read q; do
  ask architect "$q"
done
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Answer returned successfully |
| 1 | Agent could not answer |
| 2 | Agent not available |
| 3 | Timeout |
| 4 | Internal error |

```bash
if ask architect "is this safe?"; then
  echo "approved"
else
  echo "needs review"
fi
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ASK_AGENTS_DIR` | `./agents` | Path to agents directory |
| `ASK_SHARED_DIR` | `./shared` | Path to shared directory |
| `ASK_TIMEOUT` | `120` | Timeout in seconds for inbox/outbox mode |
| `ASK_MODEL` | `sonnet` | Claude model for agent invocations |
| `ASK_CONTEXT_LIMIT` | `70` | Auto-reset session at this context % |
| `ASK_VERBOSE` | `0` | Set to `1` for debug output on stderr |

## Directory Structure

```
agents/
  architect/
    AGENT.md          # Role definition and context
    memory.md         # Distilled current state
    memory.log.md     # Append-only interaction history
    inbox/            # Incoming questions (inbox/outbox mode)
    outbox/           # Outgoing responses (inbox/outbox mode)
    .pid              # PID file when running via `ask up`

shared/
  messages/
    events.jsonl      # Append-only event log (all interactions)
  artifacts/          # Shared documents
  decisions/          # Recorded decisions
```

## Creating an Agent

Create a directory under `agents/` with an `AGENT.md` file:

```bash
mkdir -p agents/englead/inbox agents/englead/outbox

cat > agents/englead/AGENT.md <<'EOF'
# Agent: Engineering Lead

## Role
You are the engineering lead. You coordinate implementation work,
manage technical priorities, and ensure code quality.

## Responsibilities
- Break down architectural decisions into implementation tasks
- Coordinate engineer agents
- Review code quality and test coverage
- Manage technical debt

## Permissions
- Read: all project files
- Write: code, tests, TODO.md
- Ask: any agent
EOF

touch agents/englead/memory.md agents/englead/memory.log.md
```

Then:

```bash
ask who              # should now list: architect, englead, product
ask englead "what is the current priority?"
```

## Event Log

All interactions are logged to `shared/messages/events.jsonl` as
newline-delimited JSON. Each entry has:

```json
{
  "id": "ask-20260318-142211-12345-9876",
  "from": "human",
  "to": "architect",
  "type": "question",
  "timestamp": "2026-03-18T14:22:11Z",
  "body": "What contract governs auth?"
}
```

View live:

```bash
ask tail
# or: tail -f shared/messages/events.jsonl | jq .
```

## What This Version Does Not Handle

This is v0 (bash proof-of-concept). Known limitations:

- No concurrent access protection (race conditions possible with inbox/outbox)
- No structured memory summarization (log grows unbounded)
- No agent-to-agent `ask` (re-entrant calls)
- No graceful shutdown / message draining
- No transport abstraction (filesystem only)

See [IMPLEMENTATION.md](../IMPLEMENTATION.md) for the v1 (hardened bash) and
v2 (Go) roadmap.
