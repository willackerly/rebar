//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package cmd

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/mcp"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Findings S9-2..S9-4, S9-7, S9-10 of the v4.0.0 review, against the
// binary over stdio.

// expectLine waits for the next stdout line.
func expectLine(t *testing.T, lines <-chan string, stderr interface{ String() string }) string {
	t.Helper()
	select {
	case l, ok := <-lines:
		if !ok {
			t.Fatalf("the server exited:\n%s", stderr.String())
		}
		return l
	case <-time.After(30 * time.Second):
		t.Fatalf("no answer:\n%s", stderr.String())
	}
	return ""
}

// TestMCPStdioRobustness: an over-long line (S9-3) is dropped and the
// session goes on; code-0 errors get JSON-RPC codes, a too-deeply nested
// request gets a parse error, and an id beyond 2^53 is refused (S9-10);
// a repo directory named "pct%zz" (S9-2) is served with escaped URIs.
func TestMCPStdioRobustness(t *testing.T) {
	repos, stub := mcpFixture(t)
	testutil.WriteFile(t, repos, "pct%zz/agents/architect/AGENT.md", "# A\n")
	testutil.WriteFile(t, repos, "pct%zz/agents/architect/memory.md", "pct memory\n")
	cmd, stdin, lines, stderr := startServer(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t))
	send := func(msg string) {
		if _, err := io.WriteString(stdin, msg+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	send(mcpInit)
	if l := expectLine(t, lines, stderr); !strings.Contains(l, `"id":1,"result"`) {
		t.Fatalf("initialize: %s", l)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	// 17 MiB without and then with a newline: dropped, the session goes on.
	big := strings.Repeat("x", 17<<20)
	io.WriteString(stdin, big[:8<<20])
	send(big[8<<20:])
	for _, tc := range []struct{ send, want string }{
		{`{"jsonrpc":"2.0","method":"ping","id":2}`, `{"jsonrpc":"2.0","id":2,"result":{}}`},
		{mcpInit[:len(mcpInit)-2] + `3}`, `{"jsonrpc":"2.0","id":3,"error":{"code":-32600,"message":"duplicate \"initialize\" received"}}`},
		{`{"jsonrpc":"2.0","method":"ping","id":4,"params":{"x":` + strings.Repeat("[", 1500) + strings.Repeat("]", 1500) + `}}`, `{"jsonrpc":"2.0","id":4,"error":{"code":-32700,"message":"Parse error: JSON nested too deeply"}}`},
		{`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"ask://memory/pct%25zz:architect"},"id":5}`, `"text":"pct memory\n"`},
		{`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_pct%zz_architect","arguments":{"question":"q"}},"id":6}`, `answer from pct%zz: --question -- architect q`},
	} {
		send(tc.send)
		if l := expectLine(t, lines, stderr); !strings.Contains(l, tc.want) {
			t.Errorf("%.80s: got %.300s, want %s", tc.send, l, tc.want)
		}
	}
	send(`{"jsonrpc":"2.0","method":"ping","id":9007199254740993}`)
	send(`{"jsonrpc":"2.0","method":"ping","id":7}`)
	if l := expectLine(t, lines, stderr); l != `{"jsonrpc":"2.0","id":7,"result":{}}` {
		t.Errorf("after the big id: %s", l)
	}
	stdin.Close()
	if code, _ := waitExit(t, cmd, time.Now(), 30*time.Second, stderr); code != 0 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{"longer than 16777216 bytes): 17825793 bytes", "id beyond ±2^53"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
}

// TestMCPCancelThenReuseID (S9-7): a cancelled call's id stays in use until
// its (cancelled) answer is written; reusing it before then is refused and
// logged, after it the new call is answered under that id.
func TestMCPCancelThenReuseID(t *testing.T) {
	repos, stub, pids := blockingFixture(t)
	cmd, stdin, lines, stderr := startServer(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub},
		mcpEnv(t, "MCP_PIDDIR="+pids, "MCP_TRAP=1"))
	send := func(msg string) {
		if _, err := io.WriteString(stdin, msg+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	send(mcpInit)
	expectLine(t, lines, stderr)
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_svc_slow","arguments":{"question":"first"}},"id":2}`)
	child, _ := askProcs(t, pids, "first")
	send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":2}}`)
	send(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_svc_slow","arguments":{"question":"too_early"}},"id":2}`)
	// The ask ignores SIGTERM: its answer comes after KillDelay.
	if l := expectLine(t, lines, stderr); !strings.Contains(l, `"id":2`) || !strings.Contains(l, "Agent query cancelled") {
		t.Fatalf("cancelled call: %s", l)
	}
	testutil.WaitGone(t, "cancelled ask", child, 5*time.Second)
	if !strings.Contains(stderr.String(), "(id of a request still running)") {
		t.Errorf("the early reuse was not refused:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(pids, "too_early.child")); err == nil {
		t.Error("the early reuse ran")
	}
	send(`{"jsonrpc":"2.0","method":"ping","id":2}`)
	if l := expectLine(t, lines, stderr); l != `{"jsonrpc":"2.0","id":2,"result":{}}` {
		t.Errorf("id 2 reused after its answer: %s", l)
	}
	stdin.Close()
	waitExit(t, cmd, time.Now(), 30*time.Second, stderr)
}

// TestMCPStdoutClosed (S9-4): the client closes stdin and stdout (and
// stderr) while a call whose ask ignores SIGTERM runs: the server must not
// die of SIGPIPE before it has killed the ask and its child.
func TestMCPStdoutClosed(t *testing.T) {
	repos, stub, pids := blockingFixture(t)
	cmd := exec.Command(testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub)
	cmd.Dir, cmd.Env = t.TempDir(), mcpEnv(t, "MCP_PIDDIR="+pids, "MCP_TRAP=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	cmd.Stdout, cmd.Stderr = outW, errW
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	outW.Close()
	errW.Close()
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	io.WriteString(stdin, mcpInit+"\n")
	if l, err := bufio.NewReader(outR).ReadString('\n'); err != nil || !strings.Contains(l, `"id":1`) {
		t.Fatalf("initialize: %q %v", l, err)
	}
	io.WriteString(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
	io.WriteString(stdin, `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"ask_svc_slow","arguments":{"question":"block"}},"id":2}`+"\n")
	child, grandchild := askProcs(t, pids, "block")
	outR.Close()
	errR.Close()
	stdin.Close()
	start := time.Now()
	waitExit(t, cmd, start, mcp.ShutdownGrace+30*time.Second, &testutil.SyncBuffer{})
	if ws := cmd.ProcessState.String(); strings.Contains(ws, "signal") {
		t.Errorf("server: %s", ws)
	}
	testutil.WaitGone(t, "ask", child, 3*time.Second)
	testutil.WaitGone(t, "ask's child", grandchild, 3*time.Second)
}

// TestServeBindRefused: a non-loopback --host without --api-key exits 2
// unless --insecure-no-auth (both servers).
func TestServeBindRefused(t *testing.T) {
	repos, _ := mcpFixture(t)
	for _, argv := range [][]string{{"mcp", "serve"}, {"ask", "serve"}} {
		_, errOut, code := runSplit(t, t.TempDir(), mcpEnv(t), "", testBinary(t), append(argv, "--host", "0.0.0.0", "--port", "0", "--repos-dir", repos)...)
		if code != 2 || !strings.Contains(errOut, "refusing to listen on non-loopback address '0.0.0.0' without an API key") {
			t.Errorf("%v: exit %d\n%s", argv, code, errOut)
		}
	}
}

// TestShimsSkipForeignBinary (S6-6): a bin/rebar beside the shims that
// does not run here (another platform's build) is skipped for REBAR_BIN,
// then PATH, instead of failing with "exec format error".
func TestShimsSkipForeignBinary(t *testing.T) {
	dir := t.TempDir()
	// An ELF header with nothing behind it: exec fails on every platform.
	if err := os.WriteFile(filepath.Join(dir, "rebar"), []byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"), 0o755); err != nil {
		t.Fatal(err)
	}
	real := testutil.WriteExecutable(t, t.TempDir(), "rebar", "#!/bin/sh\necho \"real rebar: $*\"\n")
	for _, shim := range []string{"ask", "ask-agent-loop", "ask-mcp-server", "ask-server"} {
		data, err := os.ReadFile(filepath.Join(testutil.RebarSource(t), "bin", shim))
		if err != nil {
			t.Fatal(err)
		}
		path := testutil.WriteExecutable(t, dir, shim, string(data))
		out, errOut, code := runSplit(t, dir, append(mcpEnv(t), "REBAR_BIN="+real), "", path, "x")
		if code != 0 || !strings.HasPrefix(out, "real rebar: ") || !strings.HasSuffix(strings.TrimSpace(out), " x") {
			t.Errorf("%s: exit %d: %s%s", shim, code, out, errOut)
		}
	}
}
