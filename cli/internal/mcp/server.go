// Package mcp is the native (Go) implementation of ASK's servers
// (CONTRACT:S3-MCP-SERVER.1.0): the MCP server bin/ask-mcp-server (`rebar
// mcp serve`: JSON-RPC 2.0 over stdio or HTTP, with the classic ASK API
// alongside), the enterprise server bin/ask-server (`rebar ask serve`) and
// the demo client bin/test-mcp-client (`rebar mcp client`). Tool names,
// schemas, descriptions, resource URIs, error codes, messages and data,
// HTTP endpoints, flags and output are the python originals'; responses
// are written with Python's json.dumps formatting.
//
// It is a minimal JSON-RPC implementation, not the official MCP Go SDK:
// the SDK answers with its own protocol version, error codes and shapes
// (tool failures as isError results, "unknown tool" for the
// case-insensitive aliases this server accepts), which would break the
// parity this port keeps with the deployed server.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// ProtocolVersion is the MCP revision the server speaks.
const ProtocolVersion = "2024-11-05"

// AskTimeout bounds one tools/call (the python server's 300 s).
var AskTimeout = 300 * time.Second

// Server is the MCP protocol state of one session (MCPServer).
type Server struct {
	Registry *Registry
	// AskCmd is the ask command a tools/call runs, before its arguments:
	// [ASK_BIN] or [rebar, "ask"].
	AskCmd []string

	mu          sync.Mutex
	initialized bool
}

// request is a decoded JSON-RPC message.
type request struct {
	raw    map[string]json.RawMessage
	hasID  bool
	id     json.RawMessage
	method any
	params any
	hasPar bool
}

// response builds {"jsonrpc", "id", "result"} in that order.
func response(id json.RawMessage, result any) object {
	return object{{"jsonrpc", "2.0"}, {"id", idValue(id)}, {"result", result}}
}

func idValue(id json.RawMessage) any {
	if id == nil {
		return nil
	}
	return id
}

// rpcError builds a JSON-RPC error response; data is omitted when empty.
func rpcError(code int, message, data string, id json.RawMessage) object {
	e := object{{"code", code}, {"message", message}}
	if data != "" {
		e = append(e, field{"data", data})
	}
	return object{{"jsonrpc", "2.0"}, {"id", idValue(id)}, {"error", e}}
}

// decode parses one message as python's json.loads does (one value).
func decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("Extra data")
	}
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("Extra data")
	}
	return v, nil
}

// pyType is the python type name of a decoded JSON value.
func pyType(v any) string {
	switch x := v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	case json.Number:
		if strings.ContainsAny(string(x), ".eE") {
			return "float"
		}
		return "int"
	}
	return fmt.Sprintf("%T", v)
}

// truthy is python truthiness of a decoded JSON value.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	}
	return true
}

// pyStr is python's str() of a decoded value, for messages.
func pyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case json.Number:
		return x.String()
	}
	return fmt.Sprint(v)
}

// noGet is the AttributeError of calling .get on a non-dict.
func noGet(v any) string {
	return fmt.Sprintf("'%s' object has no attribute 'get'", pyType(v))
}

// Handle answers one decoded message; reply is false for a notification.
func (s *Server) Handle(msg any) (resp object, reply bool) {
	obj, ok := msg.(map[string]any)
	if !ok {
		return rpcError(-32600, "Invalid Request", "Request must be JSON object", nil), true
	}
	if v, ok := obj["jsonrpc"].(string); !ok || v != "2.0" {
		return rpcError(-32600, "Invalid Request", "Must be JSON-RPC 2.0", nil), true
	}
	idv, hasID := obj["id"]
	if !hasID {
		return nil, false // a notification: never answered
	}
	id := rawID(idv)
	method := obj["method"]
	params, hasParams := obj["params"]
	if !hasParams {
		params = map[string]any{}
	}
	switch m, _ := method.(string); {
	case method == "initialize" || m == "initialize":
		return s.initialize(id), true
	case m == "tools/list":
		return s.toolsList(id), true
	case m == "tools/call":
		return s.toolsCall(params, id), true
	case m == "resources/list":
		return s.resourcesList(id), true
	case m == "resources/read":
		return s.resourcesRead(params, id), true
	}
	return rpcError(-32601, "Method not found", "Unknown method: "+pyStr(method), id), true
}

// rawID re-encodes a decoded id for echoing.
func rawID(v any) json.RawMessage {
	return json.RawMessage(pyDumps(v, 0))
}

