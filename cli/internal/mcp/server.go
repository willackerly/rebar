// Package mcp is the native (Go) implementation of ASK's servers
// (CONTRACT:S3-MCP-SERVER.1.0): the MCP server bin/ask-mcp-server (`rebar
// mcp serve`: MCP over stdio or Streamable HTTP, with the classic ASK API
// alongside), the enterprise server bin/ask-server (`rebar ask serve`) and
// the demo client bin/test-mcp-client (`rebar mcp client`).
//
// The MCP protocol layer is the official Go SDK
// (github.com/modelcontextprotocol/go-sdk): it negotiates the protocol
// version, frames and validates messages, and reports tool failures as
// isError results. This package adds the ASK tools and resources, the
// case-insensitive tool-name and resource aliases (compat.go) and the stdio
// hygiene Claude Code relies on (stdio.go). The classic ASK API and
// ask-server keep the python originals' HTTP endpoints and json.dumps
// bodies.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is serverInfo.version: rebar's version (cmd sets it).
var Version = "dev"

const (
	serverName        = "ask-mcp-server"
	serverDescription = "MCP server for ASK agent coordination platform"
)

// AskTimeout bounds one tools/call (the python server's 300 s).
var AskTimeout = 300 * time.Second

// Server is the ASK side of the MCP server: the repos it hosts and the ask
// command its tools run.
type Server struct {
	Registry *Registry
	// AskCmd is the ask command a tools/call runs, before its arguments:
	// [ASK_BIN] or [rebar, "ask"].
	AskCmd []string
	// Remote: callers are remote (HTTP), so failures are reported briefly
	// and their details written to Log.
	Remote bool
	Log    io.Writer
	// MaxAsks caps the asks running at once (0: no cap); more are
	// refused as busy.
	MaxAsks int

	stop     stopper
	asks     limiter
	asksOnce sync.Once
}

// acquire takes an ask slot (see MaxAsks).
func (s *Server) acquire() (func(), bool) {
	s.asksOnce.Do(func() { s.asks = newLimiter(s.MaxAsks) })
	return s.asks.acquire()
}

// inputSchema is every tool's input schema (unchanged since the python
// server; the SDK validates arguments against it).
var inputSchema = json.RawMessage(`{"type": "object", "properties": {` +
	`"question": {"type": "string", "description": "Question to ask the agent"}, ` +
	`"verbose": {"type": "boolean", "description": "Request verbose response with rationale", "default": false}}, ` +
	`"required": ["question"]}`)

// askInput is a tools/call's arguments, validated against inputSchema.
type askInput struct {
	Question string `json:"question"`
	Verbose  bool   `json:"verbose"`
}

// ToolName is the MCP tool of an agent: ask_<repo>_<role>.
func ToolName(a Agent) string { return "ask_" + strings.ReplaceAll(a.ID(), ":", "_") }

// resourceKinds are the resources of each agent: ask://<kind>/<repo>:<role>.
var resourceKinds = []struct{ kind, name, description string }{
	{"memory", "Memory", "Current distilled memory for %[2]s agent in %[1]s"},
	{"log", "Log", "Full interaction history for %[2]s agent in %[1]s"},
	{"agent", "Definition", "Agent role definition and configuration for %[2]s in %[1]s"},
}

// NewMCPServer builds the MCP server: a tool per agent, three resources
// per agent, and the alias middleware. The registry is read once, so the
// tool list is fixed for the server's lifetime (listChanged: false). Repo
// and role names are path-escaped in resource URIs (a repo directory named
// "pct%zz" is ask://memory/pct%25zz:architect); an agent the SDK still
// refuses is left out with a warning, and an SDK panic is returned as an
// error rather than taking the server down.
func (s *Server) NewMCPServer() (srv *sdk.Server, err error) {
	defer func() {
		if r := recover(); r != nil {
			srv, err = nil, fmt.Errorf("building the MCP server: %v", r)
		}
	}()
	srv = sdk.NewServer(&sdk.Implementation{Name: serverName, Version: Version, Description: serverDescription},
		&sdk.ServerOptions{Capabilities: &sdk.ServerCapabilities{
			Tools:     &sdk.ToolCapabilities{},
			Resources: &sdk.ResourceCapabilities{},
		}})
	for _, a := range s.Registry.Agents() {
		if err := s.addAgent(srv, a); err != nil {
			if s.Registry.Warn != nil {
				fmt.Fprintf(s.Registry.Warn, "%s: warning: skipping agent %s: %v\n", serverName, a.ID(), err)
			}
		}
	}
	srv.AddReceivingMiddleware(s.aliases)
	return srv, nil
}

