package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

// The MCP server (`rebar mcp serve`) and ask-server (`rebar ask serve`)
// against transcripts in testdata/mcp: an MCP session over stdio and HTTP
// request sequences. The MCP protocol layer is the official Go SDK since
// rebar 3.2 (S9 of the go-native stack); the goldens were regenerated then
// and the PR lists every difference from the python server's transcripts.
// The classic ASK API and ask-server still answer in python's json.dumps
// format. tools/call runs a stub ask (--ask-bin); TestMCPNativeAsk runs the
// server with its own ask against a stub claude.

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

// mcpStep is one line of a stdio transcript and whether the server
// answers it.
type mcpStep struct {
	msg   string
	reply bool
}

// mcpSession is the stdio transcript, sent one line at a time.
var mcpSession = []mcpStep{
	{`{"jsonrpc":"2.0","method":"tools/list","id":0}`, true},
	{`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}},"id":1}`, true},
	{`{"jsonrpc":"2.0","method":"notifications/initialized"}`, false},
	{`{"jsonrpc":"2.0","method":"tools/list","id":2}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":"Why REST?"}},"id":3}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_product","arguments":{"question":"Who?","verbose":true}},"id":"str-id"}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_WEB_ARCHITECT","arguments":{"question":"case?"}},"id":4}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_WEB_ARCHITECT","arguments":{"question":"case?","verbose":"no"}},"id":4.5}`, false},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":"bool?","verbose":"no"}},"id":45}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_broken","arguments":{"question":"fail?"}},"id":5}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_nope_architect","arguments":{"question":"q"}},"id":6}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_nobody","arguments":{"question":"q"}},"id":7}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"web_architect","arguments":{"question":"q"}},"id":8}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_webarchitect","arguments":{"question":"q"}},"id":9}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"arguments":{"question":"q"}},"id":10}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{}},"id":11}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":""}},"id":12}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":null,"id":13}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":null},"id":14}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_architect","arguments":{"question":42}},"id":15}`, true},
	{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":7,"arguments":{"question":"q"}},"id":16}`, true},
	{`{"jsonrpc":"2.0","method":"resources/list","id":17}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/web:product"},"id":18}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://log/WEB:Product"},"id":19}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://agent/web:architect"},"id":20}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/web:architect"},"id":21}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://agent/web:ghost"},"id":22}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://secrets/web:architect"},"id":23}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory"},"id":24}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/web"},"id":25}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/nope:x"},"id":26}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"file:///etc/passwd"},"id":27}`, true},
	{`{"jsonrpc":"2.0","method":"resources/read","params":{},"id":28}`, true},
	{`{"jsonrpc":"2.0","method":"prompts/list","id":29}`, true},
	{`{"jsonrpc":"2.0","method":"no/such/method","id":33}`, true},
	{`{"jsonrpc":"2.0","method":"ping","id":34}`, true},
	{`{"jsonrpc":"2.0","id":30}`, false},
	{`{"jsonrpc":"1.0","method":"initialize","id":31}`, false},
	{`{"method":"initialize"}`, false},
	{`[{"jsonrpc":"2.0","method":"tools/list","id":32}]`, true},
	{`[]`, false},
	{`"just a string"`, false},
	{`this is not json`, false},
	{``, false},
	{`   `, false},
	{`{"jsonrpc":"2.0","method":"tools/list","id":null}`, false},
	{`{"jsonrpc":"2.0","method":"initialize","id":{"nested":[1,2]}}`, false},
	{`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":3}}`, false},
	{`{"jsonrpc":"2.0","method":"ping","id":35}`, true},
}

// mcpInit is an initialize request.
const mcpInit = `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}},"id":1}`

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

