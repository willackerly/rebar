//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// httpFixture is a registry with repo "web" (agent architect) and a stub
// ask (body) for both HTTP servers.
func httpFixture(t *testing.T, askBody string) (*Registry, []string, string) {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFile(t, root, "web/agents/architect/AGENT.md", "# Architect\n")
	stub := testutil.WriteExecutable(t, root, "ask", askBody)
	reg := NewRegistry("t", io.Discard, io.Discard, true)
	reg.AddRepo("web", filepath.Join(root, "web"))
	return reg, []string{stub}, root
}

const echoAsk = "#!/bin/sh\necho \"answer: $*\"\n"

// servers starts both HTTP servers on loopback listeners.
func servers(t *testing.T, askBody string, maxAsks int) map[string]*httptest.Server {
	t.Helper()
	reg, askCmd, _ := httpFixture(t, askBody)
	s := &Server{Registry: reg, AskCmd: askCmd, Remote: true, Log: io.Discard, MaxAsks: maxAsks}
	mh, err := NewMCPHandler(s, "", false, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ah := &AskServer{Registry: reg, AskCmd: askCmd, MaxAsks: maxAsks}
	out := map[string]*httptest.Server{}
	for name, h := range map[string]http.Handler{"mcp": mh, "ask-server": ah} {
		ts := httptest.NewUnstartedServer(h)
		ts.Config = newHTTPServer(h)
		ts.Start()
		t.Cleanup(ts.Close)
		out[name] = ts
	}
	return out
}

// raw sends a raw HTTP/1.1 request and returns the status line.
func raw(t *testing.T, ts *httptest.Server, req string) string {
	t.Helper()
	c, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(10 * time.Second))
	io.WriteString(c, req)
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return "error: " + err.Error()
	}
	return strings.TrimSpace(line)
}

func post(t *testing.T, ts *httptest.Server, path, ctype, body string, hdr ...string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(body))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	for i := 0; i+1 < len(hdr); i += 2 {
		if hdr[i] == "Host" {
			req.Host = hdr[i+1]
		} else {
			req.Header.Set(hdr[i], hdr[i+1])
		}
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data), resp.Header
}

const askBody = `{"repo":"web","role":"architect","question":"q"}`

