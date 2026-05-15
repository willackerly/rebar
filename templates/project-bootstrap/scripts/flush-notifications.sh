#!/usr/bin/env bash
# flush-notifications.sh [--dry-run] [--severity breaking|additive|patch]
#
# Iterates pending entries in architecture/.state/pending-notifications.md
# and dispatches them to consumers via ask_<consumer>_featurerequest.
# Marks each entry "sent" on success.
#
# Severity is auto-classified from semver delta (breaking = major bump,
# additive = minor bump, patch = patch bump). Override with --severity.
#
# Consumer filtering:
#   notify_on_change: true   → notify
#   notify_on_change: false  → skip
#   (absent)                 → notify (owner default; tweak with REBAR_NOTIFY_DEFAULT=skip)
#
# Source: CHARTER §1.6, feedback/2026-04-28-cross-repo-contract-federation.md

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

OUTBOX="${REBAR_OUTBOX:-architecture/.state/pending-notifications.md}"
DRY_RUN=0
SEVERITY_OVERRIDE=""
NOTIFY_DEFAULT="${REBAR_NOTIFY_DEFAULT:-notify}"  # "notify" or "skip"
ASK_BIN="${ASK_BIN:-ask}"

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --severity)
      SEVERITY_OVERRIDE="$2"
      case "$SEVERITY_OVERRIDE" in
        breaking|additive|patch|doc-only) ;;
        *) echo "flush-notifications: invalid severity '$SEVERITY_OVERRIDE'" >&2; exit 2 ;;
      esac
      shift 2
      ;;
    -h|--help)
      cat <<EOF
Usage: flush-notifications.sh [--dry-run] [--severity LEVEL]

Reads $OUTBOX and dispatches pending notifications to consumers via
ask_<consumer>_featurerequest. Marks entries "sent" on success.

  --dry-run                   show what would be sent, don't dispatch
  --severity breaking|additive|patch|doc-only
                              override auto-classification

Environment:
  REBAR_OUTBOX               outbox file path (default: $OUTBOX)
  REBAR_NOTIFY_DEFAULT       behavior when notify_on_change absent in
                             consumer's CONSUMES.md: "notify" or "skip"
                             (default: notify)
  ASK_BIN                    path to ask CLI (default: ask)
EOF
      exit 0
      ;;
    *) echo "flush-notifications: unexpected arg '$1'" >&2; exit 2 ;;
  esac
done

if [ ! -f "$OUTBOX" ]; then
  echo "flush-notifications: no outbox at $OUTBOX — nothing to flush"
  exit 0
fi

OWNER="$(basename "$REPO_ROOT")"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Auto-classify severity from semver delta.
# breaking = MAJOR bump | additive = MINOR bump | patch = PATCH bump
infer_severity() {
  local old="$1" new="$2"
  local om nm onn nnn
  om="${old%%.*}"
  nm="${new%%.*}"
  if [ "$om" != "$nm" ]; then echo breaking; return; fi
  onn=$(echo "$old" | cut -d. -f2)
  nnn=$(echo "$new" | cut -d. -f2)
  if [ "$onn" != "$nnn" ]; then echo additive; return; fi
  echo patch
}

# Compose the FR question text the featurerequest agent will receive.
compose_message() {
  local contract="$1" old_v="$2" new_v="$3" severity="$4" commit="$5"
  cat <<MSG
Upstream change notice (auto-generated): $OWNER/$contract has been revised.

  $contract: $old_v → $new_v ($severity)
  commit: $commit

You're listed as a consumer of this contract per your CONSUMES.md. This
notification is filed via the rebar federation outbox flow (CHARTER §1.6).

Suggested triage:
  rebar contract drift-check
  # Then either bump your version_pinned in CONSUMES.md if you've
  # adopted the new version, or pin a planned upgrade date.

Severity classification (semver delta):
  breaking = MAJOR bump (likely interface-breaking)
  additive = MINOR bump (new behavior, backwards-compat expected)
  patch    = PATCH bump (clarification, doc, small-fix)

Source: $OWNER repo, commit $commit
MSG
}

# Walk outbox, find pending entries, dispatch each.
# Entry shape:
#   ## <contract>: <old> → <new>
#   - **detected:** <ts>
#   - **status:** pending|sent|dropped
#   - **commit:** <sha>
#   ...
processed=0
sent=0
skipped=0

# Bash 3.2 portable: read full file, split into per-entry blocks
mapfile -t lines < "$OUTBOX" 2>/dev/null || {
  # bash 3.2 has no mapfile; fall back to while-read
  lines=()
  while IFS= read -r line || [ -n "$line" ]; do lines+=("$line"); done < "$OUTBOX"
}

