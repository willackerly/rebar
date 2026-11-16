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

// readJSON is _read_json: the body (Content-Length bytes) as JSON, or a
// 400 "Invalid JSON" answer.
func readJSON(w http.ResponseWriter, r *http.Request) (any, bool) {
	n, _ := strconv.Atoi(r.Header.Get("Content-Length"))
	if n < 0 {
		n = 0
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r.Body, data); err != nil {
		jsonError(w, 400, "Invalid JSON: "+err.Error())
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

// MCPHandler serves bin/ask-mcp-server's HTTP mode: POST /mcp (JSON-RPC),
// GET /mcp/schema, and unless MCPOnly the classic API's /v1/health,
// /v1/agents, /v1/repos and POST /v1/ask. One protocol session is shared
// by all requests (the python server made a new one per request, so only
// initialize itself could ever succeed over HTTP).
type MCPHandler struct {
	Server  *Server
	APIKey  string
	MCPOnly bool
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
	if !h.auth(w, r) {
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
	if header[7:] != h.APIKey {
		jsonError(w, 403, "Invalid API key")
		return false
	}
	return true
}

// askFields reads {repo, role, question, verbose} of a /v1/ask body.
func askFields(w http.ResponseWriter, body any) (string, string, string, bool, bool) {
	obj, ok := body.(map[string]any)
	if !ok {
		jsonError(w, 500, "Internal error: "+noGet(body))
		return "", "", "", false, false
	}
	str := func(k string) (string, bool) {
		v, present := obj[k]
		if !present {
			return "", true
		}
		s, ok := v.(string)
		return pyStrip(s), ok
	}
	repo, ok1 := str("repo")
	role, ok2 := str("role")
	question, ok3 := str("question")
	if !ok1 || !ok2 || !ok3 {
		jsonError(w, 500, "Internal error: fields repo, role and question must be strings")
		return "", "", "", false, false
	}
	if repo == "" || role == "" || question == "" {
		jsonError(w, 400, "Required fields: repo, role, question")
		return "", "", "", false, false
	}
	return repo, role, question, truthy(obj["verbose"]), true
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
