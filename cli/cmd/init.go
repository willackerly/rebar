package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
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

	// Bootstrap v2 files if missing
	bootstrapped := bootstrapV2Files(root)

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
	if bootstrapped > 0 {
		fmt.Printf("  Bootstrap:  %d v2 file(s) created\n", bootstrapped)
	}
	fmt.Printf("\nRun 'rebar verify' to check integrity.\n")
	fmt.Println("Run 'rebar context' to view the Cold Start Quad.")

	return nil
}

// bootstrapV2Files creates essential v2 files if they don't exist.
// Returns the number of files created.
func bootstrapV2Files(root string) int {
	created := 0

	// .rebar-version
	versionPath := filepath.Join(root, ".rebar-version")
	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		os.WriteFile(versionPath, []byte("v2.0.0\n"), 0644)
		fmt.Println("  Created .rebar-version")
		created++
	}

	// Cold Start Quad — only create if missing (don't overwrite existing)
	quadFiles := map[string]string{
		"QUICKCONTEXT.md": `# Quick Context

<!-- freshness: ` + time.Now().Format("2006-01-02") + ` -->
<!-- last-synced: ` + time.Now().Format("2006-01-02") + ` -->

**Current state of the project.**

## What's Next (in priority order)

1. Define first contract
2. Set up testing cascade
3. Configure CI enforcement

## Active Work

**In progress:** Initial REBAR setup

## Branch & State

- **Active branch:** main
`,
		"TODO.md": `# TODO

<!-- last-synced: ` + time.Now().Format("2006-01-02") + ` -->

Active tasks only. Priorities live in QUICKCONTEXT.md "What's Next."

## Open Items

- [ ] Define first contract for core component
- [ ] Set up testing cascade (T0-T5)
- [ ] Configure pre-commit hook enforcement

## Known Issues & Blockers

_None currently._

<details>
<summary><strong>Completed</strong></summary>

- [x] REBAR bootstrap — ` + time.Now().Format("2006-01-02") + `

</details>
`,
	}

	for name, content := range quadFiles {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.WriteFile(path, []byte(content), 0644)
			fmt.Printf("  Created %s\n", name)
			created++
		}
	}

	// scripts/refresh-context.sh
	scriptsDir := filepath.Join(root, "scripts")
	refreshPath := filepath.Join(scriptsDir, "refresh-context.sh")
	if _, err := os.Stat(refreshPath); os.IsNotExist(err) {
		os.MkdirAll(scriptsDir, 0755)
		// Find rebar's copy to use as source
		rebarRoot := findRebarRoot()
		if rebarRoot != "" {
			src := filepath.Join(rebarRoot, "scripts", "refresh-context.sh")
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(refreshPath, data, 0755)
				fmt.Println("  Created scripts/refresh-context.sh")
				created++
			}
		}
	}

	// architecture/ directory
	archDir := filepath.Join(root, "architecture")
	if _, err := os.Stat(archDir); os.IsNotExist(err) {
		os.MkdirAll(archDir, 0755)
		// Copy contract template
		rebarRoot := findRebarRoot()
		if rebarRoot != "" {
			src := filepath.Join(rebarRoot, "architecture", "CONTRACT-TEMPLATE.md")
			dst := filepath.Join(archDir, "CONTRACT-TEMPLATE.md")
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0644)
				fmt.Println("  Created architecture/ with contract template")
				created++
			}
		} else {
			os.WriteFile(filepath.Join(archDir, ".gitkeep"), []byte(""), 0644)
			fmt.Println("  Created architecture/")
			created++
		}
	}

	// .mcp.json — Claude Code MCP config for ASK
	if ensureMCPConfig(root) {
		created++
	}

	// agents/ — populated via `ask init` so adopters get a working
	// `ask architect` from minute one. Without this, a brand-new project
	// has .mcp.json wired but zero agents enumerable, and the MCP tool
	// list is empty in Claude Code.
	if ensureAgentsScaffolding(root) {
		created++
	}

	return created
}

// ensureMCPConfig writes a project-local .mcp.json that registers the rebar
// ASK MCP server with Claude Code. Skipped silently if the file already exists.
// Skipped with a message if ask-mcp-server can't be located.
func ensureMCPConfig(root string) bool {
	mcpPath := filepath.Join(root, ".mcp.json")
	if _, err := os.Stat(mcpPath); err == nil {
		return false
	}

	serverPath := findMCPServerPath()
	if serverPath == "" {
		fmt.Println("  Skipped .mcp.json — ask-mcp-server not found; see SETUP.md §MCP to configure manually")
		return false
	}

	// --repos-dir = parent of this project so sibling rebar-adopted repos also register
	reposDir := filepath.Dir(root)

	content := fmt.Sprintf(`{
  "mcpServers": {
    "rebar-ask": {
      "command": %q,
      "args": ["--stdio", "--repos-dir", %q]
    }
  }
}
`, serverPath, reposDir)

	if err := os.WriteFile(mcpPath, []byte(content), 0644); err != nil {
		fmt.Printf("  Warning: could not write .mcp.json: %v\n", err)
		return false
	}
	fmt.Println("  Created .mcp.json (Claude Code MCP wiring for ASK)")
	return true
}

// findMCPServerPath locates the ask-mcp-server executable.
// Tries: same bin/ as this rebar CLI, then findRebarRoot()/bin/, then PATH.
func findMCPServerPath() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "ask-mcp-server")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if rebarRoot := findRebarRoot(); rebarRoot != "" {
		candidate := filepath.Join(rebarRoot, "bin", "ask-mcp-server")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("ask-mcp-server"); err == nil {
		return p
	}
	return ""
}

// findAskBin locates the ask CLI executable using the same lookup chain
// as findMCPServerPath. Returns empty string if not found.
func findAskBin() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "ask")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if rebarRoot := findRebarRoot(); rebarRoot != "" {
		candidate := filepath.Join(rebarRoot, "bin", "ask")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("ask"); err == nil {
		return p
	}
	return ""
}

// ensureAgentsScaffolding runs `ask init` in the project root to populate
// agents/<role>/AGENT.md skeletons. Without this step, a fresh `rebar new`
// or `rebar adopt` produces a project with .mcp.json wired but zero ASK
// agents — and `ask architect` will fail with "no agents directory."
// Skipped silently when agents/ already exists or `ask` can't be located.
func ensureAgentsScaffolding(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "agents")); err == nil {
		return false
	}

	askBin := findAskBin()
	if askBin == "" {
		fmt.Println("  Skipped agents/ — ask CLI not found; run `ask init` from the project to create role agents")
		return false
	}

	cmd := exec.Command(askBin, "init")
	cmd.Dir = root
	// Suppress ask init's verbose output — surface a single tidy line instead.
	if _, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("  ⚠ ask init failed: %v — run `ask init` manually from the project\n", err)
		return false
	}
	fmt.Println("  Created agents/ (architect, product, englead, steward, merger, featurerequest)")
	return true
}

// findRebarRoot locates the rebar framework repo by checking common locations.
func findRebarRoot() string {
	// Check if we're inside the rebar repo
	if _, err := os.Stat("DESIGN.md"); err == nil {
		if _, err := os.Stat("architecture/CONTRACT-TEMPLATE.md"); err == nil {
			cwd, _ := os.Getwd()
			return cwd
		}
	}

	// Check common locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "dev", "rebar"),
		filepath.Join(home, "src", "rebar"),
		filepath.Join(home, "code", "rebar"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "DESIGN.md")); err == nil {
			return c
		}
	}
	return ""
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
