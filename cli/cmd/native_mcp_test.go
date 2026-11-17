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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// The MCP server (`rebar mcp serve`) and ask-server (`rebar ask serve`)
// against transcripts in testdata/mcp: a JSON-RPC session over stdio and
// HTTP request sequences. rebar 3.2 sent the same sessions to the python
// originals and required the same responses (byte for byte over stdio:
// the server writes Python's json.dumps format), so the goldens hold the
// python servers' behaviour, except where a request documents a deliberate
// deviation. tools/call runs a stub ask (--ask-bin); TestMCPNativeAsk runs
// the server with its own ask against a stub claude.

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

// stdioErrLines hides each json library's own words in parse errors (the
// python server's differed from Go's).
func stdioErrLines(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "ask-mcp-server: parse error:") {
			l = "ask-mcp-server: parse error: <msg>"
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// TestMCPStdioGolden: one session through `rebar mcp serve` and through
// the bin/ask-mcp-server shim (what .mcp.json files name).
func TestMCPStdioGolden(t *testing.T) {
	repos, stub := mcpFixture(t)
	input := strings.Join(mcpSession, "\n") + "\n"
	args := []string{"--stdio", "--repos-dir", repos, "--ask-bin", stub}
	env := mcpEnv(t, "ASK_SERVER=elsewhere:1")
	goOut, goErr, goCode := mcpStdio(t, append([]string{testBinary(t), "mcp", "serve"}, args...), env, input)
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, repos, "$REPOS"), stub, "$STUB")
	}
	checkGolden(t, "mcp", "stdio session", norm(goOut+"--- stderr ---\n"+stdioErrLines(goErr)), "\x00", goCode)

	shimDir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(testutil.RebarSource(t), "bin", "ask-mcp-server"))
	if err != nil {
		t.Fatal(err)
	}
	shim := testutil.WriteExecutable(t, shimDir, "ask-mcp-server", string(data))
	testutil.WriteExecutable(t, shimDir, "rebar", "#!/bin/sh\nREBAR_TEST_RUN_CLI=1 exec '"+testBinary(t)+"' \"$@\"\n")
	shOut, shErr, shCode := mcpStdio(t, append([]string{shim}, args...), env, input)
	if shOut != goOut || shErr != goErr || shCode != goCode {
		t.Errorf("bin/ask-mcp-server differs from rebar mcp serve:\n%s%s\n(exit %d)", shOut, shErr, shCode)
	}
}

// TestMCPStdioErrors: the server's start-up failures.
func TestMCPStdioErrors(t *testing.T) {
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
			out, errOut, code := mcpStdio(t, append([]string{testBinary(t), "mcp", "serve"}, tc.args...), mcpEnv(t), "")
			checkGolden(t, "mcp", "stdio error "+tc.name, strings.ReplaceAll(out+"--- stderr ---\n"+errOut, repos, "$REPOS"), "\x00", code)
		})
	}
	// A usage error exits 2 (as argparse did; the wording is rebar's).
	_, goErr, goCode := mcpStdio(t, []string{testBinary(t), "mcp", "serve", "--stdio"}, mcpEnv(t), "")
	if goCode != 2 || !strings.Contains(goErr, "Provide --repos-dir or --repos") {
		t.Errorf("no repos flag: exit %d:\n%s", goCode, goErr)
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
	// diverges documents a deliberate difference from the python server.
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

// compareHTTP sends reqs to the server and compares status and body with
// the golden transcript.
func compareHTTP(t *testing.T, golden string, port int, reqs []httpReq, normalize func(string) string) {
	t.Helper()
	var rec strings.Builder
	for _, r := range reqs {
		status, _, raw := exchange(t, port, r)
		fmt.Fprintf(&rec, "%s %s %s\n%d %s\n", r.method, r.path, r.body, status, normalize(raw))
	}
	checkGolden(t, "mcp", golden, rec.String(), "\x00", 0)
}

// TestMCPHTTPGolden: the MCP server's HTTP mode (with and without
// --mcp-only and an API key).
func TestMCPHTTPGolden(t *testing.T) {
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
			goPort := freePort(t)
			flags := append([]string{"--repos-dir", repos, "--ask-bin", stub, "--host", "127.0.0.1"}, mode.flags...)
			startHTTP(t, append([]string{testBinary(t), "mcp", "serve", "--port", strconv.Itoa(goPort)}, flags...), mcpEnv(t), goPort)
			compareHTTP(t, mode.name, goPort, mode.reqs, norm)
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

// TestAskServerGolden: ask-server's HTTP API.
func TestAskServerGolden(t *testing.T) {
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
			goPort := freePort(t)
			flags := append([]string{"--repos-dir", repos, "--ask-bin", statusStub, "--host", "127.0.0.1"}, mode.flags...)
			startHTTP(t, append([]string{testBinary(t), "ask", "serve", "--port", strconv.Itoa(goPort)}, flags...), mcpEnv(t), goPort)
			compareHTTP(t, mode.name, goPort, mode.reqs, func(s string) string { return norm(strings.ReplaceAll(s, statusStub, "$STUB")) })
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

// TestMCPClientGolden: `rebar mcp client` against an HTTP MCP server
// prints what bin/test-mcp-client (python, removed in rebar 4) printed.
func TestMCPClientGolden(t *testing.T) {
	repos, stub := mcpFixture(t)
	port := freePort(t)
	startHTTP(t, []string{testBinary(t), "mcp", "serve", "--port", strconv.Itoa(port), "--host", "127.0.0.1", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), port)
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	goOut, goErr, goCode := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), "mcp", "client", url)
	if goCode != 0 {
		t.Fatalf("exit %d:\n%s%s", goCode, goOut, goErr)
	}
	checkGolden(t, "mcp", "client", strings.ReplaceAll(goOut, repos, "$REPOS"), "\x00none\x00", goCode)
	usageOut, _, usageCode := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), "mcp", "client")
	if usageCode != 1 || !strings.HasPrefix(usageOut, "Usage: test-mcp-client <server-url>") {
		t.Errorf("usage: exit %d\n%s", usageCode, usageOut)
	}
}
