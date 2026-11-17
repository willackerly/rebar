#!/usr/bin/env bash
# Regression test: refresh-context.sh survives a repo with many commits since
# QUICKCONTEXT was last synced.
#
# It listed them with `git log … | head -20` under set -euo pipefail. With more
# output than head reads, git log got SIGPIPE, pipefail failed the pipeline and
# set -e ended the script with exit 141 (virtru-analyze, 420 commits,
# 2026-11-15). The repo here gets 5000 commits from git fast-import.
# Bash 3.2 compatible.
set -uo pipefail
. "$(dirname "$0")/lib.sh"

rebar_tmpdir refresh-context
P="$TMP/project"
mkdir -p "$P"
( cd "$P" && g init -q -b main ) || fail "git init"

# 5000 empty-tree commits after the sync date, in one fast-import stream.
{
  i=1
  while [ $i -le 5000 ]; do
    printf 'commit refs/heads/main\ncommitter t <t@example.com> %d +0000\ndata %d\ncommit %05d\n' \
      $((1700000000 + i)) 12 $i
    printf '\n'
    i=$((i + 1))
  done
} | ( cd "$P" && g fast-import --quiet ) || fail "fast-import"
( cd "$P" && g reset -q --hard main ) || fail "reset"
printf '# QUICKCONTEXT\n\nlast-synced: 2023-01-01\n' > "$P/QUICKCONTEXT.md"

out=$(run_in "$P" refresh-context.sh)
code=$?
[ $code -eq 0 ] || { echo "$out" | tail -5; fail "refresh-context.sh exited $code with 5000 commits since the last sync"; }
contains "$out" 'commit 05000' || fail "newest commit not listed: $(echo "$out" | tail -5)"
contains "$out" '\.\.\. and 4980 more' || fail "commit count summary missing: $(echo "$out" | tail -5)"
ok "refresh-context lists 20 of 5000 commits and exits 0"
