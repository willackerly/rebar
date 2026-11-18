package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AskServer serves bin/ask-server's HTTP API (`rebar ask serve`):
//
//	GET  /v1/health, /v1/agents, /v1/repos
//	POST /v1/ask                    {repo, role, question, verbose?}
//	GET  /v1/status/<repo>/<role>, /v1/log/<repo>/<role>
//	POST /v1/reset/<repo>/<role>
type AskServer struct {
	Registry *Registry
	APIKey   string
	AskCmd   []string
	// Log gets the one-line request log ("  POST /v1/ask → 200"); ErrLog
	// the details of failures, which callers are not shown.
	Log, ErrLog io.Writer
	// MaxAsks caps the asks (and status/reset commands) running at once
	// (0: no cap); more get 503.
	MaxAsks int

	stop     stopper
	asks     limiter
	asksOnce sync.Once
}

// Stop cancels the running commands (shutdown).
func (h *AskServer) Stop() { h.stop.Stop() }

// logf writes a failure's details to ErrLog.
func (h *AskServer) logf(format string, args ...any) {
	if h.ErrLog != nil {
		fmt.Fprintf(h.ErrLog, "ask-server: "+format+"\n", args...)
	}
}

func (h *AskServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &statusRecorder{ResponseWriter: w, status: 200}
	defer func() {
		if h.Log != nil {
			fmt.Fprintf(h.Log, "  %s %s → %d\n", r.Method, r.RequestURI, rec.status)
		}
	}()
	w = rec
	if !guardHost(w, r) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		unsupported(w, r)
		return
	}
	if h.APIKey != "" && !keyMatches(r.Header.Get("Authorization"), "Bearer "+h.APIKey) {
		jsonError(w, 401, "Unauthorized. Provide Authorization: Bearer <key>")
		return
	}
	if !guardBody(w, r) {
		return
	}
	path := pathOf(r)
	parts := strings.Split(path, "/")
	if r.Method == http.MethodGet {
		switch {
		case path == "/v1/health":
			jsonOK(w, object{{"status", "ok"}, {"repos", h.Registry.Len()}})
		case path == "/v1/agents":
			jsonOK(w, h.Registry.agentsJSON())
		case path == "/v1/repos":
			jsonOK(w, h.Registry.reposJSON())
		case strings.HasPrefix(path, "/v1/status/"):
			if len(parts) != 5 {
				jsonError(w, 404, "Expected /v1/status/<repo>/<role>")
				return
			}
			h.status(w, parts[3], parts[4])
		case strings.HasPrefix(path, "/v1/log/"):
			if len(parts) != 5 {
				jsonError(w, 404, "Expected /v1/log/<repo>/<role>")
				return
			}
			h.log(w, parts[3], parts[4])
		default:
			jsonError(w, 404, "Not found")
		}
		return
	}
	switch {
	case path == "/v1/ask":
		h.ask(w, r)
	case strings.HasPrefix(path, "/v1/reset/"):
		if len(parts) != 5 {
			jsonError(w, 404, "Expected /v1/reset/<repo>/<role>")
			return
		}
		h.reset(w, parts[3], parts[4])
	default:
		jsonError(w, 404, "Not found")
	}
}

func (h *AskServer) shown() string { return strings.Join(h.AskCmd, " ") }

// errBusy: every command slot is taken (MaxAsks).
var errBusy = errors.New("busy")

// run runs the ask command with args in the repo.
func (h *AskServer) run(ctx context.Context, path string, timeout time.Duration, args ...string) (string, string, int, error) {
	h.asksOnce.Do(func() { h.asks = newLimiter(h.MaxAsks) })
	release, ok := h.asks.acquire()
	if !ok {
		return "", "", 0, errBusy
	}
	defer release()
	ctx, cancel := h.stop.with(ctx)
	defer cancel()
	return runAskContext(ctx, append(append([]string{}, h.AskCmd...), args...), path, timeout)
}

// failed answers a command that could not run: busy (503), or 500 with a
// short message (the details go to ErrLog).
func (h *AskServer) failed(w http.ResponseWriter, what string, err error) {
	switch {
	case errors.Is(err, errBusy):
		busy(w, h.MaxAsks)
	case errors.Is(err, context.Canceled):
		h.logf("%s: cancelled", what)
		jsonError(w, 503, "Server shutting down")
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		h.logf("%s: ask CLI not found at '%s': %v", what, h.shown(), err)
		jsonError(w, 500, "ask CLI not found. Set ASK_BIN or ensure ask is in PATH.")
	default:
		h.logf("%s: %v", what, err)
		jsonError(w, 500, "Internal error (see the server log)")
	}
}

