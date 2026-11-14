package integrity

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func touch(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The contracts category follows .rebarrc contracts_dir; architecture/ may
// then hold unrelated docs, including a stray CONTRACT-*.md, which is not
// protected as a contract.
func TestScanProtectedFilesHonorsContractsDir(t *testing.T) {
	root := t.TempDir()
	touch(t, root, ".rebarrc", "tier = 3\ncontracts_dir = specs   # contracts\n")
	touch(t, root, "specs/CONTRACT-S1-FOO.1.0.md", "# S1\n")
	touch(t, root, "specs/CONTRACT-TEMPLATE.md", "# template\n")
	touch(t, root, "specs/CONTRACT-S1-FOO.1.0.impl.md", "# impl\n")
	touch(t, root, "architecture/CONTRACT-S9-OLD.1.0.md", "# stray\n")
	touch(t, root, "architecture/overview.md", "# notes\n")

	got, err := ScanProtectedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	contracts := got[CategoryContracts]
	sort.Strings(contracts)
	want := []string{filepath.Join("specs", "CONTRACT-S1-FOO.1.0.md")}
	if len(contracts) != len(want) || contracts[0] != want[0] {
		t.Fatalf("contracts = %v, want %v", contracts, want)
	}
}

func TestScanProtectedFilesDefaultContractsDir(t *testing.T) {
	root := t.TempDir()
	touch(t, root, ".rebarrc", "tier = 3\n")
	touch(t, root, "architecture/CONTRACT-S1-FOO.1.0.md", "# S1\n")

	got, err := ScanProtectedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if c := got[CategoryContracts]; len(c) != 1 || c[0] != filepath.Join("architecture", "CONTRACT-S1-FOO.1.0.md") {
		t.Fatalf("contracts = %v", c)
	}
}
