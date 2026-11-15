package sourcescan

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestScopeFor(t *testing.T) {
	root := t.TempDir()
	s := ScopeFor(root)
	if got := strings.Join(s.Dirs, " "); got != DefaultSourceDirs {
		t.Errorf("default Dirs = %q", got)
	}
	if got := strings.Join(s.ExcludeDirs, " "); got != "node_modules vendor dist .git .claude" {
		t.Errorf("default ExcludeDirs = %q", got)
	}

	testutil.WriteFile(t, root, ".rebarrc", "source_dirs = cli scripts # where\nsource_extensions = .go sh\nsource_exclude_dirs = generated\n")
	s = ScopeFor(root)
	if got := strings.Join(s.Dirs, " "); got != "cli scripts" {
		t.Errorf("Dirs = %q", got)
	}
	if got := strings.Join(s.ExtensionGlobs(), " "); got != "*.go *.sh" {
		t.Errorf("ExtensionGlobs = %q (a leading dot is dropped)", got)
	}
	if got := strings.Join(s.ExcludeDirs, " "); got != "generated .git .claude" {
		t.Errorf("ExcludeDirs = %q (.git and .claude are always skipped)", got)
	}
}

func displays(files []File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Display)
	}
	return out
}

func TestFilterFiles(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"src/b.go", "src/a.go", "src/a.txt", "src/sub/c.go", "src/TODO.md",
		"src/vendor/v.go", "src/distance.go", "vendor/w.go", ".github/x.go",
		"src/.claude/worktrees/w/old.go", "lib/l.go",
	} {
		testutil.WriteFile(t, root, rel, "x\n")
	}
	f := Filter{
		Include:     []string{"*.go", "*.md"},
		Exclude:     []string{"TODO.md"},
		ExcludeDirs: []string{"vendor", ".git", ".claude"},
	}

	got := displays(f.Files(root, []string{"src"}))
	want := []string{"src/a.go", "src/b.go", "src/distance.go", "src/sub/c.go"}
	if !slices.Equal(got, want) {
		t.Errorf("src: got %v, want %v", got, want)
	}

	// Operands as given: ./ kept, trailing slash dropped, excluded and
	// missing operands skipped, file operands filtered, order kept.
	got = displays(f.Files(root, []string{"./lib/", "vendor", "missing", "src/a.txt", "src/b.go"}))
	want = []string{"./lib/l.go", "src/b.go"}
	if !slices.Equal(got, want) {
		t.Errorf("operands: got %v, want %v", got, want)
	}

	// Hidden directories other than the excluded ones are walked.
	got = displays(f.Files(root, []string{"."}))
	if !slices.Contains(got, "./.github/x.go") || slices.Contains(got, "./vendor/w.go") {
		t.Errorf(".: got %v", got)
	}
}

func TestFilterFilesSymlinks(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "real/a.go", "x\n")
	testutil.WriteFile(t, root, "src/b.go", "x\n")
	if err := os.Symlink(filepath.Join(root, "real", "a.go"), filepath.Join(root, "src", "link.go")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "src", "linkdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "top")); err != nil {
		t.Fatal(err)
	}
	f := Filter{Include: []string{"*.go"}}
	if got := displays(f.Files(root, []string{"src"})); !slices.Equal(got, []string{"src/b.go"}) {
		t.Errorf("links met while walking are skipped: got %v", got)
	}
	if got := displays(f.Files(root, []string{"top"})); !slices.Equal(got, []string{"top/a.go"}) {
		t.Errorf("a symlinked operand is followed: got %v", got)
	}
}

func TestSearch(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "crlf.go", "one\r\nFIXME: two\r\nthree")
	testutil.WriteFile(t, root, "tail.go", "x\nFIXME: last, no newline")
	testutil.WriteFile(t, root, "latin1.go", "// caf\xe9 FIXME: latin-1\n")
	testutil.WriteFile(t, root, "bin.go", "a\x00b FIXME: binary\n")
	f := Filter{Include: []string{"*.go"}}
	lines := Search(f.Files(root, []string{"."}), func(s string) bool { return strings.Contains(s, "FIXME:") })
	var got []string
	for _, l := range lines {
		got = append(got, l.String())
	}
	want := []string{
		"./crlf.go:2:FIXME: two\r",
		"./latin1.go:1:// caf\xe9 FIXME: latin-1",
		"./tail.go:2:FIXME: last, no newline",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
