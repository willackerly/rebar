package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <script> [args...]",
	Short: "Run one of rebar's scripts against the current project",
	Long: `Run a script from the rebar installation against the current project,
for example:

  rebar run steward              quality scan
  rebar run compute-registry     regenerate CONTRACT-REGISTRY.md
  rebar run check-ground-truth   verify METRICS
  rebar run inbox-watch inbox    watch the peer inbox

The script runs in the project root with REBAR_PROJECT_ROOT set, and reads
the project's .rebarrc. Without arguments, lists the available scripts.`,
	DisableFlagParsing: true,
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

func listScripts() error {
	rebarRoot := findRebarRoot()
	if rebarRoot == "" {
		return fmt.Errorf("rebar install not found (set REBAR_ROOT to your rebar checkout)")
	}
	paths, _ := filepath.Glob(filepath.Join(rebarRoot, "scripts", "*.sh"))
	names := []string{}
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".sh")
		if strings.HasPrefix(name, "_") {
			continue // sourced helpers, not commands
		}
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Printf("Scripts in %s:\n", filepath.Join(rebarRoot, "scripts"))
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
	return nil
}
