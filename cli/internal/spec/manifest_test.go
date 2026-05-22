package spec

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Load non-existent manifest (should create empty)
	manifest, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if manifest.Version != "1.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.0")
	}

	if len(manifest.Mappings) != 0 {
		t.Errorf("New manifest has %d mappings, want 0", len(manifest.Mappings))
	}
}

func TestSaveManifest(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := &Manifest{
		Version: "1.0",
		Mappings: []SpecMapping{
			{
				Contract:         "architecture/CONTRACT-TEST.1.0.md",
				ContractChecksum: "abc123",
				Exports: []ExportedSpec{
					{Type: "gherkin", Path: "specs/gherkin/test.feature", Checksum: "def456"},
				},
			},
		},
	}

	if err := SaveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}

	// Verify file created
	manifestPath := filepath.Join(tmpDir, ManifestFile)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("Manifest file not created")
	}

	// Load and verify
	loaded, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if len(loaded.Mappings) != 1 {
		t.Fatalf("Loaded manifest has %d mappings, want 1", len(loaded.Mappings))
	}

	mapping := loaded.Mappings[0]
	if mapping.Contract != "architecture/CONTRACT-TEST.1.0.md" {
		t.Errorf("Contract = %q, want %q", mapping.Contract, "architecture/CONTRACT-TEST.1.0.md")
	}

	if mapping.ContractChecksum != "abc123" {
		t.Errorf("ContractChecksum = %q, want %q", mapping.ContractChecksum, "abc123")
	}

	if len(mapping.Exports) != 1 {
		t.Fatalf("Exports count = %d, want 1", len(mapping.Exports))
	}
}

func TestFindMapping(t *testing.T) {
	manifest := &Manifest{
		Version: "1.0",
		Mappings: []SpecMapping{
			{Contract: "architecture/CONTRACT-A.md"},
			{Contract: "architecture/CONTRACT-B.md"},
		},
	}

	tests := []struct {
		path string
		want bool
	}{
		{"architecture/CONTRACT-A.md", true},
		{"architecture/CONTRACT-B.md", true},
		{"architecture/CONTRACT-C.md", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			mapping := manifest.FindMapping(tt.path)
			found := mapping != nil
			if found != tt.want {
				t.Errorf("FindMapping(%q) found = %v, want %v", tt.path, found, tt.want)
			}
		})
	}
}

func TestUpdateMapping(t *testing.T) {
	manifest := &Manifest{
		Version: "1.0",
		Mappings: []SpecMapping{
			{Contract: "architecture/CONTRACT-A.md", ContractChecksum: "old"},
		},
	}

	// Update existing
	updated := SpecMapping{
		Contract:         "architecture/CONTRACT-A.md",
		ContractChecksum: "new",
	}

	manifest.UpdateMapping(updated)

	if len(manifest.Mappings) != 1 {
		t.Errorf("Mappings count = %d, want 1", len(manifest.Mappings))
	}

	if manifest.Mappings[0].ContractChecksum != "new" {
		t.Error("Mapping not updated")
	}

	// Add new
	newMapping := SpecMapping{
		Contract:         "architecture/CONTRACT-B.md",
		ContractChecksum: "xyz",
	}

	manifest.UpdateMapping(newMapping)

	if len(manifest.Mappings) != 2 {
		t.Errorf("Mappings count = %d, want 2", len(manifest.Mappings))
	}
}

func TestRemoveMapping(t *testing.T) {
	manifest := &Manifest{
		Version: "1.0",
		Mappings: []SpecMapping{
			{Contract: "architecture/CONTRACT-A.md"},
			{Contract: "architecture/CONTRACT-B.md"},
		},
	}

	manifest.RemoveMapping("architecture/CONTRACT-A.md")

	if len(manifest.Mappings) != 1 {
		t.Errorf("Mappings count = %d, want 1", len(manifest.Mappings))
	}

	if manifest.Mappings[0].Contract == "architecture/CONTRACT-A.md" {
		t.Error("Wrong mapping removed")
	}

	// Remove non-existent (should not error)
	manifest.RemoveMapping("architecture/CONTRACT-Z.md")

	if len(manifest.Mappings) != 1 {
		t.Error("Mappings count changed after removing non-existent")
	}
}

func TestComputeChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Compute checksum
	checksum1, err := ComputeChecksum(testFile)
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}

	if checksum1 == "" {
		t.Error("Checksum is empty")
	}

	// Same content should produce same checksum
	checksum2, err := ComputeChecksum(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if checksum1 != checksum2 {
		t.Error("Checksums don't match for same content")
	}

	// Different content should produce different checksum
	if err := os.WriteFile(testFile, []byte("Different"), 0644); err != nil {
		t.Fatal(err)
	}

	checksum3, err := ComputeChecksum(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if checksum1 == checksum3 {
		t.Error("Checksums match for different content")
	}
}

func TestHasContractChanged(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	contractPath := filepath.Join(tmpDir, "contract.md")
	if err := os.WriteFile(contractPath, []byte("Original"), 0644); err != nil {
		t.Fatal(err)
	}

	originalChecksum, _ := ComputeChecksum(contractPath)

	mapping := &SpecMapping{
		Contract:         "contract.md",
		ContractChecksum: originalChecksum,
	}

	// No change
	changed, err := HasContractChanged(contractPath, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("Contract reported as changed when unchanged")
	}

	// Modify file
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(contractPath, []byte("Modified"), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err = HasContractChanged(contractPath, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("Contract not reported as changed after modification")
	}

	// No mapping (new contract)
	changed, err = HasContractChanged(contractPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("New contract (nil mapping) not reported as changed")
	}
}

func TestHasSpecChanged(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test spec
	specPath := filepath.Join(tmpDir, "test.feature")
	if err := os.WriteFile(specPath, []byte("Original"), 0644); err != nil {
		t.Fatal(err)
	}

	originalChecksum, _ := ComputeChecksum(specPath)

	mapping := &SpecMapping{
		Exports: []ExportedSpec{
			{Type: "gherkin", Path: "test.feature", Checksum: originalChecksum},
		},
	}

	// No change
	changed, changedPaths, err := HasSpecChanged(tmpDir, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("Spec reported as changed when unchanged")
	}
	if len(changedPaths) > 0 {
		t.Error("ChangedPaths not empty for unchanged spec")
	}

	// Modify spec
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(specPath, []byte("Modified"), 0644); err != nil {
		t.Fatal(err)
	}

	changed, changedPaths, err = HasSpecChanged(tmpDir, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("Spec not reported as changed after modification")
	}
	if len(changedPaths) != 1 {
		t.Errorf("ChangedPaths count = %d, want 1", len(changedPaths))
	}
}
