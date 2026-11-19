package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The ASK parity cases (harness: native_ask_test.go).

// askAgents is a project whose agents cover every description rule of
// `ask who`: a first paragraph, "You are…" sentences stripped down to the
// Responsibilities fallback, the Role fallback, truncation at a word, and
// directories that are not agents.
var askAgents = map[string]string{
	"README.md": "# Demo Project\n\nA demo.\n",
	"agents/architect/AGENT.md": "# Agent: Architect\n\n## Role\nYou are the architect agent for Demo. You own system\n" +
		"design, contracts, and technical architecture.\n\n## Responsibilities\n- Answer questions about system architecture\n- Evaluate trade-offs\n",
	"agents/product/AGENT.md":    "# Product\n\nOwns personas and priorities for the demo.\nSecond line of the paragraph.   \n\nMore text.\n",
	"agents/englead/AGENT.md":    "# Englead\n\nYou are the englead. You own delivery.\n\n## Role\n\n   Coordinates the build and the release train.\n## Next\n",
	"agents/steward/AGENT.md":    "# Steward\n\nYou receive reports. Reports project health, contract lifecycle, drift and every gate that matters to anyone shipping here.\n",
	"agents/tester/AGENT.md":     "# Tester\n\nYou are the tester.\n",
	"agents/notes/README.md":     "not an agent\n",
	"agents/.hidden/AGENT.md":    "Hidden agent.\n",
	"agents/Mixed/AGENT.md":      "Mixed-case agent.\n",
	"agents/architect/memory.md": "Remembered: the calc module is pure.\n",
}

// withAsk is askAgents plus files.
func withAsk(extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range askAgents {
		out[k] = v
	}
	for k, v := range extra {
		if v == "" {
			delete(out, k)
			continue
		}
		out[k] = v
	}
	return out
}

var stewardCommands = withAsk(map[string]string{
	"agents/steward/commands/summary.sh": "#!/bin/sh\n# Print a health summary\necho \"summary: $# args: $*\"\necho \"cwd: $(pwd)\"\n",
	"agents/steward/commands/fail.sh":    "#!/bin/sh\necho failing >&2\nexit 3\n",
	"agents/steward/commands/plain.sh":   "echo plain ran\n",
	"agents/steward/commands/notexec.sh": "#!/bin/sh\necho never\n",
})

