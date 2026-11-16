package cmd

import (
	"crypto/rand"
	"fmt"
	"github.com/willackerly/rebar/cli/internal/ask"
	"io"
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
		os.WriteFile(versionPath, []byte(badgeVersion()+"\n"), 0644)
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

	// scripts/ — the thin entry points from rebar's
	// templates/project-bootstrap/scripts/ (identical to what the
	// `cp -r templates/project-bootstrap/. <target>/` adoption path
	// delivers); the checks themselves run from the rebar install.
	// Never overwrites existing files.
	scriptsDir := filepath.Join(root, "scripts")
	if rebarRoot := findRebarRoot(); rebarRoot == "" {
		fmt.Println("  ⚠ Skipped scripts/ and .claude/ — rebar checkout not found (set REBAR_ROOT to your rebar clone)")
	} else {
		srcScripts := filepath.Join(rebarRoot, "templates", "project-bootstrap", "scripts")
		copiedScripts := 0
		if entries, err := os.ReadDir(srcScripts); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				dst := filepath.Join(scriptsDir, e.Name())
				if _, err := os.Stat(dst); err == nil {
					continue
				}
				data, readErr := os.ReadFile(filepath.Join(srcScripts, e.Name()))
				if readErr != nil {
					continue
				}
				os.MkdirAll(scriptsDir, 0755)
				os.WriteFile(dst, data, 0755)
				copiedScripts++
			}
		}
		if copiedScripts > 0 {
			fmt.Printf("  Created scripts/ (%d files: ci-check.sh + pre-commit.sh entry points; checks run from the rebar install)\n", copiedScripts)
			created++
		}
	}

	// contracts directory (architecture/ unless .rebarrc contracts_dir says otherwise)
	contractsDir := config.ContractsDirFor(root)
	archDir := filepath.Join(root, contractsDir)
	if _, err := os.Stat(archDir); os.IsNotExist(err) {
		os.MkdirAll(archDir, 0755)
		// Copy contract template
		rebarRoot := findRebarRoot()
		if rebarRoot != "" {
			src := filepath.Join(rebarRoot, "architecture", "CONTRACT-TEMPLATE.md")
			dst := filepath.Join(archDir, "CONTRACT-TEMPLATE.md")
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0644)
				fmt.Printf("  Created %s/ with contract template\n", contractsDir)
				created++
			}
		} else {
			os.WriteFile(filepath.Join(archDir, ".gitkeep"), []byte(""), 0644)
			fmt.Printf("  Created %s/\n", contractsDir)
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

	// .claude/ — SessionStart cold-start hook + rebar skills (v3).
	// Copied from rebar's templates/project-bootstrap/.claude/ so the
	// canonical content lives in one place; skipped silently when the
	// rebar checkout can't be located (same contract as refresh-context).
	if ensureClaudeAssets(root) {
		created++
	}

	return created
}

// ensureClaudeAssets copies templates/project-bootstrap/.claude/ (settings.json
// with the SessionStart hook, skills/*/SKILL.md) into the project. Existing
// files are never overwritten.
func ensureClaudeAssets(root string) bool {
	rebarRoot := findRebarRoot()
	if rebarRoot == "" {
		return false
	}
	srcDir := filepath.Join(rebarRoot, "templates", "project-bootstrap", ".claude")
	if _, err := os.Stat(srcDir); err != nil {
		return false
	}
	dstDir := filepath.Join(root, ".claude")
	copied := 0
	hookInstalled := false
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return nil
		}
		dst := filepath.Join(dstDir, rel)
		if _, statErr := os.Stat(dst); statErr == nil {
			return nil // never overwrite
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		os.MkdirAll(filepath.Dir(dst), 0755)
		if os.WriteFile(dst, data, 0644) == nil {
			copied++
			if rel == "settings.json" {
				hookInstalled = true
			}
		}
		return nil
	})
	if copied > 0 {
		if hookInstalled {
			fmt.Printf("  Created .claude/ (%d files: SessionStart hook + rebar skills)\n", copied)
		} else {
			fmt.Printf("  Created .claude/skills (%d files; existing settings.json preserved — merge the SessionStart hook manually from templates/project-bootstrap/.claude/settings.json)\n", copied)
		}
		return true
	}
	return false
}

// ensureMCPConfig writes a project-local .mcp.json that registers rebar's
// ASK MCP server (`rebar mcp serve --stdio`) with Claude Code. Skipped
// silently if the file already exists.
func ensureMCPConfig(root string) bool {
	mcpPath := filepath.Join(root, ".mcp.json")
	if _, err := os.Stat(mcpPath); err == nil {
		return false
	}

	rebar := findRebarCommand()
	if rebar == "" {
		fmt.Println("  Skipped .mcp.json — the rebar binary's path is unknown; see docs/MCP-SETUP.md to configure manually")
		return false
	}

	// --repos-dir = parent of this project so sibling rebar-adopted repos also register
	reposDir := filepath.Dir(root)

	content := fmt.Sprintf(`{
  "mcpServers": {
    "rebar-ask": {
      "command": %q,
      "args": ["mcp", "serve", "--stdio", "--repos-dir", %q]
    }
  }
}
`, rebar, reposDir)

	if err := os.WriteFile(mcpPath, []byte(content), 0644); err != nil {
		fmt.Printf("  Warning: could not write .mcp.json: %v\n", err)
		return false
	}
	fmt.Println("  Created .mcp.json (Claude Code MCP wiring for ASK)")
	return true
}

