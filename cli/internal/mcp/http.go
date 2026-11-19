package mcp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// The HTTP side of both servers: python's BaseHTTPRequestHandler answered
// HTTP/1.0 with JSON bodies; these handlers send the same statuses and
// bodies.

// pathOf is urlparse(self.path).path.rstrip("/"): the raw (undecoded) path.
func pathOf(r *http.Request) string {
	p := r.RequestURI
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if !strings.HasPrefix(p, "/") {
		p = r.URL.EscapedPath() // absolute-form target
	}
	return strings.TrimRight(p, "/")
}

// writeJSON sends status and body (json.dumps(data, indent=2) for 200s).
func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	io.WriteString(w, body)
}

func jsonOK(w http.ResponseWriter, data any) { writeJSON(w, 200, pyDumps(data, 2)) }

func jsonError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, pyDumps(object{{"error", message}}, 0))
}

// readJSON is _read_json (v3.1.2): a body of at most MaxBodyBytes with a
// valid Content-Length (411 without one, 413 beyond the cap — refused
// before reading, never allocated from the header), parsed as JSON (400
// "Invalid JSON" otherwise).
func readJSON(w http.ResponseWriter, r *http.Request) (any, bool) {
	if len(r.TransferEncoding) > 0 {
		jsonError(w, 411, "Content-Length required (chunked bodies not supported)")
		return nil, false
	}
	if _, err := strconv.Atoi(r.Header.Get("Content-Length")); err != nil {
		jsonError(w, 411, "Valid Content-Length required")
		return nil, false
	}
	if r.ContentLength > MaxBodyBytes {
		tooLargeError(w)
		return nil, false
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		if tooLarge(err) {
			tooLargeError(w)
			return nil, false
		}
		jsonError(w, 400, "Request body shorter than Content-Length")
		return nil, false
	}
	v, err := decode(data)
	if err != nil {
		msg := err.Error()
		if len(data) == 0 {
			msg = "Expecting value: line 1 column 1 (char 0)"
		}
		jsonError(w, 400, "Invalid JSON: "+msg)
		return nil, false
	}
	return v, true
}

// statusRecorder remembers the status of a response, for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush and Unwrap keep the SDK's event streams working through the
// recorder.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// MCPHandler serves bin/ask-mcp-server's HTTP mode: /mcp (the SDK's
// Streamable HTTP transport, JSON responses), GET /mcp/schema, and unless
// MCPOnly the classic API's /v1/health, /v1/agents, /v1/repos and POST
// /v1/ask. Build it with NewMCPHandler.
type MCPHandler struct {
	Server  *Server
	APIKey  string
	MCPOnly bool
	// Log gets python's default request log lines (stderr).
	Log io.Writer

	sdk *sdk.Server
	mcp http.Handler
}

// SessionTimeout closes a Streamable HTTP session idle this long, and
// MaxSessions caps the sessions open at once (a new initialize beyond it
// gets 503): without them every initialize kept a session for the life of
// the server.
var (
	SessionTimeout = 15 * time.Minute
	MaxSessions    = 1000
)

// NewMCPHandler builds the MCP server and the HTTP handler around it (at
// start-up, so that a failure is an error, not a broken /mcp).
func NewMCPHandler(server *Server, apiKey string, mcpOnly bool, log io.Writer) (*MCPHandler, error) {
	srv, err := server.NewMCPServer()
	if err != nil {
		return nil, err
	}
	h := &MCPHandler{Server: server, APIKey: apiKey, MCPOnly: mcpOnly, Log: log, sdk: srv}
	h.mcp = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv },
		&sdk.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: SessionTimeout, MaxRequestBodyBytes: MaxBodyBytes})
	return h, nil
}

// sessions is the number of MCP sessions open.
func (h *MCPHandler) sessions() int {
	n := 0
	for range h.sdk.Sessions() {
		n++
	}
	return n
}