// commandModes makes the steward commands executable but notexec.sh.
func commandModes(t *testing.T, base string) {
	for _, f := range []string{"summary.sh", "fail.sh", "plain.sh"} {
		if err := os.Chmod(filepath.Join(base, "proj/agents/steward/commands", f), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// livePID writes this test process's pid as the agent's .pid: an agent
// that is running.
func livePID(agent string) func(t *testing.T, base string) {
	return func(t *testing.T, base string) {
		testutil.WriteFile(t, filepath.Join(base, "proj"), "agents/"+agent+"/.pid", strconv.Itoa(os.Getpid())+"\n")
	}
}

// responder is a running agent: it answers each inbox question with body.
func responder(body string) func(t *testing.T, base string) {
	return func(t *testing.T, base string) {
		livePID("architect")(t, base)
		dir := filepath.Join(base, "proj/agents/architect")
		done := make(chan struct{})
		t.Cleanup(func() { close(done) })
		go func() {
			seen := map[string]bool{}
			for {
				select {
				case <-done:
					return
				case <-time.After(20 * time.Millisecond):
				}
				entries, _ := os.ReadDir(filepath.Join(dir, "inbox"))
				for _, e := range entries {
					if seen[e.Name()] || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}
					seen[e.Name()] = true
					_ = os.WriteFile(filepath.Join(dir, "outbox", e.Name()+".tmp"), []byte(body), 0o644)
					_ = os.Rename(filepath.Join(dir, "outbox", e.Name()+".tmp"), filepath.Join(dir, "outbox", e.Name()))
				}
			}
		}()
	}
}

// otherProject is a second registered project, beside the working one.
var otherProject = withAsk(map[string]string{
	"../other/agents/architect/AGENT.md": "# Other architect\n\nKnows the other project.\n",
	"../other/README.md":                 "# Other\n",
})

var askRegistry = map[string]string{".config/ask/projects": "Other = $BASE/other\ngone = $BASE/nowhere\n# comment = x\n"}

const streamOut = `{"type":"system","subtype":"init","session_id":"0b6f4a52-1111-4222-8333-944455556666"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Let me look at the code first, then edit it.\nSecond line is not shown."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"src/calc.go"}}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./... -run 'TestCalculatorHandlesEveryOperatorWithoutPanicking' -count=1 -v"}}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Glob","input":{}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}
{"type":"assistant","message":{"content":[]}}
{"type":"result","subtype":"success","result":"Edited src/calc.go.","session_id":"0b6f4a52-1111-4222-8333-944455556666","modelUsage":{"m":{"inputTokens":10,"outputTokens":5,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"contextWindow":1000}}}
`

func compactLog(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("---\n**2026-01-0" + strconv.Itoa(i%9+1) + "T00:00:00Z** Q: question " + strconv.Itoa(i) + "\nA: answer " + strconv.Itoa(i) + "\n")
	}
	return b.String()
}

var askCases = []askCase{
	// help, version, flags
	{name: "no arguments", files: askAgents},
	{name: "help", files: askAgents, args: []string{"help"}},
	{name: "version", args: []string{"--version"}},
	{name: "version in a project", files: map[string]string{".rebar-version": " v3.1.0 \n"}, args: []string{"-V"}},
	{name: "help flag", args: []string{"--help"}, diverges: "the script's flag loop rejected --help as an unknown flag"},
	{name: "h flag", args: []string{"-h"}, diverges: "the script's flag loop rejected -h as an unknown flag"},
	{name: "unknown long flag", args: []string{"--json", "architect", "q"}},
	{name: "unknown short flag", args: []string{"-x"}},
	{name: "end of flags", files: askAgents, args: []string{"-v", "--", "who"}},

	// who
	{name: "who", files: askAgents, args: []string{"who"}},
	{name: "who narrow terminal", files: askAgents, args: []string{"who"}, env: []string{"COLUMNS=50"}},
	{name: "who without agents", args: []string{"who"}},
	{name: "who with ASK_AGENTS_DIR", files: withAsk(map[string]string{"elsewhere/x/AGENT.md": "Agent x.\n"}), args: []string{"who"}, env: []string{"ASK_AGENTS_DIR=elsewhere"}},

	// where
	{name: "where", files: askAgents, args: []string{"where", "product"}},
	{name: "where needs a name", files: askAgents, args: []string{"where"}},
	{name: "where unknown agent", files: askAgents, args: []string{"where", "prodcut"}},
	{name: "where other case", files: askAgents, args: []string{"where", "PRODUCT"}, diverges: "the script resolved the path of the name as typed (realpath fails on a case-sensitive file system)"},

	// status
	{name: "status stopped", files: askAgents, args: []string{"status", "architect"}},
	{name: "status stale pid", files: withAsk(map[string]string{"agents/architect/.pid": "999999\n"}), args: []string{"status", "architect"}},
	{name: "status running", files: withAsk(map[string]string{"agents/architect/inbox/a.json": "{}", "agents/architect/inbox/sub/b.json": "{}", "agents/architect/inbox/c.txt": "x"}), setup: livePID("architect"), args: []string{"status", "architect"}},
	{name: "status running with outbox", files: withAsk(map[string]string{"agents/architect/outbox/a.json": "{}"}), setup: livePID("architect"), args: []string{"status", "architect"}, diverges: "the script computed LAST_ACTIVE with BSD stat -f %m only; with GNU stat it crashed"},

	// log
	{name: "log empty", files: askAgents, args: []string{"log", "architect"}},
	{name: "log", files: withAsk(map[string]string{"agents/architect/memory.log.md": compactLog(2)}), args: []string{"log", "Architect"}},

	// peek
	{name: "peek no inbox", files: askAgents, args: []string{"peek", "architect"}},
	{name: "peek empty inbox", files: withAsk(map[string]string{"agents/architect/inbox/.keep": ""}), setup: func(t *testing.T, base string) {
		os.MkdirAll(filepath.Join(base, "proj/agents/architect/inbox"), 0o755)
	}, args: []string{"peek", "architect"}},
	{name: "peek messages", files: withAsk(map[string]string{
		"agents/architect/inbox/1.json": `{"id":"m1","from":"englead","timestamp":"2026-01-01T00:00:00Z","body":"Which DB?"}`,
		"agents/architect/inbox/2.json": `{"body":{"nested":["a",1]},"id":7}`,
		"agents/architect/inbox/3.json": "not json",
		"agents/architect/inbox/4.json": `{"id":null,"body":false,"timestamp":"t"}`,
		"agents/architect/inbox/5.json": `"just a string"`,
	}), args: []string{"peek", "architect"}},

	// tail and watch
	{name: "tail without event log", files: askAgents, args: []string{"tail"}},
	{name: "watch without progress", files: askAgents, args: []string{"watch", "architect"}},
	{name: "watch finished", files: withAsk(map[string]string{"agents/architect/.progress": "# ask architect — 2026-01-01T00:00:00Z\n# Q: q\n---\n[1] Read x\n---\n# status: done — 2026-01-01T00:00:01Z\n"}), args: []string{"watch", "architect"}},

	// reset
	{name: "reset", files: withAsk(map[string]string{"agents/architect/.session-id": "abc-123\n"}), args: []string{"reset", "architect"}},
	{name: "reset without session", files: askAgents, args: []string{"reset", "architect"}},
	{name: "reset unknown agent", files: askAgents, args: []string{"reset", "nobody"}},

	// compact
	{name: "compact without log", files: askAgents, args: []string{"compact", "architect"}},
	{name: "compact below threshold", files: withAsk(map[string]string{"agents/architect/memory.log.md": compactLog(3)}), args: []string{"compact", "architect"}},
	{name: "compact with claude", files: withAsk(map[string]string{"agents/architect/memory.log.md": compactLog(5)}), args: []string{"compact", "architect"},
		env: []string{"ASK_COMPACT_ENTRIES=4", "ASK_COMPACT_KEEP=2"}, claude: map[string]string{"claude-out": "- The calc module is pure.\n- Answers were 1 to 3.\n\n"}},
	{name: "compact without claude output", files: withAsk(map[string]string{"agents/architect/memory.log.md": "<!-- header -->\n" + compactLog(4)}), args: []string{"compact", "architect"},
		env: []string{"ASK_COMPACT_ENTRIES=2", "ASK_COMPACT_KEEP=1"}, claude: map[string]string{"claude-out": "", "claude-exit": "1"}},
	{name: "compact by size keeps everything", files: withAsk(map[string]string{"agents/architect/memory.log.md": compactLog(2)}), args: []string{"compact", "architect"},
		env: []string{"ASK_COMPACT_SIZE=10", "ASK_COMPACT_KEEP=5"}},

	// register and projects
	{name: "register default name", files: map[string]string{"README.md": "# x\n"}, args: []string{"register"}},
	{name: "register name", files: map[string]string{"README.md": "# x\n"}, args: []string{"register", "demo"}},
	{name: "register again", files: map[string]string{"README.md": "# x\n"}, home: map[string]string{".config/ask/projects": "demo = $BASE/proj\n"}, args: []string{"register", "demo"}},
	{name: "register moved project", files: map[string]string{"README.md": "# x\n"}, home: map[string]string{".config/ask/projects": "a = /a\ndemo = /old/place\nb = /b\n"}, args: []string{"register", "demo"}},
	{name: "register moved only project", files: map[string]string{"README.md": "# x\n"}, home: map[string]string{".config/ask/projects": "demo = /old/place\n"}, args: []string{"register", "demo"}, diverges: "the script's grep -v exited it silently when no other entry was left"},
	{name: "projects none", args: []string{"projects"}},
	{name: "projects", files: otherProject, home: askRegistry, args: []string{"projects"}},

	// init
	{name: "init", files: map[string]string{"README.md": "# Demo Project\r\n", "docs/x.md": "x", "go.mod": "module x\n", "architecture/CONTRACT-A.1.0.md": "a", "architecture/CONTRACT-B.1.0.md": "b"}, args: []string{"init"}},
	{name: "init again", files: withAsk(map[string]string{".gitignore": "node_modules/\nagents/*/inbox/\n"}), args: []string{"init"}},
	{name: "init package name", files: map[string]string{"package.json": `{"name":"pkg-name"}`, "contracts/CONTRACT-X.1.0.md": "x", ".rebarrc": "contracts_dir = contracts/   # mine\n", "tests/a": "", "test/b": ""}, args: []string{"init"}},
	{name: "init REBAR_CONTRACTS_DIR", files: map[string]string{"README.md": "no heading\n", "specs/CONTRACT-X.1.0.md": "x", "src/a": "", "packages/b": "", "internal/c": "", "METRICS": "", "docs/specifications/s": "", "Cargo.toml": "", "pyproject.toml": "", "DESIGN.md": "", "methodology.md": "", "product/p": ""}, args: []string{"init"}, env: []string{"REBAR_CONTRACTS_DIR=specs"}},

	// asking directly
	{name: "ask first question", files: askAgents, args: []string{"architect", "What is the answer?"}},
	{name: "ask verbose", files: askAgents, args: []string{"-v", "product", "Who is it for?"}},
	{name: "ask resumes session", files: withAsk(map[string]string{"agents/architect/.session-id": "11111111-2222-4333-8444-555555555555\n"}), args: []string{"architect", "And now?"}},
	{name: "ask resumes verbose", files: withAsk(map[string]string{"agents/architect/.session-id": "11111111-2222-4333-8444-555555555555\n"}), args: []string{"--verbose", "architect", "And now?"}},
	{name: "ask other case", files: askAgents, args: []string{"ARCHITECT", "q"}, divergesOn: "darwin"}, // the script kept "ARCHITECT" where the file system ignores case
	{name: "ask debug", files: askAgents, args: []string{"-d", "architect", "q"}},
	{name: "ask debug without usage", files: askAgents, args: []string{"-d", "architect", "q"}, claude: map[string]string{"claude-out": `{"result":"r","session_id":"s1"}`}},
	{name: "ask plain text answer", files: askAgents, args: []string{"architect", "q"}, claude: map[string]string{"claude-out": "Not JSON at all.\nSecond line.\n"}},
	{name: "ask content answer", files: askAgents, args: []string{"architect", "q"}, claude: map[string]string{"claude-out": `{"content":"from content","result":null}`}},
	{name: "ask object answer", files: askAgents, args: []string{"architect", "q"}, claude: map[string]string{"claude-out": `{"result":{"a":[1,2]},"session_id":""}`}},
	{name: "ask context limit", files: askAgents, args: []string{"-v", "architect", "q"}, env: []string{"ASK_CONTEXT_LIMIT=2"}},
	{name: "ask claude fails", files: askAgents, args: []string{"architect", "q"}, claude: map[string]string{"claude-out": "partial", "claude-err": "API error 529\n", "claude-exit": "1"}},
	{name: "ask stale session", files: withAsk(map[string]string{"agents/architect/.session-id": "11111111-2222-4333-8444-555555555555\n"}), args: []string{"architect", "q"},
		claude: map[string]string{"claude-out.1": "", "claude-err.1": "No conversation found with session ID: 11111111-2222-4333-8444-555555555555\n", "claude-exit.1": "1"}},
	{name: "ask stale session twice", files: withAsk(map[string]string{"agents/architect/.session-id": "11111111-2222-4333-8444-555555555555\n"}), args: []string{"architect", "q"},
		claude: map[string]string{"claude-out": "", "claude-err": "No conversation found with session ID: x\n", "claude-exit": "1"}},
	{name: "ask without claude", files: askAgents, args: []string{"architect", "q"}, noClaude: true},
	{name: "ask write mode", files: askAgents, args: []string{"-w", "architect", "Fix the bug"}, claude: map[string]string{"claude-out": streamOut}},
	{name: "ask write mode fails", files: askAgents, args: []string{"-w", "architect", "Fix the bug"}, claude: map[string]string{"claude-out": streamOut, "claude-exit": "2"}},
	{name: "ask featurerequest writes", files: withAsk(map[string]string{"agents/featurerequest/AGENT.md": "# FR\n\nIntake.\n"}), args: []string{"-v", "featurerequest", "Please add X"}, claude: map[string]string{"claude-out": streamOut}},
	{name: "ask ASK_WRITE", files: askAgents, args: []string{"architect", "q"}, env: []string{"ASK_WRITE=1", "ASK_MODEL=opus"}, claude: map[string]string{"claude-out": streamOut}},
	{name: "ask unknown agent", files: askAgents, args: []string{"archtect", "q"}},
	{name: "ask unknown agent far", files: askAgents, args: []string{"zzzzzzzz", "q"}},
	{name: "ask flag after agent", files: askAgents, args: []string{"architect", "-v"}},
	{name: "ask question starting with a dash", files: askAgents, args: []string{"architect", "-verbose flag?"}},
	{name: "ask piped question", files: askAgents, args: []string{"architect"}, stdin: "Piped question?\n\n"},
	{name: "ask nothing", files: stewardCommands, setup: commandModes, args: []string{"steward"}},
	{name: "ask nothing no commands", files: askAgents, args: []string{"architect"}},
	{name: "memory log grows", files: withAsk(map[string]string{"agents/architect/memory.log.md": compactLog(1)}), args: []string{"architect", "q"}},

	// agent commands
	{name: "command", files: stewardCommands, setup: commandModes, args: []string{"steward", "summary", "a b", "c"}},
	{name: "command exit status", files: stewardCommands, setup: commandModes, args: []string{"steward", "fail"}},
	{name: "command without shebang", files: stewardCommands, setup: commandModes, args: []string{"steward", "plain"}},
	{name: "command not executable is a question", files: stewardCommands, setup: commandModes, args: []string{"steward", "notexec"}},
	{name: "default command", files: withAsk(map[string]string{"agents/steward/commands/default.sh": "#!/bin/sh\necho default ran\nexit 5\n"}), setup: func(t *testing.T, base string) {
		os.Chmod(filepath.Join(base, "proj/agents/steward/commands/default.sh"), 0o755)
	}, args: []string{"steward"}},
	{name: "command word that is a path is a question", files: stewardCommands, setup: commandModes, args: []string{"steward", "../commands/summary"}},
	{name: "question mode never runs a command", files: stewardCommands, setup: commandModes, args: []string{"--question", "steward", "summary"}},
	{name: "question mode needs two arguments", files: stewardCommands, setup: commandModes, args: []string{"--question", "steward"}},
	{name: "agent name that is a path", files: askAgents, args: []string{"../agents/architect", "q"}},

	// cross-project
	{name: "cross project", files: otherProject, home: askRegistry, args: []string{"other:architect", "How?"}},
	{name: "cross project exact case", files: otherProject, home: askRegistry, args: []string{"Other:architect", "How?"}},
	{name: "cross project unknown", files: otherProject, home: askRegistry, args: []string{"nope:architect", "q"}},
	{name: "cross project gone", files: otherProject, home: askRegistry, args: []string{"gone:architect", "q"}},
	{name: "cross project unknown agent", files: otherProject, home: askRegistry, args: []string{"other:nobody", "q"}},
	{name: "cross project malformed", files: otherProject, home: askRegistry, args: []string{":architect", "q"}},
	{name: "cross project without askRegistry", files: map[string]string{"README.md": "# x\n"}, args: []string{"other:architect", "q"}},

	// running agents (inbox protocol)
	{name: "inbox answer", files: askAgents, setup: responder(`{"answer": "From the running agent.", "rationale": "r", "refs": []}` + "\n"), args: []string{"architect", "Through the inbox?"}},
	{name: "inbox raw answer", files: askAgents, setup: responder("plain answer\n\n"), args: []string{"architect", "q"}},
	{name: "inbox null answer", files: askAgents, setup: responder(`{"answer": null}`), args: []string{"architect", "q"}},
	{name: "inbox timeout", files: askAgents, setup: livePID("architect"), args: []string{"architect", "q"}, env: []string{"ASK_TIMEOUT=1"}},

	// up
	{name: "up with run.sh", files: withAsk(map[string]string{"agents/architect/run.sh": "#!/bin/sh\necho \"run.sh for $(basename \"$(pwd)\") args:$#\"\n"}), setup: func(t *testing.T, base string) {
		os.Chmod(filepath.Join(base, "proj/agents/architect/run.sh"), 0o755)
	}, args: []string{"up", "architect"}},
	{name: "up without claude", files: askAgents, args: []string{"up", "architect"}, noClaude: true},

	// remote mode
	{name: "remote ask", files: askAgents, servers: 1, args: []string{"web:architect", "Remote?"}, env: []string{"ASK_SERVER=$SERVER1", "ASK_API_KEY=k3y"}},
	{name: "remote ask verbose", servers: 2, args: []string{"-v", "web:architect", "Remote?"}, env: []string{"ASK_SERVER=$SERVER1,$SERVER2"}},
	{name: "remote ask round robin", servers: 2, home: map[string]string{".config/ask/.rr-index": "1\n"}, args: []string{"web:architect", "Remote?"}, env: []string{"ASK_SERVER=http://$SERVER1,$SERVER2,"}},
	{name: "remote ask not found moves on", servers: 2, args: []string{"web:elsewhere", "q"}, env: []string{"ASK_SERVER=$SERVER1,$SERVER2"}},
	{name: "remote ask agent error", servers: 1, args: []string{"web:broken", "q"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote ask all fail", servers: 1, args: []string{"-v", "missing:architect", "q"}, env: []string{"ASK_SERVER=$DEAD,$SERVER1"}},
	{name: "remote ask needs a question", servers: 1, args: []string{"web:architect"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote local first", files: otherProject, home: askRegistry, servers: 1, args: []string{"OTHER:architect", "Local?"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote status", servers: 1, args: []string{"status", "web:architect"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote log", servers: 1, args: []string{"log", "web:architect"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote reset", servers: 1, args: []string{"reset", "web:architect"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote reset unreachable", args: []string{"reset", "web:architect"}, env: []string{"ASK_SERVER=$DEAD"}},
	{name: "remote agents", servers: 1, args: []string{"agents"}, env: []string{"ASK_SERVER=$SERVER1"}},
	{name: "remote agents merged", servers: 2, args: []string{"agents"}, env: []string{"ASK_SERVER=$SERVER2,$DEAD,$SERVER1"}},
	{name: "remote agents without server", args: []string{"agents"}},

	// ask serve (ask-server, python)
	{name: "serve needs repos", args: []string{"serve", "--port", "0"}, diverges: "the usage line is rebar's, not argparse's (the error line and exit 2 match)"},
}

// TestAskGolden: `rebar ask` and the bin/ask shim match every case's
// golden (see runAskGolden).
func TestAskGolden(t *testing.T) {
	runAskGolden(t, askCases)
}

// loopQuestions are two questions waiting in the architect's inbox; the
// stub claude answers the first and fails the second.
var loopQuestions = withAsk(map[string]string{
	"agents/architect/inbox/q1.json": `{"id":"q1","from":"englead","type":"question","timestamp":"2026-01-01T00:00:00Z","body":"Which DB?"}`,
	"agents/architect/inbox/q2.json": `{"id":"q2","type":"question","body":"And the cache?"}`,
})

var loopClaude = map[string]string{
	"claude-out.1": `{"answer": "Postgres.", "rationale": "It is what we run.", "refs": ["docs/db.md"]}` + "\n",
	"claude-out.2": "", "claude-exit.2": "1",
}

// TestAskLoopGolden: the agent loop — `ask up` without a run.sh, and
// bin/ask-agent-loop — answers its inbox as the bash loop did (goldens):
// outbox files, event log, memory log, claude calls, output, and a clean
// exit (pid file removed) on SIGTERM.
func TestAskLoopGolden(t *testing.T) {
	for _, tool := range []struct {
		name, script string
		args         []string
	}{
		{"ask up", "ask", []string{"up", "architect"}},
		{"ask-agent-loop", "ask-agent-loop", []string{"agents/architect"}},
	} {
		t.Run(tool.name, func(t *testing.T) {
			var recs [1]string
			for i, how := range []string{"native"} {
				tc := askCase{files: loopQuestions, claude: loopClaude}
				base := askTree(t, tc)
				proj := filepath.Join(base, "proj")
				env := askEnv(t, base, filepath.Join(base, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
				cmd := exec.Command(testBinary(t), append([]string{tool.script}, tool.args...)...)
				env = append(env, "REBAR_TEST_RUN_CLI=1")
				before := snapshot(t, base)
				var stdout, stderr bytes.Buffer
				cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = proj, env, &stdout, &stderr
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(20 * time.Second)
				for {
					left, _ := filepath.Glob(filepath.Join(proj, "agents/architect/inbox/*.json"))
					answered, _ := filepath.Glob(filepath.Join(proj, "agents/architect/outbox/*.json"))
					if len(left) == 0 && len(answered) == 2 && strings.Contains(stdout.String(), "answered: q2") {
						break
					}
					if time.Now().After(deadline) {
						cmd.Process.Kill()
						t.Fatalf("%s: the loop did not answer in time:\n%s%s", how, stdout.String(), stderr.String())
					}
					time.Sleep(50 * time.Millisecond)
				}
				pid := strconv.Itoa(cmd.Process.Pid)
				cmd.Process.Signal(syscall.SIGTERM)
				code := 0
				if err := cmd.Wait(); err != nil {
					code = -1
				}
				claudeLog, _ := os.ReadFile(filepath.Join(base, "bin", "claude.log"))
				rec := fmt.Sprintf("--- exit %d ---\n--- stdout ---\n%s--- stderr ---\n%s--- claude ---\n%s--- changes ---\n%s",
					code, stdout.String(), stderr.String(), claudeLog, changes(before, snapshot(t, base)))
				rec = strings.NewReplacer(base, "$BASE", "pid "+pid, "pid $PID").Replace(rec)
				recs[i] = askTS.ReplaceAllString(rec, "<TS>")
			}
			checkGolden(t, "ask", "loop "+tool.name, recs[0], noRoot, 0)
		})
	}
}
