package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/ask"
	"github.com/willackerly/rebar/cli/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "ASK's MCP server (Claude Code tools for every agent)",
	Long: `ASK's Model Context Protocol server, built in.

  rebar mcp serve --stdio --repos-dir ~/dev     # what .mcp.json runs
  rebar mcp serve --port 7232 --repos-dir /srv  # HTTP: POST /mcp + classic API

Every agents/<role>/AGENT.md in the repos found becomes a tool
ask_<repo>_<role>; tools/call runs 'rebar ask <role> "<question>"' in that
repo. bin/ask-mcp-server runs 'rebar mcp serve', so existing .mcp.json files
keep working; 'rebar migrate' proposes pointing them at rebar directly.`,
}

var mcpServeCmd = &cobra.Command{
	Use:                "serve [--stdio] [--port N] [--host H] [--repos-dir DIR] [--repos PATHS] [--mcp-only] [--api-key KEY] [--ask-bin PATH]",
	Short:              "Run the MCP server (bin/ask-mcp-server)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("REBAR_CHECK_IMPL") == "shell" {
			return execLegacyPython("ask-mcp-server", args)
		}
		os.Exit(mcp.ServeMain(args, selfExe(), os.Stdin, os.Stdout, os.Stderr))
		return nil
	},
}

var mcpClientCmd = &cobra.Command{
	Use:                "client <server-url>",
	Short:              "Exercise an HTTP MCP server (bin/test-mcp-client)",
	Hidden:             true,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("REBAR_CHECK_IMPL") == "shell" {
			return execLegacyPython("test-mcp-client", args)
		}
		os.Exit(mcp.ClientMain(args, os.Stdout, os.Stderr))
		return nil
	},
}

func init() {
	mcpCmd.GroupID = "agents"
	mcpCmd.AddCommand(mcpServeCmd, mcpClientCmd)
	rootCmd.AddCommand(mcpCmd)
	// `rebar ask serve` (ask-server) is native too.
	ask.Serve = func(args []string) int {
		if os.Getenv("REBAR_CHECK_IMPL") == "shell" {
			if err := execLegacyPython("ask-server", args); err != nil {
				os.Stderr.WriteString("rebar: " + err.Error() + "\n")
			}
			return 1
		}
		return mcp.AskServerMain(args, selfExe(), os.Stdout, os.Stderr)
	}
}

// selfExe is this rebar binary, the default ask of the servers.
func selfExe() string {
	exe, err := os.Executable()
	if err != nil {
		return "rebar"
	}
	return exe
}

// execLegacyPython replaces rebar with the python original of an ASK
// server (bin/legacy/<name>.py), for REBAR_CHECK_IMPL=shell.
func execLegacyPython(name string, args []string) error {
	python, err := exec.LookPath("python3")
	if err != nil {
		return err
	}
	for _, dir := range askBinDirs() {
		script := filepath.Join(dir, "legacy", name+".py")
		if _, err := os.Stat(script); err == nil {
			return syscall.Exec(python, append([]string{"python3", script}, args...), os.Environ())
		}
	}
	return fmt.Errorf("REBAR_CHECK_IMPL=shell: bin/legacy/%s.py not found beside rebar or in the rebar install", name)
}
