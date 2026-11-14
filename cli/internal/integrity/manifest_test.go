package integrity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func intPtr(n int) *int { return &n }

func TestSaveKeepsHeaderStable(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest("repo")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	m.GeneratedAt = created
	m.GeneratedBy = "rebar init"
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}

	m.Checksums[CategoryTests]["a_test.go"] = FileEntry{SHA256: "aa", Role: "tester"}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}

	got, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got.GeneratedAt.Equal(created) || got.GeneratedBy != "rebar init" {
		t.Fatalf("header restamped on save: %v %q", got.GeneratedAt, got.GeneratedBy)
	}
}

func TestSaveDropsLegacyRatchetCurrent(t *testing.T) {
	dir := t.TempDir()
	legacy := `{
  "schema_version": "1.0",
  "generated_at": "2026-01-02T03:04:05Z",
  "generated_by": "rebar commit",
  "repo_id": "repo",
  "checksums": {"tests": {}},
  "ratchets": {"total_assertions": {"min": 7, "current": 9}}
}`
	if err := os.WriteFile(ManifestPath(dir), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ManifestPath(dir))
	if strings.Contains(string(data), `"current"`) {
		t.Fatalf("legacy ratchet current still written:\n%s", data)
	}
	if !strings.Contains(string(data), `"min": 7`) {
		t.Fatalf("legacy ratchet floor lost:\n%s", data)
	}
}

func TestNewManifestOmitsRatchets(t *testing.T) {
	dir := t.TempDir()
	if err := NewManifest("repo").Save(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ManifestPath(dir))
	if strings.Contains(string(data), `"ratchets"`) {
		t.Fatalf("new manifest writes stored ratchets:\n%s", data)
	}
}

func TestCheckRatchetsUsesDerivedAndLegacyFloors(t *testing.T) {
	m := NewManifest("repo")
	m.Checksums[CategoryTests]["a_test.go"] = FileEntry{AssertCount: intPtr(5)}
	m.Checksums[CategoryTests]["b_test.go"] = FileEntry{AssertCount: intPtr(3)}
	m.Checksums[CategoryContracts]["architecture/CONTRACT-X.1.0.md"] = FileEntry{}
	m.Ratchets = map[string]Ratchet{"contract_count": {Min: 4}}

	floors := map[string]int{}
	for _, r := range CheckRatchets(m, map[string]int{}) {
		floors[r.Name] = r.Min
	}
	want := map[string]int{"total_assertions": 8, "test_file_count": 2, "contract_count": 4}
	for name, min := range want {
		if floors[name] != min {
			t.Errorf("%s floor = %d, want %d", name, floors[name], min)
		}
	}

	ok := CheckRatchets(m, map[string]int{"total_assertions": 8, "test_file_count": 2, "contract_count": 4})
	for _, r := range ok {
		if r.Violated {
			t.Errorf("%s violated at its floor", r.Name)
		}
	}

	low := CheckRatchets(m, map[string]int{"total_assertions": 7, "test_file_count": 2, "contract_count": 4})
	violated := false
	for _, r := range low {
		if r.Name == "total_assertions" && r.Violated {
			violated = true
		}
	}
	if !violated {
		t.Error("assertion decrease not reported as a violation")
	}
}

// TestBranchesTouchingDifferentFilesMergeCleanly is the regression test for
// busy repos: two branches that each update the entry of a different
// protected file, including adjacent entries, and add an entry, must merge
// with plain `git merge` and keep every change.
func TestBranchesTouchingDifferentFilesMergeCleanly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	rebarDir := filepath.Join(repo, ".rebar")
	if err := os.MkdirAll(rebarDir, 0755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) error {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("git %v: %s", args, out)
		}
		return err
	}
	update := func(path string, sha string, asserts int) {
		m, err := LoadManifest(rebarDir)
		if err != nil {
			t.Fatal(err)
		}
		m.Checksums[CategoryTests][path] = FileEntry{
			SHA256: sha, Role: "tester", RoleHMAC: sha + "-hmac",
			ModifiedAt: time.Now().UTC(), AssertCount: intPtr(asserts),
		}
		if err := m.Save(rebarDir); err != nil {
			t.Fatal(err)
		}
	}

	if err := git("init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	base := NewManifest("repo")
	base.GeneratedBy = "rebar init"
	for _, p := range []string{"a_test.go", "b_test.go", "c_test.go"} {
		base.Checksums[CategoryTests][p] = FileEntry{SHA256: p, Role: "tester", RoleHMAC: p, AssertCount: intPtr(1)}
	}
	if err := base.Save(rebarDir); err != nil {
		t.Fatal(err)
	}
	if git("add", ".") != nil || git("commit", "-q", "-m", "base") != nil {
		t.Fatal("base commit failed")
	}

	if git("checkout", "-q", "-b", "feature-a") != nil {
		t.Fatal("branch a")
	}
	update("a_test.go", "a2", 4)
	if git("commit", "-q", "-am", "a") != nil {
		t.Fatal("commit a")
	}

	if git("checkout", "-q", "-b", "feature-b", "main") != nil {
		t.Fatal("branch b")
	}
	update("b_test.go", "b2", 6)
	update("bb_test.go", "bb1", 2)
	if git("add", ".") != nil || git("commit", "-q", "-m", "b") != nil {
		t.Fatal("commit b")
	}

	if git("checkout", "-q", "main") != nil || git("merge", "-q", "--no-edit", "feature-a") != nil {
		t.Fatal("merge a")
	}
	if err := git("merge", "-q", "--no-edit", "feature-b"); err != nil {
		t.Fatalf("branches touching different files conflicted: %v", err)
	}

	merged, err := LoadManifest(rebarDir)
	if err != nil {
		t.Fatalf("merged manifest is not valid JSON: %v", err)
	}
	tests := merged.Checksums[CategoryTests]
	if tests["a_test.go"].SHA256 != "a2" || tests["b_test.go"].SHA256 != "b2" || tests["bb_test.go"].SHA256 != "bb1" {
		t.Fatalf("merge lost an update: %+v", tests)
	}
	if got := DerivedFloors(merged)["total_assertions"]; got != 4+6+2+1 {
		t.Fatalf("merged total_assertions floor = %d, want 13", got)
	}
}
