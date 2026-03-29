package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:                "ask [agent] [question]",
	Short:              "Query a role-based agent (delegates to bin/ask)",
	Long:               `Passes all arguments through to the ASK CLI. Equivalent to running 'ask' directly.`,
	DisableFlagParsing: true,
	RunE:               runAsk,
}

func runAsk(cmd *cobra.Command, args []string) error {
	askPath := filepath.Join(cfg.BinDir, "ask")
	if _, err := os.Stat(askPath); err != nil {
		return fmt.Errorf("ask CLI not found at %s — run bin/install first", askPath)
	}

	// Use syscall.Exec to fully replace this process with ask.
	// This preserves TTY, signals, exit codes — zero overhead.
	env := os.Environ()
	return syscall.Exec(askPath, append([]string{"ask"}, args...), env)
}
