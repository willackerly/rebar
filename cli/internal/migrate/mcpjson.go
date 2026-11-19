package migrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/willackerly/rebar/cli/internal/jqjson"
)

// MCPEdit is the rewrite of a project's .mcp.json: Claude Code MCP servers
// that run bin/ask-mcp-server (the python server, now a hand-off to the
// rebar binary) are pointed at `rebar mcp serve` itself.
type MCPEdit struct {
	File      string
	Data, New []byte // as read when planning, and as rewritten
	Servers   []MCPServerEdit
}

// MCPServerEdit is one rewritten server entry.
type MCPServerEdit struct {
	Name, From, To string
}

// isAskMCPServer reports whether a command path names ask-mcp-server.
func isAskMCPServer(cmd string) bool {
	base := filepath.Base(cmd)
	return base == "ask-mcp-server" || base == "ask-mcp-server.py"
}

// commandLine renders a command and its arguments for the report.
func commandLine(cmd string, args []any) string {
	parts := []string{cmd}
	for _, a := range args {
		parts = append(parts, jqjson.Raw(a))
	}
	return strings.Join(parts, " ")
}

// PlanMCP plans the .mcp.json rewrite of the project (nil when there is
// nothing to rewrite). rebarBin is the rebar to point at when the old
// command has no rebar beside it that serves MCP ("" for "rebar" on PATH):
// a 3.x install's bin/rebar (versions/v3.1.2/bin, which `rebar new` wrote
// into .mcp.json) has no `mcp serve`.
func PlanMCP(project, rebarBin string) (*MCPEdit, error) {
	file := filepath.Join(project, ".mcp.json")
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	vals, err := jqjson.ParseStream(data)
	if err != nil || len(vals) != 1 {
		return nil, fmt.Errorf(".mcp.json is not one JSON object; not rewritten")
	}
	root, ok := vals[0].(jqjson.Object)
	if !ok {
		return nil, fmt.Errorf(".mcp.json is not a JSON object; not rewritten")
	}
	serversV, _ := root.Get("mcpServers")
	servers, ok := serversV.(jqjson.Object)
	if !ok {
		return nil, nil
	}
	edit := &MCPEdit{File: file, Data: data}
	for i, s := range servers {
		entry, ok := s.Val.(jqjson.Object)
		if !ok {
			continue
		}
		cmdV, _ := entry.Get("command")
		cmd, _ := cmdV.(string)
		argsV, _ := entry.Get("args")
		args, _ := argsV.([]any)
		if argsV != nil && args == nil {
			continue // args is not a list: leave the entry alone
		}
		var rest []any
		old := cmd
		switch {
		case isAskMCPServer(cmd):
			rest = args
		case (filepath.Base(cmd) == "python3" || filepath.Base(cmd) == "python") && len(args) > 0 && isAskMCPServer(jqjson.Raw(args[0])):
			old = jqjson.Raw(args[0])
			rest = args[1:]
		default:
			continue
		}
		rebar := rebarBin
		if sibling := filepath.Join(filepath.Dir(old), "rebar"); filepath.IsAbs(old) && isExecutableFile(sibling) && servesMCP(sibling) {
			rebar = sibling
		}
		if rebar == "" {
			rebar = "rebar"
		}
		newArgs := append([]any{"mcp", "serve"}, rest...)
		updated := setKey(setKey(append(jqjson.Object{}, entry...), "command", rebar), "args", newArgs)
		servers[i].Val = updated
		edit.Servers = append(edit.Servers, MCPServerEdit{Name: s.Key, From: commandLine(cmd, args), To: commandLine(rebar, newArgs)})
	}
	if len(edit.Servers) == 0 {
		return nil, nil
	}
	edit.New = []byte(jqjson.Pretty(root))
	return edit, nil
}

// servesMCP reports whether the rebar at path has `rebar mcp serve` (4.0
// and later; 3.x answers "unknown command" with exit 1, and a binary built
// for another platform does not run).
func servesMCP(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "mcp", "serve", "--help")
	return cmd.Run() == nil
}

func isExecutableFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// setKey sets key in o, in place or appended.
func setKey(o jqjson.Object, key string, val any) jqjson.Object {
	for i := range o {
		if o[i].Key == key {
			o[i].Val = val
			return o
		}
	}
	return append(o, jqjson.Field{Key: key, Val: val})
}
