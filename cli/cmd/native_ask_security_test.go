package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// R1/R2 of the v4.0.0 review: a question's first word picked
// agents/<role>/commands/<word>.sh with no path check, so a remote caller
// (ask-server, the MCP server's /v1/ask and tools) could run any *.sh
// ("../../../../evil/x"), and a role could be a path to another repo's
// agent, even one opted out of MCP. Now command words are plain names of
// scripts directly in commands/, servers ask with `ask --question` (never a
// command or subcommand), and a role must be one of the repo's agents —
// the rules of the v3.1.2 bash/python fix.

// markScript writes a script that leaves $MARK/<name> behind.
func markScript(name string) string {
	return "#!/bin/sh\ntouch \"$MARK/" + name + "\"\necho ran " + name + "\n"
}

func marks(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// securityTree is $BASE/proj with agent architect (commands pwn, -x,
// default), $BASE/evil/x.sh and $BASE/hidden/agents/secret, all leaving
// marks in $BASE/marks when run.
func securityTree(t *testing.T) (base string, env []string) {
	base = askTree(t, askCase{files: map[string]string{"agents/architect/AGENT.md": "# Architect\n"}})
	for rel, body := range map[string]string{
		"proj/agents/architect/commands/pwn.sh":     markScript("pwn"),
		"proj/agents/architect/commands/-x.sh":      markScript("dash-x"),
		"proj/agents/architect/commands/default.sh": markScript("default"),
		"evil/x.sh": markScript("evil"),
	} {
		testutil.WriteExecutable(t, base, rel, body)
	}
	testutil.WriteFile(t, base, "hidden/agents/secret/AGENT.md", "# Secret\n")
	testutil.WriteFile(t, base, "marks/.keep", "")
	os.Remove(filepath.Join(base, "marks/.keep"))
	env = append(askEnv(t, base, filepath.Join(base, "bin")+string(os.PathListSeparator)+os.Getenv("PATH")),
		"MARK="+filepath.Join(base, "marks"), "REBAR_TEST_RUN_CLI=1")
	return base, env
}

func TestAskCommandWordsAndAgentNames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		onlyAsk   bool // REBAR_ASK_QUESTION_ONLY=1
		code      int
		mark      string // the script that must have run ("" none)
		askedLLM  bool
		stderrHas string
	}{
		{"traversing word is a question", []string{"architect", "../../../../evil/x"}, false, 0, "", true, ""},
		{"traversing word, --question", []string{"architect", "../../../../evil/x"}, true, 0, "", true, ""},
		{"dash word is a question", []string{"architect", "-x"}, false, 0, "", true, ""},
		{"command still runs", []string{"architect", "pwn"}, false, 0, "pwn", false, ""},
		{"--question never runs a command", []string{"architect", "pwn"}, true, 0, "", true, ""},
		{"--question, flag-like question", []string{"architect", "-v"}, true, 0, "", true, ""},
		{"--question needs a question", []string{"architect"}, true, 4, "", false, "--question takes exactly two arguments"},
		{"--question takes two arguments", []string{"architect", "a", "b"}, true, 4, "", false, "--question takes exactly two arguments"},
		{"--question is not a subcommand", []string{"status", "architect"}, true, 2, "", false, "agent 'status' not found"},
		{"--question no path", []string{"../../hidden/agents/secret", "q"}, true, 2, "", false, "invalid agent name"},
		{"agent path is not an agent", []string{"../../hidden/agents/secret", "q"}, false, 2, "", false, "invalid agent name"},
		{"dotted agent is not an agent", []string{"..", "q"}, false, 2, "", false, "invalid agent name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base, env := securityTree(t)
			args := append([]string{"ask"}, tc.args...)
			if tc.onlyAsk {
				args = append([]string{"ask", "--question", "--"}, tc.args...)
			}
			_, stderr, code := runSplit(t, filepath.Join(base, "proj"), env, "piped question\n", testBinary(t), args...)
			if code != tc.code {
				t.Errorf("exit %d, want %d:\n%s", code, tc.code, stderr)
			}
			got := marks(t, filepath.Join(base, "marks"))
			if want := []string(nil); tc.mark != "" {
				want = []string{tc.mark}
				if strings.Join(got, ",") != strings.Join(want, ",") {
					t.Errorf("scripts run: %v, want %v", got, want)
				}
			} else if len(got) > 0 {
				t.Errorf("scripts run: %v, want none", got)
			}
			_, err := os.Stat(filepath.Join(base, "bin", "claude.log"))
			if asked := err == nil; asked != tc.askedLLM {
				t.Errorf("claude asked: %v, want %v\n%s", asked, tc.askedLLM, stderr)
			}
			if !strings.Contains(stderr, tc.stderrHas) {
				t.Errorf("stderr lacks %q:\n%s", tc.stderrHas, stderr)
			}
		})
	}
}

