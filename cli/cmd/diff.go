package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show changes to protected files since last verified state",
	RunE:  runDiff,
}

func runDiff(cmd *cobra.Command, args []string) error {
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		return fmt.Errorf("no manifest — run 'rebar init' first")
	}

	changed := false
	for cat, files := range manifest.Checksums {
		for path, entry := range files {
			fullPath := filepath.Join(cfg.RepoRoot, path)
			currentHash, err := integrity.HashFile(fullPath)
			if err != nil {
				fmt.Printf("MISSING: %s (%s)\n", path, cat)
				changed = true
				continue
			}

			if currentHash != entry.SHA256 {
				fmt.Printf("\n─── %s (%s, role: %s) ───\n", path, cat, entry.Role)
				// Show git diff for this file
				gitDiff := exec.Command("git", "-C", cfg.RepoRoot, "diff", "--", path)
				gitDiff.Stdout = os.Stdout
				gitDiff.Stderr = os.Stderr
				gitDiff.Run()
				changed = true
			}
		}
	}

	if !changed {
		fmt.Println("No changes to protected files since last verified state.")
	}

	return nil
}
