#!/usr/bin/env bash
# test-e2e-live.sh — End-to-end smoke tests with live LLM and live MCP.
#
# Validates that THIS machine has the latest rebar built and that the
# system actually works against a real LLM. Designed to be re-run after
# every meaningful update so adopters have a single "is this still
# working?" command.
#
# Gated cleanly:
#   - claude CLI on PATH (master gate — ASK queries route through claude)
#     → if missing, the entire suite skips with status 0 (CI-friendly)
#   - LMStudio at $LMSTUDIO_URL — opportunistic check; recorded but not
#     a hard gate (LMStudio is the local-LLM backend for `rebar adopt
#     --local`, separate codepath from ASK)
#   - Per-repo tests skip if the repo isn't present on disk
#   - ASK HTTP server tests skip if no server listening on $ASK_SERVER
#
# Modes:
#   ./scripts/test-e2e-live.sh                # run full suite
#   ./scripts/test-e2e-live.sh --no-llm       # skip live LLM queries (fast)
#   ./scripts/test-e2e-live.sh --quiet        # only print failures + summary
#   ./scripts/test-e2e-live.sh --json         # JSON summary on stdout
#
# Exit code: 0 if no failures (skips OK), 1 if any test failed.
#
# Bash 3.2 compatible (macOS default).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

LMSTUDIO_URL="${LMSTUDIO_URL:-http://localhost:1234/v1}"
ASK_SERVER_URL="${ASK_SERVER:-localhost:7232}"
DEV_DIR="${REBAR_DEV_DIR:-$HOME/dev}"

NO_LLM=0
QUIET=0
JSON_OUT=0
LLM_TIMEOUT=60  # seconds per LLM query

while [ $# -gt 0 ]; do
  case "$1" in
    --no-llm) NO_LLM=1; shift ;;
    --quiet) QUIET=1; shift ;;
    --json) JSON_OUT=1; QUIET=1; shift ;;
    --timeout) LLM_TIMEOUT="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "test-e2e-live: unknown arg '$1'" >&2; exit 2 ;;
  esac
done

# ─── Output helpers ────────────────────────────────────────────────────────
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
GRAY='\033[0;90m'
NC='\033[0m'

passed=0; failed=0; skipped=0
results_tmp="$(mktemp)"
trap 'rm -f "$results_tmp"' EXIT
: > "$results_tmp"

# Record a result. status: PASS|FAIL|SKIP. detail: short reason.
record() {
  local status="$1" name="$2" detail="${3:-}"
  echo "${status}|${name}|${detail}" >> "$results_tmp"
  case "$status" in
    PASS) passed=$((passed + 1)); [ "$QUIET" -eq 0 ] && printf "  ${GREEN}✓${NC} %s\n" "$name" ;;
    FAIL) failed=$((failed + 1)); printf "  ${RED}✗${NC} %s — %s\n" "$name" "$detail" ;;
    SKIP) skipped=$((skipped + 1)); [ "$QUIET" -eq 0 ] && printf "  ${GRAY}-${NC} %s ${GRAY}(skip: %s)${NC}\n" "$name" "$detail" ;;
  esac
}

section() {
  [ "$QUIET" -eq 0 ] && printf "\n${YELLOW}━━━ %s ━━━${NC}\n" "$1"
}

# ─── Pre-flight: claude CLI (master gate) ──────────────────────────────────
section "Pre-flight"

if ! command -v claude >/dev/null 2>&1; then
  if [ "$JSON_OUT" -eq 1 ]; then
    printf '{"status":"skipped","reason":"claude CLI not on PATH","passed":0,"failed":0,"skipped":0}\n'
  else
    printf "${YELLOW}claude CLI not on PATH.${NC}\n"
    printf "${GRAY}Entire suite skipped — ASK queries route through claude.${NC}\n"
    printf "${GRAY}Install: https://docs.anthropic.com/en/docs/claude-code${NC}\n"
  fi
  exit 0
fi
record PASS "claude CLI on PATH" "$(command -v claude)"

# LMStudio is opportunistic — recorded if reachable, not a hard gate.
http_code="$(curl -s -o /dev/null -m 3 -w "%{http_code}" "${LMSTUDIO_URL}/models" 2>/dev/null || echo "000")"
if [ "$http_code" = "200" ]; then
  record PASS "LMStudio reachable (opportunistic)" "$LMSTUDIO_URL"
else
  record SKIP "LMStudio reachable" "not up at $LMSTUDIO_URL — only matters for \`rebar adopt --local\`"
fi

# ─── Version triple-check ─────────────────────────────────────────────────
section "Version triple-check"

binary_version="$(rebar version 2>&1 | head -1 | awk '{print $NF}')"
file_version="$(cat "$PROJECT_ROOT/.rebar-version" 2>/dev/null || echo "unknown")"
tag_version="$(git -C "$PROJECT_ROOT" tag --sort=-v:refname 2>/dev/null | head -1)"

