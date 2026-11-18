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
	// LoopbackOnly: bound to loopback, so only loopback Host headers.
	LoopbackOnly bool
	// Log gets the one-line request log ("  POST /v1/ask → 200").
	Log io.Writer
}

func (h *AskServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &statusRecorder{ResponseWriter: w, status: 200}
	defer func() {
		if h.Log != nil {
			fmt.Fprintf(h.Log, "  %s %s → %d\n", r.Method, r.RequestURI, rec.status)
		}
	}()
	w = rec
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		unsupported(w, r)
		return
	}
	if !checkHost(w, r, h.LoopbackOnly) {
		return
	}
	if h.APIKey != "" && !apiKeyMatches(r.Header.Get("Authorization"), h.APIKey) {
		jsonError(w, 401, "Unauthorized. Provide Authorization: Bearer <key>")
		return
	}
	if r.Method == http.MethodPost && !checkJSONContentType(w, r) {
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

// run runs the ask command with args in the repo.
func (h *AskServer) run(path string, timeout time.Duration, args ...string) (string, string, int, error) {
	return runAsk(append(append([]string{}, h.AskCmd...), args...), path, timeout)
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
	stdout, stderr, code, err := h.run(path, AskTimeout, askArgs(role, question, verbose)...)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		jsonError(w, 504, fmt.Sprintf("Agent '%s:%s' timed out (%ds)", repo, role, int(AskTimeout.Seconds())))
		return
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		jsonError(w, 500, fmt.Sprintf("ask CLI not found at '%s'. Set ASK_BIN or ensure ask is in PATH.", h.shown()))
		return
	case err != nil:
		jsonError(w, 500, fmt.Sprintf("Internal error: %v", err))
		return
	}
	var errField any
	if code != 0 {
		errField = pyStrip(stderr)
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
	stdout, _, _, err := h.run(path, 10*time.Second, "status", role)
	if err != nil {
		jsonError(w, 500, fmt.Sprintf("Error checking status: %v", err))
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
			jsonError(w, 500, "Internal error: "+err.Error())
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
	stdout, _, _, err := h.run(path, 10*time.Second, "reset", role)
	if err != nil {
		jsonError(w, 500, fmt.Sprintf("Error resetting agent: %v", err))
		return
	}
	jsonOK(w, object{{"repo", repo}, {"role", role}, {"message", pyStrip(stdout)}})
}
