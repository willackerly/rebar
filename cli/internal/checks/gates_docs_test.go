package checks

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestBreRegexp(t *testing.T) {
	cases := []struct {
		bre   string
		match []string
		miss  []string
	}{
		{"^docs/", []string{"docs/a.md"}, []string{"xdocs/a", "doc/"}},
		{"^d.cs/", []string{"docs/a", "dxcs/b"}, []string{"dcs/"}},
		{"^a*b/", []string{"b/", "aab/x"}, []string{"cb/"}},
		{"*x", []string{"*x"}, []string{"x"}},                  // * is literal at the start
		{"^*x", []string{"*x"}, []string{"x"}},                 // and after ^
		{"^a\\(*\\)b", []string{"a*b"}, []string{"ab", "acb"}}, // and after \(
		{"^d[aeiou]cs/", []string{"docs/"}, []string{"dxcs/"}},
		{"^[]x]/", []string{"]/", "x/"}, []string{"a/"}},
		{"^[^]x]/", []string{"a/"}, []string{"]/", "x/"}},
		{"^[[:digit:]]/", []string{"7/"}, []string{"a/"}},
		{"^[a\\]/", []string{"a/", "\\/"}, []string{"b/"}},
		{"^a+b(c)|d{1}/", []string{"a+b(c)|d{1}/"}, []string{"aab/"}}, // ERE characters are literal
		{"^ab\\{2\\}/", []string{"abb/"}, []string{"ab/"}},
		{"^a\\|b/", []string{"a", "b/"}, []string{"c/"}},
		{"^a$b/", []string{"a$b/"}, []string{"a/"}},
		{"^a\\.b/", []string{"a.b/"}, []string{"axb/"}},
		{"^\\<ab/", []string{"ab/"}, nil},
		{"^café/", []string{"café/x"}, []string{"cafe/"}},
		{"x$", []string{"ax"}, []string{"xa"}},
	}
	for _, tc := range cases {
		re := breRegexp(tc.bre)
		if re == nil {
			t.Errorf("breRegexp(%q) = nil", tc.bre)
			continue
		}
		for _, s := range tc.match {
			if !re.MatchString(s) {
				t.Errorf("%q (%s) should match %q", tc.bre, re, s)
			}
		}
		for _, s := range tc.miss {
			if re.MatchString(s) {
				t.Errorf("%q (%s) should not match %q", tc.bre, re, s)
			}
		}
	}
	for _, bad := range []string{"^a[b/", "^a\\(b/", "^\\(a\\)\\1/", "a\\", "^[[:alpha:/"} {
		if re := breRegexp(bad); re != nil {
			t.Errorf("breRegexp(%q) = %s, want nil (grep rejects it or Go cannot run it)", bad, re)
		}
	}
}

func TestGlobMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, s string
		want       bool
	}{
		{"*.spec.ts", "e2e/a.spec.ts", true},
		{"*.spec.ts", "a.spec.tsx", false},
		{"*test_*.py", "tests/test_x.py", true},
		{"*test_*.py", "tests/x_test.py", false},
		{"*playwright*.config.*", "a/playwright.e2e.config.ts", true},
		{".github/workflows/*.yml", ".github/workflows/sub/ci.yml", true},
		{".github/workflows/*.yml", "x/.github/workflows/ci.yml", false},
		{"demo-*", "demo-", true},
		{"*uaks*", "uaks", true},
		{"*ab*ab", "abab", true},
		{"*ab*ab", "aba", false},
		{"exact", "exact", true},
	} {
		if got := globMatch(tc.pattern, tc.s); got != tc.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

func TestContractPathVersion(t *testing.T) {
	for _, tc := range []struct{ path, id, version string }{
		{"architecture/CONTRACT-S1-FOO.1.0.md", "S1-FOO", "1.0"},
		{"architecture/CONTRACT-C1-AGENTS.2.10.3.md", "C1-AGENTS", "2.10.3"},
		{"a/CONTRACT-S1.FOO.1.0.md", "S1.FOO", "1.0"},
		{"a/CONTRACT-A..1.0.md", "A.", "1.0"},
		{"a/CONTRACT-.x.1.0.md", "x", "1.0"}, // awk joins no dot after an empty first field
		{"a/CONTRACT-1.0.md", "", "1.0"},
		{"a/CONTRACT-1.2.3.md", "", "1.2.3"},
		{"a/CONTRACT-S1-FOO.md", "", ""},
		{"a/CONTRACT-S1-FOO.v2.md", "", ""},
		{"a/CONTRACT-.md", "", ""},
	} {
		id, v := contractPathVersion(tc.path)
		if id != tc.id || v != tc.version {
			t.Errorf("contractPathVersion(%q) = %q, %q; want %q, %q", tc.path, id, v, tc.id, tc.version)
		}
	}
}

func TestIfsColonFields(t *testing.T) {
	// What bash's IFS=: read -r a b c assigns (bash 3.2 and 5).
	for _, tc := range []struct{ line, a, b, c string }{
		{"x:12:y:z:", "x", "12", "y:z:"},
		{"x:12:y:", "x", "12", "y"},
		{"x:12::", "x", "12", ""},
		{"x:12:", "x", "12", ""},
		{":12:y", "", "12", "y"},
		{"a:b.md:3:t", "a", "b.md", "3:t"},
	} {
		a, b, c := ifsColonFields(tc.line)
		if a != tc.a || b != tc.b || c != tc.c {
			t.Errorf("ifsColonFields(%q) = %q %q %q; want %q %q %q", tc.line, a, b, c, tc.a, tc.b, tc.c)
		}
	}
}

func TestResolveDocRef(t *testing.T) {
	for _, tc := range []struct {
		dir, target, want string
		ok                bool
	}{
		{".", "a.md", "a.md", true},
		{"docs", "../README.md", "README.md", true},
		{"docs", "./x.md#a", "docs/x.md", true},
		{"docs", "/top.md?x", "top.md", true},
		{"docs", "a/././b.md", "docs/a/./b.md", true}, // sed's g does not overlap
		{".", "../../x.md", "x.md", true},             // sed folds .. against ..
		{"d", "a//../b.md", "d/a//../b.md", true},     // the script loops forever here
		{".", "//../b.md", "/../b.md", true},          // and here
		{".", "word", "", false},
		{".", "#x", "", false},
		{".", "{a}.md", "", false},
		{".", "?q", "", false},
		{"docs", "x/?q", "docs/x/", true},
	} {
		got, ok := resolveDocRef(tc.dir, tc.target)
		if got != tc.want || ok != tc.ok {
			t.Errorf("resolveDocRef(%q, %q) = %q, %v; want %q, %v", tc.dir, tc.target, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLinkTargets(t *testing.T) {
	got := linkTargets(`[a](x.md "T") [b [c](y.md) [d](	z.md	) [e](w.md  "1" "2") [f]( "t")`)
	want := []string{"x.md", "y.md", "z.md", `w.md  "1"`, ""}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("linkTargets = %q, want %q", got, want)
	}
}

func TestCompliancePieces(t *testing.T) {
	for _, tc := range []struct {
		rc   string
		want string
		ok   bool
	}{
		{"tier = 2\n", "2", true},
		{"tier = 1 # note\n", "1#note", true},
		{"x = 1\ntiers=3\ntier = 2\n", "3", true},
		{"tier\t2\n", "tier\t2", true},
		{"tier =\t 3\r\n", "3\r", true},
		{"source_dirs = src\n", "", false},
	} {
		got, ok := rcTierValue([]byte(tc.rc))
		if got != tc.want || ok != tc.ok {
			t.Errorf("rcTierValue(%q) = %q, %v; want %q, %v", tc.rc, got, ok, tc.want, tc.ok)
		}
	}
	if got := deleteBytes("  v3. 1\t.0\xff\n", posixSpace); got != "v3.1.0\xff" {
		t.Errorf("deleteBytes = %q", got)
	}
}

func TestDriftCheckWired(t *testing.T) {
	root := t.TempDir()
	if driftCheckWired(root) {
		t.Fatal("empty project is wired")
	}
	testutil.WriteFile(t, root, ".github/workflows/.hidden.yml", "drift-check\n")
	if driftCheckWired(root) {
		t.Error("a hidden workflow counted")
	}
	testutil.WriteFile(t, root, ".github/workflows/sub/.deep.yml", "rebar contract drift-check\n")
	if !driftCheckWired(root) {
		t.Error("a workflow subdirectory is searched as grep -r searches it")
	}
}

// TestNoColour: none of the checks ported here colours its output (only
// check-ground-truth does, through its own constants).
func TestNoColour(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"check-compliance", "check-doc-refs", "check-decay-patterns", "check-fix-commit", "check-bypass-flags", "check-version-bump"} {
		c, _ := Lookup(name)
		var out bytes.Buffer
		c.Run(&Context{Root: root, Stdout: &out, Stderr: &out}, nil)
		if strings.Contains(out.String(), "\x1b[") {
			t.Errorf("%s printed a colour escape:\n%s", name, out.String())
		}
	}
}
