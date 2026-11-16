package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Parity of the native MCP server (`rebar mcp serve`) and ask-server
// (`rebar ask serve`) with the python originals (bin/legacy/*.py): the
// same JSON-RPC session over stdio, and the same HTTP requests, go to
// both, and every response must match — compared structurally (decoded
// JSON) and, for stdio, byte for byte, since the native server writes
// Python's json.dumps format. tools/call runs a stub ask (--ask-bin) on
// both sides, so answers and failures are the same; TestMCPNativeAsk runs
// the native server with its own ask against a stub claude. Transcripts
// are goldens in testdata/mcp. The python side is skipped without python3.

// mcpRepos is a --repos-dir: two repos (one nested a level down, one with
// "_" in its name), a repo opted out, a non-repo, and skipped directories.
var mcpRepos = map[string]string{
	"web/agents/architect/AGENT.md":            "# Architect\n\nYou are the architect agent for web. You own design.\nRepo-specific: owns the API gateway.\n\n## More\n",
	"web/agents/product/AGENT.md":              "# Product\n\nOwns personas for the web app.\n",
	"web/agents/product/memory.md":             "Remembered: users are admins.\n",
	"web/agents/product/memory.log.md":         "---\n**2026-01-01T00:00:00Z** Q: hi\nA: hello\n",
	"web/agents/custom/AGENT.md":               "You are the custom agent. You don't guess — never. Handles the custom stuff, mostly.\n",
	"web/agents/empty/AGENT.md":                "# Only a heading\n",
	"web/agents/notanagent/README.md":          "x\n",
	"group/dsp_2_8/agents/steward/AGENT.md":    "# Steward\n\nKeeps score: é ünïcode ✓ and emoji 🚀.\r\nSecond line.\r\n",
	"skipme/agents/architect/AGENT.md":         "x\n",
	"skipme/.rebar-skip-mcp":                   "",
	"node_modules/x/agents/architect/AGENT.md": "x\n",
	".hidden/agents/architect/AGENT.md":        "x\n",
	"plain/README.md":                          "not a repo\n",
	"web/agents/broken/AGENT.md":               "Fails.\n",
}

// stubAsk answers like ask: the repo, the args; role broken fails.
const stubAsk = `#!/bin/sh
case "$*" in
  *broken*) echo "partial output"; echo "ask: error: claude invocation failed" >&2; exit 3 ;;
esac
printf '  answer from %s: %s  \n\n' "$(basename "$(pwd)")" "$*"
`

// mcpSession is the stdio transcript: each line is sent as is.
var mcpSession = []string{
	`{"jsonrpc":"2.0","method":"tools/list","id":0}`,
	`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}},"id":1}`,
	`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	`{"jsonrpc":"2.0","method":"tools/list","id":2}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":"Why REST?"}},"id":3}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_product","arguments":{"question":"Who?","verbose":true}},"id":"str-id"}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_WEB_ARCHITECT","arguments":{"question":"case?","verbose":"no"}},"id":4.5}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_broken","arguments":{"question":"fail?"}},"id":5}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_nope_architect","arguments":{"question":"q"}},"id":6}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_nobody","arguments":{"question":"q"}},"id":7}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"web_architect","arguments":{"question":"q"}},"id":8}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_webarchitect","arguments":{"question":"q"}},"id":9}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"arguments":{"question":"q"}},"id":10}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{}},"id":11}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":""}},"id":12}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":null,"id":13}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":null},"id":14}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":42}},"id":15}`,
	`{"jsonrpc":"2.0","method":"tools/call","params":{"name":7,"arguments":{"question":"q"}},"id":16}`,
	`{"jsonrpc":"2.0","method":"resources/list","id":17}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/web:product"},"id":18}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://log/WEB:Product"},"id":19}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://agent/web:architect"},"id":20}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/web:architect"},"id":21}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://agent/web:ghost"},"id":22}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://secrets/web:architect"},"id":23}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory"},"id":24}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/web"},"id":25}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/nope:x"},"id":26}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"file:///etc/passwd"},"id":27}`,
	`{"jsonrpc":"2.0","method":"resources/read","params":{},"id":28}`,
	`{"jsonrpc":"2.0","method":"prompts/list","id":29}`,
	`{"jsonrpc":"2.0","id":30}`,
	`{"jsonrpc":"1.0","method":"initialize","id":31}`,
	`{"method":"initialize"}`,
	`[{"jsonrpc":"2.0","method":"tools/list","id":32}]`,
	`"just a string"`,
	`this is not json`,
	``,
	`   `,
	`{"jsonrpc":"2.0","method":"tools/list","id":null}`,
	`{"jsonrpc":"2.0","method":"initialize","id":{"nested":[1,2]}}`,
	`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":3}}`,
}

