package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClientMain is bin/test-mcp-client (`rebar mcp client <server-url>`): it
// connects to an HTTP MCP server with the SDK's Streamable HTTP client,
// then walks it through tools/list, resources/list, a tools/call of the
// first tool and a resources/read of the first resource, printing what it
// gets.
func ClientMain(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stdout, "Usage: test-mcp-client <server-url>")
		fmt.Fprintln(stdout, "Example: test-mcp-client http://localhost:7232")
		if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
			return 0
		}
		return 1
	}
	url := strings.TrimRight(args[0], "/")
	ctx := context.Background()
	fail := func(err error) int {
		fmt.Fprintf(stderr, "test-mcp-client: %v\n", err)
		return 1
	}

	fmt.Fprint(stdout, "=== MCP Client Test ===\n\n")
	fmt.Fprintln(stdout, "1. Initializing MCP session...")
	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: Version}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: url + "/mcp"}, nil)
	if err != nil {
		return fail(err)
	}
	defer session.Close()
	init := session.InitializeResult()
	name := ""
	if init.ServerInfo != nil {
		name = init.ServerInfo.Name
	}
	fmt.Fprintf(stdout, "   Server: %s\n   Protocol: %s\n\n", name, init.ProtocolVersion)

	fmt.Fprintln(stdout, "2. Listing available tools...")
	var tools []*sdk.Tool
	for t, err := range session.Tools(ctx, nil) {
		if err != nil {
			return fail(err)
		}
		tools = append(tools, t)
	}
	fmt.Fprintf(stdout, "   Found %d agent tools:\n", len(tools))
	for i, t := range tools {
		if i == 3 {
			break
		}
		fmt.Fprintf(stdout, "   %d. %s: %s\n", i+1, t.Name, t.Description)
	}
	if len(tools) > 3 {
		fmt.Fprintf(stdout, "   ... and %d more\n", len(tools)-3)
	}
	fmt.Fprintln(stdout)

	fmt.Fprintln(stdout, "3. Listing available resources...")
	var resources []*sdk.Resource
	for r, err := range session.Resources(ctx, nil) {
		if err != nil {
			return fail(err)
		}
		resources = append(resources, r)
	}
	fmt.Fprintf(stdout, "   Found %d agent resources:\n", len(resources))
	for i, r := range resources {
		if i == 3 {
			break
		}
		fmt.Fprintf(stdout, "   %d. %s (%s)\n", i+1, r.Name, r.URI)
	}
	if len(resources) > 3 {
		fmt.Fprintf(stdout, "   ... and %d more\n", len(resources)-3)
	}
	fmt.Fprintln(stdout)

	if len(tools) > 0 {
		fmt.Fprintln(stdout, "4. Calling an agent tool...")
		fmt.Fprintf(stdout, "   Using tool: %s\n", tools[0].Name)
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: tools[0].Name,
			Arguments: map[string]any{"question": "What is your role and purpose?", "verbose": false}})
		switch {
		case err != nil:
			fmt.Fprintf(stdout, "   Error: %v\n", err)
		case res.IsError:
			fmt.Fprintf(stdout, "   Error: %s\n", firstText(res.Content))
		default:
			fmt.Fprintf(stdout, "   Agent response: %s...\n", first200(firstText(res.Content)))
		}
		fmt.Fprintln(stdout)
	}
	if len(resources) > 0 {
		fmt.Fprintln(stdout, "5. Reading an agent resource...")
		fmt.Fprintf(stdout, "   Reading: %s\n", resources[0].Name)
		res, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: resources[0].URI})
		if err != nil {
			fmt.Fprintf(stdout, "   Error: %v\n", err)
		} else {
			text := ""
			if len(res.Contents) > 0 {
				text = res.Contents[0].Text
			}
			fmt.Fprintf(stdout, "   Content: %s...\n", first200(text))
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintln(stdout, "=== MCP Test Complete ===")
	return 0
}

// firstText is the text of the first content block.
func firstText(content []sdk.Content) string {
	if len(content) == 0 {
		return ""
	}
	if t, ok := content[0].(*sdk.TextContent); ok {
		return t.Text
	}
	return ""
}

// first200 is s[:200] (characters).
func first200(s string) string {
	if utf8.RuneCountInString(s) <= 200 {
		return s
	}
	return string([]rune(s)[:200])
}
