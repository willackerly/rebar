# Implementation Notes: Agent Runtime & Ask Shell

## Status: Design Notes

Concrete implementation details for [AGENT-RUNTIME.md](AGENT-RUNTIME.md)
and [ASK-SHELL.md](ASK-SHELL.md), progressing from a bash proof-of-concept
to a mature Go implementation.

---

## v0: Bash Proof-of-Concept (build in an afternoon)

The entire system can be prototyped in ~100 lines of bash. This is
intentional — if the design requires more than bash to prove out, it's
too complex.

### `ask` CLI (v0)

```bash
#!/usr/bin/env bash
# ask — query an agent
# Usage: ask <agent> "<question>"
#        ask up <agent>
#        ask who
#        ask status <agent>

set -euo pipefail

AGENTS_DIR="${AGENTS_DIR:-./agents}"
CMD="${1:-}"
AGENT="${2:-}"

case "$CMD" in
  up)
    exec "$AGENTS_DIR/$AGENT/run.sh"
    ;;

  who)
    for d in "$AGENTS_DIR"/*/; do
      [ -f "$d/AGENT.md" ] && basename "$d"
    done
    exit 0
    ;;

  status)
    pidfile="$AGENTS_DIR/$AGENT/.pid"
    if [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
      inbox_count=$(ls "$AGENTS_DIR/$AGENT/inbox/" 2>/dev/null | wc -l | tr -d ' ')
      echo "STATUS: running"
      echo "INBOX: $inbox_count pending"
      echo "PID: $(cat "$pidfile")"
    else
      echo "STATUS: stopped"
    fi
    exit 0
    ;;

  log)
    cat "$AGENTS_DIR/$AGENT/memory.log.md" 2>/dev/null
    exit 0
    ;;

  peek)
    ls -1t "$AGENTS_DIR/$AGENT/inbox/" 2>/dev/null
    exit 0
    ;;

  *)
    # Default: ask a question
    AGENT="$CMD"
    QUESTION="$2"

    # Generate message ID
    MSG_ID="ask-$(date +%Y%m%d-%H%M%S)-$$"

    # Write question to inbox
    cat > "$AGENTS_DIR/$AGENT/inbox/$MSG_ID.json" <<EOJSON
{
  "id": "$MSG_ID",
  "from": "human",
  "type": "question",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "body": "$QUESTION"
}
EOJSON

    # Poll for response (timeout 120s)
    TIMEOUT=120
    elapsed=0
    while [ ! -f "$AGENTS_DIR/$AGENT/outbox/$MSG_ID.json" ]; do
      sleep 1
      elapsed=$((elapsed + 1))
      [ "$elapsed" -ge "$TIMEOUT" ] && echo "TIMEOUT" && exit 3
    done

    # Format response as plain text
    answer=$(cat "$AGENTS_DIR/$AGENT/outbox/$MSG_ID.json")
    echo "$answer" | jq -r '"ANSWER: \(.answer)\n\nRATIONALE:\n\(.rationale)\n\nREFS:\n\(.refs // [] | map("- " + .) | join("\n"))"' 2>/dev/null \
      || echo "$answer"

    # Cleanup
    rm -f "$AGENTS_DIR/$AGENT/outbox/$MSG_ID.json"
    exit 0
    ;;
esac
```

### Agent Loop (v0)

