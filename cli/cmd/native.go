package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"
	// Register the native steward and utilities with the checks.
	_ "github.com/willackerly/rebar/cli/internal/steward"
	_ "github.com/willackerly/rebar/cli/internal/utils"
)

// nativeCmd runs one of rebar's built-in checks as its script ran: against
// REBAR_PROJECT_ROOT, from the working directory, never a project's own
// copy. The script regression tests (scripts/tests/lib.sh run_in) drive the
// checks through it.
var nativeCmd = &cobra.Command{
	Use:   "native <check> [args...]",
	Short: "Run one of rebar's built-in checks, as its script ran",
	Long: `Run one of rebar's built-in checks against the current project
(--repo-root, else REBAR_PROJECT_ROOT, else the enclosing repository) from
the working directory, with the arguments, output and exit code its script
had. Unlike rebar run, it never runs the project's own copy of a script.
Without arguments, lists the checks.`,
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
// given (`rebar --repo-root X native …`), else REBAR_PROJECT_ROOT (as the
// scripts read it), else the repository around the working directory, else
// the working directory.
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
