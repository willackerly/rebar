package mcp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
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

// readJSON is _read_json: a JSON body of at most MaxBodyBytes, read within
// RequestTimeout, never allocated from what Content-Length claims; else
// the error answer (411 without a Content-Length, 413 too large, 400
// short or invalid).
func readJSON(w http.ResponseWriter, r *http.Request) (any, bool) {
	fail := func(status int, msg string) (any, bool) {
		w.Header().Set("Connection", "close")
		jsonError(w, status, msg)
		return nil, false
	}
	if len(r.TransferEncoding) > 0 {
		return fail(411, "Content-Length required (chunked bodies not supported)")
	}
	n, err := strconv.Atoi(strings.TrimSpace(r.Header.Get("Content-Length")))
	if err != nil {
		return fail(411, "Valid Content-Length required")
	}
	if n < 0 {
		return fail(400, "Invalid Content-Length")
	}
	if n > MaxBodyBytes {
		return fail(413, fmt.Sprintf("Request body too large (max %d bytes)", MaxBodyBytes))
	}
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(RequestTimeout))
	data := make([]byte, n)
	_, err = io.ReadFull(r.Body, data)
	_ = rc.SetReadDeadline(time.Time{})
	if err != nil {
		return fail(400, "Request body shorter than Content-Length")
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

// MCPHandler serves bin/ask-mcp-server's HTTP mode: POST /mcp (JSON-RPC),
// GET /mcp/schema, and unless MCPOnly the classic API's /v1/health,
// /v1/agents, /v1/repos and POST /v1/ask. One protocol session is shared
// by all requests (the python server made a new one per request, so only
// initialize itself could ever succeed over HTTP).
type MCPHandler struct {
	Server  *Server
	APIKey  string
	MCPOnly bool
	// LoopbackOnly: the server is bound to loopback, so only loopback
	// Host headers are answered.
	LoopbackOnly bool
	// Log gets python's default request log lines (stderr).
	Log io.Writer
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
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		unsupported(w, r)
		return
	}
	if !checkHost(w, r, h.LoopbackOnly) || !h.auth(w, r) {
		return
	}
	if r.Method == http.MethodPost && !checkJSONContentType(w, r) {
		return
	}
	path := pathOf(r)
	if r.Method == http.MethodGet {
		switch {
		case path == "/mcp/schema":
			jsonOK(w, object{
				{"protocol", "mcp"},
				{"version", ProtocolVersion},
				{"capabilities", object{
					{"tools", object{{"listChanged", false}}},
					{"resources", object{{"subscribe", false}, {"listChanged", false}}},
				}},
				{"serverInfo", object{
					{"name", "ask-mcp-server"},
					{"version", "1.0.0"},
					{"description", "MCP server for ASK agent coordination"},
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
	case path == "/mcp":
		body, ok := readJSON(w, r)
		if !ok {
			return
		}
		resp, reply := h.Server.Handle(body)
		if !reply {
			jsonOK(w, nil)
			return
		}
		jsonOK(w, resp)
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
		answer, errText := h.Server.ExecuteAsk(repo, role, question, verbose)
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
	if !apiKeyMatches(header, h.APIKey) {
		jsonError(w, 403, "Invalid API key")
		return false
	}
	return true
}

// askFields reads {repo, role, question, verbose} of a /v1/ask body: an
// object with string fields; verbose only when it is true.
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

// ServeStdio runs the MCP stdio transport: one JSON-RPC message per line
// in, one response per line out (none for notifications); unparseable
// lines are reported on stderr and skipped (a reply with id null breaks
// Claude Code's client).
func (s *Server) ServeStdio(in io.Reader, out, errw io.Writer) error {
	r := bufio.NewReader(in)
	for {
		line, err := r.ReadString('\n')
		if text := strings.TrimSpace(line); text != "" {
			msg, perr := decode([]byte(text))
			if perr != nil {
				fmt.Fprintf(errw, "ask-mcp-server: parse error: %v\n", perr)
			} else if resp, reply := s.Handle(msg); reply {
				if _, werr := io.WriteString(out, pyDumps(resp, 0)+"\n"); werr != nil {
					return werr
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