// TestRequestGuards: body size (Content-Length or chunked), content type,
// cross-origin and Host checks, on every path of both servers.
func TestRequestGuards(t *testing.T) {
	big := `{"repo":"web","role":"architect","question":"` + strings.Repeat("a", int(MaxBodyBytes)) + `"}`
	for name, ts := range servers(t, echoAsk, 0) {
		t.Run(name, func(t *testing.T) {
			// S6-3: a huge Content-Length is refused before any read (it
			// used to be allocated: out of memory, server gone).
			for _, path := range []string{"/v1/ask", "/mcp"} {
				if got := raw(t, ts, "POST "+path+" HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Type: application/json\r\nContent-Length: 140000000000000\r\n\r\n{"); got != "HTTP/1.1 413 Request Entity Too Large" {
					t.Errorf("%s huge Content-Length: %s", path, got)
				}
			}
			// Chunked: /v1/ask needs a Content-Length (411, as in v3.1.2);
			// /mcp (the SDK) takes chunked bodies up to the cap.
			for path, want := range map[string]int{"/v1/ask": 411, "/mcp": 413} {
				if name == "ask-server" && path == "/mcp" {
					continue
				}
				req, _ := http.NewRequest(http.MethodPost, ts.URL+path, io.MultiReader(strings.NewReader(big)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				req.ContentLength = -1
				resp, err := ts.Client().Do(req)
				if err != nil {
					t.Errorf("%s chunked: %v", path, err)
					continue
				}
				resp.Body.Close()
				if resp.StatusCode != want {
					t.Errorf("%s chunked body over the cap: %d, want %d", path, resp.StatusCode, want)
				}
			}
			if got := raw(t, ts, "POST /v1/ask HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Type: application/json\r\nContent-Length: 1048577\r\n\r\n{"); got != "HTTP/1.1 413 Request Entity Too Large" {
				t.Errorf("Content-Length over the cap: %s", got)
			}
			for _, tc := range []struct {
				what, path, ctype string
				hdr               []string
				want              int
			}{
				{"text/plain", "/v1/ask", "text/plain", nil, 415},
				{"no content type", "/v1/ask", "", nil, 415},
				{"form", "/v1/ask", "application/x-www-form-urlencoded", nil, 415},
				{"json with charset", "/v1/ask", "application/json; charset=utf-8", nil, 200},
				{"cross-site fetch", "/v1/ask", "application/json", []string{"Sec-Fetch-Site", "cross-site"}, 403},
				{"foreign origin", "/v1/ask", "application/json", []string{"Origin", "https://evil.example"}, 403},
				{"rebinding host", "/v1/ask", "application/json", []string{"Host", "evil.example"}, 403},
				{"rebinding host /mcp", "/mcp", "application/json", []string{"Host", "evil.example:80"}, 403},
				{"localhost", "/v1/ask", "application/json", []string{"Host", "localhost:7232"}, 200},
			} {
				code, body, _ := post(t, ts, tc.path, tc.ctype, askBody, tc.hdr...)
				if code != tc.want {
					t.Errorf("%s: %d %s, want %d", tc.what, code, body, tc.want)
				}
			}
			// GETs get the Host check too.
			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/health", nil)
			req.Host = "evil.example"
			if resp, err := ts.Client().Do(req); err != nil || resp.StatusCode != 403 {
				t.Errorf("GET with a rebinding Host: %v %v", resp, err)
			} else {
				resp.Body.Close()
			}
		})
	}
}

// TestBodyDeadline: a body that does not arrive within BodyReadTimeout is
// cut off, and the deadline is lifted once the body is read, so an ask
// longer than it still answers.
func TestBodyDeadline(t *testing.T) {
	defer func(d time.Duration) { BodyReadTimeout = d }(BodyReadTimeout)
	BodyReadTimeout = 300 * time.Millisecond
	for name, ts := range servers(t, "#!/bin/sh\nsleep 1\necho slow answer\n", 0) {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			got := raw(t, ts, "POST /v1/ask HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"re")
			if d := time.Since(start); d > 5*time.Second || strings.Contains(got, " 200 ") {
				t.Errorf("stalled body: %q after %v", got, d)
			}
			code, body, _ := post(t, ts, "/v1/ask", "application/json", askBody)
			if code != 200 || !strings.Contains(body, "slow answer") {
				t.Errorf("an ask longer than the body deadline: %d %s", code, body)
			}
		})
	}
}

// TestMaxAsks: asks beyond --max-asks are refused at once (503 with
// Retry-After; an isError result for MCP tools).
func TestMaxAsks(t *testing.T) {
	dir := t.TempDir()
	block := "#!/bin/sh\ntouch '" + dir + "'/$$.started\nsleep 2\necho done\n"
	for name, ts := range servers(t, block, 1) {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				if code, body, _ := post(t, ts, "/v1/ask", "application/json", askBody); code != 200 {
					t.Errorf("first ask: %d %s", code, body)
				}
			}()
			for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
				if m, _ := filepath.Glob(filepath.Join(dir, "*.started")); len(m) > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the first ask did not start")
				}
			}
			code, body, hdr := post(t, ts, "/v1/ask", "application/json", askBody)
			if code != 503 || hdr.Get("Retry-After") == "" || !strings.Contains(body, "Server busy") {
				t.Errorf("second ask: %d %s %v", code, body, hdr)
			}
			wg.Wait()
			matches, _ := filepath.Glob(filepath.Join(dir, "*.started"))
			for _, m := range matches {
				os.Remove(m)
			}
		})
	}
	reg, askCmd, _ := httpFixture(t, block)
	s := &Server{Registry: reg, AskCmd: askCmd, MaxAsks: 1}
	release, _ := s.acquire()
	defer release()
	_, _, err := s.askTool(Agent{"web", "architect"})(context.Background(), nil, askInput{Question: "q"})
	if err == nil || !strings.Contains(err.Error(), "Server busy") {
		t.Errorf("tools/call when busy: %v", err)
	}
}

// TestCheckBind: a non-loopback address needs an API key or the explicit
// --insecure-no-auth.
func TestCheckBind(t *testing.T) {
	for _, tc := range []struct {
		o  options
		ok bool
	}{
		{options{host: "127.0.0.1"}, true},
		{options{host: "::1"}, true},
		{options{host: "localhost"}, true},
		{options{host: "0.0.0.0"}, false},
		{options{host: ""}, false},
		{options{host: "192.168.1.5"}, false},
		{options{host: "0.0.0.0", apiKey: "k"}, true},
		{options{host: "0.0.0.0", insecureNoAuth: true}, true},
	} {
		if err := checkBind("t", &tc.o, io.Discard); (err == nil) != tc.ok {
			t.Errorf("%+v: %v", tc.o, err)
		}
	}
}

