package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/willackerly/rebar/cli/internal/config"
)

// setupTestRepo creates a temporary git repo with the Cold Start Quad
// and optional extra files. Returns the repo root path and a cleanup func.
func setupTestRepo(t *testing.T, extraFiles map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")

	// Create Cold Start Quad
	writeFile(t, dir, "README.md", "# Test Project\nThis is a test project.\n")
	writeFile(t, dir, "QUICKCONTEXT.md", `# Quick Context
<!-- freshness: 2026-03-30 -->
<!-- last-synced: 2026-03-30 -->

## What's Next
1. Build the thing
`)
	writeFile(t, dir, "TODO.md", "# TODO\n\n- [ ] First task\n")
	writeFile(t, dir, "AGENTS.md", "# Agent Guidelines\n\nHow we work.\n")

	// Create extra files
	for path, content := range extraFiles {
		writeFile(t, dir, path, content)
	}

	// Initial commit so git operations work
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", "initial")

	return dir
}

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// --- Tests for roleFiles mapping ---

func TestRoleFilesContainsColdStartQuad(t *testing.T) {
	coldStart := []string{"README.md", "QUICKCONTEXT.md", "TODO.md", "AGENTS.md"}

	for role, patterns := range roleFiles {
		for _, required := range coldStart {
			found := false
			for _, p := range patterns {
				if p == required {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("role %q missing Cold Start Quad file %q", role, required)
			}
		}
	}
}

func TestRoleFilesOrder(t *testing.T) {
	// Cold Start Quad must be the first 4 entries in every role
	coldStart := []string{"README.md", "QUICKCONTEXT.md", "TODO.md", "AGENTS.md"}

	for role, patterns := range roleFiles {
		if len(patterns) < 4 {
			t.Errorf("role %q has fewer than 4 patterns", role)
			continue
		}
		for i, expected := range coldStart {
			if patterns[i] != expected {
				t.Errorf("role %q: pattern[%d] = %q, want %q (Cold Start Quad must be first)",
					role, i, patterns[i], expected)
			}
		}
	}
}

func TestAllRolesRegistered(t *testing.T) {
	expectedRoles := []string{"", "architect", "product", "security", "developer"}
	for _, role := range expectedRoles {
		if _, ok := roleFiles[role]; !ok {
			t.Errorf("role %q not registered in roleFiles", role)
		}
	}
}

func TestArchitectRoleIncludesDesignAndContracts(t *testing.T) {
	patterns := roleFiles["architect"]
	hasDesign := false
	hasContracts := false
	for _, p := range patterns {
		if p == "DESIGN.md" {
			hasDesign = true
		}
		if strings.Contains(p, "architecture/CONTRACT-") {
			hasContracts = true
		}
	}
	if !hasDesign {
		t.Error("architect role missing DESIGN.md")
	}
	if !hasContracts {
		t.Error("architect role missing architecture/CONTRACT-*.md pattern")
	}
}

func TestSecurityRoleIncludesClaudeAndSecurityContracts(t *testing.T) {
	patterns := roleFiles["security"]
	hasClaude := false
	hasAuthContracts := false
	hasCryptoContracts := false
	for _, p := range patterns {
		if p == "CLAUDE.md" {
			hasClaude = true
		}
		if strings.Contains(p, "AUTH") {
			hasAuthContracts = true
		}
		if strings.Contains(p, "CRYPTO") {
			hasCryptoContracts = true
		}
	}
	if !hasClaude {
		t.Error("security role missing CLAUDE.md")
	}
	if !hasAuthContracts {
		t.Error("security role missing AUTH contract pattern")
	}
	if !hasCryptoContracts {
		t.Error("security role missing CRYPTO contract pattern")
	}
}

func TestProductRoleIncludesProductFiles(t *testing.T) {
	patterns := roleFiles["product"]
	hasPersonas := false
	hasFeatures := false
	for _, p := range patterns {
		if strings.Contains(p, "product/personas") {
			hasPersonas = true
		}
		if strings.Contains(p, "product/features") {
			hasFeatures = true
		}
	}
	if !hasPersonas {
		t.Error("product role missing product/personas pattern")
	}
	if !hasFeatures {
		t.Error("product role missing product/features pattern")
	}
}

// --- Tests for unknown role handling ---

func TestUnknownRoleReturnsError(t *testing.T) {
	repoDir := setupTestRepo(t, nil)
	setupCfg(repoDir)

	err := runContext(&cobra.Command{}, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown role, got nil")
	}
	if !strings.Contains(err.Error(), "unknown role") {
		t.Errorf("error should mention 'unknown role', got: %v", err)
	}
	if !strings.Contains(err.Error(), "architect") {
		t.Errorf("error should list available roles, got: %v", err)
	}
}

// --- Integration tests with real temp repos ---

func TestContextDefaultPrintsColdStartQuad(t *testing.T) {
	repoDir := setupTestRepo(t, nil)
	setupCfg(repoDir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{})
	})

	// Should contain all 4 Cold Start Quad files
	for _, file := range []string{"README.md", "QUICKCONTEXT.md", "TODO.md", "AGENTS.md"} {
		header := "═══ " + file + " ═══"
		if !strings.Contains(output, header) {
			t.Errorf("output missing header for %s", file)
		}
	}

	// Should contain actual content
	if !strings.Contains(output, "Test Project") {
		t.Error("output missing README content")
	}
	if !strings.Contains(output, "Quick Context") {
		t.Error("output missing QUICKCONTEXT content")
	}
}

