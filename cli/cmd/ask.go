package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/ask"
)

var askCmd = &cobra.Command{
	Use:   "ask [agent] [question]",
	Short: "Query a role-based agent (ASK)",
	Long: `The ASK CLI, built in: everything after "ask" is ASK's own command line.

  rebar ask <agent> "<question>"     # ask an agent
  rebar ask who                      # list the agents
  rebar ask help                     # ASK's help

'rebar ask' and the 'ask' command (bin/ask, which runs 'rebar ask') are
the same: flags, agent names, environment variables (ASK_*), files and exit
codes. It works in any directory, not only in a rebar project.
REBAR_CHECK_IMPL=shell runs the bash implementation, bin/ask, instead.`,
	DisableFlagParsing: true,
	RunE:               runAsk,
}

// askAgentLoopCmd is bin/ask-agent-loop: the inbox loop of one agent
// directory, for use as an agent's run.sh.
var askAgentLoopCmd = &cobra.Command{
	Use:   "ask-agent-loop <agent-dir>",
	Short: "Run an ASK agent's inbox loop (bin/ask-agent-loop)",
	Long: `Watch <agent-dir>/inbox, answer each question with claude into
<agent-dir>/outbox, and log it, until interrupted. What 'ask up <agent>'
runs when the agent has no run.sh; bin/ask-agent-loop runs it too.`,
	Hidden:             true,
	DisableFlagParsing: true,
	RunE:               runAskAgentLoop,
}

func init() {
	rootCmd.AddCommand(askAgentLoopCmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	if os.Getenv("REBAR_CHECK_IMPL") == "shell" {
		return execShellTool("ask", args)
	}
	ask.BinDirs = askBinDirs()
	os.Exit(ask.Main(args))
	return nil
}

func runAskAgentLoop(cmd *cobra.Command, args []string) error {
	if os.Getenv("REBAR_CHECK_IMPL") == "shell" {
		return execShellTool("ask-agent-loop", args)
	}
	os.Exit(ask.RunAgentLoop(args, os.Stdout, os.Stderr))
	return nil
}

// askBinDirs are where the ASK tools of this install live: beside the rebar
// binary, then in the rebar install's bin/.
func askBinDirs() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, stableBinDir(filepath.Dir(exe)))
	}
	if root := findRebarRoot(); root != "" {
		dirs = append(dirs, filepath.Join(root, "bin"))
	}
	return dirs
}

// execShellTool replaces rebar with the bash/python original of an ASK
// tool (bin/<name>), for REBAR_CHECK_IMPL=shell; the tool sees that
// setting and does not hand back to rebar.
func execShellTool(name string, args []string) error {
	for _, dir := range askBinDirs() {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return syscall.Exec(path, append([]string{name}, args...), os.Environ())
		}
	}
	return fmt.Errorf("REBAR_CHECK_IMPL=shell: %s not found beside rebar or in the rebar install's bin/", name)
}
