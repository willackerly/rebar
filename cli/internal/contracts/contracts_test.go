package contracts

import (
	"slices"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestNormDirAndDir(t *testing.T) {
	for in, want := range map[string]string{
		"architecture": "architecture", "./docs/specs/": "docs/specs", "././a//": "a",
		"./": "", "/": "/", "//": "/", "/abs/x/": "/abs/x", "../up": "../up",
	} {
		if got := NormDir(in); got != want {
			t.Errorf("NormDir(%q) = %q, want %q", in, got, want)
		}
	}

	root := t.TempDir()
	t.Setenv("REBAR_CONTRACTS_DIR", "")
	t.Setenv("REBAR_RCFILE", "")
	if got := Dir(root); got != "architecture" {
		t.Errorf("default Dir = %q", got)
	}
	testutil.WriteFile(t, root, ".rebarrc", "contracts_dir = ./specs/  # moved\n")
	if got := Dir(root); got != "specs" {
		t.Errorf(".rebarrc Dir = %q", got)
	}
	testutil.WriteFile(t, root, ".rebarrc", "contracts_dir = ./\n")
	if got := Dir(root); got != "architecture" {
		t.Errorf("Dir for ./ = %q, want the default", got)
	}
	testutil.WriteFile(t, root, ".rebarrc", "contracts_dir = /abs/specs\n")
	if got := Dir(root); got != "/abs/specs" {
		t.Errorf("absolute Dir = %q, want it kept as the scripts keep it", got)
	}
	t.Setenv("REBAR_CONTRACTS_DIR", "env/dir/")
	if got := Dir(root); got != "env/dir" {
		t.Errorf("env Dir = %q", got)
	}
}

func TestListAndNames(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"CONTRACT-b.1.0.md", "CONTRACT-A_C.1.0.md", "CONTRACT-A-B.1.0.md", "CONTRACT-.md", "CONTRACT-x.txt", "README.md", "XCONTRACT-a.md"} {
		testutil.WriteFile(t, dir, n, "x\n")
	}
	testutil.WriteFile(t, dir, "CONTRACT-DIR.1.0.md/x", "a directory is listed, as the glob lists it\n")
	want := []string{"CONTRACT-.md", "CONTRACT-A-B.1.0.md", "CONTRACT-A_C.1.0.md", "CONTRACT-DIR.1.0.md", "CONTRACT-b.1.0.md"}
	if got := List(dir); !slices.Equal(got, want) {
		t.Errorf("List = %q, want %q (byte order)", got, want)
	}
	if got := List(dir + "/missing"); got != nil {
		t.Errorf("List(missing) = %q", got)
	}

	for _, n := range []string{"CONTRACT-TEMPLATE.md", "CONTRACT-SEAM-TEMPLATE.md", "CONTRACT-REGISTRY.md", "CONTRACT-REGISTRY.template.md", "CONTRACT-GAPS.md"} {
		if !IsIndex(n) {
			t.Errorf("IsIndex(%s) = false", n)
		}
	}
	if IsIndex("CONTRACT-REGISTRY.old.md") || IsIndex("CONTRACT-S1-FOO.1.0.md") {
		t.Error("IsIndex matched a contract")
	}
	if !IsCompanion("CONTRACT-S1-FOO.1.0.impl.md") || IsCompanion("CONTRACT-S1-FOO.1.0.md") {
		t.Error("IsCompanion")
	}

	for name, want := range map[string][3]string{
		"CONTRACT-S1-FOO.2.1.md":   {"S1-FOO", "2", "1"},
		"CONTRACT-S1-FOO.2.md":     {"S1-FOO", "2", "0"},
		"CONTRACT-S1-FOO.md":       {"S1-FOO", "1", "0"},
		"CONTRACT-A.1.2.3.md":      {"A.1", "2", "3"},
		"CONTRACT-I.01.10.md":      {"I", "01", "10"},
		"CONTRACT-X.1.0.impl.md":   {"X.1.0.impl", "1", "0"},
		"CONTRACT-SEAM-A-B.1.0.md": {"SEAM-A-B", "1", "0"},
	} {
		id, major, minor := ParseName(name)
		if got := [3]string{id, major, minor}; got != want {
			t.Errorf("ParseName(%s) = %q, want %q", name, got, want)
		}
	}
}

func TestReferences(t *testing.T) {
	m := Marker
	for text, want := range map[string]string{
		"// " + m + "S1-FOO.1.0":             "S1-FOO.1.0",
		m + "S1-FOO.1.0 and " + m + "S2.1.0": "S1-FOO.1.0",
		m + "S1-FOO.1.0.7":                   "S1-FOO.1.0",
		m + "S1-FOO.1":                       "",
		m + "..":                             "..",
		m + "S1-FOO.10.20\r":                 "S1-FOO.10.20",
		"no reference":                       "",
		m + "ns:S1-FOO.1.0":                  "",
	} {
		if got := FirstRef(text); got != want {
			t.Errorf("FirstRef(%q) = %q, want %q", text, got, want)
		}
		if HasRef(text) != (want != "") {
			t.Errorf("HasRef(%q) = %v", text, !(want != ""))
		}
	}
	if !HasQualifiedRef(m+"github.com/a/b:S1-FOO.1.0") || HasQualifiedRef(m+"S1-FOO") {
		t.Error("HasQualifiedRef")
	}

	if got := RefPrefix(""); got != m {
		t.Errorf("RefPrefix() = %q", got)
	}
	if got := RefPrefix("github.com/a.b"); got != m+`(github\.com/a\.b:)?` {
		t.Errorf("RefPrefix(ns) = %q", got)
	}

	ns := "github.com/acme/app"
	for text, want := range map[string]struct {
		out   string
		cross int
	}{
		m + ns + ":S1-FOO.1.0":                             {m + "S1-FOO.1.0", 0},
		m + "github.com/other:S2-BAR.1.0":                  {"CROSSREF:github.com/other:S2-BAR.1.0", 1},
		m + "S1-FOO.1.0:see.2":                             {m + "S1-FOO.1.0:see.2", 0},
		m + "o:S2.1.0 then " + m + "S1-FOO.1.0":            {"CROSSREF:o:S2.1.0 then " + m + "S1-FOO.1.0", 1},
		m + "a:S1.1.0 " + m + "b:S2.2.0 CROSSREF: in text": {"CROSSREF:a:S1.1.0 CROSSREF:b:S2.2.0 CROSSREF: in text", 3},
		m + "github.comXacme/app:S1-FOO.1.0":               {"CROSSREF:github.comXacme/app:S1-FOO.1.0", 1},
	} {
		out, cross := Localize(text, ns)
		if out != want.out || cross != want.cross {
			t.Errorf("Localize(%q) = %q, %d; want %q, %d", text, out, cross, want.out, want.cross)
		}
	}
}
