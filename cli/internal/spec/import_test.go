package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImport(t *testing.T) {
	tmpDir := t.TempDir()
	contractDir := filepath.Join(tmpDir, "architecture")
	specDir := filepath.Join(tmpDir, "specs")

	if err := os.MkdirAll(contractDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy test spec
	testSpec := filepath.Join("testdata", "sample.feature")
	specData, err := os.ReadFile(testSpec)
	if err != nil {
		t.Fatal(err)
	}

	targetSpec := filepath.Join(specDir, "sample.feature")
	if err := os.WriteFile(targetSpec, specData, 0644); err != nil {
		t.Fatal(err)
	}

	// Import
	opts := ImportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		SpecPaths:   []string{targetSpec},
		Force:       true,
	}

	if err := Import(opts); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Verify contract created
	contractPath := filepath.Join(contractDir, "CONTRACT-SAMPLE.1.0.md")
	if _, err := os.Stat(contractPath); os.IsNotExist(err) {
		t.Errorf("Contract not created: %s", contractPath)
	}

	// Verify contract content
	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("Reading contract: %v", err)
	}

	content := string(contractData)
	if !contains(content, "CONTRACT-SAMPLE.1.0") {
		t.Error("Contract doesn't contain expected ID")
	}

	if !contains(content, "Feature: User Registration") {
		t.Error("Contract doesn't contain Gherkin content")
	}

	if !contains(content, "```gherkin") {
		t.Error("Contract doesn't have Gherkin code block")
	}
}

func TestImportForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	contractDir := filepath.Join(tmpDir, "architecture")
	specDir := filepath.Join(tmpDir, "specs")

	if err := os.MkdirAll(contractDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing contract
	existingContract := filepath.Join(contractDir, "CONTRACT-SAMPLE.1.0.md")
	if err := os.WriteFile(existingContract, []byte("# Existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy test spec
	testSpec := filepath.Join("testdata", "sample.feature")
	specData, err := os.ReadFile(testSpec)
	if err != nil {
		t.Fatal(err)
	}

	targetSpec := filepath.Join(specDir, "sample.feature")
	if err := os.WriteFile(targetSpec, specData, 0644); err != nil {
		t.Fatal(err)
	}

	// Import with force (should overwrite)
	opts := ImportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		SpecPaths:   []string{targetSpec},
		Force:       true,
	}

	if err := Import(opts); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Verify contract overwritten
	contractData, err := os.ReadFile(existingContract)
	if err != nil {
		t.Fatal(err)
	}

	if contains(string(contractData), "# Existing") {
		t.Error("Contract was not overwritten")
	}

	if !contains(string(contractData), "Feature: User Registration") {
		t.Error("Contract doesn't contain new content")
	}
}

func TestImportDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	contractDir := filepath.Join(tmpDir, "architecture")
	specDir := filepath.Join(tmpDir, "specs")

	if err := os.MkdirAll(contractDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy test spec
	testSpec := filepath.Join("testdata", "sample.feature")
	specData, err := os.ReadFile(testSpec)
	if err != nil {
		t.Fatal(err)
	}

	targetSpec := filepath.Join(specDir, "sample.feature")
	if err := os.WriteFile(targetSpec, specData, 0644); err != nil {
		t.Fatal(err)
	}

	// Import with dry-run
	opts := ImportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		SpecPaths:   []string{targetSpec},
		DryRun:      true,
	}

	if err := Import(opts); err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Verify no contract created
	contractPath := filepath.Join(contractDir, "CONTRACT-SAMPLE.1.0.md")
	if _, err := os.Stat(contractPath); err == nil {
		t.Error("Contract was created in dry-run mode")
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename string
		want     SpecFormat
	}{
		{"test.feature", FormatGherkin},
		{"test.mmd", FormatMermaid},
		{"test.yaml", ""},      // Need content check for OpenAPI
		{"test.json", ""},      // Need content check for Schema
		{"test.txt", ""},
		{"test", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte{}, 0644); err != nil {
				t.Fatal(err)
			}

			got := detectFormat(path)
			if got != tt.want {
				t.Errorf("detectFormat(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
