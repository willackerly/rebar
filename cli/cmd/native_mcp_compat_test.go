package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client compatibility of `rebar mcp serve --stdio` with Claude Code.
//
// testdata/mcp/claude-code-<version>.jsonl are the requests Claude Code
// sent to the server over stdio, recorded with a tee in front of it
// (`claude -p --mcp-config … --allowedTools mcp__rebar-ask__…`):
//
//   - 2.1.224 initializes with protocolVersion 2025-11-25, sends
//     notifications/initialized, then tools/list, resources/list and
//     tools/call;
//   - 2.1.311 first probes server/discover with the 2026-07-28 protocol
//     (falling back to initialize 2025-11-25 when the server does not know
//     it); against the SDK server it stays on 2026-07-28: every request
//     carries the protocol version, client info and capabilities in _meta.
//
// The recorded tool names were pointed at this fixture's repos, and a
// failing tool (ask_web_broken) and an unknown tool (ask_web_ghost) were
// added in the shape of the recorded tools/call. The test replays each
// file and reads every answer the way Claude Code's client does.

// claudeCodeVersions are the recorded clients.
var claudeCodeVersions = []string{"2.1.224", "2.1.311"}

// toolNamePattern is the tool-name rule of the Anthropic API that Claude
// Code applies to MCP tools.
var toolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func decodeJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("not a JSON object: %s: %v", s, err)
	}
	return m
}

func jsonPath(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func encodeJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestMCPClaudeCodeCompat(t *testing.T) {
	repos, stub := mcpFixture(t)
	for _, version := range claudeCodeVersions {
		t.Run(version, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "mcp", "claude-code-"+version+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			var steps []mcpStep
			var reqs []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				req := decodeJSON(t, line)
				_, call := req["id"]
				steps = append(steps, mcpStep{line, call})
				reqs = append(reqs, req)
			}
			transcript, stderr, code := mcpConverse(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t), steps)
			if code != 0 || strings.Contains(stderr, "ignored invalid message") || strings.Contains(transcript, "(after end of input)") {
				t.Fatalf("exit %d:\n%s%s", code, transcript, stderr)
			}
			var answers []string
			for _, l := range strings.Split(transcript, "\n") {
				if a, ok := strings.CutPrefix(l, "< "); ok {
					answers = append(answers, a)
				}
			}
			i := 0
			for k, req := range reqs {
				if !steps[k].reply {
					continue
				}
				checkClaudeCodeAnswer(t, req, decodeJSON(t, answers[i]), answers[i])
				i++
			}
		})
	}
}

// checkClaudeCodeAnswer checks one answer as Claude Code reads it.
func checkClaudeCodeAnswer(t *testing.T, req, resp map[string]any, raw string) {
	t.Helper()
	method, _ := req["method"].(string)
	fail := func(why string) { t.Errorf("%s (id %v): %s:\n%s", method, req["id"], why, raw) }
	if resp["jsonrpc"] != "2.0" {
		fail("not JSON-RPC 2.0")
	}
	if encodeJSON(resp["id"]) != encodeJSON(req["id"]) {
		fail("id does not echo the request's")
	}
	result, hasResult := resp["result"].(map[string]any)
	errObj, hasError := resp["error"].(map[string]any)
	if hasResult == hasError {
		fail("needs exactly one of result and error")
		return
	}
	newProtocol := jsonPath(req, "params", "_meta", "io.modelcontextprotocol/protocolVersion")
	if hasResult && newProtocol != nil && result["resultType"] != "complete" {
		fail(`2026-07-28 results carry resultType "complete"`)
	}
	switch method {
	case "initialize":
		if result["protocolVersion"] != jsonPath(req, "params", "protocolVersion") {
			fail("protocolVersion is not the requested one")
		}
		if jsonPath(result, "serverInfo", "name") != "ask-mcp-server" || jsonPath(result, "serverInfo", "version") == "" {
			fail("serverInfo")
		}
		if _, ok := jsonPath(result, "capabilities", "tools").(map[string]any); !ok {
			fail("no tools capability")
		}
	case "server/discover":
		versions, _ := result["supportedVersions"].([]any)
		if !slices.Contains(versions, newProtocol) {
			fail("supportedVersions lacks the requested protocol")
		}
		if _, ok := jsonPath(result, "capabilities", "tools").(map[string]any); !ok {
			fail("no tools capability")
		}
	case "tools/list":
		tools, _ := result["tools"].([]any)
		if len(tools) != 6 || result["nextCursor"] != nil {
			fail("want the 6 agent tools on one page")
		}
		for _, tool := range tools {
			name, _ := jsonPath(tool, "name").(string)
			if !toolNamePattern.MatchString(name) {
				fail("tool name " + name + " breaks the Anthropic API's rule")
			}
			if d, _ := jsonPath(tool, "description").(string); d == "" {
				fail("tool without description")
			}
			if jsonPath(tool, "inputSchema", "type") != "object" ||
				jsonPath(tool, "inputSchema", "properties", "question", "type") != "string" ||
				encodeJSON(jsonPath(tool, "inputSchema", "required")) != `["question"]` {
				fail("input schema of " + name)
			}
		}
	case "resources/list":
		resources, _ := result["resources"].([]any)
		if len(resources) != 18 {
			fail("want 3 resources per agent")
		}
		for _, r := range resources {
			if uri, _ := jsonPath(r, "uri").(string); !strings.HasPrefix(uri, "ask://") {
				fail("resource URI " + uri)
			}
		}
	case "tools/call":
		name := jsonPath(req, "params", "name")
		question := jsonPath(req, "params", "arguments", "question")
		text, _ := jsonPath(result, "content").([]any)
		var first string
		if len(text) > 0 && jsonPath(text[0], "type") == "text" {
			first, _ = jsonPath(text[0], "text").(string)
		}
		isError := result["isError"] == true
		switch {
		case name == "ask_web_ghost":
			if !hasError || encodeJSON(errObj["code"]) != "-32602" {
				fail("an unknown tool is a JSON-RPC error -32602 (a protocol error, not a result)")
			}
		case name == "ask_web_broken":
			if !hasResult || !isError || !strings.Contains(first, "(exit code: 3)") {
				fail("a failing ask is an isError result with its output")
			}
		case question == "":
			if !hasResult || !isError || first != "Question required" {
				fail("an empty question is an isError result")
			}
		default:
			if !hasResult || isError || first != "answer from web: --question -- architect "+question.(string) {
				fail("want the stub's answer as text content")
			}
		}
	}
}