func TestContextArchitectIncludesContractsAndDesign(t *testing.T) {
	repoDir := setupTestRepo(t, map[string]string{
		"DESIGN.md": "# Methodology\n\nThe full philosophy.\n",
		"architecture/CONTRACT-C1-AUTH.1.0.md":     "# Auth Contract\n",
		"architecture/CONTRACT-S2-STORAGE.1.0.md":  "# Storage Contract\n",
		"architecture/.state/report.json":          `{"should": "be skipped"}`,
		"architecture/CONTRACT-REGISTRY.template.md": "# Template — should be skipped\n",
	})
	setupCfg(repoDir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{"architect"})
	})

	// Should include DESIGN.md and contracts
	if !strings.Contains(output, "═══ DESIGN.md ═══") {
		t.Error("architect output missing DESIGN.md")
	}
	if !strings.Contains(output, "Auth Contract") {
		t.Error("architect output missing auth contract content")
	}
	if !strings.Contains(output, "Storage Contract") {
		t.Error("architect output missing storage contract content")
	}

	// Should skip .state/ files and templates
	if strings.Contains(output, "should be skipped") {
		t.Error("architect output should skip .state/ files")
	}
	if strings.Contains(output, "Template — should be skipped") {
		t.Error("architect output should skip .template.md files")
	}
}

func TestContextSecurityIncludesAuthAndCryptoContracts(t *testing.T) {
	repoDir := setupTestRepo(t, map[string]string{
		"CLAUDE.md": "# Claude Config\n\n## Hard Rules\nNO server-side signing.\n",
		"architecture/CONTRACT-S1-AUTH.1.0.md":        "# Auth Contract\n",
		"architecture/CONTRACT-C3-CRYPTO-BRIDGE.1.0.md": "# Crypto Contract\n",
		"architecture/CONTRACT-S2-STORAGE.1.0.md":     "# Storage — should NOT appear\n",
	})
	setupCfg(repoDir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{"security"})
	})

	if !strings.Contains(output, "Claude Config") {
		t.Error("security output missing CLAUDE.md")
	}
	if !strings.Contains(output, "Auth Contract") {
		t.Error("security output missing AUTH contract")
	}
	if !strings.Contains(output, "Crypto Contract") {
		t.Error("security output missing CRYPTO contract")
	}
	// Storage contract should NOT appear in security context
	if strings.Contains(output, "Storage — should NOT appear") {
		t.Error("security output should NOT include non-security contracts")
	}
}

func TestContextProductIncludesPersonasAndFeatures(t *testing.T) {
	repoDir := setupTestRepo(t, map[string]string{
		"product/personas/sarah.md":           "# Sarah\nSecurity analyst.\n",
		"product/features/auth.feature":       "Feature: Authentication\n",
		"product/features/encryption.md":      "# Encryption Feature\n",
		"product/user-stories/upload-doc.md":  "# Upload Document\n",
	})
	setupCfg(repoDir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{"product"})
	})

	if !strings.Contains(output, "Sarah") {
		t.Error("product output missing persona content")
	}
	if !strings.Contains(output, "Feature: Authentication") {
		t.Error("product output missing .feature content")
	}
	if !strings.Contains(output, "Encryption Feature") {
		t.Error("product output missing features .md content")
	}
	if !strings.Contains(output, "Upload Document") {
		t.Error("product output missing user stories content")
	}
}

func TestContextMissingFilesSkippedGracefully(t *testing.T) {
	// Create a minimal repo with only README — other Cold Start Quad files missing
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, "README.md", "# Just README\n")
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", "init")
	setupCfg(dir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{})
	})

	// Should still print README
	if !strings.Contains(output, "Just README") {
		t.Error("output should include README even when other files are missing")
	}
	// Should NOT error — missing files are skipped
	if !strings.Contains(output, "═══ README.md ═══") {
		t.Error("output should have README header")
	}
}

