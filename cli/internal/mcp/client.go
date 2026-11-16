package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// ClientMain is bin/test-mcp-client (`rebar mcp client <server-url>`): it
// walks an HTTP MCP server through initialize, tools/list,
// resources/list, a tools/call of the first tool and a resources/read of
// the first resource, printing what it gets.
func ClientMain(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stdout, "Usage: test-mcp-client <server-url>")
		fmt.Fprintln(stdout, "Example: test-mcp-client http://localhost:7232")
		return 1
	}
	url := args[0]
	id := 0
	send := func(method string, params any) (map[string]any, error) {
		id++
		body := pyDumps(object{{"jsonrpc", "2.0"}, {"method", method}, {"params", params}, {"id", id}}, 0)
		resp, err := http.Post(url+"/mcp", "application/json", strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%d Error for url: %s/mcp", resp.StatusCode, url)
		}
		var out map[string]any
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&out); err != nil {
			return nil, err
		}
		return out, nil
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "test-mcp-client: %v\n", err)
		return 1
	}
	result := func(resp map[string]any, key string) (any, error) {
		res, ok := resp["result"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("no result: %s", errorMessage(resp))
		}
		return res[key], nil
	}

	fmt.Fprint(stdout, "=== MCP Client Test ===\n\n")
	fmt.Fprintln(stdout, "1. Initializing MCP session...")
	init, err := send("initialize", object{{"protocolVersion", ProtocolVersion}, {"capabilities", object{}},
		{"clientInfo", object{{"name", "test-client"}, {"version", "1.0.0"}}}})
	if err != nil {
		return fail(err)
	}
	info, err := result(init, "serverInfo")
	if err != nil {
		return fail(err)
	}
	version, _ := result(init, "protocolVersion")
	fmt.Fprintf(stdout, "   Server: %v\n   Protocol: %v\n\n", lookup(info, "name"), version)

	fmt.Fprintln(stdout, "2. Listing available tools...")
	toolsResp, err := send("tools/list", object{})
	if err != nil {
		return fail(err)
	}
	toolsV, err := result(toolsResp, "tools")
	if err != nil {
		return fail(err)
	}
	tools, _ := toolsV.([]any)
	fmt.Fprintf(stdout, "   Found %d agent tools:\n", len(tools))
	for i, tool := range tools {
		if i == 3 {
			break
		}
		fmt.Fprintf(stdout, "   %d. %v: %v\n", i+1, lookup(tool, "name"), lookup(tool, "description"))
	}
	if len(tools) > 3 {
		fmt.Fprintf(stdout, "   ... and %d more\n", len(tools)-3)
	}
	fmt.Fprintln(stdout)

	fmt.Fprintln(stdout, "3. Listing available resources...")
	resResp, err := send("resources/list", object{})
	if err != nil {
		return fail(err)
	}
	resV, err := result(resResp, "resources")
	if err != nil {
		return fail(err)
	}
	resources, _ := resV.([]any)
	fmt.Fprintf(stdout, "   Found %d agent resources:\n", len(resources))
	for i, r := range resources {
		if i == 3 {
			break
		}
		fmt.Fprintf(stdout, "   %d. %v (%v)\n", i+1, lookup(r, "name"), lookup(r, "uri"))
	}
	if len(resources) > 3 {
		fmt.Fprintf(stdout, "   ... and %d more\n", len(resources)-3)
	}
	fmt.Fprintln(stdout)

	if len(tools) > 0 {
		fmt.Fprintln(stdout, "4. Calling an agent tool...")
		name := lookup(tools[0], "name")
		fmt.Fprintf(stdout, "   Using tool: %v\n", name)
		resp, err := send("tools/call", object{{"name", name}, {"arguments", object{{"question", "What is your role and purpose?"}, {"verbose", false}}}})
		if err != nil {
			return fail(err)
		}
		if content, ok := resp["result"]; ok {
			fmt.Fprintf(stdout, "   Agent response: %s...\n", first200(firstText(content)))
		} else {
			fmt.Fprintf(stdout, "   Error: %s\n", errorMessage(resp))
		}
		fmt.Fprintln(stdout)
	}
	if len(resources) > 0 {
		fmt.Fprintln(stdout, "5. Reading an agent resource...")
		fmt.Fprintf(stdout, "   Reading: %v\n", lookup(resources[0], "name"))
		resp, err := send("resources/read", object{{"uri", lookup(resources[0], "uri")}})
		if err != nil {
			return fail(err)
		}
		if res, ok := resp["result"]; ok {
			contents, _ := lookup(res, "contents").([]any)
			text := ""
			if len(contents) > 0 {
				text, _ = lookup(contents[0], "text").(string)
			}
			fmt.Fprintf(stdout, "   Content: %s...\n", first200(text))
		} else {
			fmt.Fprintf(stdout, "   Error: %s\n", errorMessage(resp))
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintln(stdout, "=== MCP Test Complete ===")
	return 0
}

func lookup(v any, key string) any {
	if m, ok := v.(map[string]any); ok {
		return m[key]
	}
	return nil
}

func firstText(result any) string {
	content, _ := lookup(result, "content").([]any)
	if len(content) == 0 {
		return ""
	}
	s, _ := lookup(content[0], "text").(string)
	return s
}

func errorMessage(resp map[string]any) string {
	if msg, ok := lookup(resp["error"], "message").(string); ok {
		return msg
	}
	return "Unknown error"
}

// first200 is s[:200] (characters).
func first200(s string) string {
	if utf8.RuneCountInString(s) <= 200 {
		return s
	}
	return string([]rune(s)[:200])
}