// TestServersAskQuestionOnly: the reviewer's reproduction against both
// HTTP servers with the native ask: a traversing question is asked, never
// run; a role naming another repo's agent (opted out of MCP) is unknown.
func TestServersAskQuestionOnly(t *testing.T) {
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	repos := filepath.Join(base, "repos")
	testutil.WriteFile(t, repos, "pub/agents/architect/AGENT.md", "# Architect\n")
	testutil.WriteExecutable(t, repos, "pub/agents/architect/commands/pwn.sh", markScript("pwn"))
	testutil.WriteFile(t, repos, "hidden/agents/secret/AGENT.md", "# Secret\n")
	testutil.WriteFile(t, repos, "hidden/.rebar-skip-mcp", "")
	testutil.WriteExecutable(t, base, "evil/x.sh", markScript("evil"))
	if err := os.MkdirAll(filepath.Join(base, "marks"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "claude", "#!/bin/sh\nprintf '{\"result\":\"llm answer\",\"session_id\":\"s\"}\\n'\n")
	env := mcpEnv(t, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+t.TempDir(), "MARK="+filepath.Join(base, "marks"))
	for _, srv := range []struct {
		name, unknownRole string
		argv              []string
	}{
		{"mcp serve", `500 {"error": "Agent ../../hidden/agents/secret not found in repository pub"}`, []string{"mcp", "serve"}},
		{"ask serve", `404 {"error": "Agent '../../hidden/agents/secret' not found in repo 'pub'. Use GET /v1/agents to list available agents."}`, []string{"ask", "serve"}},
	} {
		t.Run(srv.name, func(t *testing.T) {
			port := freePort(t)
			// --repos names the opted-out repo explicitly: still skipped.
			startHTTP(t, append([]string{testBinary(t)}, append(srv.argv, "--port", strconv.Itoa(port),
				"--repos", filepath.Join(repos, "pub")+","+filepath.Join(repos, "hidden"))...), env, port)
			for _, tc := range []struct{ body, want string }{
				{`{"repo":"pub","role":"architect","question":"../../../../../evil/x"}`, `"answer": "llm answer"`},
				{`{"repo":"pub","role":"architect","question":"pwn"}`, `"answer": "llm answer"`},
				{`{"repo":"pub","role":"../../hidden/agents/secret","question":"q"}`, srv.unknownRole},
				{`{"repo":"hidden","role":"secret","question":"q"}`, `"error": "`},
			} {
				status, _, raw, _ := exchange(t, port, httpReq{method: http.MethodPost, path: "/v1/ask", body: tc.body})
				got := strconv.Itoa(status) + " " + raw
				if !strings.Contains(got, tc.want) {
					t.Errorf("%s: %s, want %s", tc.body, got, tc.want)
				}
			}
			if got := marks(t, filepath.Join(base, "marks")); len(got) > 0 {
				t.Errorf("scripts run: %v", got)
			}
		})
	}
}