// TestMCPUnsupportedProtocolVersion: an initialize asking for a version the
// server does not speak is answered with the newest one the handshake
// negotiates (the client then decides whether to disconnect).
func TestMCPUnsupportedProtocolVersion(t *testing.T) {
	repos, stub := mcpFixture(t)
	out, errOut, code := mcpStdio(t, []string{testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub}, mcpEnv(t),
		`{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"1999-01-01","capabilities":{},"clientInfo":{"name":"t","version":"1"}},"id":1}`+"\n")
	if code != 0 || jsonPath(decodeJSON(t, out), "result", "protocolVersion") != "2025-11-25" {
		t.Errorf("exit %d:\n%s%s", code, out, errOut)
	}
}

// TestMCPSDKClient: the SDK's own client (as other MCP hosts use it) runs
// the server over stdio: tools, a tool error, an unknown tool, an aliased
// resource.
func TestMCPSDKClient(t *testing.T) {
	repos, stub := mcpFixture(t)
	cmd := exec.Command(testBinary(t), "mcp", "serve", "--stdio", "--repos-dir", repos, "--ask-bin", stub)
	cmd.Env, cmd.Dir = mcpEnv(t), t.TempDir()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	ctx := context.Background()
	session, err := sdk.NewClient(&sdk.Implementation{Name: "rebar-test", Version: "1"}, nil).Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v\n%s", err, stderr.String())
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 6 {
		t.Fatalf("tools/list: %v %v", tools, err)
	}
	res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: "ask_web_product", Arguments: map[string]any{"question": "Who?", "verbose": true}})
	if err != nil || res.IsError || firstTextContent(res) != "answer from web: -v --question -- product Who?" {
		t.Errorf("tools/call: %+v %v", res, err)
	}
	res, err = session.CallTool(ctx, &sdk.CallToolParams{Name: "ask_web_broken", Arguments: map[string]any{"question": "q"}})
	if err != nil || !res.IsError || !strings.Contains(firstTextContent(res), "ask: error: claude invocation failed") {
		t.Errorf("failing tool: %+v %v", res, err)
	}
	if _, err := session.CallTool(ctx, &sdk.CallToolParams{Name: "ask_web_ghost", Arguments: map[string]any{"question": "q"}}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("unknown tool: %v", err)
	}
	read, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: "ask://memory/WEB:Product"})
	if err != nil || len(read.Contents) != 1 || read.Contents[0].Text != "Remembered: users are admins.\n" {
		t.Errorf("resources/read alias: %+v %v", read, err)
	}
}

func firstTextContent(res *sdk.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*sdk.TextContent); ok {
		return tc.Text
	}
	return ""
}