// TestSessionsReaped (S9-1): idle Streamable HTTP sessions are closed after
// SessionTimeout, and at most MaxSessions are open (the reviewer's 20,000
// initializes grew the server by 220 MB, never released).
func TestSessionsReaped(t *testing.T) {
	defer func(d time.Duration, n int) { SessionTimeout, MaxSessions = d, n }(SessionTimeout, MaxSessions)
	SessionTimeout, MaxSessions = 300*time.Millisecond, 1<<30
	reg, askCmd, _ := httpFixture(t, echoAsk)
	h, err := NewMCPHandler(&Server{Registry: reg, AskCmd: askCmd}, "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	defer ts.Close()
	initialize := func() int {
		code, body, _ := post(t, ts, "/mcp", "application/json", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"x","version":"1"}}}`)
		if code != 200 && code != 503 {
			t.Fatalf("initialize: %d %s", code, body)
		}
		return code
	}
	const n = 2000
	for i := 0; i < n; i++ {
		initialize()
	}
	if open := h.sessions(); open == 0 || open > n {
		t.Errorf("%d sessions open after %d initializes", open, n)
	}
	waitSessions(t, h, 0, 10*time.Second)

	MaxSessions = 5
	for i := 0; i < 5; i++ {
		if code := initialize(); code != 200 {
			t.Fatalf("initialize %d: %d", i, code)
		}
	}
	if code := initialize(); code != 503 {
		t.Errorf("initialize beyond MaxSessions: %d", code)
	}
	waitSessions(t, h, 0, 10*time.Second)
	if code := initialize(); code != 200 {
		t.Errorf("initialize after the idle sessions closed: %d", code)
	}
}

func waitSessions(t *testing.T, h *MCPHandler, want int, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for h.sessions() != want {
		if time.Now().After(deadline) {
			t.Fatalf("%d sessions open, want %d", h.sessions(), want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestEscapedResourceURIs (S9-2): a repo directory whose name is not a
// valid URI path segment ("pct%zz") used to panic at start-up (stdio) or
// in the first /mcp request; its URIs are path-escaped now.
func TestEscapedResourceURIs(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "pct%zz/agents/architect/AGENT.md", "# A\n")
	testutil.WriteFile(t, root, "pct%zz/agents/architect/memory.md", "remembered\n")
	reg := NewRegistry("t", io.Discard, io.Discard, true)
	reg.ScanDir(root)
	s := &Server{Registry: reg, AskCmd: []string{"true"}}
	if _, err := NewMCPHandler(s, "", false, nil); err != nil {
		t.Fatal(err)
	}
	srv, err := s.NewMCPServer()
	if err != nil {
		t.Fatal(err)
	}
	if got := resourceURI("memory", Agent{"pct%zz", "architect"}); got != "ask://memory/pct%25zz:architect" {
		t.Errorf("uri = %s", got)
	}
	for in, want := range map[string]string{
		"ask://memory/pct%25zz:architect": "ask://memory/pct%25zz:architect",
		"ask://memory/PCT%25ZZ:Architect": "ask://memory/pct%25zz:architect",
		"ask://memory/pct%zz:architect":   "ask://memory/pct%25zz:architect", // sent unescaped
	} {
		if got := s.canonicalURI(in); got != want {
			t.Errorf("canonicalURI(%s) = %s, want %s", in, got, want)
		}
	}
	_ = srv
}

// TestReadLinesCap (S9-3): a line longer than maxLineLength is dropped
// (reported, never buffered whole) and the input goes on.
func TestReadLinesCap(t *testing.T) {
	defer func(n int) { maxLineLength = n }(maxLineLength)
	maxLineLength = 1024
	in := strings.Repeat("x", 5000) + "\nshort\n" + strings.Repeat("y", 3000) // last: no newline
	var got []string
	for l := range readLines(strings.NewReader(in)) {
		switch {
		case l.err != nil:
			got = append(got, "err")
		case l.dropped > 0:
			got = append(got, fmt.Sprintf("dropped %d", l.dropped))
		default:
			got = append(got, string(l.b))
		}
	}
	if strings.Join(got, "|") != "dropped 5001|short\n|dropped 3000" {
		t.Errorf("lines: %q", got)
	}
	var log strings.Builder
	f := &stdinFilter{lines: readLines(strings.NewReader(strings.Repeat("z", 4096) + "\n" + `{"jsonrpc":"2.0","method":"ping"}` + "\n")),
		log: &log, pending: newPending(), shutdown: func() {}, stopping: make(chan struct{})}
	data, _ := io.ReadAll(f)
	if string(data) != `{"jsonrpc":"2.0","method":"ping"}`+"\n" || !strings.Contains(log.String(), "longer than 1024 bytes): 4097 bytes") {
		t.Errorf("passed %q, log %q", data, log.String())
	}
}

// TestLingeringChildren (S9-8): what a finished ask leaves running in its
// group is killed at the server's exit (KillChildren), or after
// LingerLimit.
func TestLingeringChildren(t *testing.T) {
	defer func(d time.Duration) { LingerLimit = d }(LingerLimit)
	dir := t.TempDir()
	script := filepath.Join(dir, "ask")
	os.WriteFile(script, []byte("#!/bin/sh\nsleep 300 >/dev/null 2>&1 &\necho $! > \"$1.pid\"\necho answered\n"), 0o755)
	run := func(name string) int {
		out, _, code, err := runAskContext(context.Background(), []string{script, filepath.Join(dir, name)}, dir, time.Minute)
		if err != nil || code != 0 || out != "answered\n" {
			t.Fatalf("ask: %q %d %v", out, code, err)
		}
		pid := testutil.ReadPIDFile(t, filepath.Join(dir, name+".pid"), 5*time.Second)
		if !testutil.Alive(pid) {
			t.Fatalf("the background child is already gone")
		}
		return pid
	}
	LingerLimit = time.Minute
	pid := run("a")
	KillChildren()
	testutil.WaitGone(t, "background child at exit", pid, 2*time.Second)

	LingerLimit = 300 * time.Millisecond
	pid = run("b")
	testutil.WaitGone(t, "background child after LingerLimit", pid, 5*time.Second)
}