if [ "$binary_version" = "$file_version" ] && [ "$binary_version" = "$tag_version" ]; then
  record PASS "Version coherent across binary, .rebar-version, latest tag" "$binary_version"
else
  record FAIL "Version mismatch" "binary=$binary_version file=$file_version tag=$tag_version"
fi

# ─── Build freshness: is bin/rebar built from current source? ──────────────
binary_mtime="$(stat -f %m "$PROJECT_ROOT/bin/rebar" 2>/dev/null || stat -c %Y "$PROJECT_ROOT/bin/rebar" 2>/dev/null)"
source_mtime="$(find "$PROJECT_ROOT/cli" -name '*.go' -newer "$PROJECT_ROOT/bin/rebar" 2>/dev/null | head -1)"
if [ -n "$source_mtime" ]; then
  record FAIL "bin/rebar fresher than cli/ source" "rebuild needed: cd cli && go build -o ../bin/rebar ."
else
  record PASS "bin/rebar built from current source" ""
fi

# ─── Repo presence + agent discovery (per repo, gated) ─────────────────────
section "Repo presence + agent discovery"

KNOWN_REPOS="rebar TALOS blindpipe filedag fontkit office180 pdf-signer-web OpenTDF/TDFLite"
present_repos=0
total_known=0

for repo in $KNOWN_REPOS; do
  total_known=$((total_known + 1))
  repo_path="$DEV_DIR/$repo"
  if [ ! -d "$repo_path/agents" ]; then
    record SKIP "Repo $repo present" "no agents/ at $repo_path"
    continue
  fi
  present_repos=$((present_repos + 1))
  agent_count=$(ls -1d "$repo_path"/agents/*/AGENT.md 2>/dev/null | wc -l | tr -d ' ')
  if [ "$agent_count" -gt 0 ]; then
    record PASS "Repo $repo discoverable" "$agent_count agents"
  else
    record FAIL "Repo $repo has agents/ but no AGENT.md files" "$repo_path/agents/*/AGENT.md missing"
  fi
done

# ─── MCP server smoke (in-process spawn) ───────────────────────────────────
section "MCP server smoke"

mcp_log="$(mktemp)"
mcp_out="$(mktemp)"
trap 'rm -f "$results_tmp" "$mcp_log" "$mcp_out"' EXIT

# Send initialize + tools/list, capture both stderr (registration log) and stdout (responses).
{
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
  echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
} | "$PROJECT_ROOT/bin/ask-mcp-server" --stdio --repos-dir "$DEV_DIR" >"$mcp_out" 2>"$mcp_log" &
MCP_PID=$!

# Wait briefly for the responses, then kill.
sleep 2
kill "$MCP_PID" 2>/dev/null
wait "$MCP_PID" 2>/dev/null

if grep -q 'serverInfo' "$mcp_out" 2>/dev/null; then
  record PASS "MCP initialize round-trip" ""
else
  record FAIL "MCP initialize round-trip" "no serverInfo in response"
fi

reg_count=$(grep -c '^  registered:' "$mcp_log" 2>/dev/null || echo 0)
if [ "$reg_count" -gt 0 ]; then
  record PASS "MCP discovery registered repos" "$reg_count repos"
else
  record FAIL "MCP discovery registered repos" "no 'registered:' lines in stderr"
fi

# tools/list response should include at least one ask_<repo>_<role> tool.
# JSON output may have whitespace between `:` and `"` so the pattern is liberal.
if grep -qE '"name":[[:space:]]*"ask_' "$mcp_out" 2>/dev/null; then
  tool_count=$(grep -oE '"name":[[:space:]]*"ask_[^"]+"' "$mcp_out" 2>/dev/null | wc -l | tr -d ' ')
  record PASS "MCP tools/list returns ask_* tools" "$tool_count tools"
else
  record FAIL "MCP tools/list returns ask_* tools" "no ask_* names in response"
fi

# ─── ASK HTTP server smoke (gated on server up) ────────────────────────────
section "ASK HTTP server"

if curl -s -o /dev/null -m 2 "http://${ASK_SERVER_URL}/v1/health" 2>/dev/null; then
  health=$(curl -s -m 5 "http://${ASK_SERVER_URL}/v1/health")
  if echo "$health" | grep -q '"status".*"ok"'; then
    repos_in_server=$(echo "$health" | grep -oE '"repos":[0-9]+' | sed 's/.*://')
    record PASS "ASK server /v1/health" "$repos_in_server repos"
  else
    record FAIL "ASK server /v1/health" "unexpected: $health"
  fi

  # /v1/agents: should list expected entries
  agents_json=$(curl -s -m 5 "http://${ASK_SERVER_URL}/v1/agents")
  if echo "$agents_json" | grep -q "rebar:architect"; then
    agent_count=$(echo "$agents_json" | grep -oE '"[^"]+:[a-z]+"' | wc -l | tr -d ' ')
    record PASS "ASK server /v1/agents lists rebar:architect" "$agent_count total agents"
  else
    record FAIL "ASK server /v1/agents lists rebar:architect" "rebar:architect missing from /v1/agents"
  fi
else
  record SKIP "ASK server smoke" "no listener at $ASK_SERVER_URL — start with: python3 bin/ask-server --port 7232 --repos-dir $DEV_DIR"
fi

# ─── Live LLM queries with keyword acceptance ──────────────────────────────
section "Live LLM queries (keyword acceptance)"

if [ "$NO_LLM" -eq 1 ]; then
  record SKIP "Live LLM queries" "--no-llm flag set"
else

  # Helper: ask a question, check that response contains AT LEAST one of the
  # accept keywords. Times out after $LLM_TIMEOUT seconds.
  #
  # We `ask reset` the target first so a stale session from a previous run
  # (claude session-ID expired, transient ask-server crash, etc.) doesn't
  # cause spurious failures. The test is for "does the live LLM round-trip
  # work?", not "does session persistence survive across runs?" — that's a
  # separate concern.
  ask_keyword_check() {
    local label="$1" target="$2" question="$3" accept_pattern="$4"
    local cwd="${5:-}"
    local response
    if [ -n "$cwd" ]; then
      ( cd "$cwd" 2>/dev/null && ASK_SERVER='' ask reset "$target" >/dev/null 2>&1 ) || true
      response="$(cd "$cwd" 2>/dev/null && ASK_SERVER='' ask "$target" "$question" 2>&1)" || true
    else
      ask reset "$target" >/dev/null 2>&1 || true
      response="$(ask "$target" "$question" 2>&1)" || true
    fi
    if [ -z "$response" ]; then
      record FAIL "$label" "empty response (LLM error or timeout)"
      return
    fi
    # Case-insensitive grep for any of the accept keywords (pipe-separated).
    if echo "$response" | grep -qiE "$accept_pattern"; then
      local snippet
      snippet="$(echo "$response" | tr '\n' ' ' | head -c 80)"
      record PASS "$label" "matched keyword (response: ${snippet}…)"
    else
      local snippet
      snippet="$(echo "$response" | tr '\n' ' ' | head -c 120)"
      record FAIL "$label" "no expected keyword. Response: ${snippet}…"
    fi
  }

  # 1) rebar:steward — what the steward does
  ask_keyword_check \
    "ASK rebar:steward describes scanning" \
    "rebar:steward" \
    "In one sentence, what does the steward script do?" \
    "scan|health|drift|contract|lifecycle|enforcement"

  # 2) rebar:architect — contract principles
  ask_keyword_check \
    "ASK rebar:architect knows contract principles" \
    "rebar:architect" \
    "What are the four contract principles in rebar? Just list them." \
    "implement|modify|update|search|plan mode"

  # 3) Local mode (cd-into-repo, no server) — works even when ASK_SERVER is unset
  ask_keyword_check \
    "ASK local-mode (cd rebar; ask architect)" \
    "architect" \
    "What is rebar in one short sentence?" \
    "swarm|contract|methodology|coordination|framework" \
    "$PROJECT_ROOT"

  # 4) Cross-repo ask (gated on filedag presence + ASK_SERVER reachable)
  if [ -d "$DEV_DIR/filedag/agents/architect" ] && curl -s -o /dev/null -m 2 "http://${ASK_SERVER_URL}/v1/health" 2>/dev/null; then
    ask_keyword_check \
      "ASK cross-repo filedag:architect" \
      "filedag:architect" \
      "What kind of system is filedag in one short phrase?" \
      "file|content|dag|federat|index|content-addressed"
  else
    record SKIP "ASK cross-repo filedag:architect" "filedag absent or ASK server down"
  fi
fi

# ─── Summary ───────────────────────────────────────────────────────────────
total=$((passed + failed + skipped))

if [ "$JSON_OUT" -eq 1 ]; then
  # Build JSON results array
  printf '{"total":%d,"passed":%d,"failed":%d,"skipped":%d,"results":[' \
    "$total" "$passed" "$failed" "$skipped"
  first=1
  while IFS='|' read -r status name detail; do
    [ "$first" = 1 ] || printf ','
    first=0
    # Minimal JSON escape: backslash and quote
    name_e="${name//\\/\\\\}"; name_e="${name_e//\"/\\\"}"
    detail_e="${detail//\\/\\\\}"; detail_e="${detail_e//\"/\\\"}"
    printf '{"status":"%s","name":"%s","detail":"%s"}' "$status" "$name_e" "$detail_e"
  done < "$results_tmp"
  printf ']}\n'
else
  printf "\n${YELLOW}━━━ Summary ━━━${NC}\n"
  # `%b` interprets the backslash escapes inside the color variables; `%s`
  # treats them as literal text and prints `\033[...]` to the terminal.
  printf "  %b%d passed%b, %b%d failed%b, %b%d skipped%b (of %d total)\n" \
    "$GREEN" "$passed" "$NC" \
    "$RED" "$failed" "$NC" \
    "$GRAY" "$skipped" "$NC" \
    "$total"
fi

[ "$failed" -eq 0 ]