func (h *AskServer) ask(w http.ResponseWriter, r *http.Request) {
	body, ok := readJSON(w, r)
	if !ok {
		return
	}
	repo, role, question, verbose, ok := askFields(w, body)
	if !ok {
		return
	}
	path := h.Registry.Path(repo)
	if path == "" {
		jsonError(w, 404, fmt.Sprintf("Repo '%s' not found. Use GET /v1/repos to list available repos.", repo))
		return
	}
	if !h.Registry.HasAgent(repo, role) {
		jsonError(w, 404, fmt.Sprintf("Agent '%s' not found in repo '%s'. Use GET /v1/agents to list available agents.", role, repo))
		return
	}
	var args []string
	if verbose {
		args = append(args, "-v")
	}
	stdout, stderr, code, err := h.run(r.Context(), path, AskTimeout, append(args, askArgs(role, question)...)...)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		jsonError(w, 504, fmt.Sprintf("Agent '%s:%s' timed out (%ds)", repo, role, int(AskTimeout.Seconds())))
		return
	case err != nil:
		h.failed(w, "ask "+repo+":"+role, err)
		return
	}
	var errField any
	if code != 0 {
		// The ask's own last error line; the rest of its output (paths,
		// command lines) stays in the server log.
		h.logf("ask %s:%s exited %d:\n%s", repo, role, code, strings.TrimRight(stderr, "\n"))
		errField = lastLine(stderr, path)
	}
	jsonOK(w, object{{"repo", repo}, {"role", role}, {"answer", pyStrip(stdout)}, {"exit_code", code}, {"error", errField}})
}

// known answers 404 unless repo has the agent role.
func (h *AskServer) known(w http.ResponseWriter, repo, role string) (string, bool) {
	path := h.Registry.Path(repo)
	if path == "" {
		jsonError(w, 404, fmt.Sprintf("Repo '%s' not found", repo))
		return "", false
	}
	if !h.Registry.HasAgent(repo, role) {
		jsonError(w, 404, fmt.Sprintf("Agent '%s' not found in repo '%s'", role, repo))
		return "", false
	}
	return path, true
}

func (h *AskServer) status(w http.ResponseWriter, repo, role string) {
	path, ok := h.known(w, repo, role)
	if !ok {
		return
	}
	stdout, _, _, err := h.run(context.Background(), path, 10*time.Second, "status", role)
	if err != nil {
		h.failed(w, "status "+repo+":"+role, err)
		return
	}
	out := object{{"repo", repo}, {"role", role}}
	for _, line := range strings.Split(pyStrip(stdout), "\n") {
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		out = setField(out, strings.ToLower(pyStrip(key)), pyStrip(val))
	}
	jsonOK(w, out)
}

// setField is a python dict assignment: replace in place or append.
func setField(o object, key string, val any) object {
	for i := range o {
		if o[i].key == key {
			o[i].val = val
			return o
		}
	}
	return append(o, field{key, val})
}

func (h *AskServer) log(w http.ResponseWriter, repo, role string) {
	path, ok := h.known(w, repo, role)
	if !ok {
		return
	}
	content := "(no log entries yet)"
	if f := filepath.Join(path, "agents", role, "memory.log.md"); isFile(f) {
		text, err := readText(f)
		if err != nil {
			h.logf("log %s:%s: %v", repo, role, err)
			jsonError(w, 500, "Internal error (see the server log)")
			return
		}
		content = text
	}
	jsonOK(w, object{{"repo", repo}, {"role", role}, {"log", content}})
}

func (h *AskServer) reset(w http.ResponseWriter, repo, role string) {
	path, ok := h.known(w, repo, role)
	if !ok {
		return
	}
	stdout, _, _, err := h.run(context.Background(), path, 10*time.Second, "reset", role)
	if err != nil {
		h.failed(w, "reset "+repo+":"+role, err)
		return
	}
	jsonOK(w, object{{"repo", repo}, {"role", role}, {"message", pyStrip(stdout)}})
}