```bash
#!/usr/bin/env bash
# run.sh — agent loop for a single agent
# Watches inbox/, processes questions, writes to outbox/

set -euo pipefail

AGENT_DIR="$(cd "$(dirname "$0")" && pwd)"
AGENT_NAME="$(basename "$AGENT_DIR")"
INBOX="$AGENT_DIR/inbox"
OUTBOX="$AGENT_DIR/outbox"

mkdir -p "$INBOX" "$OUTBOX"
echo $$ > "$AGENT_DIR/.pid"
trap 'rm -f "$AGENT_DIR/.pid"' EXIT

echo "[$AGENT_NAME] online, watching $INBOX"

while true; do
  for msg in "$INBOX"/*.json; do
    [ -f "$msg" ] || continue

    MSG_ID=$(jq -r '.id' "$msg")
    QUESTION=$(jq -r '.body' "$msg")

    echo "[$AGENT_NAME] processing: $QUESTION"

    # Load context: AGENT.md defines what this agent reads
    CONTEXT=$(cat "$AGENT_DIR/AGENT.md" "$AGENT_DIR/memory.md" 2>/dev/null || true)

    # Invoke Claude (or any LLM) with the agent's context + question
    # This is the only line that's model-specific
    RESPONSE=$(claude --print \
      "You are the $AGENT_NAME agent. Your role and context:
$CONTEXT

Question: $QUESTION

Respond with JSON: {\"answer\": \"...\", \"rationale\": \"...\", \"refs\": [...]}" \
      2>/dev/null || echo '{"answer": "error", "rationale": "invocation failed"}')

    # Write response
    echo "$RESPONSE" > "$OUTBOX/$MSG_ID.json"

    # Append to memory log
    echo "---" >> "$AGENT_DIR/memory.log.md"
    echo "**$(date -u +%Y-%m-%dT%H:%M:%SZ)** Q: $QUESTION" >> "$AGENT_DIR/memory.log.md"
    echo "A: $(echo "$RESPONSE" | jq -r '.answer' 2>/dev/null)" >> "$AGENT_DIR/memory.log.md"

    # Consume message
    rm "$msg"

    echo "[$AGENT_NAME] answered: $MSG_ID"
  done

  sleep 2
done
```

### What v0 proves

- The `ask` interface works
- Filesystem transport works
- Agent isolation works
- Memory accumulation works
- The whole thing is inspectable with `ls`, `cat`, `jq`
- Zero dependencies beyond bash, jq, and a Claude CLI

### What v0 doesn't handle

- Concurrent access to inbox (race conditions)
- Structured memory summarization
- Agent-to-agent `ask` (re-entrant calls)
- Health monitoring beyond pid checks
- Graceful shutdown / message draining
- Transport abstraction (it IS the filesystem)

---

## v1: Bash with Guardrails

Same architecture, hardened:

- `flock` for inbox file locking (prevents races)
- `fswatch` or `inotifywait` instead of polling (less CPU)
- Proper signal handling (SIGTERM drains in-flight, SIGKILL is caught)
- `start-agents.sh` manages all agents as background processes
- `ask tail` uses `tail -f` on the event log
- Memory summarization via periodic Claude invocation
  ("summarize memory.log.md into memory.md, keep it under 100 lines")

This is the version you run for weeks on a real project to validate the
model before investing in Go.

---

## v2: Go Implementation (the mature version)

### Why Go

- **Single binary.** `ask` and `agent-runner` are two binaries with zero
  runtime dependencies. Copy to any machine and run.
- **Concurrency.** Each agent is a goroutine (or a managed subprocess).
  Inbox watching, message routing, health checks — all natural with
  goroutines and channels.
- **Fast startup.** Agents are processes that start in milliseconds.
  `ask` CLI returns instantly for cached/fast responses.
- **Cross-platform.** Same binary on macOS, Linux, CI runners.
- **Stdlib is sufficient.** Filesystem ops, JSON, HTTP server (for future
  web UI), signal handling — all in stdlib. No frameworks needed.

### Not Go-specific things that should stay out

- **LLM invocation.** Shell out to `claude` CLI (or any LLM). The agent
  runner doesn't import AI SDKs — it's a process manager that happens to
  invoke AI tools, not an AI framework.
- **Transport protocol.** Define as an interface. Filesystem is the default
  implementation. NATS/Redis are additional implementations. The agent
  code never knows which transport it's using.

### Architecture

