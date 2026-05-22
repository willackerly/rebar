package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExport(t *testing.T) {
	// Setup temp directories
	tmpDir := t.TempDir()
	contractDir := filepath.Join(tmpDir, "architecture")
	outDir := filepath.Join(tmpDir, "specs")

	if err := os.MkdirAll(contractDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy test contract to temp
	testContract := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contractData, err := os.ReadFile(testContract)
	if err != nil {
		t.Fatal(err)
	}

	targetContract := filepath.Join(contractDir, "CONTRACT-AUTH.1.0.md")
	if err := os.WriteFile(targetContract, contractData, 0644); err != nil {
		t.Fatal(err)
	}

	// Export
	opts := ExportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		Force:       true,
	}

	if err := Export(opts); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify exports (filenames use contract ID, not full name)
	gherkinPath := filepath.Join(outDir, "gherkin", "AUTH.1.0.feature")
	if _, err := os.Stat(gherkinPath); os.IsNotExist(err) {
		t.Errorf("Gherkin export not found: %s", gherkinPath)
	}

	mermaidPath := filepath.Join(outDir, "mermaid", "AUTH.1.0.mmd")
	if _, err := os.Stat(mermaidPath); os.IsNotExist(err) {
		t.Errorf("Mermaid export not found: %s", mermaidPath)
	}

	openapiPath := filepath.Join(outDir, "openapi", "AUTH.1.0.yaml")
	if _, err := os.Stat(openapiPath); os.IsNotExist(err) {
		t.Errorf("OpenAPI export not found: %s", openapiPath)
	}

	schemaPath := filepath.Join(outDir, "schemas", "AUTH.1.0.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Errorf("Schema export not found: %s", schemaPath)
	}

	// Verify manifest
	manifestPath := filepath.Join(outDir, ManifestFile)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Errorf("Manifest not found: %s", manifestPath)
	}

	manifest, err := LoadManifest(outDir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if len(manifest.Mappings) == 0 {
		t.Error("Manifest has no mappings")
	}

	mapping := manifest.Mappings[0]
	if mapping.ContractChecksum == "" {
		t.Error("Contract checksum is empty")
	}

	if len(mapping.Exports) == 0 {
		t.Error("No exports in mapping")
	}

	for _, exp := range mapping.Exports {
		if exp.Checksum == "" {
			t.Errorf("Export %s has empty checksum", exp.Path)
		}
	}
}

func TestExportSingleFormat(t *testing.T) {
	tmpDir := t.TempDir()
	contractDir := filepath.Join(tmpDir, "architecture")
	outDir := filepath.Join(tmpDir, "specs")

	if err := os.MkdirAll(contractDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy test contract
	testContract := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contractData, err := os.ReadFile(testContract)
	if err != nil {
		t.Fatal(err)
	}

	targetContract := filepath.Join(contractDir, "CONTRACT-AUTH.1.0.md")
	if err := os.WriteFile(targetContract, contractData, 0644); err != nil {
		t.Fatal(err)
	}

	// Export only Gherkin
	opts := ExportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		Format:      "gherkin",
		Force:       true,
	}

	if err := Export(opts); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify only Gherkin exported
	gherkinPath := filepath.Join(outDir, "gherkin", "AUTH.1.0.feature")
	if _, err := os.Stat(gherkinPath); os.IsNotExist(err) {
		t.Errorf("Gherkin export not found")
	}

	mermaidPath := filepath.Join(outDir, "mermaid", "AUTH.1.0.mmd")
	if _, err := os.Stat(mermaidPath); err == nil {
		t.Error("Mermaid should not be exported with format=gherkin")
	}
}

func TestExportDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	contractDir := filepath.Join(tmpDir, "architecture")
	outDir := filepath.Join(tmpDir, "specs")

	if err := os.MkdirAll(contractDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy test contract
	testContract := filepath.Join("testdata", "CONTRACT-AUTH.1.0.md")
	contractData, err := os.ReadFile(testContract)
	if err != nil {
		t.Fatal(err)
	}

	targetContract := filepath.Join(contractDir, "CONTRACT-AUTH.1.0.md")
	if err := os.WriteFile(targetContract, contractData, 0644); err != nil {
		t.Fatal(err)
	}

	// Export with dry-run
	opts := ExportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		DryRun:      true,
	}

	if err := Export(opts); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify no files created
	if _, err := os.Stat(outDir); err == nil {
		entries, _ := os.ReadDir(outDir)
		if len(entries) > 0 {
			t.Error("Files were created in dry-run mode")
		}
	}
}
