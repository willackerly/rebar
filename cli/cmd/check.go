package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/suite"
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

The suites and checks are built into rebar (native Go), so a project needs no
copies of rebar's scripts; repo-specific settings live in .rebarrc. A project
that still has its own scripts/ci-check.sh (or pre-commit.sh,
cold-start-checks.sh) runs that copy instead, with a deprecation notice, and
so does a project's own copy of a check the suite runs.

  rebar check               full suite (CI), as scripts/ci-check.sh
  rebar check --pre-commit  fast checks for the git pre-commit hook
  rebar check --cold-start  advisory session-start summary (always exits 0)

Skip a check with its SKIP_* variable (SKIP_FRESHNESS=1; see
scripts/README.md). The project's own checks in .rebarrc checks_extra
(checks_extra_pre_commit for --pre-commit) run after rebar's and count in
the summary (a project's own copy of the suite script does not run them).`,
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

	warnExtrasIgnored(name)
	warnBrokenHooks(os.Stderr, cfg.RepoRoot)
	exitCode, found, err := runRebarScript(name, scriptArgs...)
	if err != nil {
		return fmt.Errorf("running %s: %w", name, err)
	}
	if !found {
		return fmt.Errorf("%s is not built into this rebar", name) // cannot happen: the suites are built in
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// warnExtrasIgnored tells the user when the suite for name will not run the
// project's checks_extra / checks_extra_pre_commit: only the built-in suite
// does, so a project's own copy of the suite script skips them.
func warnExtrasIgnored(name string) {
	key := map[string]string{"ci-check.sh": suite.ExtraKey, "pre-commit.sh": suite.ExtraPreCommitKey}[name]
	if key == "" || len(suite.Extras(cfg.RepoRoot, key)) == 0 {
		return
	}
	if s, ok := resolveScript(cfg.RepoRoot, name); ok && s.Legacy {
		fmt.Fprintf(os.Stderr, "rebar: .rebarrc %s not run: the project's own scripts/%s runs instead (only the built-in suite runs it)\n", key, name)
	}
}