// resourceURI is ask://<kind>/<repo>:<role>, repo and role path-escaped.
func resourceURI(kind string, a Agent) string {
	return "ask://" + kind + "/" + url.PathEscape(a.Repo) + ":" + url.PathEscape(a.Role)
}

// addAgent adds a's tool and resources (an SDK panic becomes an error).
func (s *Server) addAgent(srv *sdk.Server, a Agent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	for _, k := range resourceKinds {
		if _, err := url.Parse(resourceURI(k.kind, a)); err != nil {
			return err
		}
	}
	sdk.AddTool(srv, &sdk.Tool{Name: ToolName(a), Description: s.description(a.Repo, a.Role), InputSchema: inputSchema}, s.askTool(a))
	for _, k := range resourceKinds {
		srv.AddResource(&sdk.Resource{
			URI:         resourceURI(k.kind, a),
			Name:        a.ID() + " " + k.name,
			Description: fmt.Sprintf(k.description, a.Repo, a.Role),
			MIMEType:    "text/markdown",
		}, s.readResource(k.kind, a))
	}
	return nil
}

// askTool runs `ask <role> <question>` in the agent's repo. Failures are
// tool execution errors (isError results the model can read), not
// protocol errors.
func (s *Server) askTool(a Agent) sdk.ToolHandlerFor[askInput, any] {
	return func(ctx context.Context, _ *sdk.CallToolRequest, in askInput) (*sdk.CallToolResult, any, error) {
		if strings.TrimSpace(in.Question) == "" {
			return nil, nil, errors.New("Question required")
		}
		release, ok := s.acquire()
		if !ok {
			return nil, nil, errors.New(busyMessage(s.MaxAsks))
		}
		defer release()
		answer, errText := s.ExecuteAsk(ctx, a.Repo, a.Role, in.Question, in.Verbose)
		if errText != "" {
			return nil, nil, errors.New(errText)
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: answer}}}, nil, nil
	}
}

// readResource reads one of the agent's files (a placeholder when it does
// not exist yet).
func (s *Server) readResource(kind string, a Agent) sdk.ResourceHandler {
	return func(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
		content, err := s.resource(kind, a.Repo, a.Role)
		if err != nil {
			return nil, err
		}
		return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{{URI: req.Params.URI, MIMEType: "text/markdown", Text: content}}}, nil
	}
}

// readText is Path.read_text(): the file decoded as UTF-8 with universal
// newlines.
func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("'utf-8' codec can't decode %s", filepath.Base(path))
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n"), nil
}

func (s *Server) resource(typ, repo, role string) (string, error) {
	path := s.Registry.Path(repo)
	if path == "" {
		return "", fmt.Errorf("Repository %s not found", repo)
	}
	dir := filepath.Join(path, "agents", role)
	read := func(name, missing string) (string, error) {
		if !isFile(filepath.Join(dir, name)) {
			return missing, nil
		}
		return readText(filepath.Join(dir, name))
	}
	switch typ {
	case "memory":
		return read("memory.md", "(no memory yet)")
	case "log":
		return read("memory.log.md", "(no log entries yet)")
	case "agent":
		return read("AGENT.md", "# Agent "+role+"\n\n(no agent definition found)")
	}
	return "", fmt.Errorf("Unknown resource type: %s", typ)
}