// requirePython skips without python3.
func requirePython(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed: no python side to compare with")
	}
	return p
}

// mcpFixture writes the repos dir and the stub ask and returns them.
func mcpFixture(t *testing.T) (repos, stub string) {
	t.Helper()
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	repos = filepath.Join(base, "repos")
	for rel, body := range mcpRepos {
		testutil.WriteFile(t, repos, rel, body)
	}
	stub = testutil.WriteExecutable(t, base, "bin/ask", stubAsk)
	return repos, stub
}

// legacy is bin/legacy/<name>.py of the rebar source.
func legacy(t *testing.T, name string) string {
	return filepath.Join(testutil.RebarSource(t), "bin", "legacy", name+".py")
}

// mcpStdio runs one server over stdio with input and returns stdout and
// stderr.
func mcpStdio(t *testing.T, argv []string, env []string, input string) (string, string, int) {
	t.Helper()
	return runSplit(t, t.TempDir(), env, input, argv[0], argv[1:]...)
}

func mcpEnv(t *testing.T, extra ...string) []string {
	var env []string
	for _, kv := range testutil.IsolatedEnv() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "ASK_") {
			continue
		}
		env = append(env, kv)
	}
	return append(append(env, "REBAR_TEST_RUN_CLI=1", "LC_ALL=C"), extra...)
}

// TestMCPStdioParity: one session through both stdio servers.
func TestMCPStdioParity(t *testing.T) {
	python := requirePython(t)
	repos, stub := mcpFixture(t)
	input := strings.Join(mcpSession, "\n") + "\n"
	args := []string{"--stdio", "--repos-dir", repos, "--ask-bin", stub}
	env := mcpEnv(t, "ASK_SERVER=elsewhere:1")
	pyOut, pyErr, pyCode := mcpStdio(t, append([]string{python, legacy(t, "ask-mcp-server")}, args...), env, input)
	goOut, goErr, goCode := mcpStdio(t, append([]string{testBinary(t), "mcp", "serve"}, args...), env, input)

	if pyCode != goCode {
		t.Errorf("exit: python %d, native %d", pyCode, goCode)
	}
	pyLines, goLines := strings.Split(pyOut, "\n"), strings.Split(goOut, "\n")
	for i := 0; i < len(pyLines) || i < len(goLines); i++ {
		var p, g string
		if i < len(pyLines) {
			p = pyLines[i]
		}
		if i < len(goLines) {
			g = goLines[i]
		}
		if p != g {
			t.Errorf("response %d differs:\npython: %s\nnative: %s", i+1, p, g)
		}
	}
	// stderr: the banner and registrations match; parse errors are each
	// json library's own words.
	parse := func(s string) string {
		var out []string
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(l, "ask-mcp-server: parse error:") {
				l = "ask-mcp-server: parse error: <msg>"
			}
			out = append(out, l)
		}
		return strings.Join(out, "\n")
	}
	if parse(pyErr) != parse(goErr) {
		t.Errorf("stderr differs:\npython:\n%s\nnative:\n%s", pyErr, goErr)
	}
	checkGolden(t, "mcp", "stdio session", strings.ReplaceAll(strings.ReplaceAll(goOut+"--- stderr ---\n"+parse(goErr), repos, "$REPOS"), stub, "$STUB"), "\x00", goCode)
}

// TestMCPStdioErrors: the servers' start-up failures.
func TestMCPStdioErrors(t *testing.T) {
	python := requirePython(t)
	repos, _ := mcpFixture(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no repos", []string{"--stdio", "--repos-dir", filepath.Join(repos, "plain")}},
		{"missing repos dir", []string{"--stdio", "--repos-dir", filepath.Join(repos, "missing")}},
		{"explicit repos", []string{"--stdio", "--repos", filepath.Join(repos, "web") + "," + filepath.Join(repos, "plain")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := mcpEnv(t)
			pyOut, pyErr, pyCode := mcpStdio(t, append([]string{python, legacy(t, "ask-mcp-server")}, tc.args...), env, "")
			goOut, goErr, goCode := mcpStdio(t, append([]string{testBinary(t), "mcp", "serve"}, tc.args...), env, "")
			if pyOut != goOut || pyErr != goErr || pyCode != goCode {
				t.Errorf("differs:\npython (exit %d):\n%s%s\nnative (exit %d):\n%s%s", pyCode, pyOut, pyErr, goCode, goOut, goErr)
			}
		})
	}
	// Usage errors exit 2 on both sides (argparse's wording is its own).
	_, _, pyCode := mcpStdio(t, []string{python, legacy(t, "ask-mcp-server"), "--stdio"}, mcpEnv(t), "")
	_, goErr, goCode := mcpStdio(t, []string{testBinary(t), "mcp", "serve", "--stdio"}, mcpEnv(t), "")
	if pyCode != 2 || goCode != 2 || !strings.Contains(goErr, "Provide --repos-dir or --repos") {
		t.Errorf("no repos flag: python exit %d, native exit %d:\n%s", pyCode, goCode, goErr)
	}
}

