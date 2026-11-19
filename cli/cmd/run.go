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

It runs in the project root and reads the project's .rebarrc. A script of
the project's own (a checks_extra entry, another scripts/*.sh, or a legacy
copy of one of rebar's scripts, which runs instead of the built-in check
with a deprecation notice) runs there with REBAR_PROJECT_ROOT set. Without
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
		return fmt.Errorf("no check or script %q (built in or in the project's scripts/) — `rebar run` lists them", args[0])
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// listScripts prints the checks and utilities built into rebar, then the
// project's own scripts (scripts/*.sh, thin wrappers aside), marking legacy
// copies of rebar's scripts.
func listScripts() error {
	fmt.Println("Built into rebar:")
	for _, n := range checks.Names() {
		fmt.Printf("  %s\n", n)
	}
	paths, _ := filepath.Glob(filepath.Join(cfg.RepoRoot, "scripts", "*.sh"))
	var lines []string
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".sh")
		if strings.HasPrefix(name, "_") {
			continue // sourced helpers, not commands
		}
		s, ok := resolveScript(cfg.RepoRoot, name)
		if !ok {
			continue
		}
		if s.Legacy {
			name += "  (legacy copy: runs instead of the built-in check)"
		}
		lines = append(lines, name)
	}
	if len(lines) == 0 {
		return nil
	}
	sort.Strings(lines)
	fmt.Printf("\nProject scripts in %s:\n", filepath.Join(cfg.RepoRoot, "scripts"))
	for _, l := range lines {
		fmt.Printf("  %s\n", l)
	}
	return nil
}
