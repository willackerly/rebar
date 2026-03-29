package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/config"
	"github.com/willackerly/rebar/cli/internal/integrity"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a REBAR repository",
	Long:  `Creates .rebar/ directory with integrity manifest, salt, and configuration.`,
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "re-generate salt and re-hash (for re-keying after clone)")
	// init handles its own repo root
	_ = rand.Reader // ensure crypto/rand is imported
}

func runInit(cmd *cobra.Command, args []string) error {
	// Use current dir or --repo-root
	root := repoRoot
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	rebarDir := filepath.Join(root, ".rebar")
	existing := false
	if _, err := os.Stat(rebarDir); err == nil {
		existing = true
		if !forceInit {
			fmt.Println("REBAR already initialized. Use --force to re-generate salt.")
		}
	}

	// Create directory structure
	if err := config.EnsureRebarDir(root); err != nil {
		return err
	}

	// Generate or preserve repo ID
	repoIDPath := filepath.Join(rebarDir, "repo-id")
	repoID := ""
	if data, err := os.ReadFile(repoIDPath); err == nil && !forceInit {
		repoID = strings.TrimSpace(string(data))
	}
	if repoID == "" {
		repoID = uuid.New().String()
		if err := os.WriteFile(repoIDPath, []byte(repoID+"\n"), 0644); err != nil {
			return fmt.Errorf("writing repo-id: %w", err)
		}
	}

	// Generate salt (always on first init, or with --force)
	saltPath := filepath.Join(rebarDir, "salt")
	if _, err := os.Stat(saltPath); os.IsNotExist(err) || forceInit {
		salt, err := integrity.GenerateSalt()
		if err != nil {
			return err
		}
		if err := integrity.SaveSalt(rebarDir, salt); err != nil {
			return err
		}
		fmt.Println("Generated integrity salt")
	}

	// Create or update manifest
	var manifest *integrity.Manifest
	if existing && !forceInit {
		manifest, _ = integrity.LoadManifest(rebarDir)
	}
	if manifest == nil {
		manifest = integrity.NewManifest(repoID)
	}

	// Scan and hash all protected files
	salt, _ := integrity.LoadSalt(rebarDir)
	files, err := integrity.ScanProtectedFiles(root)
	if err != nil {
		return fmt.Errorf("scanning files: %w", err)
	}

	totalFiles := 0
	for cat, paths := range files {
		if manifest.Checksums[cat] == nil {
			manifest.Checksums[cat] = map[string]integrity.FileEntry{}
		}
		role := integrity.DefaultRoleForCategory(cat)
		for _, p := range paths {
			fullPath := filepath.Join(root, p)
			hash, err := integrity.HashFile(fullPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not hash %s: %v\n", p, err)
				continue
			}

			entry := integrity.FileEntry{
				SHA256:     hash,
				Role:       role,
				ModifiedAt: time.Now().UTC(),
			}

			// Compute role HMAC if salt available
			if salt != nil {
				roleSalt := integrity.ComputeRoleSalt(salt, role)
				entry.RoleHMAC = integrity.ComputeRoleHMAC(roleSalt, hash)
			}

			// Count assertions for test files
			if cat == integrity.CategoryTests {
				count, err := integrity.CountAssertions(fullPath)
				if err == nil {
					entry.AssertCount = &count
				}
			}

			manifest.Checksums[cat][p] = entry
			totalFiles++
		}
	}

	// Compute initial ratchets
	computed, err := integrity.ComputeRatchets(root)
	if err == nil {
		integrity.UpdateRatchets(manifest, computed)
	}

	manifest.GeneratedBy = "rebar init"
	if err := manifest.Save(rebarDir); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	// Ensure .gitignore covers secrets
	ensureGitignore(root)

	// Create .rebarrc if missing
	ensureRebarRC(root)

	// Summary
	fmt.Printf("\nREBAR initialized\n")
	fmt.Printf("  Repo ID:    %s\n", repoID)
	fmt.Printf("  Directory:  %s\n", rebarDir)
	fmt.Printf("  Protected:  %d files tracked\n", totalFiles)
	for cat, paths := range files {
		if len(paths) > 0 {
			fmt.Printf("    %-14s %d files\n", cat+":", len(paths))
		}
	}
	if cfg != nil {
		fmt.Printf("  Tier:       %d\n", cfg.Tier)
	}
	fmt.Printf("\nRun 'rebar verify' to check integrity.\n")

	return nil
}

func ensureGitignore(root string) {
	gitignorePath := filepath.Join(root, ".gitignore")
	content, _ := os.ReadFile(gitignorePath)
	lines := string(content)

	additions := []string{}
	if !strings.Contains(lines, ".rebar/salt") {
		additions = append(additions, ".rebar/salt")
	}
	if !strings.Contains(lines, ".rebar/keys/") {
		additions = append(additions, ".rebar/keys/")
	}

	if len(additions) > 0 {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		if len(content) > 0 && content[len(content)-1] != '\n' {
			f.WriteString("\n")
		}
		f.WriteString("\n# REBAR integrity secrets\n")
		for _, a := range additions {
			f.WriteString(a + "\n")
		}
	}
}

func ensureRebarRC(root string) {
	rcPath := filepath.Join(root, ".rebarrc")
	if _, err := os.Stat(rcPath); err == nil {
		return // already exists
	}
	content := `# REBAR Configuration
# See: https://github.com/willackerly/rebar

# Enforcement tier (1=Partial, 2=Adopted, 3=Enforced)
tier = 1
`
	os.WriteFile(rcPath, []byte(content), 0644)
}
