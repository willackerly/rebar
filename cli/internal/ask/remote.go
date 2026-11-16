package ask

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/jqjson"
)

// Remote mode: with ASK_SERVER set (comma-separated host:port list),
// project:agent questions for projects not registered here go to an ASK
// server over HTTP, round-robin across the servers (~/.config/ask/.rr-index).

// servers is `IFS=',' read -ra` of ASK_SERVER: its first line split at
// commas, a trailing empty field dropped.
func (a *App) servers() []string {
	line, _, _ := strings.Cut(a.server, "\n")
	if line == "" {
		return nil
	}
	s := strings.Split(line, ",")
	if s[len(s)-1] == "" {
		s = s[:len(s)-1]
	}
	return s
}

// nextRRIndex returns the stored round-robin index and stores the next.
func nextRRIndex(count int) int {
	file := os.Getenv("HOME") + "/.config/ask/.rr-index"
	idx := 0
	if text, ok := readTrimmed(file); ok {
		if n, ok := atoi(text); ok {
			idx = n
		}
	}
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	if count > 0 {
		writeLine(file, strconv.Itoa((idx+1)%count))
	}
	return idx
}

func normalizeServerURL(server string) string {
	if strings.HasPrefix(server, "http://") || strings.HasPrefix(server, "https://") {
		return server
	}
	return "http://" + server
}

// httpClient is the client of remote mode (curl --max-time 300).
var httpClient = &http.Client{Timeout: 300 * time.Second}

// curlOne is _curl_one: one request; ok when the server answered with a
// status below 400 (curl --fail-with-body).
func (a *App) curlOne(server, method, endpoint string, body []byte) (string, bool) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, normalizeServerURL(server)+endpoint, r)
	if err != nil {
		return "", false
	}
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return strings.TrimRight(string(data), "\n"), resp.StatusCode < 400
}

// serverCurl is _server_curl: try the servers round-robin until one
// answers without a "not found" error.
func (a *App) serverCurl(method, endpoint string, body []byte) (string, bool) {
	servers := a.servers()
	count := len(servers)
	start := nextRRIndex(count)
	for i := 0; i < count; i++ {
		idx := ((start+i)%count + count) % count
		server := servers[idx]
		resp, ok := a.curlOne(server, method, endpoint, body)
		if !ok {
			a.log("Server %s: unreachable — trying next", server)
			continue
		}
		if hasError([]byte(resp)) {
			msg := jqRaw([]byte(resp), []string{"error"}, nil, "")
			if strings.Contains(strings.ToLower(msg), "not found") {
				a.log("Server %s: %s — trying next", server, msg)
				continue
			}
		}
		return resp, true
	}
	a.errf("ask: error: all servers failed for %s %s\n  tried: %s\n", method, endpoint, strings.Join(servers, ", "))
	return "", false
}

// hasError is `jq -e '.error'`.
func hasError(data []byte) bool {
	vals, err := jqjson.ParseStream(data)
	if err != nil || len(vals) == 0 {
		return false
	}
	var last any
	for _, v := range vals {
		got, ok := jqjson.Path(v, "error")
		if !ok {
			return false
		}
		last = got
	}
	return jqjson.Truthy(last)
}

// remoteAsk is _remote_ask.
func (a *App) remoteAsk(repo, role, question string) {
	body := jqjson.Pretty(jqjson.Object{
		{Key: "repo", Val: jqjson.RepairUTF8(repo)},
		{Key: "role", Val: jqjson.RepairUTF8(role)},
		{Key: "question", Val: jqjson.RepairUTF8(question)},
		{Key: "verbose", Val: a.verbose},
	})
	resp, ok := a.serverCurl("POST", "/v1/ask", []byte(strings.TrimSuffix(body, "\n")))
	if !ok {
		a.exit(1)
	}
	data := []byte(resp)
	answer := jqRaw(data, []string{"answer"}, jqEmpty, "")
	code := jqRaw(data, []string{"exit_code"}, json0, "")
	errText := jqRaw(data, []string{"error"}, jqEmpty, "")
	if answer != "" {
		a.formatResponse(role, answer, repo)
	}
	if errText != "" {
		a.errf("%s\n", errText)
	}
	if code == "" {
		a.exit(0)
	}
	n, ok := atoi(code)
	if !ok {
		a.errf("ask: exit: %s: numeric argument required\n", code)
		a.exit(2)
	}
	a.exit(n & 0xff)
}

// json0 is the JSON number 0, jq's `// 0`.
var json0 = jqjsonNumber("0")

