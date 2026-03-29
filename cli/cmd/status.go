package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Repository health dashboard",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("REBAR Status")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Git info
	branch := gitOutput("branch", "--show-current")
	dirty := gitOutput("status", "--porcelain")
	gitStatus := "clean"
	if dirty != "" {
		lines := strings.Count(dirty, "\n")
		if !strings.HasSuffix(dirty, "\n") {
			lines++
		}
		gitStatus = fmt.Sprintf("%d file(s) modified", lines)
	}
	fmt.Printf("\n  Git branch:  %s (%s)\n", branch, gitStatus)

	// Tier
	tierLabels := map[int]string{1: "Partial", 2: "Adopted", 3: "Enforced"}
	fmt.Printf("  Tier:        %d (%s)\n", cfg.Tier, tierLabels[cfg.Tier])

	if cfg.Version != "" {
		fmt.Printf("  Version:     %s\n", cfg.Version)
	}

	// Integrity
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		fmt.Printf("\n  Integrity:   not initialized — run 'rebar init'\n")
		return nil
	}

	fmt.Printf("  Last check:  %s\n", manifest.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))

	// File counts per category
	total := 0
	for cat, files := range manifest.Checksums {
		count := len(files)
		total += count
		if count > 0 {
			fmt.Printf("  %-13s %d files\n", cat+":", count)
		}
	}
	fmt.Printf("  Total:       %d protected files\n", total)

	// Ratchets
	if len(manifest.Ratchets) > 0 {
		fmt.Println("\n  Ratchets:")
		for name, r := range manifest.Ratchets {
			fmt.Printf("    %-20s current: %d, min: %d\n", name, r.Current, r.Min)
		}
	}

	// Quick verify (just check if clean)
	salt, _ := integrity.LoadSalt(cfg.RebarDir)
	result, err := integrity.Verify(cfg.RepoRoot, manifest, salt)
	if err == nil {
		if result.Clean && len(result.Untracked) == 0 {
			fmt.Println("\n  Integrity:   ✓ verified")
		} else {
			violations := 0
			for _, f := range result.Files {
				if !f.HashMatch {
					violations++
				}
			}
			violations += len(result.Missing)
			if violations > 0 {
				fmt.Printf("\n  Integrity:   ✗ %d violation(s) — run 'rebar verify' for details\n", violations)
			}
			if len(result.Untracked) > 0 {
				fmt.Printf("  Untracked:   %d protected file(s) not in manifest\n", len(result.Untracked))
			}
		}
	}

	return nil
}

func gitOutput(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cfg.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
