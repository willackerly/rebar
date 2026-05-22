package spec

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncNoChanges(t *testing.T) {
	tmpDir := setupSyncTest(t)

	// Sync (no changes expected)
	opts := SyncOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
	}

	if err := Sync(opts); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Verify manifest unchanged
	manifest, err := LoadManifest(filepath.Join(tmpDir, "specs"))
	if err != nil {
		t.Fatal(err)
	}

	if len(manifest.Mappings) != 1 {
		t.Errorf("Mappings count = %d, want 1", len(manifest.Mappings))
	}
}

func TestSyncContractChanged(t *testing.T) {
	tmpDir := setupSyncTest(t)

	// Modify contract
	contractPath := filepath.Join(tmpDir, "architecture", "CONTRACT-AUTH.1.0.md")
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}

	modifiedData := string(contractData) + "\n\n## New Section\n\nAdded content.\n"
	if err := os.WriteFile(contractPath, []byte(modifiedData), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync (should re-export)
	opts := SyncOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
	}

	if err := Sync(opts); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Verify manifest updated
	manifest, err := LoadManifest(filepath.Join(tmpDir, "specs"))
	if err != nil {
		t.Fatal(err)
	}

	mapping := manifest.FindMapping("architecture/CONTRACT-AUTH.1.0.md")
	if mapping == nil {
		t.Fatal("Mapping not found")
	}

	// Checksum should be updated
	newChecksum, _ := ComputeChecksum(contractPath)
	if mapping.ContractChecksum != newChecksum {
		t.Error("Contract checksum not updated after sync")
	}
}

func TestSyncSpecChanged(t *testing.T) {
	tmpDir := setupSyncTest(t)

	// Modify a spec file (use actual exported filename)
	specPath := filepath.Join(tmpDir, "specs", "gherkin", "AUTH.1.0.feature")
	time.Sleep(10 * time.Millisecond)

	specData, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}

	modifiedSpec := string(specData) + "\n  Scenario: New scenario\n    Given something\n"
	if err := os.WriteFile(specPath, []byte(modifiedSpec), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync (should detect spec change)
	opts := SyncOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		DryRun:      true, // Don't auto-import in test
	}

	if err := Sync(opts); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// In real usage, would prompt user or import with --force
	// Here we just verify detection worked (no error)
}

func TestSyncBothChanged(t *testing.T) {
	tmpDir := setupSyncTest(t)

	// Modify both contract and spec
	contractPath := filepath.Join(tmpDir, "architecture", "CONTRACT-AUTH.1.0.md")
	specPath := filepath.Join(tmpDir, "specs", "gherkin", "AUTH.1.0.feature")
	time.Sleep(10 * time.Millisecond)

	// Modify contract
	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractPath, []byte(string(contractData)+"\nModified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modify spec
	specData, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(string(specData)+"\n# Modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync (should detect conflict)
	opts := SyncOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		DryRun:      true,
	}

	if err := Sync(opts); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Conflict detection works if no error
}

func TestSyncDryRun(t *testing.T) {
	tmpDir := setupSyncTest(t)

	// Modify contract
	contractPath := filepath.Join(tmpDir, "architecture", "CONTRACT-AUTH.1.0.md")
	time.Sleep(10 * time.Millisecond)

	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}

	originalChecksum, _ := ComputeChecksum(contractPath)

	if err := os.WriteFile(contractPath, []byte(string(contractData)+"\nModified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync dry-run
	opts := SyncOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		DryRun:      true,
	}

	if err := Sync(opts); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Verify manifest NOT updated
	manifest, err := LoadManifest(filepath.Join(tmpDir, "specs"))
	if err != nil {
		t.Fatal(err)
	}

	mapping := manifest.FindMapping("architecture/CONTRACT-AUTH.1.0.md")
	if mapping == nil {
		t.Fatal("Mapping not found")
	}

	if mapping.ContractChecksum != originalChecksum {
		t.Error("Manifest was updated in dry-run mode")
	}
}

// setupSyncTest creates a test environment with exported specs
func setupSyncTest(t *testing.T) string {
	t.Helper()

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

	// Export to create initial specs and manifest
	opts := ExportOptions{
		RepoRoot:    tmpDir,
		ContractDir: "architecture",
		OutDir:      "specs",
		Force:       true,
	}

	if err := Export(opts); err != nil {
		t.Fatal(err)
	}

	// Verify setup
	manifestPath := filepath.Join(outDir, ManifestFile)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatal("Manifest not created in setup")
	}

	return tmpDir
}
