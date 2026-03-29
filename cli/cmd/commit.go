package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/integrity"
	"github.com/willackerly/rebar/cli/internal/scripts"
)

var commitMessage string
var commitRole string

var commitCmd = &cobra.Command{
	Use:   "commit [files...]",
	Short: "Enforced commit — runs checks, updates integrity manifest",
	Long: `Like git commit, but with structural enforcement:
  - Runs pre-commit checks (no bypass possible)
  - Updates integrity hashes for protected files
  - Checks ratchets (assertion counts cannot decrease)
  - NO --no-verify flag exists`,
	RunE: runCommit,
}

func init() {
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "commit message")
	commitCmd.Flags().StringVar(&commitRole, "role", "", "override role for HMAC signing")
}

func runCommit(cmd *cobra.Command, args []string) error {
	if commitMessage == "" {
		return fmt.Errorf("commit message required (-m)")
	}

	// 1. Run pre-commit checks
	fmt.Println("Running pre-commit checks...")
	preCommitPath := filepath.Join(cfg.ScriptsDir, "pre-commit.sh")
	if _, err := os.Stat(preCommitPath); err == nil {
		exitCode, err := scripts.RunPassthrough(cfg.ScriptsDir, "pre-commit.sh")
		if err != nil {
			return fmt.Errorf("running pre-commit: %w", err)
		}
		if exitCode != 0 {
			return fmt.Errorf("pre-commit checks failed (exit %d) — fix issues and retry", exitCode)
		}
		fmt.Println("Pre-commit checks passed ✓")
	}

	// 2. Load or create manifest
	manifest, err := integrity.LoadManifest(cfg.RebarDir)
	if err != nil {
		// No manifest yet — that's OK, we'll create one
		repoID := ""
		if data, err := os.ReadFile(filepath.Join(cfg.RebarDir, "repo-id")); err == nil {
			repoID = strings.TrimSpace(string(data))
		}
		manifest = integrity.NewManifest(repoID)
	}

	salt, _ := integrity.LoadSalt(cfg.RebarDir)

	// 3. Get staged files
	stagedOut, err := exec.Command("git", "-C", cfg.RepoRoot, "diff", "--cached", "--name-only").Output()
	if err != nil {
		return fmt.Errorf("getting staged files: %w", err)
	}

	// Also stage any files passed as args
	if len(args) > 0 {
		addCmd := exec.Command("git", append([]string{"-C", cfg.RepoRoot, "add"}, args...)...)
		if out, err := addCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("staging files: %s", string(out))
		}
		// Re-read staged files
		stagedOut, _ = exec.Command("git", "-C", cfg.RepoRoot, "diff", "--cached", "--name-only").Output()
	}

	staged := strings.Split(strings.TrimSpace(string(stagedOut)), "\n")
	if len(staged) == 1 && staged[0] == "" {
		staged = nil
	}

	if len(staged) == 0 {
		return fmt.Errorf("nothing staged to commit — stage files first or pass them as arguments")
	}

	// 4. Update hashes for staged protected files
	protectedFiles, _ := integrity.ScanProtectedFiles(cfg.RepoRoot)
	protectedSet := map[string]string{} // path → category
	for cat, paths := range protectedFiles {
		for _, p := range paths {
			protectedSet[p] = cat
		}
	}

	updated := 0
	for _, file := range staged {
		cat, isProtected := protectedSet[file]
		if !isProtected {
			continue
		}

		fullPath := filepath.Join(cfg.RepoRoot, file)
		hash, err := integrity.HashFile(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not hash %s: %v\n", file, err)
			continue
		}

		role := commitRole
		if role == "" {
			role = integrity.DefaultRoleForCategory(cat)
		}

		entry := integrity.FileEntry{
			SHA256:     hash,
			Role:       role,
			ModifiedAt: time.Now().UTC(),
		}

		if salt != nil {
			roleSalt := integrity.ComputeRoleSalt(salt, role)
			entry.RoleHMAC = integrity.ComputeRoleHMAC(roleSalt, hash)
		}

		if cat == integrity.CategoryTests {
			count, err := integrity.CountAssertions(fullPath)
			if err == nil {
				entry.AssertCount = &count
			}
		}

		if manifest.Checksums[cat] == nil {
			manifest.Checksums[cat] = map[string]integrity.FileEntry{}
		}
		manifest.Checksums[cat][file] = entry
		updated++
	}

	// 5. Check ratchets
	computed, err := integrity.ComputeRatchets(cfg.RepoRoot)
	if err == nil {
		results := integrity.CheckRatchets(manifest, computed)
		for _, r := range results {
			if r.Violated {
				return fmt.Errorf("ratchet violation: %s is %d, minimum is %d — cannot decrease", r.Name, r.Current, r.Min)
			}
		}
		integrity.UpdateRatchets(manifest, computed)
	}

	// 6. Save manifest and stage it
	manifest.GeneratedBy = "rebar commit"
	if err := manifest.Save(cfg.RebarDir); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	manifestPath := filepath.Join(cfg.RebarDir, "integrity.json")
	relManifest, _ := filepath.Rel(cfg.RepoRoot, manifestPath)
	stageCmd := exec.Command("git", "-C", cfg.RepoRoot, "add", relManifest)
	if out, err := stageCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("staging manifest: %s", string(out))
	}

	// 7. Commit
	gitCommit := exec.Command("git", "-C", cfg.RepoRoot, "commit", "-m", commitMessage)
	gitCommit.Stdout = os.Stdout
	gitCommit.Stderr = os.Stderr
	if err := gitCommit.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	if updated > 0 {
		fmt.Printf("Integrity: %d protected file(s) updated in manifest\n", updated)
	}

	return nil
}
