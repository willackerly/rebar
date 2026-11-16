// Package suite holds the native (Go) implementations of rebar's check
// suites: ci-check (scripts/ci-check.sh), pre-commit (scripts/pre-commit.sh)
// and cold-start-checks (scripts/cold-start-checks.sh). They are registered
// with the checks package under their scripts' names, so `rebar check`,
// `rebar run` and `rebar native` find them like any check, and behave like
// the scripts: same check order, output, SKIP_* variables and exit codes.
//
// A suite runs each check's native implementation in process. A check with
// none (steward, sync-bootstrap, tests/run.sh) runs from the install's
// scripts directory (checks.Context.Install), and a project's own copy of a
// check script still wins over both, with the deprecation notice the
// scripts print (_rebar_script in scripts/_rebar-config.sh): it is the
// adopter's code. The adopter's own extra checks (.rebarrc checks_extra,
// checks_extra_pre_commit) run after rebar's.
package suite

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/willackerly/rebar/cli/internal/checks"
	"github.com/willackerly/rebar/cli/internal/config"
)

// script is what a suite runs for one of rebar's check scripts.
type script struct {
	// native is the in-process implementation (nil: run path).
	native *checks.Check
	// path is the script to run when native is nil; "" when neither the
	// project nor the install has it.
	path string
}

// thinWrapperMarker tags a project script that only hands off to the CLI;
// _rebar_script never picks such a file.
const thinWrapperMarker = "# rebar-thin-wrapper"

// resolve picks what runs for script name (relative to scripts/, e.g.
// check-todos.sh or tests/run.sh), as _rebar_script does: the project's own
// scripts/<name> when it has one that is not the install's and not a thin
// wrapper, with the deprecation notice on ctx.Stderr unless quiet (the
// cold-start hook sends it to /dev/null); else the native check; else the
// install's script.
func resolve(ctx *checks.Context, name string, quiet bool) script {
	if local := filepath.Join(ctx.Root, "scripts", name); isProjectCopy(ctx, local) {
		if !quiet {
			fmt.Fprintf(ctx.Stderr, "rebar: running project copy scripts/%s (deprecated: move its settings to .rebarrc and delete it)\n", name)
		}
		return script{path: local}
	}
	if c, ok := checks.Lookup(name); ok {
		return script{native: c}
	}
	if ctx.Install == "" {
		return script{}
	}
	return script{path: filepath.Join(ctx.Install, name)}
}

// isProjectCopy is _rebar_script's test for a project copy: the file
// exists, the project's scripts/ is not the install's, and no line starts
// with the thin-wrapper marker.
func isProjectCopy(ctx *checks.Context, local string) bool {
	if !isRegular(local) {
		return false
	}
	if ctx.Install != "" && samePath(filepath.Join(ctx.Root, "scripts"), ctx.Install) {
		return false
	}
	data, err := os.ReadFile(local)
	if err != nil {
		return true
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, thinWrapperMarker) {
			return false
		}
	}
	return true
}

// found is the suites' [ -x "$script" ] (ci-check, pre-commit): a native
// check always runs; a script must be executable.
func (s script) found() bool {
	return s.native != nil || isExecutable(s.path)
}

// run runs s with args in the project root, its output on ctx's writers,
// and returns its exit code. Scripts are started as "$script" "$@" (their
// own interpreter); the cold-start hook runs them with bash instead.
func (s script) run(ctx *checks.Context, args []string, withBash bool) int {
	if s.native != nil {
		return s.native.Run(&checks.Context{Root: ctx.Root, Dir: ctx.Root, Install: ctx.Install, Stdout: ctx.Stdout, Stderr: ctx.Stderr}, args)
	}
	return runFile(ctx, s.path, args, withBash)
}

// runFile runs the file at path with args in the project root, with
// REBAR_PROJECT_ROOT and the native hand-off settings (checks.ScriptEnv)
// added to the environment, as the suite scripts' children get them from
// the CLI. A file the system cannot execute directly (no #! line) runs
// with bash, as bash does with such a command.
func runFile(ctx *checks.Context, path string, args []string, withBash bool) int {
	return runFileIn(ctx, ctx.Root, path, args, withBash)
}

// runFileIn is runFile started in dir.
func runFileIn(ctx *checks.Context, dir, path string, args []string, withBash bool) int {
	name, argv := path, args
	if withBash {
		name, argv = "bash", append([]string{path}, args...)
	}
	code, err := start(ctx, dir, name, argv)
	if err != nil && !withBash && errors.Is(err, syscall.ENOEXEC) {
		code, err = start(ctx, dir, "bash", append([]string{path}, args...))
	}
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "%s: %v\n", path, err)
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return 127
		}
		return 126
	}
	return code
}