// roleDescriptions are the caller-facing preambles of known roles.
var roleDescriptions = map[string]string{
	"architect": "Architect agent for {repo}. Owns system design, contract definitions, " +
		"component boundaries, and architectural trade-offs. " +
		"Best for: 'why' questions about design choices, integration decisions, " +
		"cross-repo interface choices, and contract-level reasoning. " +
		"Prefer over grep when the question is intent-shaped rather than location-shaped.",
	"product": "Product agent for {repo}. Owns user personas, requirements, priorities, " +
		"and scope decisions. " +
		"Best for: 'who is this for?', 'what does success look like?', " +
		"'does this match user needs?', and forward-from-intent reasoning.",
	"englead": "Engineering Lead agent for {repo}. Owns implementation strategy, " +
		"test approach, build/deploy mechanics, and codebase navigation. " +
		"Best for: 'where do I start?', 'how do I run X?', 'what test tier?', " +
		"and operational questions. Default for ENG-style human questions.",
	"steward": "Steward agent for {repo}. Runs automated quality scans, tracks " +
		"contract lifecycle, detects drift, and reports compliance posture. " +
		"Best for: 'is the codebase healthy?', 'which contracts need attention?', " +
		"'what's drifting?'. Returns structured summaries computed from current state.",
	"merger": "Merger agent for {repo}. Coordinates merges, post-fanout integration, " +
		"conflict resolution, and cherry-pick ordering. " +
		"Best for: 'how do I land N parallel agent branches?', 'safe merge order?', " +
		"'is this conflict mechanical or semantic?'.",
	"tester": "Tester agent for {repo}. Owns test strategy, regression analysis, " +
		"rigor evaluation, and tier-appropriate coverage. " +
		"Best for: 'is my test actually proving what its name claims?', " +
		"'what negative controls?', 'is this passing-but-narrow?'.",
	"engineer": "Engineer agent for {repo}. Scoped to implementation within an " +
		"assigned contract — the most narrowly bounded of the agents. " +
		"Best for: 'help me implement this contract', 'what's in scope vs out?', " +
		"'is this change inside my contract?'.",
	"featurerequest": "Feature-request intake agent for {repo}. Gated MCP contribution " +
		"channel for callers without git write access. Receives typed " +
		"missing-feature asks, scores them against CHARTER.md (§3 hard " +
		"gates: in-scope, not blocked, concrete use case, novel), and " +
		"either files a structured FR-*.md artifact or returns a precise " +
		"rejection reason. " +
		"Best for: 'we hit X gap doing Y in our project, please track it'. " +
		"NOT for: open-ended discussion, design proposals, or methodology " +
		"questions — fork the repo for those (CHARTER §4).",
}

// agentVoice is the python pattern for sentences addressed to the agent.
var agentVoice = regexp.MustCompile(`(?i)^(You are\b[^.]*[.!?](?:\s*[—-]\s*[^.]*[.!?])?` +
	`|You (own|make decisions about|represent|are responsible for|do not|don't|receive|run|report)\b[^.]*[.!?])\s*`)

// description is the caller-facing tool description of repo:role.
func (s *Server) description(repo, role string) string {
	preamble := "Query the " + role + " agent for " + repo + "."
	if t, ok := roleDescriptions[role]; ok {
		preamble = strings.ReplaceAll(t, "{repo}", repo)
	}
	path := s.Registry.Path(repo)
	if path == "" {
		return preamble
	}
	md := filepath.Join(path, "agents", role, "AGENT.md")
	if !isFile(md) {
		return preamble
	}
	content, err := readText(md)
	if err != nil {
		return preamble
	}
	var para []string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			if para != nil {
				break
			}
			continue
		}
		para = append(para, line)
	}
	if para == nil {
		return preamble
	}
	prose := strings.Join(para, " ")
	for {
		loc := agentVoice.FindStringIndex(prose)
		if loc == nil {
			break
		}
		prose = strings.TrimSpace(prose[loc[1]:])
	}
	if prose != "" {
		return preamble + " Repo specifics: " + prose
	}
	return preamble
}

// pyStrip is str.strip().
func pyStrip(s string) string { return strings.TrimSpace(s) }