// freePort returns a port nothing listens on.
func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startHTTP starts a server on port and waits until it answers.
func startHTTP(t *testing.T, argv, env []string, port int) {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Dir = t.TempDir()
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	deadline := time.Now().Add(60 * time.Second)
	for {
		if c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			c.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%v did not start:\n%s", argv, logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// httpReq is one request of an HTTP transcript.
type httpReq struct {
	method, path, body, auth string
	// diverges documents a deliberate difference: the python answer is
	// only logged.
	diverges string
}

// exchange sends a request and returns "status body-as-JSON".
func exchange(t *testing.T, port int, r httpReq) (int, any, string) {
	t.Helper()
	req, err := http.NewRequest(r.method, "http://127.0.0.1:"+strconv.Itoa(port)+r.path, strings.NewReader(r.body))
	if err != nil {
		t.Fatal(err)
	}
	if r.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.auth != "" {
		req.Header.Set("Authorization", r.auth)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", r.method, r.path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return resp.StatusCode, "(not JSON) " + resp.Header.Get("Content-Type"), string(data)
	}
	return resp.StatusCode, v, string(data)
}

// invalidJSON hides each json library's own error words.
func invalidJSON(v any) any {
	if m, ok := v.(map[string]any); ok {
		if s, ok := m["error"].(string); ok && strings.HasPrefix(s, "Invalid JSON:") {
			m["error"] = "Invalid JSON: <msg>"
		}
	}
	return v
}

// compareHTTP sends reqs to both servers and compares status and JSON.
func compareHTTP(t *testing.T, golden string, pyPort, goPort int, reqs []httpReq, normalize func(string) string) {
	t.Helper()
	var rec strings.Builder
	for _, r := range reqs {
		pyStatus, pyBody, _ := exchange(t, pyPort, r)
		goStatus, goBody, goRaw := exchange(t, goPort, r)
		pyBody, goBody = invalidJSON(pyBody), invalidJSON(goBody)
		if r.diverges != "" {
			t.Logf("%s %s: deliberate deviation (%s); python gave %d %v", r.method, r.path, r.diverges, pyStatus, pyBody)
		} else if pyStatus != goStatus || !reflect.DeepEqual(pyBody, goBody) {
			t.Errorf("%s %s %s: python %d %v, native %d %v", r.method, r.path, r.body, pyStatus, pyBody, goStatus, goBody)
		}
		fmt.Fprintf(&rec, "%s %s %s\n%d %s\n", r.method, r.path, r.body, goStatus, normalize(goRaw))
	}
	checkGolden(t, "mcp", golden, rec.String(), "\x00", 0)
}

// TestMCPHTTPParity: the MCP server's HTTP mode (with and without
// --mcp-only and an API key).
func TestMCPHTTPParity(t *testing.T) {
	python := requirePython(t)
	repos, stub := mcpFixture(t)
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, repos, "$REPOS"), stub, "$STUB")
	}
	init := `{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}`
	for _, mode := range []struct {
		name  string
		flags []string
		reqs  []httpReq
	}{
		{"http", nil, []httpReq{
			{"GET", "/mcp/schema", "", "", ""},
			{"GET", "/v1/health", "", "", ""},
			{"GET", "/v1/agents/", "", "", ""},
			{"GET", "/v1/repos?x=1", "", "", ""},
			{"GET", "/v1/status/web/architect", "", "", ""},
			{"POST", "/mcp", init, "", ""},
			{"POST", "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "", ""},
			{"POST", "/mcp", `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":"Why?"}},"id":2}`, "", "the session outlives a request (python: a new, uninitialized one per request)"},
			{"POST", "/mcp", `not json`, "", ""},
			{"POST", "/mcp", ``, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"product","question":" Who? ","verbose":true}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"broken","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"WEB","role":"product","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","question":"q"}`, "", ""},
			{"POST", "/v1/reset/web/architect", "", "", ""},
			{"PUT", "/mcp", "{}", "", ""},
		}},
		{"http mcp-only with key", []string{"--mcp-only", "--api-key", "s3cret"}, []httpReq{
			{"GET", "/mcp/schema", "", "", ""},
			{"GET", "/mcp/schema", "", "Bearer s3cret", ""},
			{"GET", "/v1/health", "", "Bearer wrong", ""},
			{"GET", "/v1/health", "", "Basic s3cret", ""},
			{"GET", "/v1/health", "", "Bearer s3cret", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"product","question":"q"}`, "Bearer s3cret", ""},
			{"POST", "/mcp", init, "Bearer s3cret", ""},
		}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			pyPort, goPort := freePort(t), freePort(t)
			flags := append([]string{"--repos-dir", repos, "--ask-bin", stub, "--host", "127.0.0.1"}, mode.flags...)
			env := mcpEnv(t)
			startHTTP(t, append([]string{python, legacy(t, "ask-mcp-server"), "--port", strconv.Itoa(pyPort)}, flags...), env, pyPort)
			startHTTP(t, append([]string{testBinary(t), "mcp", "serve", "--port", strconv.Itoa(goPort)}, flags...), env, goPort)
			compareHTTP(t, mode.name, pyPort, goPort, mode.reqs, norm)
		})
	}
}

// TestMCPHTTPSession: over HTTP the native server keeps the session across
// requests (python built a new, uninitialized one per request, so tools/list
// after initialize failed with -32002).
func TestMCPHTTPSession(t *testing.T) {
	repos, stub := mcpFixture(t)
	port := freePort(t)
	startHTTP(t, []string{testBinary(t), "mcp", "serve", "--port", strconv.Itoa(port), "--host", "127.0.0.1", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), port)
	exchange(t, port, httpReq{"POST", "/mcp", `{"jsonrpc":"2.0","method":"initialize","id":1}`, "", ""})
	_, body, raw := exchange(t, port, httpReq{"POST", "/mcp", `{"jsonrpc":"2.0","method":"tools/list","id":2}`, "", ""})
	if m, _ := body.(map[string]any); m["result"] == nil {
		t.Errorf("tools/list after initialize over HTTP: %s", raw)
	}
}

// TestAskServerParity: ask-server's HTTP API.
func TestAskServerParity(t *testing.T) {
	python := requirePython(t)
	repos, stub := mcpFixture(t)
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, repos, "$REPOS"), stub, "$STUB")
	}
	statusStub := testutil.WriteExecutable(t, filepath.Dir(stub), "ask-status", `#!/bin/sh
case "$1" in
  status) printf 'STATUS: stopped\nweird line\nKey With Space:  v:2 \n' ;;
  reset) echo "Session cleared for $2 (was: abc)" ;;
  *) exec "`+stub+`" "$@" ;;
esac
`)
	for _, mode := range []struct {
		name  string
		flags []string
		reqs  []httpReq
	}{
		{"ask-server", nil, []httpReq{
			{"GET", "/v1/health", "", "", ""},
			{"GET", "/v1/agents", "", "", ""},
			{"GET", "/v1/repos/", "", "", ""},
			{"GET", "/v1/status/web/architect", "", "", ""},
			{"GET", "/v1/status/web/ghost", "", "", ""},
			{"GET", "/v1/status/nope/architect", "", "", ""},
			{"GET", "/v1/status/web", "", "", ""},
			{"GET", "/v1/log/web/product", "", "", ""},
			{"GET", "/v1/log/web/architect", "", "", ""},
			{"GET", "/v1/log/web/architect/extra", "", "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"architect","question":"Why?"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"product","question":"Who?","verbose":1}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"broken","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"nope","role":"architect","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"ghost","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"architect","question":"  "}`, "", ""},
			{"POST", "/v1/ask", `{bad`, "", ""},
			{"POST", "/v1/reset/web/architect", "", "", ""},
			{"POST", "/v1/reset/web", "", "", ""},
			{"POST", "/v1/nothing", "", "", ""},
			{"GET", "/elsewhere", "", "", ""},
		}},
		{"ask-server with key", []string{"--api-key", "k"}, []httpReq{
			{"GET", "/v1/health", "", "", ""},
			{"GET", "/v1/health", "", "Bearer k", ""},
			{"GET", "/v1/health", "", "bearer k", ""},
		}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			pyPort, goPort := freePort(t), freePort(t)
			flags := append([]string{"--repos-dir", repos, "--ask-bin", statusStub, "--host", "127.0.0.1"}, mode.flags...)
			env := mcpEnv(t)
			startHTTP(t, append([]string{python, legacy(t, "ask-server"), "--port", strconv.Itoa(pyPort)}, flags...), env, pyPort)
			startHTTP(t, append([]string{testBinary(t), "ask", "serve", "--port", strconv.Itoa(goPort)}, flags...), env, goPort)
			compareHTTP(t, mode.name, pyPort, goPort, mode.reqs, func(s string) string { return norm(strings.ReplaceAll(s, statusStub, "$STUB")) })
		})
	}
}