// remoteStatus is _remote_status: the server's status object as KEY: value
// lines.
func (a *App) remoteStatus(repo, role string) {
	resp, ok := a.serverCurl("GET", "/v1/status/"+repo+"/"+role, nil)
	if !ok {
		a.exit(1)
	}
	vals, err := jqjson.ParseStream([]byte(resp))
	var lines []string
	failed := err != nil
	for _, v := range vals {
		obj, isObj := v.(jqjson.Object)
		if !isObj {
			failed = true
			break
		}
		for _, f := range obj {
			lines = append(lines, upperASCII(f.Key)+": "+interp(f.Val))
		}
	}
	if failed {
		lines = append(lines, resp)
	}
	if out := strings.TrimRight(strings.Join(lines, "\n"), "\n"); out != "" || len(lines) > 0 {
		a.out("%s\n", out)
	}
}

func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}

// remoteField prints jq -r '.key // empty' of a server response, or the
// response where it is not JSON.
func (a *App) remoteField(method, endpoint, key string) {
	resp, ok := a.serverCurl(method, endpoint, nil)
	if !ok {
		a.exit(1)
	}
	if out := jqRaw([]byte(resp), []string{key}, jqEmpty, resp); out != "" {
		a.out("%s\n", out)
	}
}

func (a *App) remoteLog(repo, role string) {
	a.remoteField("GET", "/v1/log/"+repo+"/"+role, "log")
}

func (a *App) remoteReset(repo, role string) {
	a.remoteField("POST", "/v1/reset/"+repo+"/"+role, "message")
}

// remoteAgents is cmd_agents: the agents of every server, merged.
func (a *App) remoteAgents() {
	if a.server == "" {
		a.errf("ask: ASK_SERVER not set. Use 'ask who' for local agents.\n")
		a.exit(4)
	}
	servers := a.servers()
	all := []any{}
	for _, server := range servers {
		resp, ok := a.curlOne(server, "GET", "/v1/agents", nil)
		if !ok {
			a.errf("  (server %s: unreachable)\n", server)
			continue
		}
		vals, err := jqjson.ParseStream([]byte(resp))
		if err != nil || len(vals) == 0 {
			continue
		}
		var add []any
		good := true
		var items []any
		switch x := vals[0].(type) {
		case []any:
			items = x
		case jqjson.Object:
			for _, f := range x {
				items = append(items, f.Val)
			}
		default:
			good = false
		}
		for _, it := range items {
			var obj jqjson.Object
			switch o := it.(type) {
			case jqjson.Object:
				obj = append(jqjson.Object{}, o...)
			case nil:
			default:
				good = false
			}
			if !good {
				break
			}
			obj = setKey(obj, "server", server)
			add = append(add, obj)
		}
		if good {
			all = append(all, add...)
		}
	}
	var lines []string
	if len(servers) > 1 {
		groups := map[string]bool{}
		sorted := append([]any{}, all...)
		sort.SliceStable(sorted, func(i, j int) bool {
			return jqLess(idOf(sorted[i]), idOf(sorted[j]))
		})
		for _, it := range sorted {
			key := jqjson.Compact(idOf(it))
			if groups[key] {
				continue
			}
			groups[key] = true
			srv, _ := jqjson.Path(it, "server")
			lines = append(lines, interp(idOf(it))+"  ← "+interp(srv))
		}
	} else {
		for _, it := range all {
			lines = append(lines, jqjson.Raw(idOf(it)))
		}
	}
	if len(lines) > 0 {
		a.out("%s\n", strings.Join(lines, "\n"))
	}
}

func idOf(v any) any {
	id, _ := jqjson.Path(v, "id")
	return id
}

// setKey is jq's `. + {key: val}`: replace in place or append.
func setKey(o jqjson.Object, key string, val any) jqjson.Object {
	for i := range o {
		if o[i].Key == key {
			o[i].Val = val
			return o
		}
	}
	return append(o, jqjson.Field{Key: key, Val: val})
}

// jqLess orders values as jq's sort does for the scalars ids are: null,
// false, true, numbers, strings, then anything else by its JSON text.
func jqLess(x, y any) bool {
	rank := func(v any) int {
		switch t := v.(type) {
		case nil:
			return 0
		case bool:
			if t {
				return 2
			}
			return 1
		case jqjsonNumber:
			return 3
		case string:
			return 4
		}
		return 5
	}
	rx, ry := rank(x), rank(y)
	if rx != ry {
		return rx < ry
	}
	switch tx := x.(type) {
	case jqjsonNumber:
		fx, _ := number(tx)
		fy, _ := number(y)
		return fx < fy
	case string:
		return tx < y.(string)
	}
	return jqjson.Compact(x) < jqjson.Compact(y)
}
