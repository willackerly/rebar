package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/scripts"
)

var checkStrict bool

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run all enforcement checks",
	Long:  `Runs steward and CI checks via scripts/ci-check.sh.`,
	RunE:  runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkStrict, "strict", true, "exit 1 on any failure")
}

func runCheck(cmd *cobra.Command, args []string) error {
	scriptArgs := []string{}
	if checkStrict {
		scriptArgs = append(scriptArgs, "--strict")
	}

	exitCode, err := scripts.RunPassthrough(cfg.ScriptsDir, "ci-check.sh", scriptArgs...)
	if err != nil {
		return fmt.Errorf("running checks: %w", err)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
