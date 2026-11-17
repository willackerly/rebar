package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/integrity"
	"github.com/willackerly/rebar/cli/internal/scripts"
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

	// 2. Run the checks: the built-in suite, or the project's own copy of
	// scripts/ci-check.sh (deprecated).
	fmt.Println("Running checks...")
	var checkOut []byte
	var code int
	if s, ok := resolveScript(cfg.RepoRoot, "ci-check.sh"); ok && s.Legacy {
		warnLegacy(s)
		checkOut, code, err = scripts.OutputInProject(s.Path, cfg.RepoRoot)
	} else {
		c, _ := checks.Lookup("ci-check")
		var buf bytes.Buffer
		code = c.Run(&checks.Context{Root: cfg.RepoRoot, Dir: cfg.RepoRoot, Stdout: &buf, Stderr: &buf}, nil)
		checkOut = buf.Bytes()
	}
	if err != nil || code != 0 {
		fmt.Println(string(checkOut))
		// Only fail if it's a real check failure, not a missing dependency
		if code == 1 {
			return fmt.Errorf("checks failed — fix before pushing")
		}
	} else {
		fmt.Println("Checks passed ✓")
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
