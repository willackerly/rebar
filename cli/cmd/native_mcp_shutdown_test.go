//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/willackerly/rebar/cli/internal/mcp"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Shutdown of `rebar mcp serve`: when the client goes away (end of input)
// or the server gets SIGINT/SIGTERM while a tool call runs, the call is
// cancelled, the server exits 0 within mcp.ShutdownGrace (plus a margin
// for slow CI machines — what matters is that it is not the 300 s ask
// timeout), and the ask process and what it started are gone.

// blockingAsk is an ask that never answers: it starts a child (the stand-in
// for claude or git), records both pids under $MCP_PIDDIR/<question>, and
// waits. With $MCP_TRAP set both ignore SIGTERM, so only SIGKILL ends them.
const blockingAsk = `#!/bin/sh
if [ -n "$MCP_TRAP" ]; then trap '' TERM; fi
q=$(echo "$2" | tr -c 'a-zA-Z0-9\n' _)
sleep 300 &
echo $! > "$MCP_PIDDIR/$q.grandchild"
echo $$ > "$MCP_PIDDIR/$q.child"
wait
`

// shutdownMargin is how much later than mcp.ShutdownGrace the server may
// exit.
const shutdownMargin = 4 * time.Second

// blockingFixture is a repo with agent "slow" and the blocking ask.
func blockingFixture(t *testing.T) (repos, stub, pids string) {
	t.Helper()
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	repos = filepath.Join(base, "repos")
	testutil.WriteFile(t, repos, "svc/agents/slow/AGENT.md", "# Slow\n\nTakes forever.\n")
	stub = testutil.WriteExecutable(t, base, "bin/ask", blockingAsk)
	pids = filepath.Join(base, "pids")
	testutil.WriteFile(t, pids, ".keep", "")
	return repos, stub, pids
}

// askProcs waits for the blocking ask of question and returns its pid and
// its child's.
func askProcs(t *testing.T, pids, question string) (int, int) {
	t.Helper()
	return testutil.ReadPIDFile(t, filepath.Join(pids, question+".child"), 30*time.Second),
		testutil.ReadPIDFile(t, filepath.Join(pids, question+".grandchild"), 30*time.Second)
}

// startServer starts argv with stdin and stdout pipes (stdout lines on the
// channel).
func startServer(t *testing.T, argv, env []string) (*exec.Cmd, io.WriteCloser, <-chan string, *syncBuffer) {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	stderr := &syncBuffer{}
	cmd.Dir, cmd.Env, cmd.Stderr = t.TempDir(), env, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(nil, 16<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	return cmd, stdin, lines, stderr
}

// syncBuffer is a bytes.Buffer safe to read while a process writes it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitExit waits for cmd within limit and returns its exit code and how
// long it took from start.
func waitExit(t *testing.T, cmd *exec.Cmd, start time.Time, limit time.Duration, stderr fmt.Stringer) (int, time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return code, time.Since(start)
	case <-time.After(limit):
		cmd.Process.Kill()
		<-done
		t.Fatalf("server still running %v after shutdown began:\n%s", limit, stderr)
		return 0, 0
	}
}

func TestMCPStdioShutdown(t *testing.T) {
	for _, tc := range []struct {
		name string
		sig  syscall.Signal // 0: end of input
		trap bool
	}{
		{"end of input", 0, false},
		{"end of input, ask ignores SIGTERM", 0, true},
		{"SIGTERM", syscall.SIGTERM, true},
		{"SIGINT", syscall.SIGINT, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repos, stub, pids := blockingFixture(t)
			env := mcpEnv(t, "MCP_PIDDIR="+pids)
			if tc.trap {
				env = append(env, "MCP_TRAP=1")
			}
			cmd, stdin, lines, stderr := startServer(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub}, env)
			send := func(msg string) {
				if _, err := io.WriteString(stdin, msg+"\n"); err != nil {
					t.Fatal(err)
				}
			}
			send(mcpInit)
			select {
			case l := <-lines:
				if !strings.Contains(l, `"id":1,"result"`) {
					t.Fatalf("initialize: %s", l)
				}
			case <-time.After(30 * time.Second):
				t.Fatalf("no initialize answer:\n%s", stderr)
			}
			send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
			send(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_svc_slow","arguments":{"question":"block"}},"id":2}`)
			child, grandchild := askProcs(t, pids, "block")

			start := time.Now()
			if tc.sig == 0 {
				stdin.Close()
			} else {
				cmd.Process.Signal(tc.sig)
			}
			code, took := waitExit(t, cmd, start, mcp.ShutdownGrace+30*time.Second, stderr)
			if code != 0 {
				t.Errorf("exit %d:\n%s", code, stderr)
			}
			if took > mcp.ShutdownGrace+shutdownMargin {
				t.Errorf("exit took %v (grace %v)", took, mcp.ShutdownGrace)
			}
			var answer string
			for l := range lines {
				if strings.Contains(l, `"id":2`) {
					answer = l
				}
			}
			if !strings.Contains(answer, `"isError":true`) || !strings.Contains(answer, "Agent query cancelled") {
				t.Errorf("the blocked call's answer: %q\nstderr:\n%s", answer, stderr)
			}
			if want := "cancelling 1 running request(s)"; !strings.Contains(stderr.String(), want) {
				t.Errorf("stderr lacks %q:\n%s", want, stderr)
			}
			if tc.sig != 0 && !strings.Contains(stderr.String(), "signal received: shutting down") {
				t.Errorf("stderr lacks the signal:\n%s", stderr)
			}
			testutil.WaitGone(t, "ask", child, 2*time.Second)
			testutil.WaitGone(t, "ask's child", grandchild, 2*time.Second)
		})
	}
}