func (h *MCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &statusRecorder{ResponseWriter: w, status: 200}
	defer func() {
		if h.Log != nil {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			fmt.Fprintf(h.Log, "%s - - [%s] \"%s %s %s\" %d -\n", host,
				time.Now().Format("02/Jan/2006 15:04:05"), r.Method, r.RequestURI, r.Proto, rec.status)
		}
	}()
	w = rec
	if !guardHost(w, r) {
		return
	}
	path := pathOf(r)
	if path != "/mcp" && r.Method != http.MethodGet && r.Method != http.MethodPost {
		unsupported(w, r)
		return
	}
	if !h.auth(w, r) || !guardBody(w, r) {
		return
	}
	if path == "/mcp" {
		if r.Method == http.MethodPost && r.Header.Get("Mcp-Session-Id") == "" && h.sessions() >= MaxSessions {
			w.Header().Set("Retry-After", "60")
			jsonError(w, 503, fmt.Sprintf("Too many MCP sessions open (%d); try again later", MaxSessions))
			return
		}
		h.mcp.ServeHTTP(notInitializedWriter{w}, r)
		return
	}
	if r.Method == http.MethodGet {
		switch {
		case path == "/mcp/schema":
			versions := sdk.SupportedProtocolVersions()
			jsonOK(w, object{
				{"protocol", "mcp"},
				{"version", versions[0]},
				{"protocolVersions", versions},
				{"capabilities", object{
					{"tools", object{}},
					{"resources", object{}},
				}},
				{"serverInfo", object{
					{"name", serverName},
					{"version", Version},
					{"description", serverDescription},
				}},
			})
		case h.MCPOnly:
			jsonError(w, 404, "Only MCP endpoints available in --mcp-only mode")
		case path == "/v1/health":
			jsonOK(w, object{{"status", "ok"}, {"repos", h.Server.Registry.Len()}, {"mcp_enabled", true}})
		case path == "/v1/agents":
			jsonOK(w, h.Server.Registry.agentsJSON())
		case path == "/v1/repos":
			jsonOK(w, h.Server.Registry.reposJSON())
		default:
			jsonError(w, 404, "Not found")
		}
		return
	}
	switch {
	case h.MCPOnly:
		jsonError(w, 404, "Only MCP endpoints available in --mcp-only mode")
	case path == "/v1/ask":
		body, ok := readJSON(w, r)
		if !ok {
			return
		}
		repo, role, question, verbose, ok := askFields(w, body)
		if !ok {
			return
		}
		release, ok := h.Server.acquire()
		if !ok {
			busy(w, h.Server.MaxAsks)
			return
		}
		defer release()
		answer, errText := h.Server.ExecuteAsk(r.Context(), repo, role, question, verbose)
		if errText != "" {
			jsonError(w, 500, errText)
			return
		}
		jsonOK(w, object{{"repo", repo}, {"role", role}, {"answer", answer}, {"exit_code", 0}, {"error", nil}})
	default:
		jsonError(w, 404, "Not found")
	}
}

// auth is the MCP server's _check_auth.
func (h *MCPHandler) auth(w http.ResponseWriter, r *http.Request) bool {
	if h.APIKey == "" {
		return true
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		jsonError(w, 401, "Bearer token required")
		return false
	}
	if !keyMatches(header[7:], h.APIKey) {
		jsonError(w, 403, "Invalid API key")
		return false
	}
	return true
}

// askFields reads {repo, role, question, verbose} of a /v1/ask body, as
// the v3.1.2 python servers do: a JSON object with string fields (400
// otherwise); verbose only when it is true.
func askFields(w http.ResponseWriter, body any) (string, string, string, bool, bool) {
	obj, ok := body.(map[string]any)
	if !ok {
		jsonError(w, 400, "Request body must be a JSON object")
		return "", "", "", false, false
	}
	var fields [3]string
	for i, k := range []string{"repo", "role", "question"} {
		v, present := obj[k]
		if !present {
			continue
		}
		s, ok := v.(string)
		if !ok {
			jsonError(w, 400, "Field '"+k+"' must be a string")
			return "", "", "", false, false
		}
		fields[i] = pyStrip(s)
	}
	repo, role, question := fields[0], fields[1], fields[2]
	if repo == "" || role == "" || question == "" {
		jsonError(w, 400, "Required fields: repo, role, question")
		return "", "", "", false, false
	}
	verbose, _ := obj["verbose"].(bool)
	return repo, role, question, verbose, true
}

// unsupported is BaseHTTPRequestHandler's 501 for a method it has no
// do_<METHOD> for.
func unsupported(w http.ResponseWriter, r *http.Request) {
	body := fmt.Sprintf("<!DOCTYPE HTML>\n<html lang=\"en\">\n    <head>\n        <meta charset=\"utf-8\">\n"+
		"        <title>Error response</title>\n    </head>\n    <body>\n        <h1>Error response</h1>\n"+
		"        <p>Error code: 501</p>\n        <p>Message: Unsupported method ('%s').</p>\n"+
		"        <p>Error code explanation: 501 - Server does not support this operation.</p>\n    </body>\n</html>\n", r.Method)
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(501)
	io.WriteString(w, body)
}