// ExecuteAsk runs `ask [-v] <role> <question>` in the repo (ASK_SERVER
// cleared, so the call never routes to a remote server) and returns the
// answer, or the error text for the caller. It runs `ask [-v] --question
// -- <role> <question>`: whatever the question says, it is a question,
// never an agent command. role must be one of the repo's agents.
//
// A local (stdio) caller gets the details of a failure: the ask's output,
// exit code and command line. A remote (HTTP) caller gets a short error;
// the details go to the server log.
func (s *Server) ExecuteAsk(ctx context.Context, repo, role, question string, verbose bool) (string, string) {
	path := s.Registry.Path(repo)
	if path == "" {
		return "", "Repository " + repo + " not found"
	}
	if !s.Registry.HasAgent(repo, role) {
		return "", "Agent " + role + " not found in repository " + repo
	}
	argv := append([]string{}, s.AskCmd...)
	if verbose {
		argv = append(argv, "-v")
	}
	argv = append(argv, askArgs(role, question)...)
	shown := strings.Join(argv, " ")
	ctx, cancel := s.withStop(ctx)
	defer cancel()
	stdout, stderr, code, err := runAskContext(ctx, argv, path, AskTimeout)
	detail, short := "", ""
	switch {
	case errors.Is(err, context.Canceled):
		detail, short = fmt.Sprintf("Agent query cancelled (cmd: %s)", shown), "Agent query cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		detail = fmt.Sprintf("Agent query timed out after %ds (cmd: %s)", int(AskTimeout.Seconds()), shown)
		short = fmt.Sprintf("Agent query timed out after %ds", int(AskTimeout.Seconds()))
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		detail = fmt.Sprintf("ask binary not found: [Errno 2] No such file or directory: '%s' (looked at: %s)", argv[0], argv[0])
		short = "ask binary not found (see the server log)"
	case err != nil:
		detail, short = fmt.Sprintf("Execution error: %v", err), "Execution error (see the server log)"
	case code == 0:
		return pyStrip(stdout), ""
	default:
		var lines []string
		if stderr != "" {
			lines = append(lines, strings.TrimRight(stderr, " \t\n\r\v\f"))
		}
		if stdout != "" {
			lines = append(lines, "(stdout) "+strings.TrimRight(stdout, " \t\n\r\v\f"))
		}
		lines = append(lines, fmt.Sprintf("(exit code: %d)", code), fmt.Sprintf("(cmd: %s cwd=%s)", shown, path))
		detail = strings.Join(lines, "\n")
		short = fmt.Sprintf("Agent %s:%s failed (exit code %d)", repo, role, code)
		if last := lastLine(stderr, path); last != "" {
			short += ": " + last
		}
	}
	if !s.Remote {
		return "", detail
	}
	if s.Log != nil {
		fmt.Fprintf(s.Log, "%s: ask %s:%s failed: %s\n", serverName, repo, role, strings.ReplaceAll(detail, "\n", "\n    "))
	}
	return "", short
}

// askArgs are the arguments that ask role question, question-only (as the
// v3.1.2 python servers ask).
func askArgs(role, question string) []string {
	return []string{"--question", "--", role, question}
}

// lastLine is the last non-empty line of an ask's stderr (its "ask:
// error: …"), at most 200 bytes, with the repo's path hidden: what a
// remote caller is told about a failure.
func lastLine(stderr, repoPath string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if repoPath != "" {
		last = strings.ReplaceAll(last, repoPath, "<repo>")
	}
	if len(last) > 200 {
		last = strings.ToValidUTF8(last[:200], "") + "…"
	}
	return last
}

// runAskContext runs argv in dir with ASK_SERVER cleared and returns its output and exit status (negative
// for a signal, as python reports it), or context.Canceled when parent is
// done first (a client cancelled the request, or the server is shutting
// down). The command runs in its own process group; ending it early sends
// the group SIGTERM, then SIGKILL after KillDelay, and kills what is left of
// the group (a backend that ignored the signal) once the command has
// exited. What a finished command leaves running in its group (rebar ask's
// background memory compaction) is killed when the server exits, or
// LingerLimit after the command, whichever comes first (settle).
func runAskContext(parent context.Context, argv []string, dir string, timeout time.Duration) (string, string, int, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(withoutEnv(os.Environ(), "ASK_SERVER", "PWD"), "ASK_SERVER=", "PWD="+dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	inOwnGroup(cmd)
	cmd.WaitDelay = KillDelay
	err := cmd.Start()
	if err == nil {
		pid := cmd.Process.Pid
		addChild(pid)
		err = cmd.Wait()
		settle(pid, ctx.Err() != nil)
		if errors.Is(err, exec.ErrWaitDelay) {
			// It exited 0; something it left running holds its output open.
			err = nil
		}
	}
	if parent.Err() != nil {
		return "", "", 0, context.Canceled
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", "", 0, context.DeadlineExceeded
	}
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return "", "", 0, err
		}
		code := ee.ExitCode()
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			code = -int(ws.Signal())
		}
		return stdout.String(), stderr.String(), code, nil
	}
	return stdout.String(), stderr.String(), 0, nil
}

// withoutEnv is env without the named variables.
func withoutEnv(env []string, names ...string) []string {
	var out []string
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, n := range names {
			if k == n {
				drop = true
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}
