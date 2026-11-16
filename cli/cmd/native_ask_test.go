package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// Parity of the native ASK (`rebar ask`, internal/ask) with bin/ask, the
// bash original: every case runs three ways on a fresh copy of its tree —
// bin/ask under REBAR_CHECK_IMPL=shell, `rebar ask` (this test binary),
// and bin/ask handing off to rebar (the shim path adopters' scripts take)
// — and the three records must agree: exit code, stdout, stderr, the
// files written under the project and HOME, every claude invocation (a
// stub claude on PATH logs its arguments, stdin and working directory and
// plays back canned output), and the requests a remote-mode case sends to
// its test ASK servers. Timestamps, message ids, session UUIDs and the
// temp paths are normalized. The records are golden files in
// testdata/ask (regenerate with go test ./cmd -run AskParity
// -update-golden); a case that documents a deliberate deviation
// (diverges) takes its golden from the native side.

// askCase is one ASK invocation and the tree it runs in.
type askCase struct {
	name string
	// files make up the project (the working directory); $BASE in them is
	// the case's temp root (the project is $BASE/proj, HOME $BASE/home).
	files map[string]string
	// home files go below HOME (.config/ask/projects is the registry).
	home map[string]string
	// claude files configure the stub claude: claude-out, claude-err and
	// claude-exit (with an optional .N suffix for the Nth call).
	claude map[string]string
	args   []string
	env    []string
	stdin  string
	// setup runs on each fresh tree.
	setup func(t *testing.T, base string)
	// servers is how many test ASK servers to start ($SERVER1… in env).
	servers int
	// noClaude runs with a PATH that has no claude.
	noClaude bool
	// diverges documents a deliberate deviation: the native record is the
	// golden one; the script's is only logged.
	diverges string
	// divergesOn is "linux" or "darwin" for a platform-specific script
	// behaviour: the native side follows the other platform, whose script
	// output is the golden one.
	divergesOn string
	// needs lists tools the script side needs beyond bash and jq.
	needs []string
}

// defaultClaude answers like `claude --print --output-format json`.
const defaultClaude = `{"type":"result","subtype":"success","result":"The answer is 42.\nIt is computed in src/calc.go.","session_id":"0b6f4a52-1111-4222-8333-944455556666","modelUsage":{"claude-sonnet":{"inputTokens":1200,"outputTokens":80,"cacheReadInputTokens":3000,"cacheCreationInputTokens":500,"contextWindow":200000}}}
`

// stubClaude is the claude on PATH in the parity tests.
const stubClaude = `#!/bin/sh
d=$(dirname "$0")
n=$(cat "$d/claude.count" 2>/dev/null || echo 0)
n=$((n+1))
echo "$n" > "$d/claude.count"
{
  echo "=== claude call $n (cwd: $(pwd))"
  for a in "$@"; do printf '[%s]\n' "$a"; done
  echo "--- stdin:"
  cat
} >> "$d/claude.log"
out="$d/claude-out.$n"; [ -f "$out" ] || out="$d/claude-out"
[ -f "$out" ] && cat "$out"
err="$d/claude-err.$n"; [ -f "$err" ] || err="$d/claude-err"
[ -f "$err" ] && cat "$err" >&2
c="$d/claude-exit.$n"; [ -f "$c" ] || c="$d/claude-exit"
code=0; [ -f "$c" ] && code=$(cat "$c")
exit "$code"
`

