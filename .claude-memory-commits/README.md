# `.claude-memory-commits/`

Durable working knowledge from agent sessions, checked into the repo instead of
living in assistant-memory.

**Established 2026-08-12.**

## Why this exists

Rebar's own preference (recorded here as `working-agreements/01`) is that
anything doctrinal belongs in the repo, because assistant-memory is invisible to
other contributors, to adopters who clone this repo, and to any agent running on
a different machine or tool. This directory is the landing zone for the class of
knowledge that is *durable* but has no natural home in `practices/`,
`conventions.md`, or `feedback/`:

- **Working agreements** — how the maintainer wants an agent to operate here.
  Not architecture, not process gates; the collaboration contract.
- **Environment gotchas** — hard-won debugging knowledge about the toolchain
  around rebar (MCP wiring, ASK routing) that cost real hours to learn and is
  not discoverable from the code.

## What is deliberately NOT here

Three memory files were reviewed and **excluded**, because copying them would
have checked in stale duplicates of things the repo already states better:

| Memory | Why excluded |
|---|---|
| `rebar-self-is-tier-3-exemplar` | Claimed "10/10 on its own audit." Measured 2026-08-12: **9.0/10** (QUICKCONTEXT freshness). `bin/rebar audit` is the live answer; a frozen copy of a score is drift waiting to happen. |
| `rebar-charter-and-featurerequest` | Duplicates `CHARTER.md` and `practices/federation.md`, which are authoritative and maintained. |
| `rebar-planning-surface` | `feedback/INVENTORY.md` documents its own role in its header. |
| `rebar_purpose`, `project_rebar_cli` | Superseded by `README.md`, `DESIGN.md`, and `cli/`. Both also carry counts that have drifted. |

Two drifted numbers found while reviewing, recorded so nobody re-derives them
from memory: assistant-memory said **8** rebar-adopted repos and **37** ASK
agents. Measured 2026-08-12: **16** repos carry `.rebar-version`, and this
session's MCP surface exposes **62** `ask_*` tools. Live counts belong in
`METRICS.md` and the registry, never in a note.

**The rule this directory follows:** persist the *non-derivable* — judgment,
preference, and hard-won diagnosis. Never persist a number, a score, or a
status you could recompute. A checked-in fact that can rot is worse than no
fact, because it reads as authoritative.

## How to use this

- **At session start**, after the cold-start quad, read
  `working-agreements/`. It is short by design.
- **Reach for `environment/`** when something in the MCP/ASK layer behaves
  inexplicably. Each file leads with the symptom, because that is what you will
  have when you arrive.

## How to update this

Same discipline as the rest of the repo: these are files, so they take PRs,
review, and commit messages. If a working agreement turns out to be wrong,
**correct it in place and say so in the commit** — unlike an inbox memo, this
is not an append-only record; it is current guidance, and stale guidance here
is worse than none.

Add a file when a preference or diagnosis has survived **more than one
session**. One-off corrections are conversation, not doctrine.
