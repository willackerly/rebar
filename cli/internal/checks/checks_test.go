package checks

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestImpl(t *testing.T) {
	for v, want := range map[string]string{"": ImplShell, "shell": ImplShell, "go": ImplGo, " go ": ImplGo, "rust": ImplShell} {
		t.Setenv("REBAR_CHECK_IMPL", v)
		if got := Impl(); got != want {
			t.Errorf("REBAR_CHECK_IMPL=%q: Impl() = %q, want %q", v, got, want)
		}
	}
}

func TestNativeAndScriptEnv(t *testing.T) {
	t.Setenv("REBAR_CHECK_IMPL", "")
	if _, ok := Native("check-todos.sh"); ok {
		t.Error("shell implementation returned a native check")
	}
	if env := ScriptEnv(); !slices.Equal(env, []string{"REBAR_NATIVE_CHECKS="}) {
		t.Errorf("shell ScriptEnv = %v, want an inherited list cleared", env)
	}

	t.Setenv("REBAR_CHECK_IMPL", "go")
	if c, ok := Native("check-todos.sh"); !ok || c.Name != "check-todos" {
		t.Errorf("go: Native(check-todos.sh) = %v, %v", c, ok)
	}
	if _, ok := Native("steward"); ok {
		t.Error("go: an unported check returned a native implementation")
	}
	env := strings.Join(ScriptEnv(), "\n")
	if !strings.Contains(env, "REBAR_BIN=") || !strings.Contains(env, "REBAR_NATIVE_CHECKS=check-todos") {
		t.Errorf("go ScriptEnv = %q", env)
	}
}

// markers spells the tags so that rebar's own check-todos, pointed at this
// file, does not count them.
var markers = strings.NewReplacer("@todo", "TO"+"DO:", "@tracked", "TRACKED"+"-TASK:")

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	testutil.WriteFile(t, root, rel, markers.Replace(body))
}

func runTodosIn(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code := runTodos(&Context{Root: root, Stdout: &out, Stderr: &out}, args)
	return out.String(), code
}

func TestTodos(t *testing.T) {
	root := t.TempDir()
	if out, code := runTodosIn(t, root); code != 0 || out != "No source directories found. Pass directories as arguments.\n" {
		t.Fatalf("no source dirs: %d %q", code, out)
	}

	write(t, root, "src/a.go", "// @tracked later\n// @todo @tracked both on one line\n")
	write(t, root, "src/TODO.md", "@todo the list itself\n")
	if out, code := runTodosIn(t, root); code != 0 || out != "OK: 0 untracked TODOs. 2 tracked tasks.\n" {
		t.Fatalf("tracked only: %d %q", code, out)
	}

	write(t, root, "src/b.go", "x\n// @todo fix\n")
	write(t, root, "cmd/c.go", "// @todo also\n")
	write(t, root, "cmd/c.txt", "// @todo not a source file\n")
	out, code := runTodosIn(t, root)
	want := `FAIL: 2 untracked TODO(s) found:

src/b.go:2:// @todo fix
cmd/c.go:1:// @todo also

Before committing, you must either:
  1. Fix the TODO immediately (remove it), OR
  2. Add it to TODO.md and convert to: @tracked description

Also found 2 tracked tasks (OK).
`
	if want = markers.Replace(want); code != 1 || out != want {
		t.Fatalf("untracked: %d\n%s\nwant\n%s", code, out, want)
	}

	// Arguments replace source_dirs.
	if out, code := runTodosIn(t, root, "cmd"); code != 1 || !strings.Contains(out, "FAIL: 1 untracked") {
		t.Fatalf("args: %d %q", code, out)
	}
	// .rebarrc source_dirs / source_extensions.
	write(t, root, ".rebarrc", "source_dirs = cmd\nsource_extensions = txt\n")
	if out, code := runTodosIn(t, root); code != 1 || !strings.Contains(out, "cmd/c.txt:1:") || strings.Contains(out, "cmd/c.go") {
		t.Fatalf(".rebarrc scope: %d %q", code, out)
	}
}