// TestMCPHTTPShutdown: SIGTERM while a /v1/ask request and an MCP
// tools/call (Streamable HTTP session) run.
func TestMCPHTTPShutdown(t *testing.T) {
	t.Parallel()
	repos, stub, pids := blockingFixture(t)
	port := freePort(t)
	env := mcpEnv(t, "MCP_PIDDIR="+pids, "MCP_TRAP=1")
	cmd := exec.Command(testBinary(t), "mcp", "serve", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--repos-dir", repos, "--ask-bin", stub)
	logs := &syncBuffer{}
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = t.TempDir(), env, logs, logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	for deadline := time.Now().Add(30 * time.Second); ; {
		if resp, err := http.Get(base + "/v1/health"); err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start:\n%s", logs)
		}
		time.Sleep(50 * time.Millisecond)
	}

	classic := make(chan string, 1)
	go func() {
		resp, err := http.Post(base+"/v1/ask", "application/json", strings.NewReader(`{"repo":"svc","role":"slow","question":"classic"}`))
		if err != nil {
			classic <- err.Error()
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		classic <- fmt.Sprintf("%d %s", resp.StatusCode, body)
	}()
	ctx := context.Background()
	session, err := sdk.NewClient(&sdk.Implementation{Name: "rebar-test", Version: "1"}, nil).
		Connect(ctx, &sdk.StreamableClientTransport{Endpoint: base + "/mcp", MaxRetries: -1}, nil)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, logs)
	}
	defer session.Close()
	tool := make(chan string, 1)
	go func() {
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: "ask_svc_slow", Arguments: map[string]any{"question": "mcp"}})
		if err != nil {
			tool <- "error: " + err.Error()
			return
		}
		tool <- fmt.Sprintf("isError=%v %s", res.IsError, firstTextContent(res))
	}()
	child1, grandchild1 := askProcs(t, pids, "classic")
	child2, grandchild2 := askProcs(t, pids, "mcp")

	start := time.Now()
	cmd.Process.Signal(syscall.SIGTERM)
	code, took := waitExit(t, cmd, start, mcp.ShutdownGrace+30*time.Second, logs)
	if code != 0 {
		t.Errorf("exit %d:\n%s", code, logs)
	}
	if took > mcp.ShutdownGrace+shutdownMargin {
		t.Errorf("exit took %v (grace %v)", took, mcp.ShutdownGrace)
	}
	if !strings.Contains(logs.String(), "ask-mcp-server: shutting down") {
		t.Errorf("no shutdown line:\n%s", logs)
	}
	// The cancelled calls answer before the server goes (or, if the
	// connection closes first, the client sees that) — either way promptly.
	for name, ch := range map[string]chan string{"/v1/ask": classic, "tools/call": tool} {
		select {
		case got := <-ch:
			if strings.HasPrefix(got, "200 ") || strings.HasPrefix(got, "isError=false") {
				t.Errorf("%s answered as if it had finished: %s", name, got)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("%s still waiting after the server exited", name)
		}
	}
	for what, pid := range map[string]int{"/v1/ask's ask": child1, "its child": grandchild1, "tools/call's ask": child2, "its child ": grandchild2} {
		testutil.WaitGone(t, what, pid, 2*time.Second)
	}
}