// TestMCPToolNameWithUnderscore: a repo with "_" in its name can be called
// (the python server split ask_dsp_2_8_steward at the first "_" of the
// rest and reported "Repository dsp not found").
func TestMCPToolNameWithUnderscore(t *testing.T) {
	repos, stub := mcpFixture(t)
	input := `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n" +
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_dsp_2_8_steward","arguments":{"question":"q"}},"id":2}` + "\n" +
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_DSP_2_8_STEWARD","arguments":{"question":"q"}},"id":3}` + "\n"
	out, errOut, code := mcpStdio(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), input)
	if code != 0 || strings.Count(out, `"text": "answer from dsp_2_8: steward q"`) != 2 {
		t.Errorf("exit %d:\n%s\n%s", code, out, errOut)
	}
}

// TestMCPNativeAsk: with no --ask-bin, tools/call runs this binary's own
// `ask` in the repo (ASK_SERVER cleared): a stub claude answers.
func TestMCPNativeAsk(t *testing.T) {
	repos, _ := mcpFixture(t)
	bin := t.TempDir()
	testutil.WriteExecutable(t, bin, "claude", "#!/bin/sh\necho \"cwd=$(pwd) server=[$ASK_SERVER]\" >&2\nprintf '{\"result\":\"native answer\",\"session_id\":\"s-1\"}\\n'\n")
	env := mcpEnv(t, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "ASK_SERVER=remote:1", "HOME="+t.TempDir())
	input := `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n" +
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_product","arguments":{"question":"Who?"}},"id":2}` + "\n"
	out, errOut, code := mcpStdio(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos}, env, input)
	if code != 0 || !strings.Contains(out, `"text": "native answer"`) {
		t.Fatalf("exit %d:\n%s\n%s", code, out, errOut)
	}
	if s, err := os.ReadFile(filepath.Join(repos, "web/agents/product/.session-id")); err != nil || strings.TrimSpace(string(s)) != "s-1" {
		t.Errorf("the native ask did not run in the repo: %q %v", s, err)
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if strings.Contains(sc.Text(), `"id": 2`) && strings.Contains(sc.Text(), "error") {
			t.Errorf("tools/call failed: %s", sc.Text())
		}
	}
}

