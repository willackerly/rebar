package suite

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestSummarize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"last Checked line wins", "Checked 1 a\nFAIL: x\nChecked 2 b\n", "Checked 2 b"},
		{"colours stripped", "\x1b[0;32mChecked 3 metrics\x1b[0m", "Checked 3 metrics"},
		{"first verdict", "noise\nOK: one\nFAIL: two", "OK: one"},
		{"verdict must start the line", " FAIL: x\nplain", " FAIL: x"},
		{"last keyword line, any case", "DRIFTED here\nall MATCH\ntrailing", "all MATCH"},
		{"no metrics", "x\nNo metrics declared\n", "No metrics declared"},
		{"first non-blank", "\n \t\nhello\nworld", "hello"},
		{"empty", "", ""},
		{"blank only", "  \n\t", ""},
	}
	for _, tc := range cases {
		if got := summarize(tc.in); got != tc.want {
			t.Errorf("%s: summarize(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestDeclaredStatus(t *testing.T) {
	dir := t.TempDir()
	cases := []struct{ body, want string }{
		{"# X\n\n**Status:** Active\n", "active"},
		{"Status: draft extra words\n", "draft"},
		{"**Status:**\tIn-Progress\n", "in-progress"},
		{"*Status:* stub\n", "*status:*"},
		{"**Status:**Status: verified\n", "verified"},
		{"**Status:** active\r\n", "active\r"},
		{"**Status:**\n**Status:** active\n", ""},
		{"  **Status:** active\n", ""},
		{"status: active\n", ""},
		{"**Status:** ÄCTIVE\n", "Äctive"},
	}
	for i, tc := range cases {
		f := testutil.WriteFile(t, dir, "c"+string(rune('a'+i))+".md", tc.body)
		if got := declaredStatus(f); got != tc.want {
			t.Errorf("declaredStatus(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestExtras(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, ".rebarrc", "checks_extra = scripts/check-openapi.sh  tools/lint.py # comment\nchecks_extra_pre_commit = /abs/fast\n")
	t.Setenv("REBAR_RCFILE", "")
	t.Setenv("REBAR_CHECKS_EXTRA", "")
	t.Setenv("REBAR_CHECKS_EXTRA_PRE_COMMIT", "")
	want := []Extra{{Name: "check-openapi", Path: "scripts/check-openapi.sh"}, {Name: "lint", Path: "tools/lint.py"}}
	if got := Extras(root, ExtraKey); !reflect.DeepEqual(got, want) {
		t.Errorf("Extras(checks_extra) = %v, want %v", got, want)
	}
	if got := Extras(root, ExtraPreCommitKey); !reflect.DeepEqual(got, []Extra{{Name: "fast", Path: "/abs/fast"}}) {
		t.Errorf("Extras(checks_extra_pre_commit) = %v", got)
	}
	for _, name := range []string{"lint", "lint.py", "tools/lint.py"} {
		if e, ok := FindExtra(root, name); !ok || e.Path != "tools/lint.py" {
			t.Errorf("FindExtra(%q) = %v, %v", name, e, ok)
		}
	}
	if _, ok := FindExtra(root, "check-todos"); ok {
		t.Error("FindExtra found a name that is not listed")
	}
	t.Setenv("REBAR_CHECKS_EXTRA", "x/y.sh")
	if got := Extras(root, ExtraKey); !reflect.DeepEqual(got, []Extra{{Name: "y", Path: "x/y.sh"}}) {
		t.Errorf("REBAR_CHECKS_EXTRA override: %v", got)
	}
}

// TestResolve: a project copy wins (with the notice unless quiet), a thin
// wrapper and the install's own directory never count as copies, then the
// native check, then the install's script.
func TestResolve(t *testing.T) {
	install := t.TempDir()
	root := t.TempDir()
	testutil.WriteExecutable(t, root, "scripts/check-todos.sh", "#!/bin/sh\necho custom\n")
	testutil.WriteExecutable(t, root, "scripts/steward.sh", "#!/bin/sh\n# rebar-thin-wrapper\n")
	testutil.WriteExecutable(t, install, "steward.sh", "#!/bin/sh\n")
	var errs strings.Builder
	ctx := &checks.Context{Root: root, Install: install, Stdout: &errs, Stderr: &errs}

	if s := resolve(ctx, "check-todos.sh", false); s.native != nil || s.path != filepath.Join(root, "scripts", "check-todos.sh") {
		t.Errorf("project copy: %+v", s)
	}
	if !strings.Contains(errs.String(), "rebar: running project copy scripts/check-todos.sh (deprecated") {
		t.Errorf("no deprecation notice: %q", errs.String())
	}
	errs.Reset()
	resolve(ctx, "check-todos.sh", true)
	if errs.Len() != 0 {
		t.Errorf("quiet resolve printed %q", errs.String())
	}
	if s := resolve(ctx, "steward.sh", false); s.path != filepath.Join(install, "steward.sh") {
		t.Errorf("thin wrapper picked: %+v", s)
	}
	if s := resolve(ctx, "check-freshness.sh", false); s.native == nil {
		t.Errorf("native not picked: %+v", s)
	}
	if s := resolve(&checks.Context{Root: root, Stderr: &errs}, "sync-bootstrap.sh", false); s.found() {
		t.Errorf("no install, no copy: %+v found", s)
	}
	// The rebar source repo: its scripts/ is the install.
	ctx.Install = filepath.Join(root, "scripts")
	if s := resolve(ctx, "check-todos.sh", false); s.native == nil {
		t.Errorf("the install's own scripts/ counted as a project copy: %+v", s)
	}
}

func TestRunFileWithoutShebang(t *testing.T) {
	root := t.TempDir()
	f := testutil.WriteExecutable(t, root, "plain", "echo \"ran in $PWD with $REBAR_PROJECT_ROOT\"\nexit 3\n")
	var out strings.Builder
	code := runFile(&checks.Context{Root: root, Stdout: &out, Stderr: &out}, f, nil, false)
	real, _ := filepath.EvalSymlinks(root)
	if code != 3 || (!strings.Contains(out.String(), "ran in "+root) && !strings.Contains(out.String(), "ran in "+real)) || !strings.HasSuffix(strings.TrimSpace(out.String()), root) {
		t.Errorf("exit %d, output %q", code, out.String())
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatal(err)
	}
}
