# AGENT-RUNTIME.md (Proposed)

## Status: Proposal (Phased Adoption)

This document proposes a lightweight runtime layer for the rebar
methodology.

It is intentionally:

- **optional**
- **incrementally adoptable**
- **local-first**
- **tool-agnostic**

You do NOT need to adopt this to use the methodology.

However, if adopted, this turns the system from:

> contract-driven documentation

into:

> **contract-driven multi-agent execution**

---

## Core Idea

Agents are not just prompts.

They are:

> **long-lived processes with scoped memory, filesystem boundaries, and a query interface**

---

## The `ask` Primitive

We introduce a single, universal interface for agent-to-agent and
human-to-agent interaction:

```bash
ask <agent> "<question>"
```

Examples:

```bash
ask product "do we require offline access?"
ask architect "what contract governs sync retries?"
ask englead "what is the current blocker?"
```

### Design Principle

> `ask` is to agents what `grep` is to code.

- Simple
- Ubiquitous
- Abstracts the implementation
- Stable over time

Agents SHOULD use `ask` instead of:

- direct file inspection of other agents
- implicit assumptions about other roles
- free-form chat

See [ASK-SHELL.md](ASK-SHELL.md) for the full shell interface specification.

---

## Agent Runtime Model

Each agent runs as a local process.

Example:

```bash
./start-agents.sh
```

Launches:

- product agent
- architect agent
- eng lead agent
- engineer agent(s)

Each agent:

### Reads

- Cold Start Quad
- its own memory files
- relevant shared artifacts

### Writes

- memory updates
- artifacts
- decisions
- responses to `ask`

### Communicates

- exclusively via `ask`

---

## Directory Structure (Proposed)

```bash
agents/
  product/
    AGENT.md          # role definition, context loading order, permissions
    memory.md         # distilled current state
    memory.log.md     # append-only history
    inbox/            # incoming questions
    outbox/           # outgoing responses
    run.sh            # agent loop entrypoint

  architect/
    AGENT.md
    memory.md
    memory.log.md
    inbox/
    outbox/
    run.sh

  englead/
    AGENT.md
    memory.md
    memory.log.md
    inbox/
    outbox/
    run.sh

  engineer/
    AGENT.md
    memory.md
    memory.log.md
    inbox/
    outbox/
    run.sh

shared/
  artifacts/          # shared documents, briefs, summaries
  decisions/          # recorded architectural decisions
  messages/           # event log
```

---

## Communication Model

### External Interface (Stable)

Agents and humans ONLY interact via:

```bash
ask <agent> "<question>"
ask up <agent>
```

- `ask` — synchronous query
- `ask up` — bring agent online to process requests

### Internal Transport (Replaceable)

The system MAY use:

- filesystem (v1)
- NATS / Redis (future)
- other transports

This is hidden from agents.

---

## Message Format (v1: JSON, internal only)

Internally, messages use JSON. This is NOT exposed to agents.

Example:

```json
{
  "id": "ask-20260317-142211-001",
  "from": "englead",
  "to": "architect",
  "type": "question",
  "timestamp": "2026-03-17T14:22:11Z",
  "body": "Do we have a requirement for offline access?",
  "context": {
    "artifacts": ["README.md"]
  }
}
```

Response:

```json
{
  "id": "ask-20260317-142211-001",
  "type": "answer",
  "answer": "yes",
  "rationale": "...",
  "refs": ["bdd/offline.feature"],
  "followup": null,
  "actions": ["created TODO in agents/product/TODO.md"]
}
```

**Important:** JSON is an implementation detail. The `ask` interface MUST
remain stable regardless of format.

---

## Agent Loop

Each agent runs:

```text
loop:
  check for new questions
  if question:
    load relevant context
    invoke reasoning (Claude / other)
    update memory
    produce answer
    emit response
```

---

## Memory Model

Each agent maintains:

```bash
memory.md       # distilled current state
memory.log.md   # append-only history
```

Agents:

- append to log
- periodically summarize into memory.md

This prevents context explosion.

---

## Permissions Model (Recommended)

| Resource  | Product | Architect | Eng Lead | Engineer |
|-----------|---------|-----------|----------|----------|
| BDD       | R/W     | R         | R        | R        |
| Contracts | R       | R/W       | R        | R        |
| Code      | —       | R         | R/W      | R/W      |
| Tests     | —       | R         | R/W      | R/W      |
| Memory    | own     | own       | own      | own      |

---

## Artifact-Centric Coordination

Agents SHOULD prefer:

> updating artifacts + emitting events

over:

> conversational coordination

**BAD:**
- "hey engineer change retry logic"

**GOOD:**
- update contract
- emit artifact update
- engineer reacts

---

## Snapshotting (Future)

```bash
.snapshots/
  2026-03-17/
    agents/
    shared/
```

Enables:

- replay
- branching
- debugging

---

## Event Log

Append-only:

```bash
shared/messages/events.jsonl
```

Every message:

- recorded
- never mutated

---

## Phased Adoption Plan

### Phase 1 (Now)

- introduce `ask` CLI
- filesystem-based messaging
- per-agent directories
- simple agent loops

### Phase 2

- structured memory updates
- action tracking (`actions` field)
- better routing / escalation

### Phase 3

- replace transport (NATS / Redis)
- add async `ask`
- add retries, timeouts

### Phase 4

- snapshotting / branching
- DAG-based artifact graph
- voting / arbitration

---

## Design Principles

### 1. Agents are processes, not prompts

They persist and evolve.

### 2. Filesystem is source of truth

Not the model context.

### 3. `ask` is the only interface

Everything else is implementation detail.

### 4. Communication is structured

Not free-form chat.

### 5. Memory is reconstructed

Agents rebuild context each loop.

### 6. Logs are append-only

Never delete history.

---

## What This Enables

- role-based reasoning
- explicit knowledge boundaries
- inspectable agent decisions
- reproducible workflows
- scalable multi-agent systems

---

## Relationship to Existing Methodology

This proposal does NOT replace:

- contracts
- BDD
- Cold Start Quad

It extends them into:

> **runtime behavior**

---

## Final Note

This system is intentionally:

- simple at the edges (`ask`)
- flexible under the hood
- inspectable at every layer

Start small. Evolve as needed. Do not overbuild upfront.
