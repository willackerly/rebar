#!/usr/bin/env bash
# Regression test: ASK must never run a script or reach an agent that the
# caller names by path, and the HTTP servers must be safe by default
# (v3.1.2 security fix; all of these worked against v3.1.1).
#
#   T1  `ask <agent> <word>` runs commands/<word>.sh only for a plain word:
#       a relative path (../../x) is a question, not a script to run
#   T2  `ask <agent> <command> [args]` still runs the agent's command
#   T3  `ask --question <agent> <word>` never runs a command script
#   T4  agent names with ../ are rejected (no agent outside agents/)
#   T5  servers (python3; skipped when missing): command-word and role
#       traversal over HTTP /v1/ask and MCP tools/call / resources/read,
#       1 MiB body cap (413, Content-Length not trusted), loopback default
#       bind, refusal to serve a public address without a key, JSON-only
#       POSTs (415), loopback-only Host header (403), API key checks.
#       See ask_security_test.py.
#
# Usage: bash scripts/tests/ask-security.test.sh
# ASK_SEC_BIN=DIR tests another bin/ (e.g. a v3.1.1 checkout, to see it fail).
# Bash 3.2 compatible (macOS default).

set -uo pipefail

. "$(dirname "$0")/lib.sh"

BIN="${ASK_SEC_BIN:-$(cd "$REBAR_TESTS_DIR/../.." && pwd)/bin}"
ASK="$BIN/ask"
[ -x "$ASK" ] || fail "no ask at $ASK"

rebar_tmpdir ask-security
FX="$TMP"
mkdir -p "$FX/repos/proj/agents/architect/commands" "$FX/hidden/agents/secret" "$FX/evil" "$FX/fakebin"
echo "# Architect" > "$FX/repos/proj/agents/architect/AGENT.md"
echo "# Secret" > "$FX/hidden/agents/secret/AGENT.md"
printf '#!/bin/sh\necho EVIL-RAN "$@" > "%s/evil/pwned"\n' "$FX" > "$FX/evil/x.sh"
printf '#!/bin/sh\necho HELLO-CMD "$@"\n' > "$FX/repos/proj/agents/architect/commands/hello.sh"
# Fake claude: a canned answer in claude's --output-format json shape.
printf '#!/bin/sh\necho "{\\"result\\":\\"FAKE-ANSWER\\",\\"session_id\\":\\"s1\\"}"\n' > "$FX/fakebin/claude"
chmod +x "$FX/evil/x.sh" "$FX/repos/proj/agents/architect/commands/hello.sh" "$FX/fakebin/claude"

# The bin/ shims run the rebar beside them. Test the binary under test
# (lib.sh's $REBAR, which run.sh builds), not the committed bin/rebar, which
# is built for one platform: copies of the shims next to it.
if [ -z "${ASK_SEC_BIN:-}" ]; then
  mkdir -p "$FX/bin"
  cp "$BIN/ask" "$BIN/ask-server" "$BIN/ask-mcp-server" "$FX/bin/"
  ln -s "$(cd "$(dirname "$REBAR")" && pwd)/$(basename "$REBAR")" "$FX/bin/rebar"
  BIN="$FX/bin"
  ASK="$BIN/ask"
fi

export PATH="$FX/fakebin:$PATH" ASK_REGISTRY="$FX/registry" NO_COLOR=1
unset ASK_SERVER ASK_API_KEY ASK_AGENTS_DIR ASK_SHARED_DIR

ask_in_proj() { ( cd "$FX/repos/proj" && "$ASK" "$@" ) 2>&1; }

# T1 — from agents/architect/commands/, five levels up is $FX.
out=$(ask_in_proj architect ../../../../../evil/x </dev/null)
[ ! -e "$FX/evil/pwned" ] || fail "T1: 'ask architect ../../../../../evil/x' ran evil/x.sh: $(cat "$FX/evil/pwned")"
for w in ./hello ../commands/hello -hello .hello 'hello extra'; do
  out=$(ask_in_proj architect "$w" </dev/null)
  contains "$out" HELLO-CMD && fail "T1: command word '$w' ran commands/hello.sh"
done
ok "T1: path-like command words are questions, never scripts"

# T2
out=$(ask_in_proj architect hello a 'b c' </dev/null)
contains "$out" -x 'HELLO-CMD a b c' || fail "T2: 'ask architect hello a \"b c\"' should run the command: $out"
out=$(ask_in_proj Architect hello </dev/null)
contains "$out" -x 'HELLO-CMD' || fail "T2: case-insensitive agent name should still run the command: $out"
ok "T2: ask <agent> <command> [args] still runs commands/<command>.sh"

# T3
out=$(ask_in_proj --question architect hello </dev/null)
contains "$out" HELLO-CMD && fail "T3: --question ran a command script: $out"
out=$(ask_in_proj --question architect </dev/null); rc=$?
[ $rc -eq 4 ] || fail "T3: --question without a question should exit 4, got $rc: $out"
out=$(ask_in_proj --question -- status architect </dev/null)
contains "$out" -i 'agent .status. not found' || fail "T3: --question should not dispatch subcommands: $out"
if command -v jq >/dev/null 2>&1 && command -v uuidgen >/dev/null 2>&1; then
  out=$(ask_in_proj --question architect ../../../../../evil/x </dev/null)
  contains "$out" FAKE-ANSWER || fail "T3: --question should ask the question: $out"
fi
[ ! -e "$FX/evil/pwned" ] || fail "T3: --question ran evil/x.sh"
ok "T3: --question never runs command scripts or subcommands"

# T4 — from agents/, three levels up is $FX.
for spec in ../../../hidden/agents/secret proj:../../../hidden/agents/secret; do
  out=$(ask_in_proj "$spec" hi </dev/null); rc=$?
  [ $rc -eq 2 ] || fail "T4: agent '$spec' should exit 2, got $rc: $out"
  [ ! -e "$FX/hidden/agents/secret/.session-id" ] || fail "T4: agent '$spec' reached hidden/agents/secret"
done
out=$(ask_in_proj status ../../../hidden/agents/secret </dev/null)
contains "$out" 'invalid agent name' || fail "T4: 'ask status <path>' should reject the name: $out"
ok "T4: agent names containing paths are rejected"

# T5
if ! command -v python3 >/dev/null 2>&1; then
  echo "SKIP T5: python3 not found (servers not tested)"
else
  out=$(ASK_SEC_FIXTURE="$FX" ASK_SEC_BIN="$BIN" python3 "$REBAR_TESTS_DIR/ask_security_test.py" 2>&1); rc=$?
  [ $rc -eq 0 ] || fail "T5: server tests failed:
$out"
  ok "T5: server tests: $(count_matches "$out" ' \.\.\. ok$') passed"
fi

echo "PASS: ASK rejects path-named commands and agents; servers are safe by default"
