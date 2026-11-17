package cmd

import (
	"os"

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
codes. It works in any directory, not only in a rebar project.`,
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
	os.Exit(ask.Main(args))
	return nil
}

func runAskAgentLoop(cmd *cobra.Command, args []string) error {
	os.Exit(ask.RunAgentLoop(args, os.Stdout, os.Stderr))
	return nil
}