```
cmd/
  ask/              # CLI binary
    main.go
  agent-runner/     # agent lifecycle manager
    main.go

internal/
  agent/            # agent lifecycle, context loading, memory
    agent.go
    memory.go
    loop.go
  transport/        # message transport abstraction
    transport.go    # interface
    filesystem.go   # v1: file-based inbox/outbox
    nats.go         # v2: NATS transport (future)
  message/          # message types, serialization
    message.go
  shell/            # ask CLI command handlers
    ask.go
    status.go
    log.go
    who.go

pkg/
  contracts/        # shared types for the ask protocol
    question.go
    answer.go
```

### Key Interfaces

```go
// transport.go — the only abstraction that matters
type Transport interface {
    Send(ctx context.Context, to string, msg *message.Question) error
    Receive(ctx context.Context, agent string) (<-chan *message.Question, error)
    Respond(ctx context.Context, msgID string, ans *message.Answer) error
    WaitResponse(ctx context.Context, msgID string, timeout time.Duration) (*message.Answer, error)
}

// agent.go — what an agent does
type Agent struct {
    Name       string
    Dir        string        // filesystem root for this agent
    Role       string        // from AGENT.md
    Transport  Transport
    Invoker    Invoker       // LLM invocation (shelled out)
    Memory     *Memory
}

// invoker.go — how we call the LLM (deliberately thin)
type Invoker interface {
    Invoke(ctx context.Context, prompt string) (string, error)
}

// ClaudeInvoker shells out to `claude --print`
// OpenAIInvoker calls the API directly
// MockInvoker returns canned responses for testing
```

### `ask` CLI

```go
// cmd/ask/main.go — the user-facing binary
func main() {
    switch os.Args[1] {
    case "up":       shell.Up(agent)
    case "who":      shell.Who()
    case "status":   shell.Status(agent)
    case "log":      shell.Log(agent)
    case "peek":     shell.Peek(agent)
    case "tail":     shell.Tail(agent)
    case "where":    shell.Where(agent)
    default:         shell.Ask(agent, question)
    }
    // Exit codes per ASK-SHELL.md spec
}
```

### `agent-runner`

```go
// cmd/agent-runner/main.go — manages agent lifecycle
func main() {
    // Load agent definitions from agents/*/AGENT.md
    // Start each agent's loop as a goroutine
    // Handle signals (SIGTERM → drain, SIGINT → immediate stop)
    // Health check loop (detect stuck agents, restart if needed)
    // Memory summarization scheduler
}
```

### Filesystem Transport (v1 default)

```go
// internal/transport/filesystem.go
type FilesystemTransport struct {
    BaseDir string // agents/
}

func (t *FilesystemTransport) Send(ctx context.Context, to string, msg *message.Question) error {
    path := filepath.Join(t.BaseDir, to, "inbox", msg.ID+".json")
    return writeJSON(path, msg)  // atomic write via temp file + rename
}

func (t *FilesystemTransport) Receive(ctx context.Context, agent string) (<-chan *message.Question, error) {
    // Use fsnotify to watch inbox/ directory
    // Yield messages as they appear
    // flock each file before reading to prevent races
}

func (t *FilesystemTransport) Respond(ctx context.Context, msgID string, ans *message.Answer) error {
    // Write to outbox/, append to event log
}
```

### Memory Manager

```go
// internal/agent/memory.go
type Memory struct {
    StateFile string  // memory.md — distilled state
    LogFile   string  // memory.log.md — append-only
    MaxLogLines int   // trigger summarization at this threshold
    Invoker   Invoker // LLM for summarization
}

func (m *Memory) Append(entry string) error {
    // Append to log
    // If log exceeds MaxLogLines, trigger summarization
}

func (m *Memory) Summarize(ctx context.Context) error {
    // Read memory.log.md
    // Invoke LLM: "Summarize into <100 lines, preserve decisions and refs"
    // Write to memory.md
    // Truncate log (keep last N entries as overlap)
}

func (m *Memory) Load() (string, error) {
    // Return memory.md content for context injection
}
```

### Event Log