// askTree builds a case's tree under a new temp root and returns it.
func askTree(t *testing.T, tc askCase) string {
	t.Helper()
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	for _, d := range []string{"proj", "home", "bin"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for rel, body := range tc.files {
		testutil.WriteFile(t, filepath.Join(base, "proj"), rel, strings.ReplaceAll(body, "$BASE", base))
	}
	for rel, body := range tc.home {
		testutil.WriteFile(t, filepath.Join(base, "home"), rel, strings.ReplaceAll(body, "$BASE", base))
	}
	if !tc.noClaude {
		testutil.WriteExecutable(t, base, "bin/claude", stubClaude)
		claude := map[string]string{"claude-out": defaultClaude}
		for k, v := range tc.claude {
			claude[k] = v
		}
		for k, v := range claude {
			testutil.WriteFile(t, filepath.Join(base, "bin"), k, v)
		}
	}
	if tc.setup != nil {
		tc.setup(t, base)
	}
	return base
}

// snapshot maps every directory ("dir/") and file below base (but the
// stub's bin/ and tools/) to its contents.
func snapshot(t *testing.T, base string) map[string]string {
	t.Helper()
	state := map[string]string{}
	tops, _ := os.ReadDir(base)
	for _, e := range tops {
		top := e.Name()
		if top == "bin" || top == "tools" || !e.IsDir() {
			continue
		}
		root := filepath.Join(base, top)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(base, p)
			rel = filepath.ToSlash(rel)
			switch {
			case d.IsDir():
				state[rel+"/"] = ""
			case d.Type().IsRegular():
				data, _ := os.ReadFile(p)
				state[rel] = string(data)
			case d.Type()&fs.ModeSymlink != 0:
				target, _ := os.Readlink(p)
				state[rel] = "-> " + target
			}
			return nil
		})
	}
	return state
}

// changes lists what differs between two snapshots.
func changes(before, after map[string]string) string {
	var paths []string
	for p, v := range after {
		if old, ok := before[p]; !ok || old != v {
			paths = append(paths, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		now, ok := after[p]
		switch {
		case !ok:
			b.WriteString("--- removed " + p + " ---\n")
		case strings.HasSuffix(p, "/"):
			b.WriteString("--- made " + p + " ---\n")
		default:
			b.WriteString("--- wrote " + p + " ---\n" + now)
			if now != "" && !strings.HasSuffix(now, "\n") {
				b.WriteString("\n[no newline at end]\n")
			}
		}
	}
	return b.String()
}

// runSplit runs name in dir with env and stdin and returns its stdout,
// stderr and exit code.
func runSplit(t *testing.T, dir string, env []string, stdin, name string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var err error
	for try := 0; ; try++ {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdin = strings.NewReader(stdin)
		stdout.Reset()
		stderr.Reset()
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err = cmd.Run()
		// Linux: a script just written may still be open for writing in a
		// child another parallel test is forking (golang/go#22315).
		if !errors.Is(err, syscall.ETXTBSY) || try == 50 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("%s: %v", name, err)
		}
		code = ee.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

// askEnv is the environment every run gets: no ASK_*/REBAR_* settings from
// the developer, HOME and PATH of the case, a dumb terminal of 80 columns,
// the C locale.
func askEnv(t *testing.T, base, path string) []string {
	var env []string
	for _, kv := range testutil.IsolatedEnv() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(k, "ASK_"), k == "HOME", k == "PATH", k == "NO_COLOR", k == "COLUMNS",
			k == "LINES", k == "TERM", k == "PAGER", k == "PWD", strings.HasPrefix(k, "LC_"), k == "LANG":
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+filepath.Join(base, "home"), "PATH="+path, "TERM=dumb", "LC_ALL=C", "PYTHONDONTWRITEBYTECODE=1",
		"REBAR_ROOT="+testutil.RebarSource(t))
	return env
}

// toolsOnly is a PATH of symlinks to the tools the script needs, without
// claude.
func toolsOnly(t *testing.T, base string) string {
	t.Helper()
	dir := filepath.Join(base, "tools")
	for _, name := range []string{"bash", "sh", "jq", "uuidgen", "tr", "sed", "awk", "grep", "cat", "mkdir",
		"date", "head", "cut", "xargs", "basename", "dirname", "find", "wc", "sort", "tput", "fmt", "rm",
		"ls", "realpath", "stat", "sleep", "touch", "tail", "readlink", "env", "python3", "curl"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.Symlink(p, filepath.Join(dir, name))
	}
	return dir
}

// askServers are a case's test ASK servers: canned answers, a log of the
// requests.
type askServers struct {
	mu   sync.Mutex
	log  []string
	urls []string
	srvs []*httptest.Server
}

func startAskServers(t *testing.T, n int) *askServers {
	s := &askServers{}
	for i := 1; i <= n; i++ {
		i := i
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.log = append(s.log, fmt.Sprintf("server%d %s %s auth=%q type=%q\n%s", i, r.Method, r.URL.Path,
				r.Header.Get("Authorization"), r.Header.Get("Content-Type"), body))
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			path := r.URL.Path
			switch {
			case path == "/v1/agents" && i == 1:
				io.WriteString(w, `[{"repo":"web","role":"architect","id":"web:architect"},{"repo":"web","role":"product","id":"web:product"}]`)
			case path == "/v1/agents":
				io.WriteString(w, `[{"repo":"api","role":"architect","id":"api:architect"},{"repo":"web","role":"architect","id":"web:architect"}]`)
			case path == "/v1/ask":
				switch {
				case bytes.Contains(body, []byte(`"repo": "missing"`)):
					w.WriteHeader(404)
					io.WriteString(w, `{"error": "Repo 'missing' not found. Use GET /v1/repos to list available repos."}`)
				case bytes.Contains(body, []byte(`"role": "elsewhere"`)) && i == 1:
					io.WriteString(w, `{"error": "Agent 'elsewhere' not found in repo 'web'"}`)
				case bytes.Contains(body, []byte(`"role": "broken"`)):
					io.WriteString(w, `{"repo": "web", "role": "broken", "answer": "", "exit_code": 2, "error": "ask: error: agent 'broken' not found"}`)
				default:
					fmt.Fprintf(w, "{\n  \"repo\": \"web\",\n  \"role\": \"architect\",\n  \"answer\": \"Remote answer from server%d.\",\n  \"exit_code\": 0,\n  \"error\": null\n}", i)
				}
			case strings.HasPrefix(path, "/v1/status/"):
				io.WriteString(w, `{"repo": "web", "role": "architect", "status": "running", "inbox": "0 pending", "pid": 77}`)
			case strings.HasPrefix(path, "/v1/log/"):
				io.WriteString(w, `{"repo": "web", "role": "architect", "log": "---\n**2026-01-01T00:00:00Z** Q: hi\nA: hello"}`)
			case strings.HasPrefix(path, "/v1/reset/"):
				io.WriteString(w, `{"repo": "web", "role": "architect", "message": "Session cleared for architect (was: abc)"}`)
			default:
				w.WriteHeader(404)
				io.WriteString(w, `{"error": "Not found"}`)
			}
		}))
		s.srvs = append(s.srvs, srv)
		s.urls = append(s.urls, strings.TrimPrefix(srv.URL, "http://"))
	}
	t.Cleanup(func() {
		for _, srv := range s.srvs {
			srv.Close()
		}
	})
	return s
}

