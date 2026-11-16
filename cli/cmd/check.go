package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	checkStrict    bool
	checkPreCommit bool
	checkColdStart bool
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run enforcement checks",
	Long: `Run rebar's enforcement checks against the current project.

The checks run from the rebar installation, so a project needs no copies of
rebar's scripts; repo-specific settings live in .rebarrc. A project that
still has its own scripts/ci-check.sh (or pre-commit.sh, cold-start-checks.sh)
runs that copy instead, with a deprecation notice.

  rebar check               full suite (CI): scripts/ci-check.sh
  rebar check --pre-commit  fast checks for the git pre-commit hook
  rebar check --cold-start  advisory session-start summary (always exits 0)

Checks that have a native (Go) implementation (rebar native lists them) run
it in place of their script; REBAR_CHECK_IMPL=shell runs the scripts.`,
	Args: cobra.NoArgs,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkStrict, "strict", true, "exit 1 on any failure (full suite)")
	checkCmd.Flags().BoolVar(&checkPreCommit, "pre-commit", false, "run the fast pre-commit checks")
	checkCmd.Flags().BoolVar(&checkColdStart, "cold-start", false, "run the advisory session-start checks")
	checkCmd.MarkFlagsMutuallyExclusive("pre-commit", "cold-start")
}

func runCheck(cmd *cobra.Command, args []string) error {
	name, scriptArgs := "ci-check.sh", []string{}
	switch {
	case checkPreCommit:
		name = "pre-commit.sh"
	case checkColdStart:
		name = "cold-start-checks.sh"
	case checkStrict:
		scriptArgs = append(scriptArgs, "--strict")
	default:
		scriptArgs = append(scriptArgs, "--no-strict")
	}

	exitCode, found, err := runRebarScript(name, scriptArgs...)
	if err != nil {
		return fmt.Errorf("running %s: %w", name, err)
	}
	if !found {
		if checkPreCommit || checkColdStart {
			// Never block a commit or a session on a missing install.
			fmt.Fprintf(os.Stderr, "rebar: %s not found in the project or a rebar install — skipping (set REBAR_ROOT)\n", name)
			return nil
		}
		return fmt.Errorf("%s not found in the project or a rebar install (set REBAR_ROOT to your rebar checkout)", name)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