```go
// Append-only JSONL — every message recorded
func (t *FilesystemTransport) logEvent(msg interface{}) {
    f, _ := os.OpenFile("shared/messages/events.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    defer f.Close()
    json.NewEncoder(f).Encode(msg)
}
```

### What Go buys over bash

| Capability | Bash v1 | Go v2 |
|-----------|---------|-------|
| Concurrent inbox access | `flock` (fragile) | `fsnotify` + channels (robust) |
| Agent-to-agent `ask` | re-entrant bash (messy) | goroutines (natural) |
| Health monitoring | pid check | goroutine with metrics |
| Graceful shutdown | trap + sleep | context cancellation + drain |
| Transport swap | rewrite scripts | swap interface implementation |
| Cross-platform | bash differences | single binary |
| Testing | shell tests (painful) | `go test` with mock transport |
| Memory summarization | cron + claude call | integrated scheduler |

---

## Build & Distribution

```bash
# Build both binaries
go build -o bin/ask ./cmd/ask
go build -o bin/agent-runner ./cmd/agent-runner

# Install
cp bin/ask /usr/local/bin/
cp bin/agent-runner /usr/local/bin/

# Or: single binary with subcommands
go build -o bin/agents ./cmd/agents
# agents ask architect "..."
# agents up architect
# agents run  (starts all agents)
```

### Single binary consideration

The `ask` CLI and `agent-runner` could be one binary with subcommands.
Pros: one thing to install. Cons: `ask` should be tiny and fast (it's the
hot path), `agent-runner` is heavier. Probably start as one binary, split
if startup time becomes an issue.

---

## Testing Strategy

```go
// Mock transport for unit tests
type MockTransport struct {
    Sent     []message.Question
    Answers  map[string]*message.Answer
}

// Mock invoker for testing without LLM
type MockInvoker struct {
    Responses map[string]string  // question pattern → response
}
```

Tests should cover:
- Message routing (question reaches correct agent)
- Timeout handling (agent doesn't respond within deadline)
- Memory accumulation and summarization trigger
- Concurrent access (two questions to same agent)
- Graceful shutdown (in-flight messages complete)
- Event log integrity (append-only, no mutations)
- Exit codes match ASK-SHELL.md spec

---

## What NOT to Build

- **Web UI.** Not yet. The CLI is the interface. A web UI is a Phase 4 concern.
- **Custom LLM integration.** Shell out to `claude` or `openai` CLI. Don't
  import SDKs into the runner. The runner is a process manager, not an AI framework.
- **Distributed agents.** v2 is local-only. Multiple machines is Phase 3+
  (when you swap to NATS).
- **Authentication.** Local filesystem permissions are sufficient for v1-v2.
  Auth matters when you add network transport.
- **Agent marketplace / registry.** Way too early. Agents are directories
  with an AGENT.md file. That's the registry.

---

## Migration Path

### From bash v0 to bash v1

- Same directory structure, same message format
- Add flock, fswatch, signal handling
- Zero migration needed — v0 messages work in v1

### From bash v1 to Go v2

- Same directory structure, same message format (JSON)
- Same inbox/outbox convention
- `ask` CLI is a drop-in replacement (same flags, same exit codes)
- `agent-runner` replaces `start-agents.sh`
- Can run mixed: Go `ask` CLI with bash `run.sh` agent loops (or vice versa)

### From filesystem to NATS (future)

- Agent code doesn't change (Transport interface)
- `ask` CLI gets a `--transport nats://...` flag
- Filesystem transport remains the default
- Event log moves from JSONL file to NATS JetStream

---

## Timeline Estimate

| Phase | Effort | Deliverable |
|-------|--------|------------|
| v0 bash | 1 afternoon | Working proof of concept |
| v1 bash | 1 week | Production-usable local system |
| v2 Go (ask CLI) | 2-3 days | Fast, cross-platform CLI |
| v2 Go (agent-runner) | 1 week | Managed agent lifecycle |
| v2 Go (memory mgmt) | 3-4 days | Automatic summarization |
| v2 Go (full) | 2-3 weeks | Complete Go implementation |
