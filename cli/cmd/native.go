package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"
)

// nativeCmd runs a check's native (Go) implementation whatever
// REBAR_CHECK_IMPL says. It is the target of _rebar_native in
// scripts/_rebar-config.sh, which a script with a native counterpart calls
// when `rebar check` / `rebar run` asked for the go implementation, and a
// way to compare the two by hand. It never runs a script, so the hand-off
// cannot loop.
var nativeCmd = &cobra.Command{
	Use:   "native <check> [args...]",
	Short: "Run the native (Go) implementation of a check (experimental)",
	Long: `Run the native (Go) implementation of one of rebar's checks against the
current project (REBAR_PROJECT_ROOT, else the enclosing repository), with
the same arguments, output and exit code as its script. Without arguments,
lists the checks that have one.

Set REBAR_CHECK_IMPL=go to make rebar check and rebar run use them.`,
	Hidden:             true,
	DisableFlagParsing: true,
	RunE:               runNative,
}

func runNative(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Println(strings.Join(checks.Names(), "\n"))
		return nil
	}
	if args[0] == "-h" || args[0] == "--help" {
		return cmd.Help()
	}
	c, ok := checks.Lookup(args[0])
	if !ok {
		return fmt.Errorf("no native implementation of %q (have: %s)", args[0], strings.Join(checks.Names(), ", "))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// Like the script, which runs where it was started: relative arguments
	// resolve against the working directory, settings come from the root.
	if code := runNativeCheck(c, nativeRoot(), cwd, args[1:]); code != 0 {
		os.Exit(code)
	}
	return nil
}

// nativeRoot is the project a native check runs against: --repo-root when
// given (`rebar --repo-root X native …`), else REBAR_PROJECT_ROOT (set by the
// CLI for the scripts it runs, so a hand-off checks the same repo the script
// would), else the repository around the working directory, else the
// working directory.
func nativeRoot() string {
	if repoRoot != "" {
		return repoRoot
	}
	if root := os.Getenv("REBAR_PROJECT_ROOT"); root != "" {
		return root
	}
	cwd, _ := os.Getwd()
	if root, err := config.FindRepoRoot(cwd); err == nil {
		return root
	}
	return cwd
}

// runNativeCheck runs c against the project at root from the working
// directory dir, on the terminal, and returns its exit code.
func runNativeCheck(c *checks.Check, root, dir string, args []string) int {
	return c.Run(&checks.Context{Root: root, Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr}, args)
}
