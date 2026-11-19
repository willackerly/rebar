package checks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestXargsEcho(t *testing.T) {
	cases := map[string]string{
		"Active":             "Active",
		"  a \t b  ":         "a b",
		`"quoted words" x`:   "quoted words x",
		`'single' "double"`:  "single double",
		`a"b c"d`:            "ab cd",
		`a\b`:                "ab",
		`a\\b`:               `a\b`,
		`\'x`:                "'x",
		`"a\b"`:              `a\b`,
		"don't":              "", // unterminated quote: xargs fails
		`x "open`:            "",
		"-n":                 "", // echo -n
		"-ne word":           "word",
		"-x word":            "-x word",
		"Windows\r":          "Windows\r", // \r is not a blank
		"tab\vand\fformfeed": "tab\vand\fformfeed",
		"":                   "",
	}
	for in, want := range cases {
		if got, _ := xargsEcho(in); got != want {
			t.Errorf("xargsEcho(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContractStatus(t *testing.T) {
	cases := map[string]string{
		"**Status:** Active\n":              "Active",
		"Status: Draft: v2\n":               "v2",
		"status: lower\n":                   "lower",
		"# T\n\nStatus:  \t spaced  out \n": "spaced out",
		"Status:stable\n":                   "stable", // GNU sed's \s
		"Status:\n":                         "draft",
		"Status: it's\n":                    "draft",
		"no status\n":                       "draft",
		"Status: one\nStatus: two\n":        "one",
		"Status: Active\n\x00binary\n":      "draft",
		"Status: last line":                 "last line",
		" Status: indented\n":               "draft",
	}
	for in, want := range cases {
		if got := contractStatus([]byte(in)); got != want {
			t.Errorf("contractStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPurpose(t *testing.T) {
	def := regexp.MustCompilePOSIX(`^## Purpose`)
	numbered := regexp.MustCompilePOSIX(`^## ([0-9]+[.][[:space:]]*)?(Purpose|Why)`)
	long := strings.Repeat("x", 81)
	cases := []struct {
		heading *regexp.Regexp
		bytes   bool
		in      string
		want    string
	}{
		{def, false, "## Purpose\n\nText.\n", "Text."},
		{def, false, "## Purpose\n\n### Sub\n  \nAfter.\n", "After."},
		{def, false, "# None\n", ""},
		{def, false, "## Purpose\n" + long + "\n", strings.Repeat("x", 77) + "..."},
		{def, false, "## Purpose\n" + strings.Repeat("x", 80) + "\n", strings.Repeat("x", 80)},
		{def, false, "## Purpose\na|b\n", `a\|b`},
		{def, false, "## Purpose\n" + strings.Repeat("é", 81) + "\n", strings.Repeat("é", 77) + "..."},
		{def, false, "## Purpose\n" + strings.Repeat("\xe9", 81) + "\n", strings.Repeat("\xe9", 77) + "..."},
		{def, true, "## Purpose\n" + strings.Repeat("é", 50) + "\n", strings.Repeat("é", 40)},
		{def, true, "## Purpose\na|b\n", "a|b"},
		{def, false, "## 2. Purpose\nnumbered\n", ""},
		{numbered, false, "## 2. Purpose\nnumbered\n", "numbered"},
		{numbered, false, "## Why\nwhy\n", "why"},
		{nil, false, "## Purpose\ntext\n", ""},
	}
	for _, tc := range cases {
		r := &registryRun{purposeHeading: tc.heading, bytesPurpose: tc.bytes}
		if got := r.purpose([]byte(tc.in)); got != tc.want {
			t.Errorf("purpose(%q, bytes=%v) = %q, want %q", tc.in, tc.bytes, got, tc.want)
		}
	}
}

func TestRegistryBody(t *testing.T) {
	a := "# R\n<!-- Generated: 2025-01-01 -->\n<!-- Regenerate: ./scripts/compute-registry.sh -->\nrow\n"
	b := "# R\n<!-- Regenerate: rebar run compute-registry -->\nrow"
	if registryBody([]byte(a)) != registryBody([]byte(b)) {
		t.Errorf("header comments and a missing final newline should not matter:\n%q\n%q", registryBody([]byte(a)), registryBody([]byte(b)))
	}
	if registryBody([]byte("row\n")) == registryBody([]byte("row\n\n")) {
		t.Error("a trailing blank line is content")
	}
}

func TestComputeRegistryCategory(t *testing.T) {
	r := &registryRun{scopePrefixes: []string{"SDK-", "MCP-"}}
	for id, want := range map[string]string{
		"S1-A": "Services", "C1": "Components", "I1": "Interfaces", "P1": "Protocols", "X1": "Other",
		"SDK-S1": "Services", "SDK-MCP-C1": "Components", "MCP-SDK-C1": "Services" /* SDK- stays: prefixes strip in order */, "s1": "Other", "": "Other",
	} {
		if got := r.category(id); got != want {
			t.Errorf("category(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestFindWalkPrunesStart: find prunes its starting point by name too, so
// a project directory named like an excluded directory searches nothing,
// and a symlinked starting point is not followed.
func TestFindWalkPrunesStart(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "vendor/a.go", "x\n")
	testutil.WriteFile(t, root, "src/b.go", "x\n")
	if err := os.Symlink(filepath.Join(root, "src"), filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	pruned := func(_, name string) bool { return name == "vendor" }
	all := func(string) bool { return true }
	if got := findWalk(filepath.Join(root, "vendor"), pruned, all, nil); len(got) != 0 {
		t.Errorf("pruned start listed %v", got)
	}
	if got := findWalk(filepath.Join(root, "link"), pruned, all, nil); len(got) != 0 {
		t.Errorf("symlinked start listed %v", got)
	}
	got := findWalk(root, pruned, all, nil)
	if len(got) != 1 || !strings.HasSuffix(got[0], "/src/b.go") {
		t.Errorf("walk listed %v, want only src/b.go", got)
	}
}

// TestComputeRegistryWriteError: without a contracts directory the
// registry cannot be written; like the script's redirect, that fails the
// run with nothing on stdout.
func TestComputeRegistryWriteError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("REBAR_RCFILE", "")
	t.Setenv("REBAR_CONTRACTS_DIR", "")
	var out, errOut strings.Builder
	code := runComputeRegistry(&Context{Root: root, Stdout: &out, Stderr: &errOut}, nil)
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "No such file or directory") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}