// HandleBytes answers one raw message (HTTP: an invalid body was already
// rejected).
func (s *Server) HandleBytes(data []byte) (object, bool, error) {
	msg, err := decode(data)
	if err != nil {
		return nil, false, err
	}
	resp, reply := s.Handle(msg)
	return resp, reply, nil
}

func (s *Server) isInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialized
}

func notInitialized(id json.RawMessage) object {
	return rpcError(-32002, "Not initialized", "Call initialize first", id)
}

func (s *Server) initialize(id json.RawMessage) object {
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()
	return response(id, object{
		{"protocolVersion", ProtocolVersion},
		{"capabilities", object{
			{"tools", object{{"listChanged", false}}},
			{"resources", object{{"subscribe", false}, {"listChanged", false}}},
		}},
		{"serverInfo", object{
			{"name", "ask-mcp-server"},
			{"version", "1.0.0"},
			{"description", "MCP server for ASK agent coordination platform"},
		}},
	})
}

// ToolName is the MCP tool of an agent: ask_<repo>_<role>.
func ToolName(a Agent) string { return "ask_" + strings.ReplaceAll(a.ID(), ":", "_") }

func (s *Server) toolsList(id json.RawMessage) object {
	if !s.isInitialized() {
		return notInitialized(id)
	}
	tools := []any{}
	for _, a := range s.Registry.Agents() {
		tools = append(tools, object{
			{"name", ToolName(a)},
			{"description", s.description(a.Repo, a.Role)},
			{"inputSchema", object{
				{"type", "object"},
				{"properties", object{
					{"question", object{{"type", "string"}, {"description", "Question to ask the agent"}}},
					{"verbose", object{{"type", "boolean"}, {"description", "Request verbose response with rationale"}, {"default", false}}},
				}},
				{"required", []any{"question"}},
			}},
		})
	}
	return response(id, object{{"tools", tools}})
}

// splitTool splits a tool name into repo and role. The python server read
// ask_<repo>_<role> by turning every "_" into ":" and splitting at the
// first, so a repo with "_" in its name (dsp-2_8_2) could not be called;
// a registered agent whose tool name matches (ignoring case) is taken
// first, then that reading.
func (s *Server) splitTool(name string) (string, string, bool) {
	for _, a := range s.Registry.Agents() {
		if strings.EqualFold(ToolName(a), name) {
			return a.Repo, a.Role, true
		}
	}
	agentID := strings.ReplaceAll(name[4:], "_", ":")
	repo, role, ok := strings.Cut(agentID, ":")
	return repo, role, ok
}

func (s *Server) toolsCall(params any, id json.RawMessage) object {
	if !s.isInitialized() {
		return notInitialized(id)
	}
	p, ok := params.(map[string]any)
	if !ok {
		return rpcError(-32603, "Internal error", noGet(params), id)
	}
	nameV := p["name"]
	var arguments any = map[string]any{}
	if v, ok := p["arguments"]; ok {
		arguments = v
	}
	if !truthy(nameV) {
		return rpcError(-32602, "Invalid params", "Tool name required", id)
	}
	name, ok := nameV.(string)
	if !ok {
		return rpcError(-32603, "Internal error", fmt.Sprintf("'%s' object has no attribute 'startswith'", pyType(nameV)), id)
	}
	if !strings.HasPrefix(name, "ask_") {
		return rpcError(-32602, "Invalid params", "Invalid tool name format", id)
	}
	repo, role, ok := s.splitTool(name)
	if !ok {
		return rpcError(-32602, "Invalid params", "Invalid agent ID format", id)
	}
	canonRepo := s.Registry.ResolveRepo(repo)
	if canonRepo == "" {
		return rpcError(-32603, "Tool execution failed", "Repository "+repo+" not found", id)
	}
	canonRole := s.Registry.ResolveRole(canonRepo, role)
	if canonRole == "" {
		return rpcError(-32603, "Tool execution failed", "Role "+role+" not found in "+canonRepo, id)
	}
	args, ok := arguments.(map[string]any)
	if !ok {
		return rpcError(-32603, "Internal error", noGet(arguments), id)
	}
	question := args["question"]
	verbose := args["verbose"]
	if !truthy(question) {
		return rpcError(-32602, "Invalid params", "Question required", id)
	}
	answer, errText := s.ExecuteAsk(canonRepo, canonRole, question, truthy(verbose))
	if errText != "" {
		return rpcError(-32603, "Tool execution failed", errText, id)
	}
	return response(id, object{{"content", []any{object{{"type", "text"}, {"text", answer}}}}})
}

