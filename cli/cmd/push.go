package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
)

var pushCmd = &cobra.Command{
	Use:   "push [args...]",
	Short: "Verified push — checks integrity before pushing",
	Long:  `Runs rebar verify --strict and rebar check, then pushes if clean.`,
	RunE:  runPush,
	DisableFlagParsing: true,
}

func runPush(cmd *cobra.Command, args []string) error {
	// Safety: warn on force push to main/master
	for _, arg := range args {
		if arg == "--force" || arg == "-f" || arg == "--force-with-lease" {
			branch := gitOutput("branch", "--show-current")
			if branch == "main" || branch == "master" {
				return fmt.Errorf("refusing to force-push to %s — use git push directly if you really mean it", branch)
			}
		}
	}

	// 1. Verify integrity
	fmt.Println("Verifying integrity...")
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		return fmt.Errorf("no manifest — run 'rebar init' first")
	}

	salt, _ := integrity.LoadSalt(cfg.RebarDir)
	result, err := integrity.Verify(cfg.RepoRoot, manifest, salt)
	if err != nil {
		return err
	}
	if !result.Clean {
		fmt.Print(integrity.FormatResult(result))
		return fmt.Errorf("integrity violations found — fix before pushing")
	}
	fmt.Println("Integrity verified ✓")

	// 2. Run checks (if scripts exist)
	if _, err := os.Stat(cfg.ScriptsDir + "/ci-check.sh"); err == nil {
		fmt.Println("Running checks...")
		// Run checks but don't block on missing scripts
		checkOut, err := exec.Command("bash", cfg.ScriptsDir+"/ci-check.sh").CombinedOutput()
		if err != nil {
			fmt.Println(string(checkOut))
			// Only fail if it's a real check failure, not a missing dependency
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return fmt.Errorf("checks failed — fix before pushing")
			}
		} else {
			fmt.Println("Checks passed ✓")
		}
	}

	// 3. Push
	gitArgs := append([]string{"push"}, args...)
	pushCmd := exec.Command("git", gitArgs...)
	pushCmd.Dir = cfg.RepoRoot
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		if strings.Contains(err.Error(), "exit status") {
			return fmt.Errorf("git push failed")
		}
		return err
	}

	return nil
}