// mcpConverse runs argv as a stdio MCP server and sends the steps one at a
// time, waiting for the answer of each step that has one, as a client
// does. It returns the transcript ("> " sent, "< " received; answers that
// arrive after the end of input are listed last), stderr and the exit code.
func mcpConverse(t *testing.T, argv, env []string, steps []mcpStep) (string, string, int) {
	t.Helper()
	var cmd *exec.Cmd
	var stdin io.WriteCloser
	var stdout io.ReadCloser
	var stderr bytes.Buffer
	for try := 0; ; try++ {
		cmd = exec.Command(argv[0], argv[1:]...)
		cmd.Dir, cmd.Env, cmd.Stderr = t.TempDir(), env, &stderr
		var err error
		if stdin, err = cmd.StdinPipe(); err != nil {
			t.Fatal(err)
		}
		if stdout, err = cmd.StdoutPipe(); err != nil {
			t.Fatal(err)
		}
		err = cmd.Start()
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.ETXTBSY) || try == 50 {
			t.Fatalf("%v: %v", argv, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(nil, 16<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	var rec strings.Builder
	for _, st := range steps {
		fmt.Fprintf(&rec, "> %s\n", st.msg)
		if _, err := io.WriteString(stdin, st.msg+"\n"); err != nil {
			t.Fatalf("writing %s: %v", st.msg, err)
		}
		if !st.reply {
			continue
		}
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("the server exited before answering %s:\n%s%s", st.msg, rec.String(), stderr.String())
			}
			fmt.Fprintf(&rec, "< %s\n", l)
		case <-time.After(60 * time.Second):
			t.Fatalf("no answer to %s:\n%s%s", st.msg, rec.String(), stderr.String())
		}
	}
	stdin.Close()
	for l := range lines {
		fmt.Fprintf(&rec, "< (after end of input) %s\n", l)
	}
	code := 0
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return rec.String(), stderr.String(), code
}

// mcpVersion hides rebar's version (serverInfo.version) in transcripts.
func mcpVersion(s string) string {
	s = strings.ReplaceAll(s, `"version":"`+Version+`"`, `"version":"$VERSION"`)
	return strings.ReplaceAll(s, `"version": "`+Version+`"`, `"version": "$VERSION"`)
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

// TestMCPStdioGolden: one session through `rebar mcp serve` and through
// the bin/ask-mcp-server shim (what .mcp.json files name).
func TestMCPStdioGolden(t *testing.T) {
	repos, stub := mcpFixture(t)
	args := []string{"--stdio", "--repos-dir", repos, "--ask-bin", stub}
	env := mcpEnv(t, "ASK_SERVER=elsewhere:1")
	goOut, goErr, goCode := mcpConverse(t, append([]string{testBinary(t), "mcp", "serve"}, args...), env, mcpSession)
	norm := func(s string) string {
		return mcpVersion(strings.ReplaceAll(strings.ReplaceAll(s, repos, "$REPOS"), stub, "$STUB"))
	}
	checkGolden(t, "mcp", "stdio session", norm(goOut+"--- stderr ---\n"+goErr), "\x00", goCode)

	shimDir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(testutil.RebarSource(t), "bin", "ask-mcp-server"))
	if err != nil {
		t.Fatal(err)
	}
	shim := testutil.WriteExecutable(t, shimDir, "ask-mcp-server", string(data))
	testutil.WriteExecutable(t, shimDir, "rebar", "#!/bin/sh\nREBAR_TEST_RUN_CLI=1 exec '"+testBinary(t)+"' \"$@\"\n")
	shOut, shErr, shCode := mcpConverse(t, append([]string{shim}, args...), env, mcpSession)
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
	// hdr is extra request headers, "Name: value" lines; for /mcp,
	// "!nosession" leaves out the session headers compareHTTP adds.
	hdr string
}

// isMCPPath: the SDK's Streamable HTTP endpoint (not /mcp/schema).
func isMCPPath(p string) bool { return p == "/mcp" || p == "/mcp/" }

// exchange sends a request and returns status, body as JSON, raw body and
// the response headers.
func exchange(t *testing.T, port int, r httpReq) (int, any, string, http.Header) {
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
	for _, h := range strings.Split(r.hdr, "\n") {
		if k, v, ok := strings.Cut(h, ": "); ok {
			req.Header.Set(k, v)
		}
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
		return resp.StatusCode, "(not JSON) " + resp.Header.Get("Content-Type"), string(data), resp.Header
	}
	return resp.StatusCode, v, string(data), resp.Header
}

// compareHTTP sends reqs to the server and compares status and body with
// the golden transcript. Requests to /mcp go as a Streamable HTTP client
// sends them: accepting JSON and event streams, with the session id and
// protocol version from the initialize answer; their transcript entries
// show those headers and the answer's Content-Type and session id.
func compareHTTP(t *testing.T, golden string, port int, reqs []httpReq, normalize func(string) string) {
	t.Helper()
	var rec strings.Builder
	var session, version string
	for _, r := range reqs {
		mcpReq := isMCPPath(r.path)
		shown := r.hdr
		if mcpReq {
			var hdr []string
			if !strings.Contains(r.hdr, "Accept: ") {
				hdr = append(hdr, "Accept: application/json, text/event-stream")
			}
			if session != "" && !strings.Contains(r.hdr, "!nosession") && !strings.Contains(r.hdr, "Mcp-Session-Id: ") {
				hdr = append(hdr, "Mcp-Session-Id: "+session, "Mcp-Protocol-Version: "+version)
			}
			r.hdr = strings.Join(append(hdr, r.hdr), "\n")
		}
		status, body, raw, h := exchange(t, port, r)
		fmt.Fprintf(&rec, "%s %s %s\n", r.method, r.path, r.body)
		if shown != "" {
			fmt.Fprintf(&rec, "  (%s)\n", strings.ReplaceAll(shown, "\n", "; "))
		}
		if mcpReq {
			if id := h.Get("Mcp-Session-Id"); id != "" {
				session = id
				if res, ok := lookupJSON(body, "result").(map[string]any); ok {
					version, _ = res["protocolVersion"].(string)
				}
			}
			fmt.Fprintf(&rec, "%d [%s] ", status, h.Get("Content-Type"))
			if session != "" {
				raw = strings.ReplaceAll(raw, session, "$SESSION")
			}
			if h.Get("Mcp-Session-Id") != "" {
				fmt.Fprintf(&rec, "[Mcp-Session-Id: $SESSION] ")
			}
			fmt.Fprintf(&rec, "%s\n", normalize(strings.TrimRight(raw, "\n")))
			continue
		}
		fmt.Fprintf(&rec, "%d %s\n", status, normalize(raw))
	}
	checkGolden(t, "mcp", golden, mcpVersion(rec.String()), "\x00", 0)
}

func lookupJSON(v any, key string) any {
	if m, ok := v.(map[string]any); ok {
		return m[key]
	}
	return nil
}

// TestMCPHTTPGolden: the MCP server's HTTP mode (with and without
// --mcp-only and an API key).
func TestMCPHTTPGolden(t *testing.T) {
	repos, stub := mcpFixture(t)
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, repos, "$REPOS"), stub, "$STUB")
	}
	init := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}},"id":1}`
	call := func(name string, id int) string {
		return `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"` + name + `","arguments":{"question":"Why?"}},"id":` + strconv.Itoa(id) + `}`
	}
	list := `{"jsonrpc":"2.0","method":"tools/list","id":9}`
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
			{"POST", "/mcp", list, "", ""},
			{"POST", "/mcp", init, "", ""},
			{"POST", "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "", ""},
			{"POST", "/mcp", call("ask_web_architect", 2), "", ""},
			{"POST", "/mcp", call("ask_WEB_ARCHITECT", 3), "", ""},
			{"POST", "/mcp", call("ask_web_broken", 4), "", ""},
			{"POST", "/mcp", call("ask_web_ghost", 5), "", ""},
			{"POST", "/mcp", `not json`, "", ""},
			{"POST", "/mcp", ``, "", ""},
			{"POST", "/mcp", list, "", "Accept: application/json"},
			{"POST", "/mcp", list, "", "Mcp-Session-Id: no-such-session"},
			{"POST", "/v1/ask", `{"repo":"web","role":"product","question":" Who? ","verbose":true}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"broken","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"WEB","role":"product","question":"q"}`, "", ""},
			{"POST", "/v1/ask", `{"repo":"web","question":"q"}`, "", ""},
			{"POST", "/v1/reset/web/architect", "", "", ""},
			{"PUT", "/mcp", "{}", "", ""},
			{"PUT", "/v1/health", "{}", "", ""},
			{"DELETE", "/mcp", "", "", ""},
			{"POST", "/mcp", list, "", ""},
		}},
		{"http mcp-only with key", []string{"--mcp-only", "--api-key", "s3cret"}, []httpReq{
			{"GET", "/mcp/schema", "", "", ""},
			{"GET", "/mcp/schema", "", "Bearer s3cret", ""},
			{"GET", "/v1/health", "", "Bearer wrong", ""},
			{"GET", "/v1/health", "", "Basic s3cret", ""},
			{"GET", "/v1/health", "", "Bearer s3cret", ""},
			{"POST", "/v1/ask", `{"repo":"web","role":"product","question":"q"}`, "Bearer s3cret", ""},
			{"POST", "/mcp", init, "", ""},
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
	out, errOut, code := mcpConverse(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), []mcpStep{
		{mcpInit, true},
		{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_dsp_2_8_steward","arguments":{"question":"q"}},"id":2}`, true},
		{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_DSP_2_8_STEWARD","arguments":{"question":"q"}},"id":3}`, true},
	})
	if code != 0 || strings.Count(out, `"text":"answer from dsp_2_8: steward q"`) != 2 {
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
	out, errOut, code := mcpConverse(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos}, env, []mcpStep{
		{mcpInit, true},
		{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_web_product","arguments":{"question":"Who?"}},"id":2}`, true},
	})
	if code != 0 || !strings.Contains(out, `"text":"native answer"`) {
		t.Fatalf("exit %d:\n%s\n%s", code, out, errOut)
	}
	if s, err := os.ReadFile(filepath.Join(repos, "web/agents/product/.session-id")); err != nil || strings.TrimSpace(string(s)) != "s-1" {
		t.Errorf("the native ask did not run in the repo: %q %v", s, err)
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if strings.Contains(sc.Text(), `"id":2`) && (strings.Contains(sc.Text(), `"error"`) || strings.Contains(sc.Text(), `"isError"`)) {
			t.Errorf("tools/call failed: %s", sc.Text())
		}
	}
}

// TestMCPClientGolden: `rebar mcp client` (the SDK's Streamable HTTP
// client) against the HTTP MCP server prints what bin/test-mcp-client
// (python, removed in rebar 3.2) printed.
func TestMCPClientGolden(t *testing.T) {
	repos, stub := mcpFixture(t)
	port := freePort(t)
	startHTTP(t, []string{testBinary(t), "mcp", "serve", "--port", strconv.Itoa(port), "--host", "127.0.0.1", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), port)
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	goOut, goErr, goCode := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), "mcp", "client", url)
	if goCode != 0 {
		t.Fatalf("exit %d:\n%s%s", goCode, goOut, goErr)
	}
	checkGolden(t, "mcp", "client", mcpVersion(strings.ReplaceAll(goOut, repos, "$REPOS")), "\x00none\x00", goCode)
	usageOut, _, usageCode := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), "mcp", "client")
	if usageCode != 1 || !strings.HasPrefix(usageOut, "Usage: test-mcp-client <server-url>") {
		t.Errorf("usage: exit %d\n%s", usageCode, usageOut)
	}
}