func start(ctx *checks.Context, dir, name string, args []string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "REBAR_PROJECT_ROOT="+ctx.Root), checks.ScriptEnv(ctx.Install)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout, cmd.Stderr = ctx.Stdout, ctx.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// RunQuiet runs one of rebar's check scripts (name relative to scripts/,
// e.g. check-todos.sh) with args as steward.sh's enforcement scan does:
// resolved like _rebar_script without its notice (a project copy, else the
// native check, else the install's script), in ctx's working directory
// (steward does not change directory), output discarded. found is false,
// and nothing runs, when there is no native check and no executable script.
func RunQuiet(ctx *checks.Context, name string, args []string) (code int, found bool) {
	s := resolve(ctx, name, true)
	if !s.found() {
		return 0, false
	}
	quiet := &checks.Context{Root: ctx.Root, Dir: ctx.WorkDir(), Install: ctx.Install, Stdout: io.Discard, Stderr: io.Discard}
	if s.native != nil {
		return s.native.Run(quiet, args), true
	}
	return runFileIn(quiet, quiet.Dir, s.path, args, false), true
}

// output runs s with stdout and stderr captured together and returns them
// as $(...) does (trailing newlines and NUL bytes dropped), with the exit
// code.
func (s script) output(ctx *checks.Context, withBash bool) (string, int) {
	var buf bytes.Buffer
	sub := *ctx
	sub.Stdout, sub.Stderr = &buf, &buf
	code := s.run(&sub, nil, withBash)
	return strings.TrimRight(strings.ReplaceAll(buf.String(), "\x00", ""), "\n"), code
}

// Extra is one of the adopter's own checks, listed in .rebarrc
// checks_extra (the full suite) or checks_extra_pre_commit (the pre-commit
// suite).
type Extra struct {
	// Name is the file's base name without its extension
	// (scripts/check-openapi.sh → check-openapi): `extra: <Name>` in the
	// suite output, and what `rebar run <Name>` finds it by.
	Name string
	// Path is the entry as written, relative to the project root or
	// absolute.
	Path string
}

// Extras keys and their environment overrides.
const (
	ExtraKey          = "checks_extra"
	ExtraPreCommitKey = "checks_extra_pre_commit"
)

var extraEnv = map[string]string{ExtraKey: "REBAR_CHECKS_EXTRA", ExtraPreCommitKey: "REBAR_CHECKS_EXTRA_PRE_COMMIT"}

// Extras returns the extra checks .rebarrc key (ExtraKey or
// ExtraPreCommitKey; env REBAR_CHECKS_EXTRA / REBAR_CHECKS_EXTRA_PRE_COMMIT
// override it) lists for the project at root: whitespace-separated paths,
// in order.
func Extras(root, key string) []Extra {
	var out []Extra
	for _, p := range strings.Fields(config.RCValueOr(root, key, extraEnv[key], "")) {
		base := filepath.Base(p)
		out = append(out, Extra{Name: strings.TrimSuffix(base, filepath.Ext(base)), Path: p})
	}
	return out
}

// FindExtra returns the extra check (from either key) that name refers to:
// its Name, its file name, or its path as written.
func FindExtra(root, name string) (Extra, bool) {
	for _, key := range []string{ExtraKey, ExtraPreCommitKey} {
		for _, e := range Extras(root, key) {
			if name == e.Name || name == filepath.Base(e.Path) || name == e.Path {
				return e, true
			}
		}
	}
	return Extra{}, false
}

// abs is the extra's file.
func (e Extra) abs(root string) string {
	if filepath.IsAbs(e.Path) {
		return e.Path
	}
	return filepath.Join(root, e.Path)
}

// Runnable reports whether the extra's file exists and is executable.
func (e Extra) Runnable(root string) bool { return isExecutable(e.abs(root)) }

// Run runs the extra in the project root, as the suites do, and returns
// its exit code.
func (e Extra) Run(ctx *checks.Context, args []string) int {
	return runFile(ctx, e.abs(ctx.Root), args, false)
}

// skipExtras is SKIP_CHECKS_EXTRA=1, which skips every extra check.
func skipExtras() bool { return os.Getenv("SKIP_CHECKS_EXTRA") == "1" }

// warnOnce reports unknown checks_enable names and an invalid
// contract_namespace once per run, before any check runs, as the suite
// scripts do (_rebar_checks_enable_warn, _rebar_contract_namespace_warn).
func warnOnce(ctx *checks.Context) {
	checks.WarnChecksEnable(ctx)
	checks.ContractNamespace(ctx)
}

func isRegular(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// isExecutable is [ -x p ] for the files a suite runs: an executable file.
func isExecutable(p string) bool {
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// echo writes s and a newline to w.
func echo(w io.Writer, s string) { fmt.Fprintln(w, s) }
