# pending-notifications.md schema

> Format identifier: `rebar-pending-notifications-v1`

The federation outbox at `architecture/.state/pending-notifications.md`
is auto-populated by `scripts/check-version-bump.sh` (post-commit hook)
and consumed by `scripts/flush-notifications.sh` (manual flush).

## File structure

```markdown
# Pending Consumer Notifications

<!-- format: rebar-pending-notifications-v1 -->

[boilerplate prose explaining the purpose]

## <CONTRACT_ID>: <OLD_VERSION> → <NEW_VERSION>
- **detected:** <ISO-8601 UTC timestamp>
- **status:** pending | sent | dropped
- **commit:** <short SHA>
- **old_path:** <path to pre-bump contract file>
- **new_path:** <path to post-bump contract file>
- **severity:** auto | breaking | additive | patch | doc-only

## <CONTRACT_ID>: <OLD_VERSION> → <NEW_VERSION>
...
```

## Field reference

| Field | Required | Set by | Notes |
|-------|----------|--------|-------|
| `detected` | yes | `check-version-bump.sh` | When the rename was observed |
| `status` | yes | `check-version-bump.sh` (init `pending`); `flush-notifications.sh` (`sent`/`dropped`) | Lifecycle state |
| `commit` | yes | `check-version-bump.sh` | The commit that introduced the bump |
| `old_path` | yes | `check-version-bump.sh` | For audit trail; not used for dispatch |
| `new_path` | yes | `check-version-bump.sh` | Same |
| `severity` | yes | `check-version-bump.sh` (init `auto`); `flush-notifications.sh` may classify | `auto` defers to flush-time inference |

## Status lifecycle

```
[bump detected]                       [flush succeeds]
       ↓                                      ↓
   pending  ────  flush-notifications.sh  ──→  sent
       │                                      ↑
       └──  manually edited to "dropped"  ────┘
            (when owner decides not to notify)
```

- **pending** — awaiting flush; appears in `rebar status` as outstanding
- **sent** — flush dispatched FRs to all eligible consumers
- **dropped** — owner explicitly chose not to notify (manual edit)

## Coalescence

When multiple bumps queue before flushing (e.g., 1.0 → 2.0 then 2.0 → 2.1
in quick succession), `flush-notifications.sh` does NOT auto-coalesce in
the v1 implementation — each bump becomes a separate FR per consumer.
This is intentionally simple. If notification fatigue surfaces, a future
revision may add `--coalesce` mode that emits one FR per (contract,
consumer) using the consumer's pinned version vs current head.

## Manual edits

Owners may edit this file by hand to:
- Change `status: pending` → `status: dropped` to suppress dispatch
- Adjust `severity` from `auto` to `breaking | additive | patch | doc-only`
  to override the flush-time classifier
- Delete entries entirely (treated as dropped)

`check-version-bump.sh` is append-only and never rewrites existing
entries, so manual edits are safe.

## Why this format

- **Markdown headings** are scannable by humans and parseable by `awk`/`sed`
- **One file** keeps the queue self-contained; no per-bump files to
  garbage-collect
- **No JSON** because the substrate is plain-text + grep + jq (CHARTER
  §1.3); markdown with structured fields is the rebar idiom
- **Append-only writes** from `check-version-bump.sh` avoid race conditions
  with concurrent commits (no full-file rewrites)
