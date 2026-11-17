package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/checks"
)

var runCmd = &cobra.Command{
	Use:   "run <script> [args...]",
	Short: "Run one of rebar's checks or utilities against the current project",
	Long: `Run one of rebar's checks or utilities (built into the binary; named after
the scripts they replace) against the current project, for example:

  rebar run steward              quality scan
  rebar run compute-registry     regenerate CONTRACT-REGISTRY.md
  rebar run check-ground-truth   verify METRICS
  rebar run inbox-watch inbox    watch the peer inbox

It runs in the project root and reads the project's .rebarrc; a script
(the project's own copy, a checks_extra entry, or the install's under
REBAR_CHECK_IMPL=shell) runs there with REBAR_PROJECT_ROOT set. Without
arguments, lists what is available.`,
	DisableFlagParsing: true,
	Annotations:        map[string]string{rootFlagsAfterName: "true"},
	RunE:               runScript,
}

func runScript(cmd *cobra.Command, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		if len(args) > 0 {
			return cmd.Help()
		}
		return listScripts()
	}
	exitCode, found, err := runRebarScript(args[0], args[1:]...)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no script %q in the project or a rebar install — `rebar run` lists them", args[0])
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// listScripts prints the checks and utilities built into rebar, then the
// scripts of the rebar install (if one is found) that have no native
// implementation, or all of them under REBAR_CHECK_IMPL=shell.
func listScripts() error {
	shell := checks.Impl() == checks.ImplShell
	if !shell {
		fmt.Println("Built into rebar:")
		for _, n := range checks.Names() {
			fmt.Printf("  %s\n", n)
		}
	}
	rebarRoot := findRebarRoot()
	if rebarRoot == "" {
		if shell {
			return fmt.Errorf("REBAR_CHECK_IMPL=shell needs a rebar install's scripts/ (set REBAR_ROOT to your rebar checkout)")
		}
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(rebarRoot, "scripts", "*.sh"))
	names := []string{}
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".sh")
		if strings.HasPrefix(name, "_") {
			continue // sourced helpers, not commands
		}
		if _, native := checks.Lookup(name); native && !shell {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if !shell {
		fmt.Println()
	}
	fmt.Printf("Scripts in %s:\n", filepath.Join(rebarRoot, "scripts"))
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
	return nil
}