// findRebarCommand is the rebar binary .mcp.json runs: this one (through
// ~/.rebar/current when that is the active version), else the install's
// bin/rebar, else rebar on PATH.
func findRebarCommand() string {
	exe, _ := os.Executable()
	return rebarCommandFrom(exe)
}

// stableBinDir maps <base>/versions/<v>/bin to <base>/current/bin when
// <base>/current points at that version, so paths recorded in project files
// (.mcp.json) follow version switches and survive removing old versions. On
// Linux os.Executable() resolves the current symlink; on macOS it is
// usually the invoked path already. Any other directory is returned as is.
func stableBinDir(dir string) string {
	verDir := filepath.Dir(dir)
	if filepath.Base(dir) != "bin" || filepath.Base(filepath.Dir(verDir)) != "versions" {
		return dir
	}
	current := filepath.Join(filepath.Dir(filepath.Dir(verDir)), "current")
	cur, err := filepath.EvalSymlinks(current)
	if err != nil {
		return dir
	}
	ver, err := filepath.EvalSymlinks(verDir)
	if err != nil || cur != ver {
		return dir
	}
	return filepath.Join(current, "bin")
}

// rebarCommandFrom is findRebarCommand for a given executable path ("" if
// unknown).
func rebarCommandFrom(exe string) string {
	if exe != "" && filepath.Base(exe) == "rebar" {
		candidate := filepath.Join(stableBinDir(filepath.Dir(exe)), "rebar")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return exe
	}
	if rebarRoot := findRebarRoot(); rebarRoot != "" {
		candidate := filepath.Join(rebarRoot, "bin", "rebar")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("rebar"); err == nil {
		return p
	}
	return exe
}

// ensureAgentsScaffolding runs `ask init` (rebar's native ASK) in the
// project root to populate agents/<role>/AGENT.md skeletons. Without this
// step, a fresh `rebar new` or `rebar adopt` produces a project with
// .mcp.json wired but zero ASK agents — and `ask architect` will fail with
// "no agents directory." Skipped silently when agents/ already exists.
func ensureAgentsScaffolding(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "agents")); err == nil {
		return false
	}
	// Suppress ask init's verbose output — surface a single tidy line instead.
	if code := ask.RunIn(root, []string{"init"}, io.Discard, io.Discard); code != 0 {
		fmt.Printf("  ⚠ ask init failed (exit %d) — run `ask init` manually from the project\n", code)
		return false
	}
	fmt.Println("  Created agents/ (architect, product, englead, steward, merger, featurerequest)")
	return true
}

// findRebarRoot locates the rebar framework repo (a source checkout, a
// setup-rebar.sh install under ~/.rebar/versions/<ver>/, or a pre-versioned
// ~/.rebar clone). The marker is
// templates/project-bootstrap/ plus the scripts/ the CLI runs — only the
// rebar SOURCE repo has both; DESIGN.md + architecture/ alone also match
// rebar-adopted projects, which made re-init in an adopted repo copy from
// itself (a no-op).
func findRebarRoot() string {
	isRebarSource := func(dir string) bool {
		if dir == "" {
			return false
		}
		return isRebarSourceDir(dir)
	}

	// Explicit override first
	if env := os.Getenv("REBAR_ROOT"); isRebarSource(env) {
		return env
	}

	// Relative to the running binary — covers the setup-rebar.sh install
	// (~/.rebar/current/bin/rebar resolves to ~/.rebar/versions/<ver>), a
	// pre-versioned ~/.rebar clone, and any checkout's bin/rebar.
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		if root := filepath.Dir(filepath.Dir(exe)); isRebarSource(root) {
			return root
		}
	}

	// Running from inside the rebar source repo itself
	if cwd, err := os.Getwd(); err == nil && isRebarSource(cwd) {
		return cwd
	}

	// Common locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".rebar", "current"),
		filepath.Join(home, ".rebar"),
		filepath.Join(home, "dev", "rebar"),
		filepath.Join(home, "src", "rebar"),
		filepath.Join(home, "code", "rebar"),
	}
	for _, c := range candidates {
		if isRebarSource(c) {
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

// isRebarSourceDir reports whether dir is a rebar source checkout or install.
// Shared with scripts/rebar-doc.sh, which tests the same two paths.
func isRebarSourceDir(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "templates", "project-bootstrap")); err != nil || !info.IsDir() {
		return false
	}
	return isFile(filepath.Join(dir, "scripts", "steward.sh"))
}