func (s *askServers) take() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := strings.Join(s.log, "\n")
	s.log = nil
	return out
}

// deadAddr is an address nothing listens on.
func deadAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

var (
	askTS     = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`)
	askMsgID  = regexp.MustCompile(`ask-\d{8}-\d{6}-\d+-\d+`)
	askUUID   = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	askAgo    = regexp.MustCompile(`\d+s ago`)
	askOnline = regexp.MustCompile(`\(pid \d+\)`)
)

// askRun runs one way of a case and returns its normalized record.
func askRun(t *testing.T, tc askCase, how string, srv *askServers, dead string) (string, int) {
	t.Helper()
	base := askTree(t, tc)
	proj := filepath.Join(base, "proj")
	path := filepath.Join(base, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	if tc.noClaude {
		path = toolsOnly(t, base)
	}
	env := askEnv(t, base, path)
	repl := []string{"$BASE", base, "$DEAD", dead}
	for i, u := range srvURLs(srv) {
		repl = append(repl, fmt.Sprintf("$SERVER%d", i+1), u)
	}
	expand := strings.NewReplacer(repl...)
	for _, kv := range tc.env {
		env = append(env, expand.Replace(kv))
	}
	args := make([]string, len(tc.args))
	for i, a := range tc.args {
		args[i] = expand.Replace(a)
	}
	before := snapshot(t, base)
	var stdout, stderr string
	var code int
	switch how {
	case "script":
		script := filepath.Join(testutil.RebarSource(t), "bin", "ask")
		stdout, stderr, code = runSplit(t, proj, append(env, "REBAR_CHECK_IMPL=shell"), tc.stdin, "bash", append([]string{script}, args...)...)
	case "native":
		stdout, stderr, code = runSplit(t, proj, append(env, "REBAR_TEST_RUN_CLI=1"), tc.stdin, testBinary(t), append([]string{"ask"}, args...)...)
	case "shim":
		shimDir := t.TempDir()
		data, err := os.ReadFile(filepath.Join(testutil.RebarSource(t), "bin", "ask"))
		if err != nil {
			t.Fatal(err)
		}
		shim := testutil.WriteExecutable(t, shimDir, "ask", string(data))
		log := filepath.Join(shimDir, "hand-offs.log")
		testutil.WriteExecutable(t, shimDir, "rebar", "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+log+"'\nREBAR_TEST_RUN_CLI=1 exec '"+testBinary(t)+"' \"$@\"\n")
		stdout, stderr, code = runSplit(t, proj, env, tc.stdin, shim, args...)
		if logged, _ := os.ReadFile(log); !strings.HasPrefix(string(logged), "ask") {
			t.Errorf("bin/ask did not hand off to rebar ask")
		}
	}
	claudeLog, _ := os.ReadFile(filepath.Join(base, "bin", "claude.log"))
	rec := "--- stdout ---\n" + stdout + "--- stderr ---\n" + stderr
	if len(claudeLog) > 0 {
		rec += "--- claude ---\n" + string(claudeLog)
	}
	rec += "--- changes ---\n" + changes(before, snapshot(t, base))
	if srv != nil {
		rec += "--- requests ---\n" + srv.take() + "\n"
	}
	norm := []string{base, "$BASE", dead, "$DEAD", "pid " + strconv.Itoa(os.Getpid()), "pid $TESTPID",
		"PID: " + strconv.Itoa(os.Getpid()), "PID: $TESTPID"}
	for i, u := range srvURLs(srv) {
		norm = append(norm, u, fmt.Sprintf("$SERVER%d", i+1))
	}
	rec = strings.NewReplacer(norm...).Replace(rec)
	rec = askTS.ReplaceAllString(rec, "<TS>")
	rec = askMsgID.ReplaceAllString(rec, "<MSGID>")
	rec = askUUID.ReplaceAllString(rec, "<UUID>")
	rec = askAgo.ReplaceAllString(rec, "<N>s ago")
	return rec, code
}

func srvURLs(s *askServers) []string {
	if s == nil {
		return nil
	}
	return s.urls
}

// noRoot is a golden root that never occurs: the records are already
// normalized ($BASE).
const noRoot = "\x00no-root\x00"

// runAskParity runs the cases.
func runAskParity(t *testing.T, cases []askCase) {
	t.Helper()
	requireBash(t)
	for _, tool := range []string{"jq", "uuidgen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("bin/ask needs %s", tool)
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // each run has its own trees, servers and stub claude
			for _, tool := range tc.needs {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("needs %s", tool)
				}
			}
			var srv *askServers
			if tc.servers > 0 {
				srv = startAskServers(t, tc.servers)
			}
			dead := deadAddr(t)
			script, scriptCode := askRun(t, tc, "script", srv, dead)
			native, nativeCode := askRun(t, tc, "native", srv, dead)
			shim, shimCode := askRun(t, tc, "shim", srv, dead)

			want, wantCode, from := script, scriptCode, "the script"
			platformDiverges := tc.divergesOn != "" && tc.divergesOn == hostOS()
			if tc.diverges != "" || platformDiverges {
				want, wantCode, from = native, nativeCode, "native"
				if script == native && scriptCode == nativeCode {
					t.Logf("documented divergence did not show here (%s%s)", tc.diverges, tc.divergesOn)
				} else {
					t.Logf("the script diverges, as documented (%s%s); it gave (exit %d):\n%s", tc.diverges, tc.divergesOn, scriptCode, script)
				}
			}
			checkGolden(t, "ask", tc.name, want, noRoot, wantCode)
			for _, got := range []struct {
				how  string
				rec  string
				code int
			}{{"native", native, nativeCode}, {"shim", shim, shimCode}} {
				if got.rec != want || got.code != wantCode {
					t.Errorf("%s differs from %s:\n%s (exit %d):\n%s\n%s (exit %d):\n%s", got.how, from, from, wantCode, want, got.how, got.code, got.rec)
				}
			}
		})
	}
}

func hostOS() string {
	out, _ := exec.Command("uname", "-s").Output()
	return strings.ToLower(strings.TrimSpace(string(out)))
}
