# ASK-SHELL.md (Proposed)

## Status: Optional Extension

This document defines a Unix-like command surface for interacting with agents.

It builds on [AGENT-RUNTIME.md](AGENT-RUNTIME.md) and introduces:

> **a shell-native interface for role-based cognition**

---

## Philosophy

The goal is not to build a framework.

The goal is to make working with agents feel like:

```bash
grep
ls
cat
```

Simple. Composable. Predictable.

---

## Core Primitive

### `ask`

```bash
ask <agent> "<question>"
```

- Synchronous
- Returns a single answer
- Exit code reflects success / failure

---

## Supporting Commands

These are intentionally minimal and orthogonal.

### `ask up`

Bring an agent online.

```bash
ask up architect
```

- Starts a listener loop
- Processes incoming questions
- Runs until stopped

### `ask log`

View historical interactions.

```bash
ask log architect
```

Outputs:

- recent questions
- answers
- timestamps

Supports:

```bash
ask log architect | grep offline
```

### `ask peek`

Inspect pending questions without consuming them.

```bash
ask peek architect
```

Useful for:

- debugging
- understanding backlog
- observability

### `ask tail`

Follow live activity.

```bash
ask tail architect
```

Equivalent to:

```bash
tail -f shared/messages/events.jsonl
```

### `ask who`

Discover available agents.

```bash
ask who
```

Outputs:

```text
product
architect
englead
engineer
```

### `ask where`

Resolve agent location.

```bash
ask where architect
```

Outputs:

```text
./agents/architect
```

### `ask status`

Check agent health.

```bash
ask status architect
```

Outputs:

```text
STATUS: running
INBOX: 2 pending
LAST_ACTIVE: 12s ago
```

---

## Exit Codes

Unix tools communicate through exit codes. `ask` should do the same.

| Code | Meaning |
|------|---------|
| 0 | Answer returned successfully |
| 1 | Agent could not answer |
| 2 | Agent not available |
| 3 | Timeout |
| 4 | Internal error |

This enables:

```bash
if ask architect "is this safe?"; then
  echo "approved"
else
  echo "needs review"
fi
```

---

## Output Format

Output must be:

- plain text
- human-readable
- pipe-friendly

Example:

```text
ANSWER: Yes

RATIONALE:
Offline access is required by BDD scenarios.

REFS:
- bdd/offline.feature
- architecture/CONTRACT-OFFLINE-SYNC.1.0.md
```

---

## Composability

The system should work with standard Unix tools.

### Filter answers

```bash
ask architect "what contracts relate to auth?" | grep CONTRACT
```

### Save responses

```bash
ask product "what are current requirements?" > requirements.txt
```

### Chain queries

```bash
ask product "what matters most?" | ask architect
```

### Batch questions

```bash
cat questions.txt | while read q; do
  ask architect "$q"
done
```

---

## Agent-to-Agent Usage

Agents SHOULD use the same interface:

```bash
ask architect "clarify retry semantics"
```

Not:

- direct file reads of other agents
- implicit assumptions

---

## Streaming Mode (Future)

```bash
ask architect --stream "audit entire system"
```

- outputs partial responses
- long-running tasks

---

## Async Mode (Future)

```bash
ask architect --async "deep review"
```

- returns immediately
- writes result to log/artifact

---

## Design Constraints

To preserve Unix-like behavior:

### 1. No hidden state in CLI

All state lives in:

- filesystem
- logs
- artifacts

### 2. Text is the interface

Even if JSON is used internally, output remains text.

### 3. Commands are orthogonal

Each command does one thing well.

### 4. Predictability over magic

No implicit routing (initially).

Explicit:

```bash
ask architect "..."
```

Not:

```bash
ask "who knows this?"
```

### 5. Errors are explicit

No silent failures.

---

## Anti-Patterns

Avoid:

- turning `ask` into chat
- hiding behavior behind magic routing
- requiring structured input formats
- overloading commands with options

---

## Future Extensions

- `ask route` — dynamic routing
- `ask vote` — multi-agent consensus
- `ask diff` — compare answers across agents
- `ask trace` — show reasoning path
- `ask replay` — replay past decision

---

## Summary

This system should feel like:

- `grep` for knowledge
- `ls` for agents
- `tail` for cognition
- pipes for composition

If it feels like a framework, it's wrong.

If it feels like a shell, it's right.