func (s *Server) resourcesList(id json.RawMessage) object {
	if !s.isInitialized() {
		return notInitialized(id)
	}
	resources := []any{}
	for _, a := range s.Registry.Agents() {
		aid := a.ID()
		resources = append(resources,
			object{{"uri", "ask://memory/" + aid}, {"name", aid + " Memory"},
				{"description", "Current distilled memory for " + a.Role + " agent in " + a.Repo}, {"mimeType", "text/markdown"}},
			object{{"uri", "ask://log/" + aid}, {"name", aid + " Log"},
				{"description", "Full interaction history for " + a.Role + " agent in " + a.Repo}, {"mimeType", "text/markdown"}},
			object{{"uri", "ask://agent/" + aid}, {"name", aid + " Definition"},
				{"description", "Agent role definition and configuration for " + a.Role + " in " + a.Repo}, {"mimeType", "text/markdown"}})
	}
	return response(id, object{{"resources", resources}})
}

func (s *Server) resourcesRead(params any, id json.RawMessage) object {
	if !s.isInitialized() {
		return notInitialized(id)
	}
	p, ok := params.(map[string]any)
	if !ok {
		return rpcError(-32603, "Internal error", noGet(params), id)
	}
	uriV := p["uri"]
	if !truthy(uriV) {
		return rpcError(-32602, "Invalid params", "Resource URI required", id)
	}
	uri, ok := uriV.(string)
	if !ok {
		return rpcError(-32603, "Internal error", fmt.Sprintf("'%s' object has no attribute 'startswith'", pyType(uriV)), id)
	}
	if !strings.HasPrefix(uri, "ask://") {
		return rpcError(-32602, "Invalid params", "Invalid ASK resource URI", id)
	}
	typ, agentID, ok := strings.Cut(uri[6:], "/")
	if !ok {
		return rpcError(-32603, "Resource read failed", "Invalid URI format", id)
	}
	repo, role, ok := strings.Cut(agentID, ":")
	if !ok {
		return rpcError(-32603, "Resource read failed", "not enough values to unpack (expected 2, got 1)", id)
	}
	if canon := s.Registry.ResolveRepo(repo); canon != "" {
		if r := s.Registry.ResolveRole(canon, role); r != "" {
			repo, role = canon, r
		}
	}
	content, err := s.resource(typ, repo, role)
	if err != nil {
		return rpcError(-32603, "Resource read failed", err.Error(), id)
	}
	return response(id, object{{"contents", []any{object{{"uri", uri}, {"mimeType", "text/markdown"}, {"text", content}}}}})
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
// answer, or the error text the python server reported.
func (s *Server) ExecuteAsk(repo, role string, question any, verbose bool) (string, string) {
	path := s.Registry.Path(repo)
	if path == "" {
		return "", "Repository " + repo + " not found"
	}
	if !s.Registry.HasAgent(repo, role) {
		return "", "Agent " + role + " not found in repository " + repo
	}
	q, ok := question.(string)
	if !ok {
		return "", fmt.Sprintf("Execution error: TypeError('expected str, bytes or os.PathLike object, not %s')", pyType(question))
	}
	argv := append([]string{}, s.AskCmd...)
	if verbose {
		argv = append(argv, "-v")
	}
	argv = append(argv, role, q)
	shown := strings.Join(argv, " ")
	stdout, stderr, code, err := runAsk(argv, path, AskTimeout)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "", fmt.Sprintf("Agent query timed out after %ds (cmd: %s)", int(AskTimeout.Seconds()), shown)
	case errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist):
		return "", fmt.Sprintf("ask binary not found: [Errno 2] No such file or directory: '%s' (looked at: %s)", argv[0], argv[0])
	case err != nil:
		return "", fmt.Sprintf("Execution error: %v", err)
	}
	if code == 0 {
		return pyStrip(stdout), ""
	}
	var lines []string
	if stderr != "" {
		lines = append(lines, strings.TrimRight(stderr, " \t\n\r\v\f"))
	}
	if stdout != "" {
		lines = append(lines, "(stdout) "+strings.TrimRight(stdout, " \t\n\r\v\f"))
	}
	lines = append(lines, fmt.Sprintf("(exit code: %d)", code), fmt.Sprintf("(cmd: %s cwd=%s)", shown, path))
	return "", strings.Join(lines, "\n")
}

// runAsk runs argv in dir with ASK_SERVER cleared and returns its output
// and exit status (negative for a signal, as python reports it).
func runAsk(argv []string, dir string, timeout time.Duration) (string, string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(withoutEnv(os.Environ(), "ASK_SERVER", "PWD"), "ASK_SERVER=", "PWD="+dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
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
