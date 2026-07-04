#!/usr/bin/env bash
# inbox-watch.sh — Live watch on peer inbox/ dirs; one line per new deposit.
# rebar-scripts: 2026.07.04
#
# Canonical implementation of practices/inbox-watch.md. Polls one or more
# inbox directories and emits
#
#   NEW INBOX DEPOSIT: <path>
#
# once for each file that appears after the watch is armed. Pre-existing
# files are never reported (the arming snapshot is the baseline). With more
# than one watched directory, the path is prefixed with the directory the
# deposit landed in.
#
# Runs until killed — arm it as a persistent background monitor at session
# start (coordination-seat cold start; see practices/session-lifecycle.md).
# Each emitted line is an event the hosting harness can surface into the
# agent's session.
#
# Usage:
#   ./scripts/inbox-watch.sh [options] [dir ...]
#
#   dir ...            inbox directories to watch (default: ./inbox)
#
# Options:
#   -i, --interval N   poll every N seconds (default: 30)
#   --preview          append the memo's first line to each deposit line
#   -h, --help         show this header
#
# A watched directory that does not exist yet is warned about once (stderr)
# and kept in the watch: it lists as empty until created, so files present
# at creation are reported as new deposits.
#
# Zero dependencies beyond POSIX tools. Bash 3.2 compatible (macOS default).

set -uo pipefail

INTERVAL=30
PREVIEW=0
DIRS=()

while [ $# -gt 0 ]; do
  case "$1" in
    -i|--interval)
      shift
      if [ $# -eq 0 ]; then
        echo "inbox-watch: --interval needs a value" >&2
        exit 2
      fi
      INTERVAL="$1"
      ;;
    --interval=*) INTERVAL="${1#--interval=}" ;;
    --preview) PREVIEW=1 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    --)
      shift
      while [ $# -gt 0 ]; do
        DIRS[${#DIRS[@]}]="${1%/}"
        shift
      done
      break
      ;;
    -*)
      echo "inbox-watch: unknown option '$1' (try --help)" >&2
      exit 2
      ;;
    *) DIRS[${#DIRS[@]}]="${1%/}" ;;
  esac
  shift
done

case "$INTERVAL" in
  ''|*[!0-9]*)
    echo "inbox-watch: interval must be a positive integer, got '$INTERVAL'" >&2
    exit 2
    ;;
esac
if [ "$INTERVAL" -lt 1 ]; then
  echo "inbox-watch: interval must be >= 1 second, got '$INTERVAL'" >&2
  exit 2
fi

if [ "${#DIRS[@]}" -eq 0 ]; then
  DIRS=("./inbox")
fi

# Prefix emitted paths with the watched dir only when watching several —
# with a single inbox the bare filename is the whole signal (matches the
# illustration loop in practices/inbox-watch.md).
MULTI=0
if [ "${#DIRS[@]}" -gt 1 ]; then
  MULTI=1
fi

STATE_DIR="$(mktemp -d)"
SLEEP_PID=""
cleanup() {
  if [ -n "$SLEEP_PID" ]; then
    kill "$SLEEP_PID" 2>/dev/null
  fi
  rm -rf "$STATE_DIR"
}
trap cleanup EXIT
trap 'exit 0' INT TERM

# List a directory's entries, one per line, sorted stably for comm(1).
# A missing dir lists as empty, so it needs no special casing later:
# once it appears, its contents diff against the empty snapshot as new.
list_dir() {
  ls -1 "$1" 2>/dev/null | LC_ALL=C sort
}

# Baseline snapshots: everything present at arm time is old news.
i=0
for dir in "${DIRS[@]}"; do
  if [ ! -d "$dir" ]; then
    echo "inbox-watch: WARN — $dir does not exist yet; watching for it to appear" >&2
  fi
  list_dir "$dir" > "$STATE_DIR/snap.$i"
  i=$((i + 1))
done

echo "inbox-watch: armed — watching ${DIRS[*]} every ${INTERVAL}s (pre-existing files not reported)" >&2

while true; do
  # Background sleep + wait keeps the loop responsive to signals: a kill
  # lands immediately instead of after the current sleep completes.
  sleep "$INTERVAL" &
  SLEEP_PID=$!
  wait "$SLEEP_PID"
  SLEEP_PID=""

  i=0
  for dir in "${DIRS[@]}"; do
    snap="$STATE_DIR/snap.$i"
    cur="$STATE_DIR/cur.$i"
    list_dir "$dir" > "$cur"
    # comm -13: lines only in the current listing = new deposits.
    comm -13 "$snap" "$cur" | while IFS= read -r name; do
      [ -n "$name" ] || continue
      if [ "$MULTI" -eq 1 ]; then
        path="$dir/$name"
      else
        path="$name"
      fi
      line="NEW INBOX DEPOSIT: $path"
      if [ "$PREVIEW" -eq 1 ] && [ -f "$dir/$name" ]; then
        first="$(head -n 1 "$dir/$name" 2>/dev/null)"
        if [ -n "$first" ]; then
          line="$line — $first"
        fi
      fi
      echo "$line"
    done
    mv "$cur" "$snap"
    i=$((i + 1))
  done
done
