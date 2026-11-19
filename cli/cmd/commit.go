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
  - Changes only the manifest entries of staged files, so branches merge cleanly
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
	if exitCode, found, err := runRebarScript("pre-commit.sh"); err != nil {
		return fmt.Errorf("running pre-commit: %w", err)
	} else if found {
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

	// 4. Check ratchets against the floors recorded before this commit
	computed, err := integrity.ComputeRatchets(cfg.RepoRoot)
	if err == nil {
		results := integrity.CheckRatchets(manifest, computed)
		for _, r := range results {
			if r.Violated {
				return fmt.Errorf("ratchet violation: %s is %d, minimum is %d — cannot decrease", r.Name, r.Current, r.Min)
			}
		}
	}

	// 5. Update hashes for staged protected files
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

	// Staged deletions (a rename counts as delete + add) leave the manifest:
	// otherwise `rebar verify --strict` reports them as Missing forever.
	pruned := pruneStagedDeletions(manifest, cfg.RepoRoot)

	// 6. Save manifest and stage it
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
	if pruned > 0 {
		fmt.Printf("Integrity: %d deleted file(s) removed from manifest\n", pruned)
	}

	// 8. Session checkpoint reminder
	checkSessionCheckpoint(cfg.RepoRoot)

	return nil
}

// checkSessionCheckpoint emits a warning if it's been many commits since
// QUICKCONTEXT was last synced. This is a behavioral nudge, not enforcement.
func checkSessionCheckpoint(repoRoot string) {
	qcPath := filepath.Join(repoRoot, "QUICKCONTEXT.md")
	content, err := os.ReadFile(qcPath)
	if err != nil {
		return
	}

	lastSynced := extractQCDate(string(content))
	if lastSynced == "" {
		return
	}

	// Count commits since last sync
	countStr := commitsSince(repoRoot, lastSynced)
	if countStr == "" {
		return
	}

	var count int
	fmt.Sscanf(countStr, "%d", &count)

	if count >= 10 {
		fmt.Printf("\n⚠  Checkpoint trigger: %d commits since QUICKCONTEXT was last synced (%s)\n", count, lastSynced)
		fmt.Println("   Consider: update QUICKCONTEXT.md, run 'rebar context session-start'")
		fmt.Println("   See: practices/session-lifecycle.md")
	}
}

// commitsSince returns `git rev-list --count` of HEAD's commits since local
// midnight starting date (YYYY-MM-DD), or "" if git fails. A bare date means
// that day at the current time of day to git, which dropped commits made
// earlier on the day itself.
func commitsSince(repoRoot, date string) string {
	return strings.TrimSpace(string(mustGitOutput(repoRoot, "rev-list", "--count", "--since="+date+" 00:00", "HEAD")))
}

// extractQCDate finds the last-synced or freshness date in QUICKCONTEXT content.
func extractQCDate(content string) string {
	for _, line := range strings.Split(content, "\n") {
		for _, key := range []string{"last-synced:", "freshness:"} {
			if idx := strings.Index(line, key); idx >= 0 {
				rest := strings.TrimSpace(line[idx+len(key):])
				for i := 0; i <= len(rest)-10; i++ {
					candidate := rest[i : i+10]
					if len(candidate) == 10 && candidate[4] == '-' && candidate[7] == '-' {
						return candidate
					}
				}
			}
		}
	}
	return ""
}

func mustGitOutput(repoRoot string, args ...string) []byte {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, _ := cmd.Output()
	return out
}

// pruneStagedDeletions removes manifest entries for files whose deletion is
// staged, and returns how many it removed.
func pruneStagedDeletions(manifest *integrity.Manifest, repoRoot string) int {
	out, err := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--name-only", "--no-renames", "--diff-filter=D").Output()
	if err != nil {
		return 0
	}
	pruned := 0
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" {
			continue
		}
		for _, files := range manifest.Checksums {
			if _, ok := files[path]; ok {
				delete(files, path)
				pruned++
			}
		}
	}
	return pruned
}