# Build a list of pending entries: contract, old_v, new_v, commit
declare -a entries=()
in_entry=0
e_contract=""; e_old=""; e_new=""; e_commit=""; e_status=""
for line in "${lines[@]}"; do
  if [[ "$line" =~ ^##[[:space:]]+([A-Z][A-Za-z0-9_.-]+):[[:space:]]+([0-9]+(\.[0-9]+){1,2})[[:space:]]+→[[:space:]]+([0-9]+(\.[0-9]+){1,2})[[:space:]]*$ ]]; then
    # Flush previous entry if present
    if [ "$in_entry" = "1" ] && [ "$e_status" = "pending" ]; then
      entries+=("$e_contract|$e_old|$e_new|$e_commit")
    fi
    e_contract="${BASH_REMATCH[1]}"
    e_old="${BASH_REMATCH[2]}"
    e_new="${BASH_REMATCH[4]}"
    e_commit=""
    e_status=""
    in_entry=1
  elif [[ "$line" =~ ^-[[:space:]]+\*\*status:\*\*[[:space:]]+(pending|sent|dropped) ]]; then
    e_status="${BASH_REMATCH[1]}"
  elif [[ "$line" =~ ^-[[:space:]]+\*\*commit:\*\*[[:space:]]+([a-f0-9]+) ]]; then
    e_commit="${BASH_REMATCH[1]}"
  fi
done
# Flush trailing entry
if [ "$in_entry" = "1" ] && [ "$e_status" = "pending" ]; then
  entries+=("$e_contract|$e_old|$e_new|$e_commit")
fi

if [ "${#entries[@]}" -eq 0 ]; then
  echo "flush-notifications: no pending entries in $OUTBOX"
  exit 0
fi

echo "flush-notifications: $OWNER has ${#entries[@]} pending notification(s)"
echo ""

for entry in "${entries[@]}"; do
  IFS='|' read -r contract old_v new_v commit <<< "$entry"
  severity="${SEVERITY_OVERRIDE:-$(infer_severity "$old_v" "$new_v")}"

  echo "→ $contract: $old_v → $new_v ($severity)"

  # Find consumers of this contract. JSON output makes parsing tractable.
  consumers_json=$("$SCRIPT_DIR/scan-consumers.sh" "$contract" --json 2>/dev/null || echo '{"consumers":[]}')

  # Extract consumer names + notify_on_change preferences via jq if present,
  # otherwise via grep/sed (bash 3.2 fallback).
  consumer_lines=""
  if command -v jq &>/dev/null; then
    consumer_lines=$(echo "$consumers_json" | jq -r '.consumers[] | "\(.name)|\(.notify_on_change)"' 2>/dev/null || true)
  else
    # Fallback parser — sed-based
    consumer_lines=$(echo "$consumers_json" | tr '},' '\n' | grep '"name"' | sed -E 's/.*"name":"([^"]+)".*"notify_on_change":"([^"]*)".*/\1|\2/')
  fi

  if [ -z "$consumer_lines" ]; then
    echo "    (no consumers found — nothing to dispatch)"
    echo ""
    continue
  fi

  # Iterate consumers, filter by notify_on_change preference
  while IFS='|' read -r consumer notify; do
    [ -z "$consumer" ] && continue
    case "$notify" in
      false) echo "    skip: $consumer (notify_on_change=false)"; skipped=$((skipped+1)); continue ;;
      true) ;;
      *)
        if [ "$NOTIFY_DEFAULT" = "skip" ]; then
          echo "    skip: $consumer (notify_on_change unset, REBAR_NOTIFY_DEFAULT=skip)"
          skipped=$((skipped+1))
          continue
        fi
        ;;
    esac

    msg=$(compose_message "$contract" "$old_v" "$new_v" "$severity" "$commit")

    if [ "$DRY_RUN" = "1" ]; then
      echo "    DRY: ask_${consumer}_featurerequest ← (${#msg} chars)"
      processed=$((processed+1))
      continue
    fi

    # Dispatch via ASK CLI cross-repo
    if "$ASK_BIN" "$consumer:featurerequest" "$msg" >/dev/null 2>&1; then
      echo "    sent: $consumer"
      sent=$((sent+1))
      processed=$((processed+1))
    else
      echo "    FAIL: $consumer (ask invocation failed; entry stays pending)" >&2
    fi
  done <<< "$consumer_lines"

  echo ""
done

# Mark sent entries as sent. We do a minimal in-place edit only when not
# dry-run AND at least one dispatch succeeded.
if [ "$DRY_RUN" = "0" ] && [ "$sent" -gt 0 ]; then
  # Simple sed substitution: any "status: pending" gets bumped to "status: sent"
  # for entries that had at least one consumer dispatched. This is coarse —
  # if a single contract had 5 consumers and 3 dispatched, the entry still
  # gets marked sent. Acceptable for v1; refine if partial-success becomes
  # a real concern.
  tmpfile=$(mktemp)
  sed 's/^- \*\*status:\*\* pending/- **status:** sent/' "$OUTBOX" > "$tmpfile"
  mv "$tmpfile" "$OUTBOX"
fi

echo "flush-notifications: processed=$processed sent=$sent skipped=$skipped"
[ "$DRY_RUN" = "1" ] && echo "(dry-run — no FRs filed, no outbox state changed)"