func TestContextFiltersTemplateAndStateFiles(t *testing.T) {
	repoDir := setupTestRepo(t, map[string]string{
		"DESIGN.md": "# Real Design\n",
		"architecture/CONTRACT-C1-FOO.1.0.md":            "# Real Contract\n",
		"architecture/CONTRACT-REGISTRY.template.md":      "# Template Skip\n",
		"architecture/.state/steward-report.json":         `{"skip": true}`,
	})
	setupCfg(repoDir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{"architect"})
	})

	if !strings.Contains(output, "Real Contract") {
		t.Error("should include real contracts")
	}
	if strings.Contains(output, "Template Skip") {
		t.Error("should skip .template.md files")
	}
	if strings.Contains(output, `"skip"`) {
		t.Error("should skip .state/ files")
	}
}

func TestContextOutputFormat(t *testing.T) {
	repoDir := setupTestRepo(t, nil)
	setupCfg(repoDir)

	output := captureOutput(t, func() error {
		return runContext(&cobra.Command{}, []string{})
	})

	// Verify the separator format
	if !strings.Contains(output, "═══ README.md ═══") {
		t.Error("output should use ═══ separators")
	}

	// Verify files appear in Cold Start Quad order
	readmeIdx := strings.Index(output, "═══ README.md ═══")
	qcIdx := strings.Index(output, "═══ QUICKCONTEXT.md ═══")
	todoIdx := strings.Index(output, "═══ TODO.md ═══")
	agentsIdx := strings.Index(output, "═══ AGENTS.md ═══")

	if readmeIdx >= qcIdx {
		t.Error("README should appear before QUICKCONTEXT")
	}
	if qcIdx >= todoIdx {
		t.Error("QUICKCONTEXT should appear before TODO")
	}
	if todoIdx >= agentsIdx {
		t.Error("TODO should appear before AGENTS")
	}
}

// --- Tests for extractDate / extractQCDate ---

func TestExtractDate(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		key      string
		expected string
	}{
		{
			name:     "standard last-synced",
			text:     "<!-- last-synced: 2026-03-30 -->",
			key:      "last-synced:",
			expected: "2026-03-30",
		},
		{
			name:     "freshness date",
			text:     "<!-- freshness: 2026-04-01 -->",
			key:      "freshness:",
			expected: "2026-04-01",
		},
		{
			name:     "date with extra text",
			text:     "<!-- last-synced: 2026-03-30 — date this file was verified -->",
			key:      "last-synced:",
			expected: "2026-03-30",
		},
		{
			name:     "multiline with date on second line",
			text:     "# Header\n<!-- last-synced: 2026-01-15 -->\nContent",
			key:      "last-synced:",
			expected: "2026-01-15",
		},
		{
			name:     "no date present",
			text:     "# Just a header\nNo dates here.",
			key:      "last-synced:",
			expected: "",
		},
		{
			name:     "wrong key",
			text:     "<!-- freshness: 2026-03-30 -->",
			key:      "last-synced:",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDate(tt.text, tt.key)
			if result != tt.expected {
				t.Errorf("extractDate(%q, %q) = %q, want %q", tt.text, tt.key, result, tt.expected)
			}
		})
	}
}

func TestExtractQCDate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "last-synced preferred",
			content:  "<!-- last-synced: 2026-03-30 -->\n<!-- freshness: 2026-03-25 -->",
			expected: "2026-03-30",
		},
		{
			name:     "freshness as fallback",
			content:  "<!-- freshness: 2026-03-25 -->",
			expected: "2026-03-25",
		},
		{
			name:     "no date",
			content:  "# Quick Context\nNo dates.\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractQCDate(tt.content)
			if result != tt.expected {
				t.Errorf("extractQCDate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// --- Helpers ---

// setupCfg sets the global cfg for tests, simulating what PersistentPreRunE does.
func setupCfg(repoRoot string) {
	cfg = &config.Config{
		RepoRoot:   repoRoot,
		RebarDir:   filepath.Join(repoRoot, ".rebar"),
		ScriptsDir: filepath.Join(repoRoot, "scripts"),
		AgentsDir:  filepath.Join(repoRoot, "agents"),
		BinDir:     filepath.Join(repoRoot, "bin"),
		Tier:       2,
	}
}

// captureOutput captures stdout during a function call.
func captureOutput(t *testing.T, fn func() error) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w

	fnErr := fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading pipe: %v", err)
	}

	if fnErr != nil {
		t.Fatalf("function returned error: %v", fnErr)
	}

	return buf.String()
}