// TestMCPClientParity: `rebar mcp client` against a native HTTP MCP server
// prints what bin/legacy/test-mcp-client.py prints (that one needs the
// requests module; without it only the golden is checked).
func TestMCPClientParity(t *testing.T) {
	repos, stub := mcpFixture(t)
	port := freePort(t)
	startHTTP(t, []string{testBinary(t), "mcp", "serve", "--port", strconv.Itoa(port), "--host", "127.0.0.1", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), port)
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	goOut, goErr, goCode := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), "mcp", "client", url)
	if goCode != 0 {
		t.Fatalf("exit %d:\n%s%s", goCode, goOut, goErr)
	}
	if python, err := exec.LookPath("python3"); err == nil && exec.Command(python, "-c", "import requests").Run() == nil {
		pyOut, pyErr, pyCode := runSplit(t, t.TempDir(), mcpEnv(t), "", python, legacy(t, "test-mcp-client"), url)
		if pyOut != goOut || pyCode != goCode {
			t.Errorf("differs:\npython (exit %d):\n%s%s\nnative (exit %d):\n%s", pyCode, pyOut, pyErr, goCode, goOut)
		}
	} else {
		t.Log("python3 with requests not available: comparing with the golden only")
	}
	checkGolden(t, "mcp", "client", strings.ReplaceAll(goOut, repos, "$REPOS"), "\x00none\x00", goCode)
	usageOut, _, usageCode := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), "mcp", "client")
	if usageCode != 1 || !strings.HasPrefix(usageOut, "Usage: test-mcp-client <server-url>") {
		t.Errorf("usage: exit %d\n%s", usageCode, usageOut)
	}
}
