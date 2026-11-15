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

func TestFnmatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"gen", "gen", true},
		{"gen", "gen2", false},
		{"gen*", "generated", true},
		{"*", "", true},
		{"*", "a/b", true}, // no FNM_PATHNAME: * crosses /
		{"s*n", "src/gen", true},
		{"a*b*c", "aXbYbZc", true},
		{"a*b*c", "aXbYbZ", false},
		{"?en", "gen", true},
		{"?en", "en", false},
		{"[!x]en", "gen", true},
		{"[!x]en", "xen", false},
		{"[^x]en", "gen", true},
		{"[a-h]en", "gen", true},
		{"[a-f]en", "gen", false},
		{"[[:digit:]]*", "2024", true},
		{"[[:digit:]]*", "v2024", false},
		{"[[:lower:]]en", "Gen", false},
		{"[[:upper:]]en", "Gen", true},
		{"[]]x", "]x", true},
		{"[!]]x", "ax", true},
		{"[a-]", "-", true},
		{`\gen`, "gen", true},
		{`\*`, "*", true},
		{`\*`, "a", false},
		{`[\]]`, "]", true},
		{"[gen", "[gen", true}, // unclosed bracket: literal
		{"[gen", "g", false},
		{`ab\`, `ab\`, true},
	}
	for _, tc := range cases {
		if got := fnmatch(tc.pattern, tc.name); got != tc.want {
			t.Errorf("fnmatch(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestFilterExcludeDirPatterns(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"src/gen/a.go", "src/x/gen/b.go", "src/keep/c.go", "src/xen/d.go"} {
		testutil.WriteFile(t, root, rel, "x\n")
	}
	cases := []struct {
		exclude  []string
		operands []string
		want     []string
	}{
		// Below an operand: by name; a pattern with a slash matches nothing.
		{[]string{"[!x]en"}, []string{"src"}, []string{"src/keep/c.go", "src/xen/d.go"}},
		{[]string{"src/gen", "*/gen"}, []string{"src"}, []string{"src/gen/a.go", "src/keep/c.go", "src/x/gen/b.go", "src/xen/d.go"}},
		{[]string{"gen/"}, []string{"src"}, []string{"src/keep/c.go", "src/xen/d.go"}},
		// An operand: by its path and every tail after a slash.
		{[]string{"src/gen"}, []string{"./src/gen", "src/keep"}, []string{"src/keep/c.go"}},
		{[]string{"x/gen"}, []string{"src/x/gen", "src/keep"}, []string{"src/keep/c.go"}},
		{[]string{"keep"}, []string{"src/keep/"}, nil},
		{[]string{".*", "*"}, []string{"."}, nil}, // "." itself is never excluded, its entries are
	}
	for _, tc := range cases {
		f := Filter{ExcludeDirs: tc.exclude}
		if got := displays(f.Files(root, tc.operands)); !slices.Equal(got, tc.want) {
			t.Errorf("exclude %q, operands %q: got %v, want %v", tc.exclude, tc.operands, got, tc.want)
		}
	}
}

func TestTextPart(t *testing.T) {
	text := []byte(strings.Repeat("line\n", BinaryBlock))
	for _, tc := range []struct {
		name string
		data []byte
		want int
	}{
		{"no NUL", text, len(text)},
		{"NUL first", append([]byte{0}, text...), 0},
		{"NUL in the block's last byte", append(append([]byte{}, text[:BinaryBlock-1]...), 0), 0},
		{"NUL just past the block", append(append([]byte{}, text[:BinaryBlock]...), 0, 'x'), BinaryBlock + 2},
		{"empty", nil, 0},
	} {
		if got := len(TextPart(tc.data)); got != tc.want {
			t.Errorf("%s: TextPart kept %d bytes, want %d", tc.name, got, tc.want)
		}
	}
}
